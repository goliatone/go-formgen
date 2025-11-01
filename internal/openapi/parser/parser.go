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

// Ensure the implementation satisfies the public interface.
var _ pkgopenapi.Parser = (*Parser)(nil)

// New constructs a Parser with the given options.
func New(options pkgopenapi.ParserOptions) pkgopenapi.Parser {
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

	if spec.Paths == nil || spec.Paths.Len() == 0 {
		if !p.options.AllowPartialDocuments {
			return nil, errors.New("openapi parser: document does not contain any paths")
		}
	}

	if err := p.resolveReferences(ctx, loader, spec); err != nil {
		return nil, err
	}

	operations := make(map[string]pkgopenapi.Operation)
	if spec.Paths != nil {
		for path, item := range spec.Paths.Map() {
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
	op.Extensions = extractExtensions(operation.Extensions)
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

func (p *Parser) extractResponseSchemas(responses *openapi3.Responses) map[string]pkgopenapi.Schema {
	if responses == nil || responses.Len() == 0 {
		return nil
	}
	result := make(map[string]pkgopenapi.Schema)
	for status, ref := range responses.Map() {
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
		Type:        firstSchemaType(src.Type),
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
		properties := make(map[string]pkgopenapi.Schema, len(src.Properties))
		for name, property := range src.Properties {
			properties[name] = convertSchema(property)
		}
		schema.Properties = propagateRelationshipMetadata(properties)
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
	schema.Extensions = extractExtensions(src.Extensions)
	mergeAllOfExtensions(&schema, src.AllOf)
	return schema
}

func firstSchemaType(types *openapi3.Types) string {
	if types == nil {
		return ""
	}
	values := types.Slice()
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	default:
		return strings.Join(values, ",")
	}
}

const (
	extensionNamespace       = "x-formgen"
	endpointExtensionKey     = "x-endpoint"
	currentValueExtensionKey = "x-current-value"
)

func extractExtensions(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	result := make(map[string]any)
	for key, value := range raw {
		switch {
		case key == extensionNamespace:
			if mapped, ok := cloneMap(value); ok && len(mapped) > 0 {
				result[key] = mapped
			}
		case strings.HasPrefix(key, extensionNamespace+"-"):
			result[key] = value
		case key == relationshipExtensionKey:
			if metadata := normaliseRelationshipExtension(value); len(metadata) > 0 {
				result[key] = metadata
			}
		case key == endpointExtensionKey:
			if mapped, ok := cloneMap(value); ok && len(mapped) > 0 {
				result[key] = mapped
			}
		case key == currentValueExtensionKey:
			if value != nil {
				result[key] = value
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mergeAllOfExtensions(target *pkgopenapi.Schema, refs openapi3.SchemaRefs) {
	if target == nil || len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		if ref == nil || ref.Value == nil {
			continue
		}
		if ext := extractExtensions(ref.Value.Extensions); len(ext) > 0 {
			if target.Extensions == nil {
				target.Extensions = make(map[string]any, len(ext))
			}
			for key, value := range ext {
				target.Extensions[key] = value
			}
		}
		mergeAllOfExtensions(target, ref.Value.AllOf)
	}
}

func cloneMap(value any) (map[string]any, bool) {
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	cloned := make(map[string]any, len(mapped))
	for k, v := range mapped {
		cloned[k] = v
	}
	return cloned, true
}
