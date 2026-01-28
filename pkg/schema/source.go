package schema

import (
	"fmt"
	"net/url"
	"path/filepath"
)

// Source identifies where a schema document originated so loaders can operate
// on files, fs.FS entries, or URLs without leaking implementation details.
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

// fileSource identifies on-disk schema documents.
type fileSource struct {
	path string
}

func (s fileSource) Location() string {
	return s.path
}

func (s fileSource) Kind() SourceKind {
	return SourceKindFile
}

// SourceFromFile returns a Source pointing to a file path.
func SourceFromFile(path string) Source {
	return fileSource{path: filepath.Clean(path)}
}

// fsSource references a path within an fs.FS.
type fsSource struct {
	name string
}

func (s fsSource) Location() string {
	return s.name
}

func (s fsSource) Kind() SourceKind {
	return SourceKindFS
}

// SourceFromFS returns a Source identifying a resource inside an fs.FS.
func SourceFromFS(name string) Source {
	return fsSource{name: name}
}

// urlSource references an HTTP/HTTPS endpoint.
type urlSource struct {
	raw string
}

func (s urlSource) Location() string {
	return s.raw
}

func (s urlSource) Kind() SourceKind {
	return SourceKindURL
}

// SourceFromURL parses the supplied URL string and returns a Source. It panics
// if the URL is invalid to surface configuration mistakes early.
func SourceFromURL(raw string) Source {
	if raw == "" {
		panic("schema: empty URL source")
	}
	if _, err := url.ParseRequestURI(raw); err != nil {
		panic(fmt.Sprintf("schema: invalid URL %q: %v", raw, err))
	}
	return urlSource{raw: raw}
}
