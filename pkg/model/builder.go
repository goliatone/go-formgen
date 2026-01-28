package model

import (
	"errors"

	internalmodel "github.com/goliatone/go-formgen/internal/model"
	"github.com/goliatone/go-formgen/pkg/schema"
)

// Builder converts normalized schema forms into form models and applies
// optional decorators for UI schema overlays.
type Builder interface {
	Build(form schema.Form) (FormModel, error)
	Decorate(form *FormModel) error
}

// BuilderOption configures the builder behaviour.
type BuilderOption func(*builderOptions)

type builderOptions struct {
	labeler    func(string) string
	decorators []Decorator
}

// WithLabeler overrides the default label generation function.
func WithLabeler(labeler func(string) string) BuilderOption {
	return func(opts *builderOptions) {
		opts.labeler = labeler
	}
}

// WithDecorators registers decorators that should run when Decorate is called.
func WithDecorators(decorators ...Decorator) BuilderOption {
	return func(opts *builderOptions) {
		if len(decorators) == 0 {
			return
		}
		opts.decorators = append(opts.decorators, decorators...)
	}
}

// NewBuilder returns a Builder backed by the internal implementation.
func NewBuilder(options ...BuilderOption) Builder {
	cfg := builderOptions{}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}

	internalOpts := internalmodel.Options{}
	if cfg.labeler != nil {
		internalOpts.Labeler = cfg.labeler
	}

	return &builder{
		delegate:   internalmodel.New(internalOpts),
		decorators: append([]Decorator(nil), cfg.decorators...),
	}
}

type builder struct {
	delegate   *internalmodel.Builder
	decorators []Decorator
}

func (b *builder) Build(form schema.Form) (FormModel, error) {
	if b == nil || b.delegate == nil {
		return FormModel{}, errors.New("model: builder delegate is nil")
	}
	return b.delegate.Build(form)
}

func (b *builder) Decorate(form *FormModel) error {
	if form == nil {
		return errors.New("model: form is nil")
	}
	if b == nil {
		return nil
	}
	for _, decorator := range b.decorators {
		if decorator == nil {
			continue
		}
		if err := decorator.Decorate(form); err != nil {
			return err
		}
	}
	return nil
}
