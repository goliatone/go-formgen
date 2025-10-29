package openapi

// Parser contracts bound to go-form-gen.md:304-357.

import "context"

// Parser normalises OpenAPI documents into operation wrappers that downstream
// packages consume. See go-form-gen.md:82-159 for the unidirectional flow.
type Parser interface {
	Operations(ctx context.Context, doc Document) (map[string]Operation, error)
}

// ParserOptions exposes toggles for future behaviour (e.g., dereferencing
// $refs). For v1 most fields remain unused but documenting them up front keeps
// progressive complexity low.
type ParserOptions struct {
	// ResolveReferences controls whether the parser eagerly resolves $ref
	// pointers. Defaults to true for full documents.
	ResolveReferences bool

	// AllowPartialDocuments gates loading component-only inputs. Defaults to
	// false per the README commitment to focus on full documents in v1.
	AllowPartialDocuments bool
}

// ParserOption mutates ParserOptions during construction.
type ParserOption func(*ParserOptions)

// WithReferenceResolution toggles eager reference resolution.
func WithReferenceResolution(enabled bool) ParserOption {
	return func(opts *ParserOptions) {
		opts.ResolveReferences = enabled
	}
}

// WithPartialDocuments toggles support for component-only documents (planned
// for v2 per go-form-gen.md:403-414).
func WithPartialDocuments(enabled bool) ParserOption {
	return func(opts *ParserOptions) {
		opts.AllowPartialDocuments = enabled
	}
}

// NewParserOptions applies ParserOption functions and returns the resulting
// configuration. Implementations under internal/openapi should call this helper
// to remain consistent.
func NewParserOptions(options ...ParserOption) ParserOptions {
	cfg := ParserOptions{
		ResolveReferences:     true,
		AllowPartialDocuments: false,
	}
	for _, opt := range options {
		opt(&cfg)
	}
	return cfg
}

// Construction helpers live in the top-level formgen package to avoid import cycles.
