package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestDefinitionMetadataDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		extensions map[string]any
		code, path string
	}{
		{"flat component", map[string]any{"x-formgen": map[string]any{"component.name": "cards", "component.config": map[string]any{"wrap": true}, "label": "Pick"}}, "", ""},
		{"host", map[string]any{"x-host": map[string]any{"component": "custom"}, "x-formgen": map[string]any{"host.custom": map[string]any{"answer": "private"}}}, "", ""},
		{"ignored component", map[string]any{"x-formgen": map[string]any{"component": "private"}}, "metadataAlias", "#/properties/name/x-formgen/component"},
		{"bad shape", map[string]any{"x-formgen": map[string]any{"component.name": 42}}, "metadataShape", "#/properties/name/x-formgen/component.name"},
		{"bad config", map[string]any{"x-formgen": map[string]any{"component.config": []any{"private"}}}, "metadataShape", "#/properties/name/x-formgen/component.config"},
		{"conflict", map[string]any{"x-formgen": map[string]any{"label": "nested"}, "x-formgen-label": "flat"}, "metadataConflict", "#/properties/name/x-formgen/label"},
		{"namespace", map[string]any{"x-formgen": "private"}, "metadataShape", "#/properties/name/x-formgen"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := model.DiagnoseExtensions(test.extensions, "#/properties/name")
			if test.code == "" {
				if len(got) != 0 {
					t.Fatal(got)
				}
				return
			}
			if len(got) != 1 || got[0].Code != test.code || got[0].Path != test.path {
				t.Fatal(got)
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "private") {
				t.Fatal("definition diagnostic contains payload")
			}
		})
	}
}
func TestDiagnoseSchemaTitleAndLegacyPermissiveness(t *testing.T) {
	definition := schema.Schema{Type: "object", Properties: map[string]schema.Schema{"answer": {Type: "string", Title: "unused"}}}
	if got := model.DiagnoseSchema(definition); len(got) != 1 || got[0].Path != "#/properties/answer/title" {
		t.Fatal(got)
	}
	child := definition.Properties["answer"]
	child.Extensions = map[string]any{"x-formgen": map[string]any{"label": "Inline", "component": "ignored"}}
	definition.Properties["answer"] = child
	got := model.DiagnoseSchema(definition)
	if len(got) != 1 || got[0].Code != "metadataAlias" {
		t.Fatal(got)
	}
	form, err := model.NewBuilder().Build(schema.Form{ID: "test", Method: "POST", Endpoint: "/", Schema: definition})
	if err != nil || form.Fields[0].Label != "Inline" || form.Fields[0].Metadata["component"] != "ignored" {
		t.Fatalf("legacy extraction changed: %+v %v", form, err)
	}
	_, hints := model.ParseUIExtensions(child.Extensions)
	if _, ok := hints["component"]; ok {
		t.Fatal("new alias silently enabled")
	}
}

func TestNullConditionsDiagnostic(t *testing.T) {
	got := model.DiagnoseExtensions(map[string]any{"x-formgen": map[string]any{"conditions": nil}}, "#")
	if len(got) != 1 || got[0].Code != "conditionShape" {
		t.Fatal(got)
	}
}
