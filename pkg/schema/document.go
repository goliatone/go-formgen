package schema

import "errors"

// Document wraps the raw schema payload and its origin.
type Document struct {
	source Source
	raw    []byte
}

// NewDocument constructs a Document wrapper while validating the inputs.
func NewDocument(src Source, raw []byte) (Document, error) {
	if src == nil {
		return Document{}, errors.New("schema: source is required")
	}
	if len(raw) == 0 {
		return Document{}, errors.New("schema: raw document is empty")
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

// Raw returns a defensive copy of the payload.
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
