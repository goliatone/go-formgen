package model

import (
	"github.com/goliatone/formgen/internal/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

// Builder converts OpenAPI operations into form models.
type Builder interface {
	Build(op pkgopenapi.Operation) (FormModel, error)
}

// BuilderOption configures the builder behaviour.
type BuilderOption func(*builderOptions)

type builderOptions struct {
	labeler func(string) string
}

// WithLabeler overrides the default label generation function.
func WithLabeler(labeler func(string) string) BuilderOption {
	return func(opts *builderOptions) {
		opts.labeler = labeler
	}
}

// NewBuilder returns a Builder backed by the internal implementation.
func NewBuilder(options ...BuilderOption) Builder {
	cfg := builderOptions{}
	for _, opt := range options {
		opt(&cfg)
	}

	internalOpts := model.Options{}
	if cfg.labeler != nil {
		internalOpts.Labeler = cfg.labeler
	}

	return model.New(internalOpts)
}
