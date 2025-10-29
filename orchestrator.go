package formgen

import (
	"context"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
)

// NewOrchestrator exposes the orchestrator constructor from the top-level
// module, mirroring the quick start guidance in go-form-gen.md:223-258.
func NewOrchestrator(options ...orchestrator.Option) *orchestrator.Orchestrator {
	return orchestrator.New(options...)
}

// GenerateHTML loads the OpenAPI source, builds a form model for the requested
// operation, and renders it using the named renderer. It is the simplest entry
// point for callers that just want HTML output.
func GenerateHTML(ctx context.Context, source pkgopenapi.Source, operationID, rendererName string, options ...orchestrator.Option) ([]byte, error) {
	gen := orchestrator.New(options...)
	return gen.Generate(ctx, orchestrator.Request{
		Source:      source,
		OperationID: operationID,
		Renderer:    rendererName,
	})
}

// GenerateHTMLFromDocument renders a form using a pre-loaded document,
// bypassing the loader stage while still delegating to the orchestrator.
func GenerateHTMLFromDocument(ctx context.Context, doc pkgopenapi.Document, operationID, rendererName string, options ...orchestrator.Option) ([]byte, error) {
	gen := orchestrator.New(options...)
	return gen.Generate(ctx, orchestrator.Request{
		Document:    &doc,
		OperationID: operationID,
		Renderer:    rendererName,
	})
}
