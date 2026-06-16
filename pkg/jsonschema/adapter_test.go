package jsonschema

import (
	"context"
	"errors"
	"testing"

	pkgmodel "github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/schema"
)

type failingLoader struct{}

func (f failingLoader) Load(ctx context.Context, src Source) (schema.Document, error) {
	return schema.Document{}, errors.New("unexpected loader call")
}

func TestAdapterNormalize_DialectRequired(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{"type":"object"}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err == nil {
		t.Fatalf("expected error for missing $schema")
	}
}

func TestAdapterNormalize_DialectUnsupported(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object"}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err == nil {
		t.Fatalf("expected error for unsupported $schema")
	}
}

func TestAdapterNormalize_Success(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.post",
  "type":"object",
  "properties":{
    "title":{"type":"string"}
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	form, ok := ir.Form("com.example.post.edit")
	if !ok {
		t.Fatalf("expected form com.example.post.edit")
	}
	if form.Schema.Properties["title"].Type != "string" {
		t.Fatalf("expected title type string, got %q", form.Schema.Properties["title"].Type)
	}
}

func TestAdapterNormalize_FormIDFilter(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.post"
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{FormID: "missing"})
	if err == nil {
		t.Fatalf("expected error for missing form id")
	}
}

func TestAdapterNormalize_UnsupportedKeyword(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "anyOf": []
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err == nil {
		t.Fatalf("expected error for unsupported keyword")
	}
}

func TestAdapterNormalize_ArrayCardinality(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.grid",
  "type":"object",
  "properties":{
    "columns":{
      "type":"array",
      "minItems":1,
      "maxItems":3,
      "items":{"type":"string"}
    },
    "matrix":{
      "type":"array",
      "minItems":2,
      "items":{
        "type":"array",
        "maxItems":4,
        "items":{"type":"number"}
      }
    },
    "minItems":{"type":"string"},
    "maxItems":{"type":"integer"}
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("com.example.grid.edit")
	if !ok {
		t.Fatalf("expected form com.example.grid.edit")
	}

	columns := form.Schema.Properties["columns"]
	if columns.MinItems == nil || *columns.MinItems != 1 {
		t.Fatalf("columns minItems = %v, want 1", columns.MinItems)
	}
	if columns.MaxItems == nil || *columns.MaxItems != 3 {
		t.Fatalf("columns maxItems = %v, want 3", columns.MaxItems)
	}
	if form.Schema.Properties["minItems"].Type != "string" {
		t.Fatalf("literal minItems property should remain a field")
	}
	if form.Schema.Properties["maxItems"].Type != "integer" {
		t.Fatalf("literal maxItems property should remain a field")
	}
	matrix := form.Schema.Properties["matrix"]
	if matrix.MinItems == nil || *matrix.MinItems != 2 {
		t.Fatalf("matrix minItems = %v, want 2", matrix.MinItems)
	}
	if matrix.Items == nil || matrix.Items.MaxItems == nil || *matrix.Items.MaxItems != 4 {
		t.Fatalf("nested array maxItems = %+v, want 4", matrix.Items)
	}

	model, err := pkgmodel.NewBuilder().Build(form)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	fields := fieldsByName(model.Fields)
	assertRules(t, fields["columns"].Validations, []pkgmodel.ValidationRule{
		{Kind: pkgmodel.ValidationRuleMinItems, Params: map[string]string{"value": "1"}},
		{Kind: pkgmodel.ValidationRuleMaxItems, Params: map[string]string{"value": "3"}},
	})
	if fields["matrix"].Items == nil {
		t.Fatalf("matrix item field missing")
	}
	assertRules(t, fields["matrix"].Items.Validations, []pkgmodel.ValidationRule{
		{Kind: pkgmodel.ValidationRuleMaxItems, Params: map[string]string{"value": "4"}},
	})
}

func TestAdapterNormalize_ArrayCardinalityRejectsInvalidBounds(t *testing.T) {
	tests := map[string]string{
		"fractional": `{"type":"array","minItems":1.5,"items":{"type":"string"}}`,
		"negative":   `{"type":"array","minItems":-1,"items":{"type":"string"}}`,
		"non-array":  `{"type":"string","minItems":1}`,
		"inverted":   `{"type":"array","minItems":3,"maxItems":2,"items":{"type":"string"}}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			adapter := NewAdapter(failingLoader{})
			raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.invalid",
  "type":"object",
  "properties":{"value":` + body + `}
}`)
			doc := MustNewDocument(SourceFromFS("root.json"), raw)
			if _, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{}); err == nil {
				t.Fatalf("expected invalid cardinality error")
			}
		})
	}
}

func TestAdapterNormalize_NullableTypeOptional(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.post",
  "type":"object",
  "required":["title"],
  "properties":{
    "title":{"type":["string","null"]}
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	form, ok := ir.Form("com.example.post.edit")
	if !ok {
		t.Fatalf("expected form com.example.post.edit")
	}
	if form.Schema.Properties["title"].Type != "string" {
		t.Fatalf("expected title type string, got %q", form.Schema.Properties["title"].Type)
	}
	for _, entry := range form.Schema.Required {
		if entry == "title" {
			t.Fatalf("expected nullable field to be optional, got required list: %#v", form.Schema.Required)
		}
	}
}

func TestAdapterNormalize_TypeUnionUnsupported(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":["string","number"]
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err == nil {
		t.Fatalf("expected error for unsupported type union")
	}
}

func fieldsByName(fields []pkgmodel.Field) map[string]pkgmodel.Field {
	out := make(map[string]pkgmodel.Field, len(fields))
	for _, field := range fields {
		out[field.Name] = field
	}
	return out
}

func assertRules(t *testing.T, got, want []pkgmodel.ValidationRule) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("validation rules length = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind {
			t.Fatalf("rule %d kind = %q, want %q; got %+v", i, got[i].Kind, want[i].Kind, got)
		}
		for key, wantValue := range want[i].Params {
			if got[i].Params[key] != wantValue {
				t.Fatalf("rule %d param %q = %q, want %q; got %+v", i, key, got[i].Params[key], wantValue, got)
			}
		}
	}
}
