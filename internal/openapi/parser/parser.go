package parser

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	return convertSchemaWithState(ref, make(map[*openapi3.Schema]pkgopenapi.Schema), make(map[*openapi3.Schema]struct{}))
}

func convertSchemaWithState(
	ref *openapi3.SchemaRef,
	cache map[*openapi3.Schema]pkgopenapi.Schema,
	active map[*openapi3.Schema]struct{},
) pkgopenapi.Schema {
	if ref == nil {
		return pkgopenapi.Schema{}
	}
	if ref.Value == nil {
		return pkgopenapi.Schema{Ref: ref.Ref}
	}

	src := ref.Value
	if cached, ok := cache[src]; ok {
		return cached
	}
	if _, ok := active[src]; ok {
		result := pkgopenapi.Schema{Ref: ref.Ref}
		cache[src] = result
		return result
	}
	active[src] = struct{}{}

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
			properties[name] = convertSchemaWithState(property, cache, active)
		}
		schema.Properties = propagateRelationshipMetadata(properties)
	}
	if src.Items != nil {
		items := convertSchemaWithState(src.Items, cache, active)
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
	mergeAllOfSchemas(&schema, src.AllOf, cache, active)
	mergeAllOfExtensions(&schema, src.AllOf, make(map[*openapi3.Schema]struct{}))

	delete(active, src)
	cache[src] = schema
	return schema
}

func mergeAllOfSchemas(target *pkgopenapi.Schema, refs openapi3.SchemaRefs, cache map[*openapi3.Schema]pkgopenapi.Schema, active map[*openapi3.Schema]struct{}) {
	if target == nil || len(refs) == 0 {
		return
	}

	for _, ref := range refs {
		if ref == nil {
			continue
		}
		merged := convertSchemaWithState(ref, cache, active)
		mergeSchema(target, merged)
	}
}

func mergeSchema(target *pkgopenapi.Schema, source pkgopenapi.Schema) {
	if target == nil {
		return
	}

	if target.Type == "" {
		target.Type = source.Type
	}
	if target.Format == "" {
		target.Format = source.Format
	}
	if target.Description == "" {
		target.Description = source.Description
	}
	if target.Default == nil && source.Default != nil {
		target.Default = source.Default
	}

	if len(source.Required) > 0 {
		required := make(map[string]struct{}, len(target.Required)+len(source.Required))
		for _, name := range target.Required {
			required[name] = struct{}{}
		}
		for _, name := range source.Required {
			required[name] = struct{}{}
		}
		if len(required) > 0 {
			keys := make([]string, 0, len(required))
			for name := range required {
				keys = append(keys, name)
			}
			sort.Strings(keys)
			target.Required = target.Required[:0]
			target.Required = append(target.Required, keys...)
		}
	}

	if len(source.Properties) > 0 {
		if target.Properties == nil {
			target.Properties = make(map[string]pkgopenapi.Schema, len(source.Properties))
		}
		for name, schema := range source.Properties {
			if _, exists := target.Properties[name]; !exists {
				target.Properties[name] = schema
			}
		}
	}

	if target.Items == nil && source.Items != nil {
		items := source.Items.Clone()
		target.Items = &items
	}

	if len(target.Enum) == 0 && len(source.Enum) > 0 {
		target.Enum = append([]any(nil), source.Enum...)
	}

	if target.Minimum == nil && source.Minimum != nil {
		value := *source.Minimum
		target.Minimum = &value
	}
	if target.Maximum == nil && source.Maximum != nil {
		value := *source.Maximum
		target.Maximum = &value
	}
	if !target.ExclusiveMinimum && source.ExclusiveMinimum {
		target.ExclusiveMinimum = true
	}
	if !target.ExclusiveMaximum && source.ExclusiveMaximum {
		target.ExclusiveMaximum = true
	}
	if target.MinLength == nil && source.MinLength != nil {
		value := *source.MinLength
		target.MinLength = &value
	}
	if target.MaxLength == nil && source.MaxLength != nil {
		value := *source.MaxLength
		target.MaxLength = &value
	}
	if target.Pattern == "" {
		target.Pattern = source.Pattern
	}

	if len(source.Extensions) > 0 {
		if target.Extensions == nil {
			target.Extensions = make(map[string]any, len(source.Extensions))
		}
		for key, value := range source.Extensions {
			if _, exists := target.Extensions[key]; !exists {
				target.Extensions[key] = value
			}
		}
	}
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
	adminExtensionNamespace  = "x-admin"
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
		case key == adminExtensionNamespace:
			if mapped, ok := cloneMap(value); ok && len(mapped) > 0 {
				result[key] = mapped
			}
		case strings.HasPrefix(key, adminExtensionNamespace+"-"):
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

func mergeAllOfExtensions(target *pkgopenapi.Schema, refs openapi3.SchemaRefs, visited map[*openapi3.Schema]struct{}) {
	if target == nil || len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		if ref == nil || ref.Value == nil {
			continue
		}
		if visited != nil {
			if _, seen := visited[ref.Value]; seen {
				continue
			}
			visited[ref.Value] = struct{}{}
		}
		if ext := extractExtensions(ref.Value.Extensions); len(ext) > 0 {
			if target.Extensions == nil {
				target.Extensions = make(map[string]any, len(ext))
			}
			for key, value := range ext {
				target.Extensions[key] = value
			}
		}
		mergeAllOfExtensions(target, ref.Value.AllOf, visited)
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
