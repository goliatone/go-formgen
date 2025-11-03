package parser

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestConvertSchemaHandlesRecursiveReferences(t *testing.T) {
	const document = `{
  "openapi": "3.0.0",
  "info": { "title": "Cycle", "version": "1.0.0" },
  "paths": {},
  "components": {
    "schemas": {
      "PublishingHouse": {
        "type": "object",
        "properties": {
          "headquarters": { "$ref": "#/components/schemas/Headquarters" }
        }
      },
      "Headquarters": {
        "type": "object",
        "properties": {
          "publisher": { "$ref": "#/components/schemas/PublishingHouse" }
        }
      }
    }
  }
}`

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(document))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}

	publishing := doc.Components.Schemas["PublishingHouse"]
	if publishing == nil {
		t.Fatalf("schema PublishingHouse not found")
	}
	convertedPublishing := convertSchema(publishing)
	if convertedPublishing.Properties == nil {
		t.Fatalf("expected properties for PublishingHouse schema")
	}
	headquarters, ok := convertedPublishing.Properties["headquarters"]
	if !ok {
		t.Fatalf("expected headquarters property on PublishingHouse schema")
	}
	if headquarters.Ref == "" {
		t.Fatalf("expected headquarters property to retain reference when resolving cycles")
	}

	headquartersRef := doc.Components.Schemas["Headquarters"]
	if headquartersRef == nil {
		t.Fatalf("schema Headquarters not found")
	}
	convertedHeadquarters := convertSchema(headquartersRef)
	if convertedHeadquarters.Properties == nil {
		t.Fatalf("expected properties for Headquarters schema")
	}
	publisher, ok := convertedHeadquarters.Properties["publisher"]
	if !ok {
		t.Fatalf("expected publisher property on Headquarters schema")
	}
	if publisher.Ref == "" {
		t.Fatalf("expected publisher property to retain reference when resolving cycles")
	}
}
