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
  "dependentRequired": {}
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	_, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err == nil {
		t.Fatalf("expected error for unsupported keyword")
	}
}

func TestAdapterNormalize_ReadOnlyAnnotation(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.readonly",
  "type":"object",
  "properties":{
    "title":{"type":"string","readOnly":true,"x-formgen":{"placeholder":"Title"}},
    "slug":{"type":"string","read_only":true},
    "public":{"type":"boolean","readOnly":false},
    "audit":{
      "type":"object",
      "properties":{
        "created_at":{"type":"string","readOnly":true}
      }
    },
    "tags":{
      "type":"array",
      "items":{"type":"string","readOnly":true}
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("com.example.readonly.edit")
	if !ok {
		t.Fatalf("expected form com.example.readonly.edit")
	}
	if !form.Schema.Properties["title"].ReadOnly {
		t.Fatalf("expected title schema to be readonly")
	}
	if !form.Schema.Properties["slug"].ReadOnly {
		t.Fatalf("expected slug schema to be readonly")
	}
	if form.Schema.Properties["public"].ReadOnly {
		t.Fatalf("public schema should not be readonly")
	}
	audit := form.Schema.Properties["audit"]
	if !audit.Properties["created_at"].ReadOnly {
		t.Fatalf("expected nested created_at schema to be readonly")
	}
	tags := form.Schema.Properties["tags"]
	if tags.Items == nil || !tags.Items.ReadOnly {
		t.Fatalf("expected array item schema to be readonly")
	}
	if got := form.Schema.Properties["title"].Extensions["x-formgen"].(map[string]any)["placeholder"]; got != "Title" {
		t.Fatalf("vendor extension changed: %v", got)
	}

	model, err := pkgmodel.NewBuilder().Build(form)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	fields := fieldsByPath(model.Fields)
	title := fields["title"]
	if !title.Readonly {
		t.Fatalf("expected title field readonly")
	}
	if title.Metadata["readonly"] != "true" {
		t.Fatalf("expected title readonly metadata, got %#v", title.Metadata)
	}
	if title.UIHints["readonly"] != "true" {
		t.Fatalf("expected title readonly UI hint, got %#v", title.UIHints)
	}
	if len(title.Validations) != 0 {
		t.Fatalf("readOnly should not add validations: %+v", title.Validations)
	}
	if !fields["slug"].Readonly {
		t.Fatalf("expected slug field readonly")
	}
	if fields["public"].Readonly {
		t.Fatalf("public field should not be readonly")
	}
	if !fields["audit.created_at"].Readonly {
		t.Fatalf("expected nested audit.created_at field readonly")
	}
	if !fields["tags.tagsItem"].Readonly {
		t.Fatalf("expected array item field readonly")
	}
}

func TestAdapterNormalize_ReadOnlyRejectsInvalidValues(t *testing.T) {
	tests := map[string]string{
		"non-boolean": `{"type":"string","readOnly":"true"}`,
		"conflict":    `{"type":"string","readOnly":true,"read_only":false}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			adapter := NewAdapter(failingLoader{})
			raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.invalid_readonly",
  "type":"object",
  "properties":{"value":` + body + `}
}`)
			doc := MustNewDocument(SourceFromFS("root.json"), raw)
			if _, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{}); err == nil {
				t.Fatalf("expected invalid readOnly error")
			}
		})
	}
}

func TestAdapterNormalize_OverlayFieldOrderRecursesIntoArrayItems(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"site_teaching_topics_menu",
  "type":"object",
  "properties":{
    "columns":{
      "type":"array",
      "items":{
        "type":"object",
        "properties":{
          "entries":{
            "type":"array",
            "items":{
              "type":"object",
              "properties":{
                "topic_slug":{"type":"string"},
                "topic_id":{"type":"string"}
              }
            }
          },
          "title":{"type":"string"}
        }
      }
    }
  }
}`)
	overlayRaw := []byte(`{
  "$schema":"x-ui-overlay/v1",
  "overrides":[
    {"path":"/properties/columns/items/properties/title","x-formgen":{"order":1}},
    {"path":"/properties/columns/items/properties/entries","x-formgen":{"order":2}},
    {"path":"/properties/columns/items/properties/entries/items/properties/topic_id","x-formgen":{"order":1}},
    {"path":"/properties/columns/items/properties/entries/items/properties/topic_slug","x-formgen":{"order":2}}
  ]
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{Overlay: overlayRaw})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("site_teaching_topics_menu.edit")
	if !ok {
		t.Fatalf("expected form site_teaching_topics_menu.edit")
	}
	model, err := pkgmodel.NewBuilder().Build(form)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	columns := fieldsByName(model.Fields)["columns"]
	if columns.Items == nil {
		t.Fatalf("columns item field missing")
	}
	if got := fieldNames(columns.Items.Nested); len(got) < 2 || got[0] != "title" || got[1] != "entries" {
		t.Fatalf("column item order = %v, want title then entries", got)
	}
	entries := columns.Items.Nested[1]
	if entries.Items == nil {
		t.Fatalf("entries item field missing")
	}
	if got := fieldNames(entries.Items.Nested); len(got) < 2 || got[0] != "topic_id" || got[1] != "topic_slug" {
		t.Fatalf("entry item order = %v, want topic_id then topic_slug", got)
	}
}

func TestAdapterNormalize_DottedOverlayFieldOrder(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"dotted_overlay_order",
  "type":"object",
  "properties":{
    "gamma":{"type":"string"},
    "alpha":{"type":"string"},
    "beta":{"type":"string"}
  }
}`)
	overlayRaw := []byte(`{
  "$schema":"x-ui-overlay/v1",
  "overrides":[
    {"path":"/properties/alpha","x-formgen.order":2},
    {"path":"/properties/beta","x-admin.order":1}
  ]
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{Overlay: overlayRaw})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("dotted_overlay_order.edit")
	if !ok {
		t.Fatalf("expected form dotted_overlay_order.edit")
	}
	model, err := pkgmodel.NewBuilder().Build(form)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	if got := fieldNames(model.Fields); len(got) != 3 || got[0] != "beta" || got[1] != "alpha" || got[2] != "gamma" {
		t.Fatalf("field order = %v, want dotted overlay orders before unordered fallback", got)
	}
}

func TestAdapterNormalize_ReadOnlyRefSiblings(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.ref_readonly",
  "type":"object",
  "properties":{
    "title":{"$ref":"#/$defs/text","readOnly":true},
    "slug":{"$ref":"#/$defs/text","read_only":true}
  },
  "$defs":{
    "text":{"type":"string"}
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("com.example.ref_readonly.edit")
	if !ok {
		t.Fatalf("expected form com.example.ref_readonly.edit")
	}
	if !form.Schema.Properties["title"].ReadOnly {
		t.Fatalf("expected readOnly $ref sibling to mark title readonly")
	}
	if !form.Schema.Properties["slug"].ReadOnly {
		t.Fatalf("expected read_only $ref sibling to mark slug readonly")
	}

	model, err := pkgmodel.NewBuilder().Build(form)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	fields := fieldsByPath(model.Fields)
	if !fields["title"].Readonly {
		t.Fatalf("expected title field readonly")
	}
	if !fields["slug"].Readonly {
		t.Fatalf("expected slug field readonly")
	}
}

func TestAdapterNormalize_NestedAnyOfArrayItems(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.columns",
  "type":"object",
  "properties":{
    "columns":{
      "type":"array",
      "items":{
        "type":"object",
        "properties":{
          "entries":{
            "type":"array",
            "items":{
              "anyOf":[
                {"type":"string","readOnly":true},
                {"type":"null"}
              ]
            }
          }
        }
      }
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("com.example.columns.edit")
	if !ok {
		t.Fatalf("expected form com.example.columns.edit")
	}
	entries := form.Schema.Properties["columns"].Items.Properties["entries"]
	if entries.Items == nil {
		t.Fatalf("expected entries item schema")
	}
	if entries.Items.Type != "string" {
		t.Fatalf("entries item type = %q, want string", entries.Items.Type)
	}
	if !entries.Items.ReadOnly {
		t.Fatalf("entries item should preserve readOnly from anyOf branch")
	}
}

func TestAdapterNormalize_AnyOfDiscriminatorUnionOutsideArrayRejected(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.standalone_anyof",
  "type":"object",
  "properties":{
    "content":{
      "anyOf":[
        {
          "type":"object",
          "properties":{
            "_type":{"const":"hero"},
            "headline":{"type":"string"}
          }
        },
        {
          "type":"object",
          "properties":{
            "_type":{"const":"text"},
            "body":{"type":"string"}
          }
        }
      ]
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	if _, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{}); err == nil {
		t.Fatalf("expected standalone discriminator anyOf to be rejected")
	}
}

func TestAdapterNormalize_AnyOfDiscriminatorUnion(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.any_blocks",
  "type":"object",
  "properties":{
    "blocks":{
      "type":"array",
      "x-formgen":{"widget":"block"},
      "items":{
        "anyOf":[
          {
            "type":"object",
            "properties":{
              "_type":{"const":"hero"},
              "headline":{"type":"string"}
            }
          },
          {
            "type":"object",
            "properties":{
              "_type":{"const":"text"},
              "body":{"type":"string"}
            }
          }
        ]
      }
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	form, ok := ir.Form("com.example.any_blocks.edit")
	if !ok {
		t.Fatalf("expected form com.example.any_blocks.edit")
	}
	items := form.Schema.Properties["blocks"].Items
	if items == nil || len(items.OneOf) != 2 {
		t.Fatalf("expected anyOf to normalize to discriminator union, got %+v", items)
	}
	for idx, option := range items.OneOf {
		typ := option.Properties["_type"]
		if !typ.ReadOnly {
			t.Fatalf("option %d _type should be readonly", idx)
		}
	}
}

func TestAdapterNormalize_AnyOfRejectsAmbiguousShape(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.ambiguous",
  "type":"object",
  "properties":{
    "value":{
      "anyOf":[
        {"type":"string"},
        {"type":"object","properties":{"label":{"type":"string"}}}
      ]
    }
  }
}`)
	doc := MustNewDocument(SourceFromFS("root.json"), raw)

	if _, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{}); err == nil {
		t.Fatalf("expected unsupported anyOf shape error")
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

func TestAdapterNormalize_NullableAnyOfOptional(t *testing.T) {
	adapter := NewAdapter(failingLoader{})
	raw := []byte(`{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "$id":"com.example.post",
  "type":"object",
  "required":["title"],
  "properties":{
    "title":{"anyOf":[{"type":"string"},{"type":"null"}]}
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
			t.Fatalf("expected nullable anyOf field to be optional, got required list: %#v", form.Schema.Required)
		}
	}

	model, err := pkgmodel.NewBuilder().Build(form)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	fields := fieldsByName(model.Fields)
	if fields["title"].Required {
		t.Fatalf("expected nullable anyOf field model to be optional")
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

func fieldNames(fields []pkgmodel.Field) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, field.Name)
	}
	return out
}

func fieldsByPath(fields []pkgmodel.Field) map[string]pkgmodel.Field {
	out := make(map[string]pkgmodel.Field)
	var visit func(prefix string, field pkgmodel.Field)
	visit = func(prefix string, field pkgmodel.Field) {
		key := field.Name
		if prefix != "" {
			key = prefix + "." + key
		}
		out[key] = field
		if field.Items != nil {
			visit(key, *field.Items)
		}
		for _, nested := range field.Nested {
			visit(key, nested)
		}
	}
	for _, field := range fields {
		visit("", field)
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
