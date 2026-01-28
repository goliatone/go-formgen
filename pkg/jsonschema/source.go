package jsonschema

import "github.com/goliatone/go-formgen/pkg/schema"

// Source identifies where a JSON Schema document originated. This is an alias
// to the canonical schema source abstraction so adapters can share loader logic.
type Source = schema.Source

// SourceKind enumerates the loader modalities.
type SourceKind = schema.SourceKind

const (
	SourceKindFile = schema.SourceKindFile
	SourceKindFS   = schema.SourceKindFS
	SourceKindURL  = schema.SourceKindURL
)

// SourceFromFile returns a Source pointing to a file path.
func SourceFromFile(path string) Source {
	return schema.SourceFromFile(path)
}

// SourceFromFS returns a Source identifying a resource inside an fs.FS.
func SourceFromFS(name string) Source {
	return schema.SourceFromFS(name)
}

// SourceFromURL parses the supplied URL string and returns a Source. It panics
// if the URL is invalid to surface configuration mistakes early.
func SourceFromURL(raw string) Source {
	return schema.SourceFromURL(raw)
}
