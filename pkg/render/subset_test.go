package render

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/formgen/pkg/model"
)

func TestApplySubset_ByGroup(t *testing.T) {
	form := sampleFormModel()

	ApplySubset(&form, FieldSubset{
		Groups: []string{"advanced"},
	})

	if len(form.Fields) != 1 || form.Fields[0].Name != "settings" {
		t.Fatalf("expected only settings field to remain, got %+v", names(form.Fields))
	}

	sections := parseSectionsMetadata(t, form.Metadata[layoutSectionsKey])
	if len(sections) != 1 || sections[0] != "advanced" {
		t.Fatalf("expected advanced section metadata, got %v", sections)
	}

	if _, ok := form.Metadata["layout.fieldOrder.advanced"]; !ok {
		t.Fatalf("expected layout.fieldOrder.advanced to remain")
	}
	if _, ok := form.Metadata["layout.fieldOrder.content"]; ok {
		t.Fatalf("unexpected layout.fieldOrder.content after filtering")
	}
}

func TestApplySubset_ByTags(t *testing.T) {
	form := sampleFormModel()

	ApplySubset(&form, FieldSubset{
		Tags: []string{"list"},
	})

	if len(form.Fields) != 1 || form.Fields[0].Name != "tags" {
		t.Fatalf("expected only tags field to remain, got %+v", names(form.Fields))
	}

	sections := parseSectionsMetadata(t, form.Metadata[layoutSectionsKey])
	if len(sections) != 1 || sections[0] != "content" {
		t.Fatalf("expected content section metadata, got %v", sections)
	}
	if _, ok := form.Metadata["layout.fieldOrder.content"]; !ok {
		t.Fatalf("expected layout.fieldOrder.content to remain")
	}
	if _, ok := form.Metadata["layout.fieldOrder.overview"]; ok {
		t.Fatalf("unexpected layout.fieldOrder.overview after filtering")
	}
}

func TestApplySubset_BySection(t *testing.T) {
	form := sampleFormModel()

	ApplySubset(&form, FieldSubset{
		Sections: []string{"overview"},
	})

	if len(form.Fields) != 1 || form.Fields[0].Name != "name" {
		t.Fatalf("expected only overview section fields to remain, got %+v", names(form.Fields))
	}

	sections := parseSectionsMetadata(t, form.Metadata[layoutSectionsKey])
	if !reflect.DeepEqual(sections, []string{"overview"}) {
		t.Fatalf("expected overview section metadata, got %v", sections)
	}
	if _, ok := form.Metadata["layout.fieldOrder.overview"]; !ok {
		t.Fatalf("expected layout.fieldOrder.overview to remain")
	}
	if _, ok := form.Metadata["layout.fieldOrder.advanced"]; ok {
		t.Fatalf("unexpected layout.fieldOrder.advanced after filtering")
	}
	if _, ok := form.Metadata["layout.fieldOrder.content"]; ok {
		t.Fatalf("unexpected layout.fieldOrder.content after filtering")
	}
}

func TestApplySubset_EmptyTokensNoop(t *testing.T) {
	form := sampleFormModel()

	ApplySubset(&form, FieldSubset{
		Groups: []string{"   "},
	})

	if len(form.Fields) != len(sampleFormModel().Fields) {
		t.Fatalf("expected no filtering when subset tokens empty, got %d fields", len(form.Fields))
	}
}

func sampleFormModel() model.FormModel {
	metadata := map[string]string{
		layoutSectionsKey:            `[{"id":"overview","title":"Overview","order":0},{"id":"content","title":"Content","order":1},{"id":"advanced","title":"Advanced","order":2}]`,
		"layout.fieldOrder.overview": `["name","summary"]`,
		"layout.fieldOrder.content":  `["tags"]`,
		"layout.fieldOrder.advanced": `["settings"]`,
		"submitLabel":                "Save",
		"success-message":            "Saved",
		"admin.tags":                 `["admin","settings"]`,
		"category":                   "inventory",
		"admin.group":                "details",
		"layout.gutter":              "md",
	}

	fields := []model.Field{
		{
			Name: "name",
			Metadata: map[string]string{
				"group":               "core",
				"tags":                `["display"]`,
				layoutSectionFieldKey: "overview",
			},
		},
		{
			Name: "settings",
			Metadata: map[string]string{
				"group":               "advanced",
				"tags":                `["behavior"]`,
				layoutSectionFieldKey: "advanced",
			},
		},
		{
			Name: "tags",
			Metadata: map[string]string{
				"group":               "taxonomy",
				"tags":                `["list"]`,
				layoutSectionFieldKey: "content",
			},
		},
		{
			Name: "untagged",
		},
	}

	return model.FormModel{
		OperationID: "createWidget",
		Endpoint:    "/widgets",
		Method:      "POST",
		Metadata:    metadata,
		Fields:      fields,
	}
}

func parseSectionsMetadata(t *testing.T, raw string) []string {
	t.Helper()
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var sections []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &sections); err != nil {
		t.Fatalf("unmarshal sections: %v", err)
	}
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		out = append(out, section.ID)
	}
	return out
}

func names(fields []model.Field) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, field.Name)
	}
	return out
}
