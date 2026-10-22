package jsonschema

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestMetadataDiagnosticsRespectOverlayAndDefinitionLocations(t *testing.T) {
	payload := map[string]any{"type": "object", "properties": map[string]any{"a/b": map[string]any{"type": "string", "title": "Ignored", "x-formgen": map[string]any{"component": "ignored"}, "default": map[string]any{"x-formgen": "answer-must-not-be-scanned"}}}}
	got := DiagnoseDefinition(payload)
	if len(got) != 2 || got[0].Path != "#/properties/a~1b/x-formgen/component" || got[1].Path != "#/properties/a~1b/title" {
		t.Fatal(got)
	}
	raw := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"overlay","type":"object","properties":{"answer":{"type":"string","x-formgen":{"component.name":"old"}}}}`)
	overlay := []byte(`{"$schema":"x-ui-overlay/v1","overrides":[{"path":"/properties/answer","x-formgen":{"component.name":"cards","label":"Overlay"}}]}`)
	adapter := NewAdapter(failingLoader{})
	doc := MustNewDocument(SourceFromFS("root.json"), raw)
	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	form, _ := ir.Form("overlay.edit")
	if form.Schema.Properties["answer"].Extensions["x-formgen"].(map[string]any)["component.name"] != "cards" {
		t.Fatal("overlay precedence changed")
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := DiagnoseDefinition(decoded); len(got) != 0 {
		t.Fatal(got)
	}
}
