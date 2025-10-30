package template

import (
	"io"
)

// TemplateRenderer mirrors the github.com/goliatone/go-template engine
// contract, providing the seam renderers rely on. See go-form-gen.md:443-460
// for the surrounding design context.
type TemplateRenderer interface {
	Render(name string, data any, out ...io.Writer) (string, error)
	RenderTemplate(name string, data any, out ...io.Writer) (string, error)
	RenderString(templateContent string, data any, out ...io.Writer) (string, error)
	RegisterFilter(name string, fn func(input any, param any) (any, error)) error
	GlobalContext(data any) error
}
