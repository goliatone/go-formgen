package jsonschema

import (
	"context"
	"errors"
	"testing"

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
