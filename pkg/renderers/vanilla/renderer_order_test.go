package vanilla

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render/template"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
)

func TestBuildLayoutContext_AppliesSectionFieldOrder(t *testing.T) {
	t.Helper()

	sections := []sectionMeta{
		{ID: "primary", Order: 0, Fieldset: true},
		{ID: "secondary", Order: 1},
	}
	sectionsPayload, err := json.Marshal(sections)
	if err != nil {
		t.Fatalf("marshal sections: %v", err)
	}

	form := model.FormModel{
		OperationID: "orderedForm",
		Metadata: map[string]string{
			layoutSectionsMetadataKey:            string(sectionsPayload),
			layoutFieldOrderPrefix + "primary":   `["title","description","created_at"]`,
			layoutFieldOrderPrefix + "secondary": `["notes"]`,
		},
		Fields: []model.Field{
			{
				Name: "description",
				Metadata: map[string]string{
					layoutSectionFieldKey:      "primary",
					componentChromeMetadataKey: componentChromeSkipKeyword,
				},
			},
			{
				Name: "title",
				Metadata: map[string]string{
					layoutSectionFieldKey:      "primary",
					componentChromeMetadataKey: componentChromeSkipKeyword,
				},
			},
			{
				Name: "created_at",
				Metadata: map[string]string{
					layoutSectionFieldKey:      "primary",
					componentChromeMetadataKey: componentChromeSkipKeyword,
				},
			},
			{
				Name: "notes",
				Metadata: map[string]string{
					layoutSectionFieldKey:      "secondary",
					componentChromeMetadataKey: componentChromeSkipKeyword,
				},
			},
		},
	}

	renderer := newComponentRenderer(&noopTemplateRenderer{}, simpleComponentRegistry(), nil, rendererTheme{}, nil)

	layout, err := buildLayoutContext(form, renderer)
	if err != nil {
		t.Fatalf("build layout: %v", err)
	}

	primary := findSectionByID(t, layout, "primary")
	if got := namesFromRendered(primary.Fields); !equalSlice(got, []string{"title", "description", "created_at"}) {
		t.Fatalf("primary fields order mismatch: %v", got)
	}

	secondary := findSectionByID(t, layout, "secondary")
	if got := namesFromRendered(secondary.Fields); !equalSlice(got, []string{"notes"}) {
		t.Fatalf("secondary fields order mismatch: %v", got)
	}
}

func TestBuildLayoutContext_SkipsNestedFieldsFromSections(t *testing.T) {
	t.Helper()

	sections := []sectionMeta{
		{ID: "collaboration", Order: 0},
	}
	payload, err := json.Marshal(sections)
	if err != nil {
		t.Fatalf("marshal sections: %v", err)
	}

	form := model.FormModel{
		OperationID: "nestedForm",
		Metadata: map[string]string{
			layoutSectionsMetadataKey: string(payload),
		},
		Fields: []model.Field{
			{
				Name: "contributors",
				Type: model.FieldTypeArray,
				Metadata: map[string]string{
					layoutSectionFieldKey:      "collaboration",
					componentChromeMetadataKey: componentChromeSkipKeyword,
				},
				Items: &model.Field{
					Name: "contributorsItem",
					Type: model.FieldTypeObject,
					Nested: []model.Field{
						{
							Name: "person_id",
							Metadata: map[string]string{
								layoutSectionFieldKey:      "collaboration",
								componentChromeMetadataKey: componentChromeSkipKeyword,
							},
						},
						{
							Name: "role",
							Metadata: map[string]string{
								layoutSectionFieldKey:      "collaboration",
								componentChromeMetadataKey: componentChromeSkipKeyword,
							},
						},
					},
				},
			},
		},
	}

	renderer := newComponentRenderer(&noopTemplateRenderer{}, simpleComponentRegistry(), nil, rendererTheme{}, nil)

	layout, err := buildLayoutContext(form, renderer)
	if err != nil {
		t.Fatalf("build layout: %v", err)
	}

	section := findSectionByID(t, layout, "collaboration")
	if got := namesFromRendered(section.Fields); !equalSlice(got, []string{"contributors"}) {
		t.Fatalf("collaboration section should only contain array field, got %v", got)
	}
}

func namesFromRendered(fields []renderedField) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, field.HTML)
	}
	return out
}

func findSectionByID(t *testing.T, layout layoutContext, id string) sectionGroup {
	t.Helper()
	for _, section := range layout.Sections {
		if section.ID == id {
			return section
		}
	}
	t.Fatalf("section %s not found in layout", id)
	return sectionGroup{}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type noopTemplateRenderer struct{}

func (n *noopTemplateRenderer) Render(string, any, ...io.Writer) (string, error) {
	return "", nil
}

func (n *noopTemplateRenderer) RenderTemplate(string, any, ...io.Writer) (string, error) {
	return "", nil
}

func (n *noopTemplateRenderer) RenderString(string, any, ...io.Writer) (string, error) {
	return "", nil
}

func (n *noopTemplateRenderer) RegisterFilter(string, func(any, any) (any, error)) error {
	return nil
}

func (n *noopTemplateRenderer) GlobalContext(any) error {
	return nil
}

func simpleComponentRegistry() *components.Registry {
	registry := components.New()
	registry.MustRegister("input", components.Descriptor{
		Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
			buf.WriteString(field.Name)
			return nil
		},
	})
	registry.MustRegister("object", components.Descriptor{
		Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
			buf.WriteString(field.Name)
			return nil
		},
	})
	registry.MustRegister("array", components.Descriptor{
		Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
			buf.WriteString(field.Name)
			return nil
		},
	})
	registry.MustRegister("select", components.Descriptor{
		Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
			buf.WriteString(field.Name)
			return nil
		},
	})
	registry.MustRegister("boolean", components.Descriptor{
		Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
			buf.WriteString(field.Name)
			return nil
		},
	})
	return registry
}

var _ template.TemplateRenderer = (*noopTemplateRenderer)(nil)
