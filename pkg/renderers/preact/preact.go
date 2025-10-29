package preact

import (
	"context"
	"errors"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/render"
)

var errNotImplemented = errors.New("preact renderer not implemented")

type Renderer struct{}

func New(_ ...interface{}) render.Renderer {
	return &Renderer{}
}

func (r *Renderer) Name() string {
	return "preact"
}

func (r *Renderer) ContentType() string {
	return "text/html; charset=utf-8"
}

func (r *Renderer) Render(context.Context, model.FormModel) ([]byte, error) {
	return nil, errNotImplemented
}
