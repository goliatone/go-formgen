# Schema Requirements for go-formgen

This document explains how to publish an OpenAPI document that go-formgen can consume. Share it with any team that wants to expose schemas for HTML form generation.

Looking for JSON Schema? See the Draft 2020-12 adapter guide in `docs/README_JSON_SCHEMA.md` for supported keywords, block unions, and UI metadata rules.

## Minimum OpenAPI Checklist

Make sure every document you hand to go-formgen satisfies the following:

1. `openapi`: the document must declare version `3.0.x` or `3.1.x`.
2. `info`: include, at minimum, `title` and `version`.
3. `paths`: define at least one path item with an HTTP operation (`get`, `post`, `put`, `patch`, `delete`, etc.).
4. `operationId`: supply a stable identifier; go-formgen looks up operations by this string. If you omit it, the fallback id is `<method>:<path>` (for example `post:/config`).
5. `requestBody`: expose the form payload under a recognised content type. go-formgen considers `application/json`, `application/x-www-form-urlencoded`, and `multipart/form-data`, picking the first one it finds.
6. `responses`: include at least one response object (a minimal `"204": { "description": "..." }` satisfies both the OpenAPI spec and go-formgen).
7. Schemas referenced from the request body must either live inline or under `components.schemas`. All `$ref` targets must resolve; dangling references are surfaced as metadata and skip form generation.

If any of these pieces are missing, the parser will stop with errors such as:

- `openapi parser: document does not contain any paths`
- `openapi parser: validate: ... must have required property 'openapi'`

## Request Body Schema Rules

go-formgen builds form fields from the request-body schema attached to the operation.

- Root type: use `type: object` so nested properties become top-level form fields. If the schema is empty (no type, no properties), the generated form will be empty.
- Properties: declare `properties` for every field. The builder sorts property names alphabetically before rendering.
- Required fields: populate the `required` array to mark inputs as mandatory.
- Default values: `default` becomes the pre-filled value.
- Enumerations: `enum` values are passed through as string/number/boolean options.
- Numeric constraints: `minimum`, `maximum`, `exclusiveMinimum`, and `exclusiveMaximum` generate min/max validation rules.
- String constraints: `minLength`, `maxLength`, and `pattern` become validation rules.
- Array constraints: `minItems` and `maxItems` become item-count validation rules.
- Formats: recognised formats (`date`, `time`, `date-time`, `email`, `uri`, `tel`, `password`, `byte`, `binary`) inform the renderer’s `inputType`.
- Arrays: `type: array` requires an `items` schema. The items schema can be another object or a primitive. Missing `items` causes `model builder: array field "<name>" missing items`.
- Nested objects: nested `type: object` properties are rendered as grouped sub-fields.
- Nullable: OpenAPI’s `nullable` flag is ignored today; model builders treat fields as non-nullable but optional unless marked required.
- Composition keywords (`allOf`, `oneOf`, `anyOf`, `not`, `dependencies`, `if/then/else`) are not expanded in OpenAPI inputs. Avoid them or dereference them into plain objects before handing the schema to go-formgen. (The JSON Schema adapter supports `oneOf` only for block unions on array items; see `docs/README_JSON_SCHEMA.md`.)

## Supported Custom Extensions

go-formgen consumes a small set of OpenAPI extensions to decorate fields.

### `x-formgen` namespace

- `x-formgen`: map of key/value hints (strings, numbers, booleans, or JSON-serialisable objects). Keys such as `label`, `placeholder`, `hint`, `widget`, `cssClass`, `section`, `accordion`, `badge`, `priority`, `submitLabel`, `successMessage`, `helpText`, `unit`, and `inputType` are recognised. JSON editor hints include `schemaHint`, `jsonExample`, `collapsed`, `editorMode`, and `editorActiveView`.
- `x-formgen-*`: shorthand—for example `x-formgen-placeholder: "Hostname"`.

Values are stringified; booleans become `"true"`/`"false"`. Empty strings are ignored.

### Relationship metadata

- `x-relationships`: describe data relationships. Supported keys: `type` (`belongsTo`, `hasOne`, `hasMany`), `target` (schema `$ref`), `foreignKey`, `inverse`, `cardinality`, `sourceField`, and `through`. go-formgen uses this to pick widgets (`select`, `subform`, `collection`) and to propagate cardinality hints.
- `x-endpoint`: enriches relationships that source options from an API (URL, HTTP method, label/value field names, query params, dynamic params, mapping rules, auth config).
- `x-current-value`: optional string that pre-populates the field with an existing relationship value (surface via metadata key `relationship.current`).

See `examples/http/schema.json` for end-to-end samples of these extensions.

## Putting It Together: Sample Config Operation

Below is a minimal OpenAPI document that wraps the configuration schema provided by the config service. Adjust titles, descriptions, and defaults as needed.

```json
{
  "openapi": "3.0.3",
  "info": {
    "title": "Config Service",
    "version": "1.0.0"
  },
  "paths": {
    "/config": {
      "post": {
        "operationId": "createConfig",
        "summary": "Create application configuration",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ApplicationConfig"
              }
            }
          }
        },
        "responses": {
          "204": {
            "description": "Configuration accepted"
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "ApplicationConfig": {
        "type": "object",
        "required": ["database", "server"],
        "properties": {
          "database": {
            "type": "object",
            "required": ["host", "port"],
            "properties": {
              "host": {
                "type": "string",
                "description": "Database host name or IP"
              },
              "port": {
                "type": "integer",
                "minimum": 1,
                "maximum": 65535
              },
              "maxRetries": {
                "type": "integer",
                "minimum": 0,
                "default": 3
              },
              "ssl": {
                "type": "boolean",
                "default": true
              }
            }
          },
          "features": {
            "type": "object",
            "properties": {
              "enabled": {
                "type": "boolean",
                "default": false
              },
              "flags": {
                "type": "array",
                "items": {
                  "type": "string"
                }
              }
            }
          },
          "server": {
            "type": "object",
            "required": ["host", "port"],
            "properties": {
              "host": {
                "type": "string"
              },
              "port": {
                "type": "integer",
                "minimum": 1,
                "maximum": 65535
              }
            }
          }
        }
      }
    }
  }
}
```

## Troubleshooting Checklist

Before handing the file to go-formgen, validate these items:

- Run `kin-openapi` validation or `speccy`/`swagger-cli validate`. Fix any reported structural errors.
- Confirm every `$ref` resolves by running go-formgen with `WithReferenceResolution(true)` (default) and checking logs.
- Ensure arrays specify `items`; relationships reference existing sibling properties; and required lists match actual property names.
- If you need richer UI behaviour, attach the relevant `x-formgen` hints or `x-relationships` metadata.

Following this guide keeps third-party schemas compatible with go-formgen and prevents round-trips caused by missing OpenAPI scaffolding.

## UI Schema Field Order Presets

UI schema documents (`ui/*.json|yaml`) can declare reusable ordering sequences so multiple sections share the same layout hints. Add an optional top-level `fieldOrderPresets` object alongside the existing `operations` block:

```json
{
  "fieldOrderPresets": {
    "defaultDetails": ["name", "description", "*", "created_at", "updated_at"],
    "audited": ["*", "created_at", "updated_at"]
  },
  "operations": {
    "post-book:create": {
      "sections": [
        {"id": "details", "title": "Details", "orderPreset": "defaultDetails"},
        {"id": "audit", "title": "Audit", "orderPreset": ["notes", "*", "updated_at"]}
      ]
    }
  }
}
```

Sections may reference a preset by name (`orderPreset: "defaultDetails"`) or inline their own array. The `*` token expands to “all remaining fields in their natural order”; omitting it appends the residual set at the end. During decoration the resolved order is stored under `FormModel.Metadata["layout.fieldOrder.<sectionID>"]` so renderers (vanilla and Preact) can respect the same sequence without recomputing it.
