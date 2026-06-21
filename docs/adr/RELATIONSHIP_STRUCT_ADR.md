# ADR: Relationship Struct Migration

## Status

Proposed

## Context

Phase 1 delivered relationship metadata via the existing `Field.Metadata` and `Field.UIHints` maps. Parser and builder helpers normalize `x-relationships` extensions into canonical keys (`relationship.type`, `relationship.target`, `relationship.foreignKey`, `relationship.cardinality`, `relationship.inverse`, etc.), and renderers rely on those strings to drive dropdowns, subforms, or collections.

As adoption grows, carrying relationship semantics as loose key/value pairs introduces a few drawbacks:

- Repeated string lookups encourage copy/paste bugs and make future migrations noisy.
- Nested structures (has-many, has-one) and sibling foreign-key resolution are harder to reason about without a dedicated type.
- Public consumers cannot lean on Go’s type system to interrogate relationships; helpers must decode string maps.

We want a typed `Relationship` struct that sits alongside `Field`. The struct should mirror the metadata we already emit without breaking existing consumers during the migration.

## Decision

Introduce a typed relationship model in two steps:

1. **Add `internal/model.Relationship`**

```go
   type RelationshipKind string

   const (
       RelationshipBelongsTo RelationshipKind = "belongsTo"
       RelationshipHasOne    RelationshipKind = "hasOne"
       RelationshipHasMany   RelationshipKind = "hasMany"
   )

   type Relationship struct {
       Kind        RelationshipKind `json:"kind"`
       Target      string           `json:"target"`      // JSON Pointer to schema
       ForeignKey  string           `json:"foreignKey"`  // Optional (belongs-to/has-one)
       Cardinality string           `json:"cardinality"` // "one"/"many" (mirrors metadata)
       Inverse     string           `json:"inverse,omitempty"`
       SourceField string           `json:"sourceField,omitempty"` // breadcrumb for sibling associations
   }
```

- `internal/model.Field` receives an optional `Relationship *Relationship` field.
- Parser and builder continue populating the existing metadata/UI hints for backward compatibility while hydrating the struct.
- Conversion helpers live under `internal/model/relationships.go` to keep main builder logic lean.

2. **Propagate the struct to `pkg/model`**
   - Export `pkg/model.Relationship` mirroring the internal type.
   - Add JSON tags so renderers can adopt the struct incrementally.
   - Keep the metadata keys around for an entire release cycle; mark them as deprecated in docs/commentary once adoption is high enough to rely on the struct alone.

### Migration Strategy

- Phase 2 implementation will populate _both_ the struct and metadata keys.
- Renderer updates can prefer the struct but fall back to metadata for legacy payloads.
- Once major consumers flip to the struct, we deprecate the raw keys (targeting v1.4+).

## Consequences

- **Pros**
  - Type safety for downstream consumers.
  - Central place to extend relationship semantics (e.g., through/optional attributes) without stringly typed maps.
  - Smoother doc/examples by showing a dedicated struct alongside metadata.
- **Cons**
  - Slightly larger `Field` struct.
  - Requires dual writes (struct + metadata) during the transition.

## Follow-up Tasks

- Update Phase 5 tasks in `REL_TSK.md` to track implementation work.
- Add TODO markers in parser/builder referencing this ADR so contributors know where struct hydration will land.
- Audit renderers and orchestrator fixtures for typed struct support once implemented.
