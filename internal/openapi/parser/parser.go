package parser

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

// Parser implements pkgopenapi.Parser using kin-openapi.
type Parser struct {
	options pkgopenapi.ParserOptions
}

// New constructs a Parser with the given options.
func New(options pkgopenapi.ParserOptions) *Parser {
	return &Parser{options: options}
}

// Operations converts a Document into a map keyed by operationId.
func (p *Parser) Operations(ctx context.Context, doc pkgopenapi.Document) (map[string]pkgopenapi.Operation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw := doc.Raw()
	if len(raw) == 0 {
		return nil, errors.New("openapi parser: document payload is empty")
	}

	loader := &openapi3.Loader{
		Context:               ctx,
		IsExternalRefsAllowed: p.options.ResolveReferences,
	}

	spec, err := loader.LoadFromData(raw)
	if err != nil {
		return nil, fmt.Errorf("openapi parser: load document: %w", err)
	}

	if spec.Paths == nil || len(spec.Paths) == 0 {
		if !p.options.AllowPartialDocuments {
			return nil, errors.New("openapi parser: document does not contain any paths")
		}
	}

	if err := p.resolveReferences(ctx, loader, spec); err != nil {
		return nil, err
	}

	operations := make(map[string]pkgopenapi.Operation)
	for path, item := range spec.Paths {
		if item == nil {
			continue
		}
		p.collectOperation(ctx, operations, "GET", path, item.Get)
		p.collectOperation(ctx, operations, "PUT", path, item.Put)
		p.collectOperation(ctx, operations, "POST", path, item.Post)
		p.collectOperation(ctx, operations, "DELETE", path, item.Delete)
		p.collectOperation(ctx, operations, "PATCH", path, item.Patch)
		p.collectOperation(ctx, operations, "HEAD", path, item.Head)
		p.collectOperation(ctx, operations, "OPTIONS", path, item.Options)
		p.collectOperation(ctx, operations, "TRACE", path, item.Trace)
	}

	if len(operations) == 0 && !p.options.AllowPartialDocuments {
		return nil, errors.New("openapi parser: no operations extracted")
	}

	return operations, nil
}

func (p *Parser) resolveReferences(ctx context.Context, loader *openapi3.Loader, spec *openapi3.T) error {
	if !p.options.ResolveReferences {
		return nil
	}
	if err := spec.Validate(ctx, openapi3.DisableExamplesValidation()); err != nil {
		return fmt.Errorf("openapi parser: validate: %w", err)
	}
	return nil
}

func (p *Parser) collectOperation(ctx context.Context, target map[string]pkgopenapi.Operation, method, path string, operation *openapi3.Operation) {
	if ctx.Err() != nil {
		return
	}
	if operation == nil {
		return
	}
	opID := operation.OperationID
	if opID == "" {
		opID = strings.ToLower(method) + ":" + path
	}
	requestSchema := p.extractRequestSchema(operation.RequestBody)
	responseSchemas := p.extractResponseSchemas(operation.Responses)

	op, err := pkgopenapi.NewOperation(opID, method, path, requestSchema, responseSchemas)
	if err != nil {
		// Invalid operations are skipped but noted by leaving them out.
		return
	}
	op.Summary = operation.Summary
	op.Description = operation.Description
	target[opID] = op
}

func (p *Parser) extractRequestSchema(requestBody *openapi3.RequestBodyRef) pkgopenapi.Schema {
	if requestBody == nil {
		return pkgopenapi.Schema{}
	}
	if requestBody.Value == nil {
		return pkgopenapi.Schema{Ref: requestBody.Ref}
	}
	content := requestBody.Value.Content
	for _, mediaType := range []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"} {
		if mt, ok := content[mediaType]; ok {
			return convertSchema(mt.Schema)
		}
	}
	for _, mt := range content {
		return convertSchema(mt.Schema)
	}
	return pkgopenapi.Schema{}
}

func (p *Parser) extractResponseSchemas(responses openapi3.Responses) map[string]pkgopenapi.Schema {
	result := make(map[string]pkgopenapi.Schema)
	for status, ref := range responses {
		if ref == nil {
			continue
		}
		var schema pkgopenapi.Schema
		if ref.Value == nil {
			schema = pkgopenapi.Schema{Ref: ref.Ref}
		} else {
			content := ref.Value.Content
			if len(content) == 0 {
				continue
			}
			if mt, ok := content["application/json"]; ok {
				schema = convertSchema(mt.Schema)
			} else {
				for _, mt := range content {
					schema = convertSchema(mt.Schema)
					break
				}
			}
			if schema.Description == "" && ref.Value.Description != nil {
				schema.Description = *ref.Value.Description
			}
		}
		if schema.Ref == "" && schema.Type == "" && schema.Items == nil && len(schema.Properties) == 0 {
			continue
		}
		result[status] = schema
	}
	return result
}

func convertSchema(ref *openapi3.SchemaRef) pkgopenapi.Schema {
	if ref == nil {
		return pkgopenapi.Schema{}
	}
	if ref.Value == nil {
		return pkgopenapi.Schema{Ref: ref.Ref}
	}
	src := ref.Value
	schema := pkgopenapi.Schema{
		Ref:         ref.Ref,
		Type:        src.Type,
		Format:      src.Format,
		Description: src.Description,
		Default:     src.Default,
	}

	if len(src.Required) > 0 {
		schema.Required = append([]string(nil), src.Required...)
	}
	if len(src.Enum) > 0 {
		schema.Enum = append([]any(nil), src.Enum...)
	}
	if len(src.Properties) > 0 {
		schema.Properties = make(map[string]pkgopenapi.Schema, len(src.Properties))
		for name, property := range src.Properties {
			schema.Properties[name] = convertSchema(property)
		}
	}
	if src.Items != nil {
		items := convertSchema(src.Items)
		schema.Items = &items
	}
	if src.Min != nil {
		value := *src.Min
		schema.Minimum = &value
	}
	if src.Max != nil {
		value := *src.Max
		schema.Maximum = &value
	}
	schema.ExclusiveMinimum = src.ExclusiveMin
	schema.ExclusiveMaximum = src.ExclusiveMax
	if src.MinLength != 0 {
		value := int(src.MinLength)
		schema.MinLength = &value
	}
	if src.MaxLength != nil {
		value := int(*src.MaxLength)
		schema.MaxLength = &value
	}
	if src.Pattern != "" {
		schema.Pattern = src.Pattern
	}
	return schema
}
