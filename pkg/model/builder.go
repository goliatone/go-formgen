package model

import (
	"errors"
	"fmt"

	internalconditions "github.com/goliatone/go-formgen/internal/conditions"
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
	portable   bool
	labeler    func(string) string
	decorators []Decorator
}

// WithPortableDefinitions enables browser-portable model checks. Check original
// JSON with DiagnosePortableDefinitionJSON before adapters to prevent lost precision.
func WithPortableDefinitions() BuilderOption {
	return func(opts *builderOptions) { opts.portable = true }
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
		portable:   cfg.portable,
		delegate:   internalmodel.New(internalOpts),
		decorators: append([]Decorator(nil), cfg.decorators...),
	}
}

type builder struct {
	portable   bool
	delegate   *internalmodel.Builder
	decorators []Decorator
}

func (b *builder) Build(form schema.Form) (FormModel, error) {
	if b == nil || b.delegate == nil {
		return FormModel{}, errors.New("model: builder delegate is nil")
	}
	output, err := b.delegate.Build(form)
	if err != nil {
		return FormModel{}, err
	}
	if b.portable {
		if diagnostics := DiagnosePortableModel(output); len(diagnostics) > 0 {
			return FormModel{}, fmt.Errorf("model: %s at %s: %s", diagnostics[0].Code, diagnostics[0].Path, diagnostics[0].Message)
		}
	}
	if _, diagnostics := internalconditions.Compile(output); len(diagnostics) > 0 {
		return FormModel{}, fmt.Errorf("model: %s at %s: %s", diagnostics[0].Code, diagnostics[0].Path, diagnostics[0].Message)
	}
	return output, nil
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
