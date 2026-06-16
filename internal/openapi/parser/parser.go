package parser

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
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

	presence := collectSchemaKeywordPresence(raw, spec)
	operations := make(map[string]pkgopenapi.Operation)
	if spec.Paths != nil {
		for path, item := range spec.Paths.Map() {
			if item == nil {
				continue
			}
			p.collectOperation(ctx, operations, "GET", path, item.Get, presence)
			p.collectOperation(ctx, operations, "PUT", path, item.Put, presence)
			p.collectOperation(ctx, operations, "POST", path, item.Post, presence)
			p.collectOperation(ctx, operations, "DELETE", path, item.Delete, presence)
			p.collectOperation(ctx, operations, "PATCH", path, item.Patch, presence)
			p.collectOperation(ctx, operations, "HEAD", path, item.Head, presence)
			p.collectOperation(ctx, operations, "OPTIONS", path, item.Options, presence)
			p.collectOperation(ctx, operations, "TRACE", path, item.Trace, presence)
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

func (p *Parser) collectOperation(ctx context.Context, target map[string]pkgopenapi.Operation, method, path string, operation *openapi3.Operation, presence schemaKeywordPresence) {
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
	requestSchema := p.extractRequestSchema(operation.RequestBody, presence)
	responseSchemas := p.extractResponseSchemas(operation.Responses, presence)

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

func (p *Parser) extractRequestSchema(requestBody *openapi3.RequestBodyRef, presence schemaKeywordPresence) pkgopenapi.Schema {
	if requestBody == nil {
		return pkgopenapi.Schema{}
	}
	if requestBody.Value == nil {
		return pkgopenapi.Schema{Ref: requestBody.Ref}
	}
	content := requestBody.Value.Content
	for _, mediaType := range []string{"application/json", "application/x-www-form-urlencoded", "multipart/form-data"} {
		if mt, ok := content[mediaType]; ok {
			return convertSchemaWithPresence(mt.Schema, presence)
		}
	}
	for _, mt := range content {
		return convertSchemaWithPresence(mt.Schema, presence)
	}
	return pkgopenapi.Schema{}
}

func (p *Parser) extractResponseSchemas(responses *openapi3.Responses, presence schemaKeywordPresence) map[string]pkgopenapi.Schema {
	if responses == nil || responses.Len() == 0 {
		return nil
	}
	result := make(map[string]pkgopenapi.Schema)
	for status, ref := range responses.Map() {
		if ref == nil {
			continue
		}
		schema, ok := responseSchema(ref, presence)
		if !ok {
			continue
		}
		result[status] = schema
	}
	return result
}

func responseSchema(ref *openapi3.ResponseRef, presence schemaKeywordPresence) (pkgopenapi.Schema, bool) {
	if ref.Value == nil {
		return pkgopenapi.Schema{Ref: ref.Ref}, true
	}
	content := ref.Value.Content
	if len(content) == 0 {
		return pkgopenapi.Schema{}, false
	}
	schema := convertSchemaWithPresence(preferredMediaTypeSchema(content), presence)
	if schema.Description == "" && ref.Value.Description != nil {
		schema.Description = *ref.Value.Description
	}
	return schema, schema.Ref != "" || schema.Type != "" || schema.Items != nil || len(schema.Properties) > 0
}

func preferredMediaTypeSchema(content openapi3.Content) *openapi3.SchemaRef {
	if mt, ok := content["application/json"]; ok {
		return mt.Schema
	}
	for _, mt := range content {
		return mt.Schema
	}
	return nil
}

type schemaKeywordPresence struct {
	minItems map[*openapi3.Schema]struct{}
}

func (p schemaKeywordPresence) hasMinItems(schema *openapi3.Schema) bool {
	if schema == nil || len(p.minItems) == 0 {
		return false
	}
	_, ok := p.minItems[schema]
	return ok
}

func (p schemaKeywordPresence) markMinItems(ref *openapi3.SchemaRef) {
	if ref == nil || ref.Value == nil || p.minItems == nil {
		return
	}
	p.minItems[ref.Value] = struct{}{}
}

func collectSchemaKeywordPresence(raw []byte, spec *openapi3.T) schemaKeywordPresence {
	presence := schemaKeywordPresence{minItems: make(map[*openapi3.Schema]struct{})}
	if spec == nil || len(raw) == 0 {
		return presence
	}
	var decoded any
	if err := yaml.Unmarshal(raw, &decoded); err != nil {
		return presence
	}
	root := asStringMap(decoded)
	if root == nil {
		return presence
	}
	markComponentSchemaKeywords(root["components"], spec.Components, presence)
	markPathSchemaKeywords(root["paths"], spec.Paths, presence)
	return presence
}

func markComponentSchemaKeywords(raw any, components *openapi3.Components, presence schemaKeywordPresence) {
	if components == nil {
		return
	}
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	rawSchemas := asStringMap(payload["schemas"])
	for name, ref := range components.Schemas {
		markSchemaKeywords(rawSchemas[name], ref, presence)
	}
	rawRequestBodies := asStringMap(payload["requestBodies"])
	for name, ref := range components.RequestBodies {
		markRequestBodySchemaKeywords(rawRequestBodies[name], ref, presence)
	}
	rawResponses := asStringMap(payload["responses"])
	for name, ref := range components.Responses {
		markResponseSchemaKeywords(rawResponses[name], ref, presence)
	}
}

func markPathSchemaKeywords(raw any, paths *openapi3.Paths, presence schemaKeywordPresence) {
	if paths == nil {
		return
	}
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	for path, item := range paths.Map() {
		rawPath := asStringMap(payload[path])
		if rawPath == nil || item == nil {
			continue
		}
		markOperationSchemaKeywords(rawPath["get"], item.Get, presence)
		markOperationSchemaKeywords(rawPath["put"], item.Put, presence)
		markOperationSchemaKeywords(rawPath["post"], item.Post, presence)
		markOperationSchemaKeywords(rawPath["delete"], item.Delete, presence)
		markOperationSchemaKeywords(rawPath["patch"], item.Patch, presence)
		markOperationSchemaKeywords(rawPath["head"], item.Head, presence)
		markOperationSchemaKeywords(rawPath["options"], item.Options, presence)
		markOperationSchemaKeywords(rawPath["trace"], item.Trace, presence)
	}
}

func markOperationSchemaKeywords(raw any, operation *openapi3.Operation, presence schemaKeywordPresence) {
	if operation == nil {
		return
	}
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	markRequestBodySchemaKeywords(payload["requestBody"], operation.RequestBody, presence)
	rawResponses := asStringMap(payload["responses"])
	if rawResponses == nil || operation.Responses == nil {
		return
	}
	for status, ref := range operation.Responses.Map() {
		markResponseSchemaKeywords(rawResponses[status], ref, presence)
	}
}

func markRequestBodySchemaKeywords(raw any, ref *openapi3.RequestBodyRef, presence schemaKeywordPresence) {
	if ref == nil || ref.Value == nil {
		return
	}
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	markContentSchemaKeywords(payload["content"], ref.Value.Content, presence)
}

func markResponseSchemaKeywords(raw any, ref *openapi3.ResponseRef, presence schemaKeywordPresence) {
	if ref == nil || ref.Value == nil {
		return
	}
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	markContentSchemaKeywords(payload["content"], ref.Value.Content, presence)
}

func markContentSchemaKeywords(raw any, content openapi3.Content, presence schemaKeywordPresence) {
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	for mediaType, mt := range content {
		if mt == nil {
			continue
		}
		rawMediaType := asStringMap(payload[mediaType])
		if rawMediaType == nil {
			continue
		}
		markSchemaKeywords(rawMediaType["schema"], mt.Schema, presence)
	}
}

func markSchemaKeywords(raw any, ref *openapi3.SchemaRef, presence schemaKeywordPresence) {
	if ref == nil || ref.Value == nil {
		return
	}
	payload := asStringMap(raw)
	if payload == nil {
		return
	}
	if _, ok := payload["minItems"]; ok {
		presence.markMinItems(ref)
	}
	rawProperties := asStringMap(payload["properties"])
	for name, child := range ref.Value.Properties {
		markSchemaKeywords(rawProperties[name], child, presence)
	}
	if rawItems, ok := payload["items"]; ok {
		markSchemaKeywords(rawItems, ref.Value.Items, presence)
	}
	markSchemaListKeywords(payload["allOf"], ref.Value.AllOf, presence)
	markSchemaListKeywords(payload["oneOf"], ref.Value.OneOf, presence)
	markSchemaListKeywords(payload["anyOf"], ref.Value.AnyOf, presence)
}

func markSchemaListKeywords(raw any, refs openapi3.SchemaRefs, presence schemaKeywordPresence) {
	items, ok := raw.([]any)
	if !ok {
		return
	}
	for idx, ref := range refs {
		if idx >= len(items) {
			return
		}
		markSchemaKeywords(items[idx], ref, presence)
	}
}

func asStringMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			str, ok := key.(string)
			if !ok {
				continue
			}
			out[str] = item
		}
		return out
	default:
		return nil
	}
}

func convertSchema(ref *openapi3.SchemaRef) pkgopenapi.Schema {
	return convertSchemaWithPresence(ref, schemaKeywordPresence{})
}

func convertSchemaWithPresence(ref *openapi3.SchemaRef, presence schemaKeywordPresence) pkgopenapi.Schema {
	return convertSchemaWithState(ref, make(map[*openapi3.Schema]pkgopenapi.Schema), make(map[*openapi3.Schema]struct{}), presence)
}

func convertSchemaWithState(
	ref *openapi3.SchemaRef,
	cache map[*openapi3.Schema]pkgopenapi.Schema,
	active map[*openapi3.Schema]struct{},
	presence schemaKeywordPresence,
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

	schema := baseSchemaFromOpenAPI(ref.Ref, src)
	applySchemaChildren(&schema, src, cache, active, presence)
	applySchemaNumberBounds(&schema, src)
	applyExclusiveMinimum(&schema, src.ExclusiveMin)
	applyExclusiveMaximum(&schema, src.ExclusiveMax)
	applySchemaStringBounds(&schema, src)
	applySchemaArrayBounds(&schema, src, presence)
	schema.Extensions = extractExtensions(src.Extensions)
	mergeAllOfSchemas(&schema, src.AllOf, cache, active, presence)
	mergeAllOfExtensions(&schema, src.AllOf, make(map[*openapi3.Schema]struct{}))

	delete(active, src)
	cache[src] = schema
	return schema
}

func baseSchemaFromOpenAPI(ref string, src *openapi3.Schema) pkgopenapi.Schema {
	schema := pkgopenapi.Schema{
		Ref:         ref,
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
	return schema
}

func applySchemaChildren(schema *pkgopenapi.Schema, src *openapi3.Schema, cache map[*openapi3.Schema]pkgopenapi.Schema, active map[*openapi3.Schema]struct{}, presence schemaKeywordPresence) {
	if len(src.Properties) > 0 {
		properties := make(map[string]pkgopenapi.Schema, len(src.Properties))
		for name, property := range src.Properties {
			properties[name] = convertSchemaWithState(property, cache, active, presence)
		}
		schema.Properties = propagateRelationshipMetadata(properties)
	}
	if src.Items != nil {
		items := convertSchemaWithState(src.Items, cache, active, presence)
		schema.Items = &items
	}
}

func applySchemaNumberBounds(schema *pkgopenapi.Schema, src *openapi3.Schema) {
	if src.Min != nil {
		value := *src.Min
		schema.Minimum = &value
	}
	if src.Max != nil {
		value := *src.Max
		schema.Maximum = &value
	}
}

func applySchemaStringBounds(schema *pkgopenapi.Schema, src *openapi3.Schema) {
	if src.MinLength != 0 {
		if value, ok := schemaLengthToInt(src.MinLength); ok {
			schema.MinLength = &value
		}
	}
	if src.MaxLength != nil {
		if value, ok := schemaLengthToInt(*src.MaxLength); ok {
			schema.MaxLength = &value
		}
	}
	if src.Pattern != "" {
		schema.Pattern = src.Pattern
	}
}

func applySchemaArrayBounds(schema *pkgopenapi.Schema, src *openapi3.Schema, presence schemaKeywordPresence) {
	if src.MinItems != 0 || presence.hasMinItems(src) {
		if value, ok := schemaLengthToInt(src.MinItems); ok {
			schema.MinItems = &value
		}
	}
	if src.MaxItems != nil {
		if value, ok := schemaLengthToInt(*src.MaxItems); ok {
			schema.MaxItems = &value
		}
	}
}

func schemaLengthToInt(value uint64) (int, bool) {
	const maxInt = int(^uint(0) >> 1)
	if value > uint64(maxInt) {
		return 0, false
	}
	return int(value), true
}

func applyExclusiveMinimum(schema *pkgopenapi.Schema, bound openapi3.ExclusiveBound) {
	if schema == nil {
		return
	}
	if bound.Value != nil {
		value := *bound.Value
		if schema.Minimum == nil || value >= *schema.Minimum {
			schema.Minimum = &value
			schema.ExclusiveMinimum = true
		}
		return
	}
	schema.ExclusiveMinimum = bound.IsTrue()
}

func applyExclusiveMaximum(schema *pkgopenapi.Schema, bound openapi3.ExclusiveBound) {
	if schema == nil {
		return
	}
	if bound.Value != nil {
		value := *bound.Value
		if schema.Maximum == nil || value <= *schema.Maximum {
			schema.Maximum = &value
			schema.ExclusiveMaximum = true
		}
		return
	}
	schema.ExclusiveMaximum = bound.IsTrue()
}

func mergeAllOfSchemas(target *pkgopenapi.Schema, refs openapi3.SchemaRefs, cache map[*openapi3.Schema]pkgopenapi.Schema, active map[*openapi3.Schema]struct{}, presence schemaKeywordPresence) {
	if target == nil || len(refs) == 0 {
		return
	}

	for _, ref := range refs {
		if ref == nil {
			continue
		}
		merged := convertSchemaWithState(ref, cache, active, presence)
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

	mergeRequired(target, source.Required)
	mergeProperties(target, source.Properties)
	if target.Items == nil && source.Items != nil {
		items := source.Items.Clone()
		target.Items = &items
	}
	if len(target.Enum) == 0 && len(source.Enum) > 0 {
		target.Enum = append([]any(nil), source.Enum...)
	}

	mergeNumericConstraints(target, source)
	mergeStringConstraints(target, source)
	mergeArrayConstraints(target, source)
	mergeSchemaExtensions(target, source.Extensions)
}

func mergeRequired(target *pkgopenapi.Schema, source []string) {
	if len(source) == 0 {
		return
	}
	required := make(map[string]struct{}, len(target.Required)+len(source))
	for _, name := range target.Required {
		required[name] = struct{}{}
	}
	for _, name := range source {
		required[name] = struct{}{}
	}
	target.Required = sortedSet(required)
}

func sortedSet(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func mergeProperties(target *pkgopenapi.Schema, source map[string]pkgopenapi.Schema) {
	if len(source) == 0 {
		return
	}
	if target.Properties == nil {
		target.Properties = make(map[string]pkgopenapi.Schema, len(source))
	}
	for name, schema := range source {
		if _, exists := target.Properties[name]; !exists {
			target.Properties[name] = schema
		}
	}
}

func mergeNumericConstraints(target *pkgopenapi.Schema, source pkgopenapi.Schema) {
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
}

func mergeStringConstraints(target *pkgopenapi.Schema, source pkgopenapi.Schema) {
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
}

func mergeArrayConstraints(target *pkgopenapi.Schema, source pkgopenapi.Schema) {
	if target.MinItems == nil && source.MinItems != nil {
		value := *source.MinItems
		target.MinItems = &value
	}
	if target.MaxItems == nil && source.MaxItems != nil {
		value := *source.MaxItems
		target.MaxItems = &value
	}
}

func mergeSchemaExtensions(target *pkgopenapi.Schema, source map[string]any) {
	if len(source) == 0 {
		return
	}
	if target.Extensions == nil {
		target.Extensions = make(map[string]any, len(source))
	}
	for key, value := range source {
		if _, exists := target.Extensions[key]; !exists {
			target.Extensions[key] = value
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
		if extensionValue, ok := normaliseExtensionValue(key, value); ok {
			result[key] = extensionValue
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normaliseExtensionValue(key string, value any) (any, bool) {
	switch {
	case key == extensionNamespace || key == adminExtensionNamespace || key == endpointExtensionKey:
		mapped, ok := cloneMap(value)
		return mapped, ok && len(mapped) > 0
	case strings.HasPrefix(key, extensionNamespace+"-") || strings.HasPrefix(key, adminExtensionNamespace+"-"):
		return value, true
	case key == relationshipExtensionKey:
		metadata := normaliseRelationshipExtension(value)
		return metadata, len(metadata) > 0
	case key == currentValueExtensionKey:
		return value, value != nil
	default:
		return nil, false
	}
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
			maps.Copy(target.Extensions, ext)
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
	maps.Copy(cloned, mapped)
	return cloned, true
}
