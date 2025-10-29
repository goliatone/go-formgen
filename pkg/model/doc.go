// Package model defines the typed form model consumed by renderers, following
// the structure documented in go-form-gen.md:97-158. Builders reside in
// internal/model but return the types defined here. Validation rules expose
// canonical identifiers (min/max, minLength/maxLength, pattern) with string
// parameters so renderers can map numeric bounds and textual constraints onto
// HTML attributes or runtime validators without sacrificing deterministic JSON.
package model
