package orchestrator_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/jsonschema"
	"github.com/goliatone/go-formgen/pkg/model"
	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/orchestrator"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/testsupport"
	"github.com/goliatone/go-formgen/pkg/visibility"
	visibilityexpr "github.com/goliatone/go-formgen/pkg/visibility/expr"
)

func TestBuildFormModel_OpenAPIGolden(t *testing.T) {
	t.Parallel()

	orch := orchestrator.New(orchestrator.WithUISchemaFS(nil))
	form, err := orch.BuildFormModel(testsupport.Context(), orchestrator.BuildRequest{
		Source:      pkgopenapi.SourceFromFile(filepath.Join("testdata", "petstore.yaml")),
		OperationID: "createPet",
	})
	if err != nil {
		t.Fatalf("BuildFormModel: %v", err)
	}

	want := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "create_pet_formmodel.golden.json"))
	if diff := testsupport.CompareGolden(want, form); diff != "" {
		t.Fatalf("form model mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildFormModel_JSONSchemaRawBytes(t *testing.T) {
	t.Parallel()

	orch := orchestrator.New(orchestrator.WithUISchemaFS(nil))
	form, err := orch.BuildFormModelFromJSONSchemaBytes(testsupport.Context(), rawJSONSchema(), "widget.edit")
	if err != nil {
		t.Fatalf("BuildFormModelFromJSONSchemaBytes: %v", err)
	}

	if form.OperationID != "widget.edit" {
		t.Fatalf("operation id mismatch: %s", form.OperationID)
	}
	if len(form.Fields) != 1 || form.Fields[0].Name != "name" {
		t.Fatalf("unexpected fields: %+v", fieldNames(form.Fields))
	}
}

func TestBuildFormModel_InMemorySchemaDocument(t *testing.T) {
	t.Parallel()

	doc := schema.MustNewDocument(jsonschema.SourceFromBytes("test-schema"), rawJSONSchema())
	orch := orchestrator.New(orchestrator.WithUISchemaFS(nil))
	form, err := orch.BuildFormModelFromSchemaDocument(
		testsupport.Context(),
		doc,
		"widget.edit",
		orchestrator.WithBuildFormat(jsonschema.DefaultAdapterName),
	)
	if err != nil {
		t.Fatalf("BuildFormModelFromSchemaDocument: %v", err)
	}

	if form.OperationID != "widget.edit" {
		t.Fatalf("operation id mismatch: %s", form.OperationID)
	}
}

func TestBuildFormModel_SubsetAndVisibility(t *testing.T) {
	t.Parallel()

	orch := orchestrator.New(
		orchestrator.WithUISchemaFS(nil),
		orchestrator.WithVisibilityEvaluator(visibilityexpr.New()),
	)
	form, err := orch.BuildFormModel(testsupport.Context(), orchestrator.BuildRequest{
		Source:      pkgopenapi.SourceFromFile(filepath.Join("testdata", "extensions.yaml")),
		OperationID: "createWidget",
		Subset: model.FieldSubset{
			Tags: []string{"display", "behavior"},
		},
		VisibilityContext: visibility.Context{
			Values: map[string]any{
				"settings.enabled": false,
				"enabled":          false,
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildFormModel: %v", err)
	}

	if got := fieldNames(form.Fields); strings.Join(got, ",") != "name,settings" {
		t.Fatalf("subset mismatch: %v", got)
	}
	settings := findBuildField(form.Fields, "settings")
	if settings == nil {
		t.Fatalf("settings field missing")
	}
	if findBuildField(settings.Nested, "threshold") != nil {
		t.Fatalf("visibility did not remove settings.threshold")
	}
}

func TestBuildFormModel_FormNotFoundListsAvailableForms(t *testing.T) {
	t.Parallel()

	orch := orchestrator.New(orchestrator.WithUISchemaFS(nil))
	_, err := orch.BuildFormModel(testsupport.Context(), orchestrator.BuildRequest{
		Source:      pkgopenapi.SourceFromFile(filepath.Join("testdata", "petstore.yaml")),
		OperationID: "missingForm",
	})
	if err == nil {
		t.Fatalf("expected missing form error")
	}
	if !strings.Contains(err.Error(), "available: createPet") {
		t.Fatalf("missing available forms in error: %v", err)
	}
}

func TestOrchestratorImportGraphIsHeadless(t *testing.T) {
	t.Parallel()

	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list deps: %v\n%s", err, out)
	}
	forbidden := []string{
		"survey",
		"pongo2",
		"go-theme",
		"bluemonday",
		"renderers/vanilla",
		"renderers/tui",
		"renderers/preact",
		"go-template",
	}
	for line := range strings.FieldsSeq(string(out)) {
		for _, pattern := range forbidden {
			if strings.Contains(line, pattern) {
				t.Fatalf("pkg/orchestrator imports forbidden dependency %q via %q", pattern, line)
			}
		}
	}
}

func rawJSONSchema() []byte {
	return []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "widget",
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {
      "type": "string",
      "title": "Name"
    }
  }
}`)
}

func fieldNames(fields []model.Field) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func findBuildField(fields []model.Field, name string) *model.Field {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}
