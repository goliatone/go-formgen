// Package model defines the typed form model consumed by renderers, following
// the structure documented in go-form-gen.md:97-158. Builders reside in
// internal/model but return the types defined here. Validation rules expose
// canonical identifiers (min/max, minLength/maxLength, pattern) with string
// parameters so renderers can map numeric bounds (including exclusive limits),
// textual constraints, and regexes onto HTML attributes or runtime validators
// without sacrificing deterministic JSON snapshots. Schema extensions under
// the `x-formgen` namespace flow into `FormModel` and `Field` metadata while
// the curated `UIHints` map surfaces renderer-facing directives such as
// `placeholder`, `helpText`, `cssClass`, `inputType`, `widget`, `repeaterLabel`,
// and visibility toggles like `hideLabel`. Renderers can rely on these hints to
// adjust layout without having to parse raw extension payloads.
package model
