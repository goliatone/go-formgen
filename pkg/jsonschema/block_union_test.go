package jsonschema

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

func TestAdapterNormalize_BlockUnionFormModel(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "com.example.page",
  "type": "object",
  "properties": {
    "title": { "type": "string" },
    "blocks": {
      "type": "array",
      "x-formgen": { "widget": "block", "label": "Page Sections" },
      "items": {
        "oneOf": [
          { "$ref": "#/$defs/hero" },
          { "$ref": "#/$defs/rich_text" }
        ]
      }
    }
  },
  "$defs": {
    "hero": {
      "type": "object",
      "x-formgen": { "label": "Hero", "icon": "hero", "collapsed": true },
      "required": ["headline"],
      "properties": {
        "_type": { "const": "hero" },
        "headline": { "type": "string" },
        "cta": { "type": "string" }
      }
    },
    "rich_text": {
      "type": "object",
      "x-formgen": { "label": "Rich Text", "icon": "text" },
      "properties": {
        "_type": { "const": "rich_text" },
        "body": { "type": "string", "x-formgen": { "widget": "wysiwyg" } }
      }
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	form, ok := ir.Form("com.example.page.edit")
	if !ok {
		t.Fatalf("expected form com.example.page.edit")
	}

	builder := model.NewBuilder()
	formModel, err := builder.Build(form)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	goldenPath := filepath.Join("testdata", "block_union_form_model.golden.json")
	testsupport.WriteFormModel(t, goldenPath, formModel)
	want := testsupport.MustLoadFormModel(t, goldenPath)
	if diff := testsupport.CompareGolden(want, formModel); diff != "" {
		t.Fatalf("form model mismatch (-want +got):\n%s", diff)
	}
}

func TestAdapterNormalize_BlockUnionRequiresType(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "com.example.page",
  "type": "object",
  "properties": {
    "blocks": {
      "type": "array",
      "items": {
        "oneOf": [
          { "type": "object", "properties": { "headline": { "type": "string" } } }
        ]
      }
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err == nil {
		t.Fatalf("expected error for missing _type discriminator")
	}
}
