package validation

import (
	"context"
	"testing"

	pkgjsonschema "github.com/goliatone/go-formgen/pkg/jsonschema"
)

func TestValidateJSONSchema_Valid(t *testing.T) {
	raw := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "com.example.sample",
  "type": "object",
  "properties": {
    "title": { "type": "string" }
  }
}`)
	result := ValidateJSONSchema(context.Background(), pkgjsonschema.SourceFromFS("schema.json"), raw, JSONSchemaValidationOptions{})
	if !result.Valid {
		t.Fatalf("expected schema to be valid: %#v", result.Issues)
	}
}

func TestValidateJSONSchema_FieldPath(t *testing.T) {
	raw := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "com.example.sample",
  "type": "object",
  "properties": {
    "title": { "type": "string", "minLength": "oops" }
  }
}`)
	result := ValidateJSONSchema(context.Background(), pkgjsonschema.SourceFromFS("schema.json"), raw, JSONSchemaValidationOptions{})
	if result.Valid {
		t.Fatalf("expected schema to be invalid")
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected validation issues")
	}
	if got := result.Issues[0].Field; got != "title" {
		t.Fatalf("expected field path title, got %q", got)
	}
}

func TestValidateJSONSchema_OverlayError(t *testing.T) {
	raw := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "com.example.sample",
  "type": "object",
  "properties": {
    "title": { "type": "string" }
  }
}`)
	overlay := []byte(`{
  "$schema": "x-ui-overlay/v1",
  "overrides": [
    { "path": "/properties/missing", "x-formgen": { "label": "Missing" } }
  ]
}`)
	opts := JSONSchemaValidationOptions{}
	opts.Normalize.Overlay = overlay
	result := ValidateJSONSchema(context.Background(), pkgjsonschema.SourceFromFS("schema.json"), raw, opts)
	if result.Valid {
		t.Fatalf("expected overlay to be invalid")
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected overlay issue")
	}
	if result.Issues[0].Path == "" {
		t.Fatalf("expected overlay path in issue")
	}
}
