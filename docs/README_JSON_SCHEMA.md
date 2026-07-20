# JSON Schema Support (Draft 2020-12)

go-formgen is adding a JSON Schema adapter so schemas can drive admin forms
directly. This document defines the supported subset and the fail-fast rules
that the adapter will enforce.

## Supported Subset (Phase 0/1)

The adapter targets Draft 2020-12 and only accepts a focused subset of
keywords needed for form generation:

- `$schema`: must declare Draft 2020-12 (see fail-fast rules).
- `$id`: used for form discovery when explicit IDs are not provided.
- `$defs`: supported as a local reference container.
- `$ref`: local references are supported; external HTTP refs are gated.
- `type`: `object`, `array`, `string`, `integer`, `number`, `boolean` (nullable
  variants via `["<type>", "null"]`).
- `properties`: field declarations for objects.
- `required`: required field list for objects.
- `items`: item schema for arrays.
- `oneOf`: block unions (array items only; see Block widget contract).
- `enum`: enumerated values.
- `const`: single-value enumerations (treated as a fixed value).
- `title`, `description`, `default`.
- `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`.
- `minLength`, `maxLength`, `pattern`.
- `minItems`, `maxItems` for array cardinality.
- `format` (same semantics as OpenAPI: `date`, `time`, `date-time`, `email`,
  `uri`, `tel`, `password`, `byte`, `binary`).
- Vendor extensions: `x-formgen`, `x-formgen-*`, `x-admin`, `x-admin-*`.

Composition keywords such as `allOf`, `anyOf`, `if/then/else`,
`dependentSchemas`, and advanced JSON Schema vocabularies are **not** supported
yet. `oneOf` is supported only for block unions on array items.

JSON numeric defaults retain their source lexemes through form-model creation.
This keeps integers beyond IEEE-754's exact range intact and lets renderers
emit concise values such as `1`, `0`, and `1.25` without float padding.

### Sensitive Fields

Fields are marked sensitive when `format` is `password` or any supported
extension is truthy:

- `x-formgen.sensitive`
- `x-formgen.secret`
- `x-admin.secret`
- `x-formgen-sensitive`
- `x-formgen-secret`
- `x-admin-secret`
- `cli.secret`

Descriptor, vanilla, and Preact renderers redact sensitive defaults unless
`RenderOptions.IncludeSensitiveDefaults` is set for a trusted server-side path.

### go-cms Validation Alignment

If you run the same schemas through go-cms for runtime validation, note that
its validator enforces a **smaller** keyword subset than the adapter above.
Currently accepted keywords are:

- `$schema`, `$id`, `$ref`, `$defs`, `$anchor`
- `type`, `properties`, `required`, `items`, `oneOf`, `allOf`
- `const`, `enum`, `default`
- `title`, `description`, `format`
- `additionalProperties`
- `metadata` and `ui` (normalized into `x-formgen`/`x-admin` hints)

`allOf` is limited to object-merging (properties/required/additionalProperties/title/description).
When targeting both go-formgen and go-cms, stick to the intersection of the two
subsets or expect go-cms to reject stricter constraints (e.g. `minimum`,
`maxLength`, `minItems`, `pattern`) at write time.

## UI Metadata and Overlays

UI hints can be embedded inline via vendor extensions or supplied via separate
overlay documents. Both are supported with explicit precedence: **overlay wins**,
inline is the default.

Inline metadata:
- `x-formgen`, `x-formgen-*`
- `x-admin`, `x-admin-*`

These extensions are normalized into form metadata/UI hints using the same keys
as OpenAPI schemas, so renderers receive consistent `widget`, `label`, and layout
metadata.

### Layout Hints via `x-formgen.grid`

You can declare per-field grid hints directly in `x-formgen`:

```json
{
  "x-formgen": {
    "grid": {
      "span": 8,
      "start": 1,
      "row": 1,
      "breakpoints": {
        "lg": { "span": 6 },
        "xl": { "span": 4, "start": 9 }
      }
    }
  }
}
```

The adapter converts this into `layout.span`, `layout.start`, and `layout.row`
UI hints (including breakpoint variants such as `layout.span.lg`).

### UI Overlay Document Format

Overlays are separate JSON documents that target schema locations using JSON
Pointer. They are applied after `$ref` resolution and before IR generation.

```json
{
  "$schema": "x-ui-overlay/v1",
  "overrides": [
    {
      "path": "/properties/blocks",
      "x-formgen": { "widget": "block", "label": "Page Sections" }
    },
    {
      "path": "/$defs/hero/properties/headline",
      "x-formgen": { "label": "Hero Headline" }
    }
  ]
}
```

Rules:
- `path` is a JSON Pointer into the **expanded** schema (after `$ref` resolution).
- Use standard JSON Pointer escaping (`~1` for `/`, `~0` for `~`).
- Array items are targeted by numeric indices (`/oneOf/0`).
- Overlay values replace inline values for the same extension key. Only vendor
  extensions (`x-formgen`, `x-admin`, and their `x-*-*` shorthands) are applied;
  other overlay keys are ignored by go-formgen.

## Block Widget Contract (Phase 3)

Block editors are modeled as arrays with `oneOf` item unions and `_type`
discriminators. The adapter enforces a strict shape so FormModel output is
deterministic.

### JSON Schema Shape

```json
{
  "type": "object",
  "properties": {
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
      "properties": {
        "_type": { "const": "hero" },
        "headline": { "type": "string" }
      }
    }
  }
}
```

Rules:
- `x-formgen.widget` **must** be set to `block` on the array field (inline or
  via UI overlay).
- `items.oneOf` **must** list block schemas (resolved after `$ref` expansion).
- Each block schema **must** include a `_type` property with a string `const`.
- `_type` is enforced as **required** and **readonly** in the normalized output
  (`x-formgen.readonly=true`).
- Block schemas may include `x-formgen` metadata such as `label`, `icon`,
  `collapsed`, or widget hints for nested fields.

### FormModel Output

The array field is emitted with `uiHints.widget=block`. Its `items` entry
includes a `oneOf` list where each option is a block definition:

- `name` matches the `_type` const value (e.g., `hero`).
- `nested` contains the block's fields, including the `_type` field marked
  readonly via `uiHints.readonly=true`.
- Block-level `x-formgen` metadata is preserved on the option's `metadata`
  map (and `uiHints` where applicable).

## Form Naming Rules

When a JSON Schema source does not provide operation IDs, the adapter derives
form IDs using the following precedence:

1. If `x-formgen.forms` is present, use the explicit `id` values in that list.
2. Else if the root schema has `$id`, derive `<$id>.edit`.
3. Else derive `<slug>.edit` using the caller-supplied content type slug.

## Headless Build API

Use `pkg/orchestrator` directly when you only need a `FormModel`:

```go
orch := orchestrator.New()

form, err := orch.BuildFormModelFromJSONSchemaBytes(ctx, rawSchema, "article.edit")
```

For parsed documents:

```go
doc := schema.MustNewDocument(jsonschema.SourceFromBytes("article"), rawSchema)
form, err := orch.BuildFormModelFromSchemaDocument(
	ctx,
	doc,
	"article.edit",
	orchestrator.WithBuildFormat(jsonschema.DefaultAdapterName),
)
```

Renderer and theme helpers live outside the headless import path. Register a
renderer before calling `Generate`, or use the root package helpers when HTML
output is the goal.

## Fail-Fast Behavior

The adapter will reject schemas early when:

- `$schema` is missing or not Draft 2020-12.
- An unsupported keyword is encountered in a position that would affect form
  generation.
- A `$ref` cannot be resolved (local refs are required to resolve).
- HTTP refs are present without an explicit opt-in flag.
- `x-formgen.forms` exists but has missing or invalid `id` values.
- `oneOf` appears outside of an array `items` schema.
- `oneOf` entries are missing a `_type` discriminator with a string `const`.

Fail-fast errors bubble up through the orchestrator so callers can correct
schemas before rendering.

Ref resolution guardrails (defaults):

- External refs cannot escape the root schema directory (`..` traversal is rejected).
- Each referenced document must be under 5MB (configurable).
- Resolution caps the total number of loaded documents (configurable, default 128).
- HTTP refs require an explicit opt-in (`AllowHTTPRefs`) plus an HTTP-enabled loader.

## Builder Preview Validation

The builder preview pipeline can validate schemas before rendering to return
field-scoped issues. Use the `pkg/validation` helpers to obtain a list of
issues with both JSON Pointer paths and dotted field identifiers:

```go
result := validation.ValidateJSONSchema(ctx, jsonschema.SourceFromFS("schema.json"), raw, validation.JSONSchemaValidationOptions{
    Normalize: schema.NormalizeOptions{
        Overlay: overlayBytes, // optional x-ui-overlay/v1 document
    },
})
if !result.Valid {
    // result.Issues includes Path + Field + Message.
}
```

This validation uses the same adapter rules described above, so any unsupported
keywords, invalid overlays, or block union mistakes are reported with clear
paths for UI display.
