package vanilla

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.tmpl templates/components/*.tmpl
var embeddedTemplates embed.FS

//go:embed assets/formgen-vanilla.css
var defaultStyles string

// TemplatesFS exposes the embedded template bundle for consumers that want to
// use the built-in form rendering out of the box.
func TemplatesFS() fs.FS {
	return embeddedTemplates
}

func defaultStylesheet() string {
	return defaultStyles
}
