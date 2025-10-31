# Parser Design Notes

`internal/openapi/parser` translates OpenAPI documents into the normalised operation types exposed by `pkg/openapi`. The relationship pipeline now targets the struct-only baseline captured in `REL_TDD.md`.

## Relationship Struct Hydration

The parser reads `x-relationships` blocks, normalises casing, and hydrates the typed `Relationship` struct attached to each field:

- `kind` → `Relationship.Kind`
- `target` → `Relationship.Target`
- `foreignKey` → `Relationship.ForeignKey`
- `cardinality` → `Relationship.Cardinality`
- `inverse` → `Relationship.Inverse`
- `sourceField` → `Relationship.SourceField`

The foreign-key host remains canonical: when the extension references a sibling, we attach the relationship struct to that sibling and leave a breadcrumb (`sourceField`) on the originating object. Renderers therefore interact solely with `Field.Relationship`, while builder/UI-hint helpers derive `Field.UIHints["input"]`/`["cardinality"]` from the same struct.
