package jsonschema

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/goliatone/go-formgen/pkg/schema"
)

const DefaultAdapterName = "jsonschema"

// Adapter wraps JSON Schema parsing and normalization behind the schema adapter interface.
type Adapter struct {
	loader   Loader
	resolver *Resolver
}

// AdapterOption configures a JSON Schema adapter.
type AdapterOption func(*adapterOptions)

type adapterOptions struct {
	resolver       *Resolver
	resolverConfig ResolveOptions
}

// WithResolver injects a custom resolver implementation.
func WithResolver(resolver *Resolver) AdapterOption {
	return func(opts *adapterOptions) {
		opts.resolver = resolver
	}
}

// WithResolverOptions supplies options to the default resolver.
func WithResolverOptions(options ResolveOptions) AdapterOption {
	return func(opts *adapterOptions) {
		opts.resolverConfig = options
	}
}

// NewAdapter constructs a JSON Schema adapter with the supplied loader.
func NewAdapter(loader Loader, options ...AdapterOption) *Adapter {
	opts := adapterOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	resolver := opts.resolver
	if resolver == nil {
		resolver = NewResolver(loader, opts.resolverConfig)
	}

	return &Adapter{
		loader:   loader,
		resolver: resolver,
	}
}

// Name returns the adapter registry identifier.
func (a *Adapter) Name() string {
	return DefaultAdapterName
}

// Detect reports whether the raw payload appears to be JSON Schema.
func (a *Adapter) Detect(_ schema.Source, raw []byte) bool {
	return detectJSONSchema(raw)
}

// Load fetches the raw JSON Schema document.
func (a *Adapter) Load(ctx context.Context, src schema.Source) (schema.Document, error) {
	if a == nil || a.loader == nil {
		return schema.Document{}, errors.New("jsonschema adapter: loader is nil")
	}
	doc, err := a.loader.Load(ctx, src)
	if err != nil {
		return schema.Document{}, err
	}
	return schema.NewDocument(doc.Source(), doc.Raw())
}

// Normalize resolves refs and converts JSON Schema into the canonical schema IR.
func (a *Adapter) Normalize(ctx context.Context, doc schema.Document, opts schema.NormalizeOptions) (schema.SchemaIR, error) {
	if a == nil || a.resolver == nil {
		return schema.SchemaIR{}, errors.New("jsonschema adapter: resolver is nil")
	}
	raw := doc.Raw()
	if len(raw) == 0 {
		return schema.SchemaIR{}, errors.New("jsonschema adapter: empty document")
	}

	payload, err := parseJSONSchema(raw)
	if err != nil {
		return schema.SchemaIR{}, err
	}

	if err := validateDialect(payload); err != nil {
		return schema.SchemaIR{}, err
	}

	resolved, err := a.resolver.Resolve(ctx, doc, payload)
	if err != nil {
		return schema.SchemaIR{}, err
	}

	canonical, err := schemaFromJSONSchema(resolved, "#")
	if err != nil {
		return schema.SchemaIR{}, err
	}

	forms, err := DiscoverFormsFromMap(payload, FormDiscoveryOptions{
		Slug:         opts.ContentTypeSlug,
		FormIDSuffix: opts.DefaultFormSuffix,
	})
	if err != nil {
		return schema.SchemaIR{}, err
	}

	if opts.FormID != "" {
		filtered := make([]schema.FormRef, 0, 1)
		for _, ref := range forms {
			if ref.ID == opts.FormID {
				filtered = append(filtered, ref)
				break
			}
		}
		if len(filtered) == 0 {
			return schema.SchemaIR{}, fmt.Errorf("jsonschema adapter: form %q not found", opts.FormID)
		}
		forms = filtered
	}

	ir := schema.NewSchemaIR()
	for _, ref := range forms {
		form := schema.Form{
			ID:          ref.ID,
			Summary:     resolveFormSummary(ref),
			Description: ref.Description,
			Schema:      canonical,
		}
		if form.Endpoint == "" {
			form.Endpoint = deriveFormEndpoint(opts.ContentTypeSlug)
		}
		if form.Method == "" {
			form.Method = defaultFormMethod
		}
		if ir.Forms == nil {
			ir.Forms = make(map[string]schema.Form)
		}
		ir.Forms[form.ID] = form
	}

	return ir, nil
}

// Forms returns the list of available form references.
func (a *Adapter) Forms(_ context.Context, ir schema.SchemaIR) ([]schema.FormRef, error) {
	return ir.FormRefs(), nil
}

func parseJSONSchema(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("jsonschema: raw schema is empty")
	}
	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("jsonschema: parse schema: %w", err)
	}
	if payload == nil {
		return nil, errors.New("jsonschema: schema is nil")
	}
	return payload, nil
}

func validateDialect(payload map[string]any) error {
	raw := readString(payload, "$schema")
	value := strings.TrimSpace(raw)
	if value == "" {
		return errors.New("jsonschema: $schema is required")
	}
	if !isDraft202012(value) {
		return fmt.Errorf("jsonschema: unsupported $schema %q", value)
	}
	return nil
}

func isDraft202012(value string) bool {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, "#")
	switch trimmed {
	case "https://json-schema.org/draft/2020-12/schema", "http://json-schema.org/draft/2020-12/schema":
		return true
	default:
		return false
	}
}

func detectJSONSchema(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] != '{' {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return false
	}
	if payload == nil {
		return false
	}
	if _, ok := payload["openapi"]; ok {
		return false
	}
	if _, ok := payload["swagger"]; ok {
		return false
	}
	if _, ok := payload["$schema"]; ok {
		return true
	}
	if _, ok := payload["$id"]; ok {
		return true
	}
	if _, ok := payload["$defs"]; ok {
		return true
	}
	if _, ok := payload["properties"]; ok {
		return true
	}
	if _, ok := payload["type"]; ok {
		return true
	}
	if _, ok := payload["items"]; ok {
		return true
	}
	return false
}

func resolveFormSummary(ref schema.FormRef) string {
	if strings.TrimSpace(ref.Summary) != "" {
		return ref.Summary
	}
	return strings.TrimSpace(ref.Title)
}

const defaultFormMethod = "POST"

func deriveFormEndpoint(slug string) string {
	value := strings.TrimSpace(slug)
	if value == "" {
		return "/"
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}
