package formgen

import (
	"io/fs"

	vanilla "github.com/goliatone/formgen/pkg/renderers/vanilla"
)

// EmbeddedTemplates exposes the built-in vanilla renderer templates so callers
// can reuse or extend them without importing the renderer package directly.
func EmbeddedTemplates() fs.FS {
	fsys := vanilla.TemplatesFS()
	return fsys
}
