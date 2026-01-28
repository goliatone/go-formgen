package jsonschema

import "github.com/goliatone/go-formgen/pkg/schema"

// Document wraps the raw JSON Schema payload and its origin. This is an alias
// to the canonical schema.Document to keep the adapter decoupled from loaders.
type Document = schema.Document

// NewDocument constructs a Document wrapper while validating the inputs.
func NewDocument(src Source, raw []byte) (Document, error) {
	return schema.NewDocument(src, raw)
}

// MustNewDocument panics if the document cannot be created. Useful for tests.
func MustNewDocument(src Source, raw []byte) Document {
	return schema.MustNewDocument(src, raw)
}
