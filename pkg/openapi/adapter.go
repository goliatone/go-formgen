package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/goliatone/go-formgen/pkg/schema"
)

const DefaultAdapterName = "openapi"

// Adapter wraps the OpenAPI loader/parser flow behind the schema adapter interface.
type Adapter struct {
	loader Loader
	parser Parser
}

// NewAdapter constructs an OpenAPI adapter with the supplied loader and parser.
func NewAdapter(loader Loader, parser Parser) *Adapter {
	return &Adapter{
		loader: loader,
		parser: parser,
	}
}

// Name returns the adapter registry identifier.
func (a *Adapter) Name() string {
	return DefaultAdapterName
}

// Detect reports whether the raw payload appears to be OpenAPI.
func (a *Adapter) Detect(_ schema.Source, raw []byte) bool {
	return detectOpenAPI(raw)
}

// Load fetches the raw OpenAPI document.
func (a *Adapter) Load(ctx context.Context, src schema.Source) (schema.Document, error) {
	if a == nil || a.loader == nil {
		return schema.Document{}, errors.New("openapi adapter: loader is nil")
	}
	doc, err := a.loader.Load(ctx, src)
	if err != nil {
		return schema.Document{}, err
	}
	return schema.NewDocument(doc.Source(), doc.Raw())
}

// Normalize parses operations and converts them into the canonical schema IR.
func (a *Adapter) Normalize(ctx context.Context, doc schema.Document, _ schema.NormalizeOptions) (schema.SchemaIR, error) {
	if a == nil || a.parser == nil {
		return schema.SchemaIR{}, errors.New("openapi adapter: parser is nil")
	}

	var openDoc Document
	raw := doc.Raw()
	if doc.Source() == nil && len(raw) == 0 {
		openDoc = Document{}
	} else {
		constructed, err := NewDocument(doc.Source(), raw)
		if err != nil {
			return schema.SchemaIR{}, err
		}
		openDoc = constructed
	}

	operations, err := a.parser.Operations(ctx, openDoc)
	if err != nil {
		return schema.SchemaIR{}, err
	}

	ir := schema.NewSchemaIR()
	for id, op := range operations {
		form := FormFromOperation(op)
		if form.ID == "" {
			form.ID = id
		}
		if ir.Forms == nil {
			ir.Forms = make(map[string]schema.Form)
		}
		ir.Forms[form.ID] = form
	}

	return ir, nil
}

// Forms returns the list of operation-backed form references.
func (a *Adapter) Forms(_ context.Context, ir schema.SchemaIR) ([]schema.FormRef, error) {
	return ir.FormRefs(), nil
}

// FormFromOperation converts an OpenAPI operation into a canonical form.
func FormFromOperation(op Operation) schema.Form {
	form := schema.Form{
		ID:          op.ID,
		Method:      op.Method,
		Endpoint:    op.Path,
		Summary:     op.Summary,
		Description: op.Description,
		Schema:      schemaFromOpenAPISchema(op.RequestBody),
		Extensions:  cloneExtensions(op.Extensions),
	}
	if len(op.Responses) > 0 {
		form.Responses = make(map[string]schema.Schema, len(op.Responses))
		for code, response := range op.Responses {
			form.Responses[code] = schemaFromOpenAPISchema(response)
		}
	}
	return form
}

func schemaFromOpenAPISchema(input Schema) schema.Schema {
	out := schema.Schema{
		Ref:              input.Ref,
		Type:             input.Type,
		Format:           input.Format,
		Description:      input.Description,
		Default:          input.Default,
		Enum:             cloneEnum(input.Enum),
		Required:         cloneStringSlice(input.Required),
		Minimum:          cloneFloatPointer(input.Minimum),
		Maximum:          cloneFloatPointer(input.Maximum),
		ExclusiveMinimum: input.ExclusiveMinimum,
		ExclusiveMaximum: input.ExclusiveMaximum,
		MinLength:        cloneIntPointer(input.MinLength),
		MaxLength:        cloneIntPointer(input.MaxLength),
		Pattern:          input.Pattern,
		Extensions:       cloneExtensions(input.Extensions),
	}
	if len(input.Properties) > 0 {
		out.Properties = make(map[string]schema.Schema, len(input.Properties))
		for key, value := range input.Properties {
			out.Properties[key] = schemaFromOpenAPISchema(value)
		}
	}
	if input.Items != nil {
		items := schemaFromOpenAPISchema(*input.Items)
		out.Items = &items
	}
	return out
}

func cloneExtensions(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneEnum(in []any) []any {
	if len(in) == 0 {
		return nil
	}
	return append([]any(nil), in...)
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}

func cloneFloatPointer(in *float64) *float64 {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func cloneIntPointer(in *int) *int {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func detectOpenAPI(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '{' {
		var payload map[string]any
		if err := json.Unmarshal(trimmed, &payload); err == nil {
			if _, ok := payload["openapi"]; ok {
				return true
			}
			if _, ok := payload["swagger"]; ok {
				return true
			}
		}
	}
	lower := strings.ToLower(string(trimmed))
	return strings.Contains(lower, "openapi:") || strings.Contains(lower, "swagger:")
}
