package parser

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
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

func TestConvertSchemaMergesAllOfSchemas(t *testing.T) {
	t.Parallel()

	const document = `{
  "openapi": "3.0.0",
  "info": { "title": "AllOf", "version": "1.0.0" },
  "paths": {
    "/users": {
      "post": {
        "operationId": "createUser",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "allOf": [
                  {"$ref": "#/components/schemas/BaseUser"},
                  {
                    "type": "object",
                    "required": ["email"],
                    "properties": {
                      "email": {"type": "string", "format": "email"}
                    }
                  }
                ]
              }
            }
          }
        },
        "responses": {
          "200": {"description": "ok"}
        }
      }
    }
  },
  "components": {
    "schemas": {
      "BaseUser": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string"},
          "age": {"type": "integer", "minimum": 1}
        }
      }
    }
  }
}`

	doc, err := pkgopenapi.NewDocument(pkgopenapi.SourceFromFile("inline.json"), []byte(document))
	if err != nil {
		t.Fatalf("construct document: %v", err)
	}

	parser := New(pkgopenapi.NewParserOptions())
	operations, err := parser.Operations(context.Background(), doc)
	if err != nil {
		t.Fatalf("parse operations: %v", err)
	}

	op, ok := operations["createUser"]
	if !ok {
		t.Fatalf("operation createUser not found")
	}

	req := op.RequestBody
	if req.Type != "object" {
		t.Fatalf("request schema type = %q, want object", req.Type)
	}

	if len(req.Properties) != 3 {
		t.Fatalf("properties length = %d, want 3", len(req.Properties))
	}

	if _, ok := req.Properties["name"]; !ok {
		t.Fatalf("expected name property from allOf ref")
	}
	if email, ok := req.Properties["email"]; !ok || email.Format != "email" {
		t.Fatalf("expected email property with format email, got %+v", email)
	}
	if age, ok := req.Properties["age"]; !ok || age.Minimum == nil || *age.Minimum != 1 {
		t.Fatalf("expected age property with minimum 1, got %+v", age)
	}

	required := make(map[string]struct{}, len(req.Required))
	for _, name := range req.Required {
		required[name] = struct{}{}
	}
	if _, ok := required["name"]; !ok {
		t.Fatalf("required set missing name")
	}
	if _, ok := required["email"]; !ok {
		t.Fatalf("required set missing email")
	}
}
