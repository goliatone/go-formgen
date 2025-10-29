package preact

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/*
var embeddedAssets embed.FS

// TemplatesFS exposes the embedded template bundle for consumers that want the
// default Preact layout.
func TemplatesFS() fs.FS {
	return embeddedTemplates
}

// AssetsFS exposes the embedded script and stylesheet bundle to copy into
// downstream distributions.
func AssetsFS() fs.FS {
	return embeddedAssets
}
