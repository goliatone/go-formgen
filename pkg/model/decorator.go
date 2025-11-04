package model

// Decorator enriches a form model with additional metadata after the canonical
// OpenAPI-derived structure has been built.
type Decorator interface {
	Decorate(*FormModel) error
}

// DecoratorFunc adapts a function into a Decorator.
type DecoratorFunc func(*FormModel) error

// Decorate calls the underlying function.
func (fn DecoratorFunc) Decorate(form *FormModel) error {
	return fn(form)
}
