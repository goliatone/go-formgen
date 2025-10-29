package formgen

import (
	internalLoader "github.com/goliatone/formgen/internal/openapi/loader"
	internalParser "github.com/goliatone/formgen/internal/openapi/parser"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

// NewLoader constructs a loader using the internal implementation while keeping
// the concrete type hidden from consumers.
func NewLoader(options ...pkgopenapi.LoaderOption) pkgopenapi.Loader {
	cfg := pkgopenapi.NewLoaderOptions(options...)
	return internalLoader.New(cfg)
}

// NewParser constructs a parser backed by the internal implementation.
func NewParser(options ...pkgopenapi.ParserOption) pkgopenapi.Parser {
	cfg := pkgopenapi.NewParserOptions(options...)
	return internalParser.New(cfg)
}
