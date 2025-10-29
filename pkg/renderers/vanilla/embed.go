package vanilla

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.tmpl templates/fields/*.tmpl
var embeddedTemplates embed.FS

// TemplatesFS exposes the embedded template bundle for consumers that want to
// use the built-in form rendering out of the box.
func TemplatesFS() fs.FS {
	return embeddedTemplates
}
