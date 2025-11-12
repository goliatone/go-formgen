package render

// Renderer interfaces align with go-form-gen.md:137-159 and go-form-gen.md:223-239.

import (
	"context"

	"github.com/goliatone/formgen/pkg/model"
)

// Renderer converts a FormModel into a byte representation (HTML, JSX, etc.).
type Renderer interface {
	Name() string
	ContentType() string
	Render(ctx context.Context, model model.FormModel, options RenderOptions) ([]byte, error)
}
