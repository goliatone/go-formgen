package openapi

// This file defines the domain wrappers described in go-form-gen.md:25-159.

import (
	"errors"
	"fmt"
)

// Source identifies where an OpenAPI document originated. It mirrors the Source
// abstraction outlined in go-form-gen.md:304-331 so loaders can operate on
// files, fs.FS entries, or URLs without leaking implementation details.
type Source interface {
	Kind() SourceKind
	Location() string
}

// SourceKind enumerates the loader modalities.
type SourceKind string

const (
	SourceKindFile SourceKind = "file"
	SourceKindFS   SourceKind = "fs"
	SourceKindURL  SourceKind = "url"
)

// Document wraps the raw OpenAPI payload and its origin. By exposing this type
// instead of kin-openapi structs we keep the public API decoupled, as committed
// in go-form-gen.md:25-77.
type Document struct {
	source Source
	raw    []byte
}

// NewDocument constructs a Document wrapper while validating the inputs.
func NewDocument(src Source, raw []byte) (Document, error) {
	if src == nil {
		return Document{}, errors.New("openapi: source is required")
	}
	if len(raw) == 0 {
		return Document{}, errors.New("openapi: raw document is empty")
	}

	clone := append([]byte(nil), raw...)
	return Document{source: src, raw: clone}, nil
}

// MustNewDocument panics if the document cannot be created. Useful for tests.
func MustNewDocument(src Source, raw []byte) Document {
	doc, err := NewDocument(src, raw)
	if err != nil {
		panic(err)
	}
	return doc
}

// Source returns the origin metadata for the document.
func (d Document) Source() Source {
	return d.source
}

// Raw returns a defensive copy of the OpenAPI payload.
func (d Document) Raw() []byte {
	return append([]byte(nil), d.raw...)
}

// Location returns the string identifier for the origin.
func (d Document) Location() string {
	if d.source == nil {
		return ""
	}
	return d.source.Location()
}

// Operation models the subset of OpenAPI operation metadata needed to build
// form models, aligning with go-form-gen.md:111-158.
type Operation struct {
	ID          string
	Method      string
	Path        string
	Summary     string
	Description string
	RequestBody Schema
	Responses   map[string]Schema
}

// NewOperation validates core fields and initialises response maps.
func NewOperation(id, method, path string, request Schema, responses map[string]Schema) (Operation, error) {
	if id == "" {
		return Operation{}, errors.New("openapi: operation id is required")
	}
	if method == "" {
		return Operation{}, errors.New("openapi: operation method is required")
	}
	if path == "" {
		return Operation{}, errors.New("openapi: operation path is required")
	}
	if responses == nil {
		responses = make(map[string]Schema)
	}

	return Operation{
		ID:          id,
		Method:      method,
		Path:        path,
		RequestBody: request,
		Responses:   responses,
	}, nil
}

// MustNewOperation panics when construction fails, assisting fixtures/tests.
func MustNewOperation(id, method, path string, request Schema, responses map[string]Schema) Operation {
	op, err := NewOperation(id, method, path, request, responses)
	if err != nil {
		panic(err)
	}
	return op
}

// HasResponse reports whether a response code has a schema registered.
func (op Operation) HasResponse(code string) bool {
	_, ok := op.Responses[code]
	return ok
}

// Schema represents request/response bodies and nested fields within an
// operation, linked to the README description in go-form-gen.md:111-158.
type Schema struct {
	Ref         string
	Type        string
	Format      string
	Required    []string
	Properties  map[string]Schema
	Items       *Schema
	Enum        []any
	Description string
	Default     any
}

// Clone creates a deep copy of the schema tree to avoid accidental mutation.
func (s Schema) Clone() Schema {
	cloned := s
	if len(s.Required) > 0 {
		cloned.Required = append([]string(nil), s.Required...)
	}
	if len(s.Enum) > 0 {
		cloned.Enum = append([]any(nil), s.Enum...)
	}
	if len(s.Properties) > 0 {
		cloned.Properties = make(map[string]Schema, len(s.Properties))
		for k, v := range s.Properties {
			cloned.Properties[k] = v.Clone()
		}
	}
	if s.Items != nil {
		items := s.Items.Clone()
		cloned.Items = &items
	}
	return cloned
}

// Validate performs basic sanity checks useful for callers before building
// form models.
func (s Schema) Validate() error {
	if s.Type == "" && s.Ref == "" {
		return errors.New("openapi: schema requires either type or ref")
	}
	if s.Type == "array" && s.Items == nil {
		return errors.New("openapi: array schema must define items")
	}
	return nil
}

// DebugString renders the schema for logging/debugging without exposing
// implementation details. This helps maintain observability without coupling to
// kin-openapi structures (see go-form-gen.md:82-159).
func (s Schema) DebugString() string {
	summary := fmt.Sprintf("type=%s", s.Type)
	if s.Ref != "" {
		summary += fmt.Sprintf(",ref=%s", s.Ref)
	}
	if len(s.Required) > 0 {
		summary += fmt.Sprintf(",required=%d", len(s.Required))
	}
	if len(s.Properties) > 0 {
		summary += fmt.Sprintf(",properties=%d", len(s.Properties))
	}
	if s.Items != nil {
		summary += ",items=true"
	}
	return summary
}
