package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	internalLoader "github.com/goliatone/formgen/internal/openapi/loader"
	internalParser "github.com/goliatone/formgen/internal/openapi/parser"
	"github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
	"github.com/goliatone/formgen/pkg/uischema"
)

const defaultRendererName = "vanilla"

// Option customises the orchestrator configuration.
type Option func(*Orchestrator)

// WithLoader injects a custom OpenAPI loader.
func WithLoader(loader pkgopenapi.Loader) Option {
	return func(o *Orchestrator) {
		o.loader = loader
	}
}

// WithParser injects a custom OpenAPI parser.
func WithParser(parser pkgopenapi.Parser) Option {
	return func(o *Orchestrator) {
		o.parser = parser
	}
}

// WithModelBuilder injects a custom form model builder.
func WithModelBuilder(builder model.Builder) Option {
	return func(o *Orchestrator) {
		o.builder = builder
	}
}

// WithRegistry injects a renderer registry.
func WithRegistry(registry *render.Registry) Option {
	return func(o *Orchestrator) {
		o.registry = registry
	}
}

// WithDefaultRenderer overrides the renderer used when a request omits an
// explicit Renderer field.
func WithDefaultRenderer(name string) Option {
	return func(o *Orchestrator) {
		o.defaultRenderer = name
	}
}

// WithSchemaTransformer registers a Transformer that can mutate form models
// after building but before UI schema decorators run.
func WithSchemaTransformer(t Transformer) Option {
	return func(o *Orchestrator) {
		o.transformer = t
	}
}

// WithUIDecorators registers decorators that should run against the generated
// form model before rendering.
func WithUIDecorators(decorators ...model.Decorator) Option {
	return func(o *Orchestrator) {
		if len(decorators) == 0 {
			return
		}
		o.decorators = append(o.decorators, decorators...)
	}
}

// WithUISchemaFS supplies an fs.FS holding UI schema documents. Pass nil to
// disable the embedded defaults.
func WithUISchemaFS(fsys fs.FS) Option {
	return func(o *Orchestrator) {
		o.uiSchemaFS = fsys
		o.uiSchemaSpecified = true
	}
}

// Orchestrator coordinates the full pipeline from OpenAPI document to rendered
// output. It applies sensible defaults (vanilla renderer, embedded templates)
// while remaining open to dependency injection for advanced callers.
type Orchestrator struct {
	loader                pkgopenapi.Loader
	parser                pkgopenapi.Parser
	builder               model.Builder
	registry              *render.Registry
	defaultRenderer       string
	initialiseErr         error
	defaultsApplied       bool
	endpointOverrides     map[string][]EndpointOverride
	decorators            []model.Decorator
	uiSchemaFS            fs.FS
	uiSchemaSpecified     bool
	uiDecoratorConfigured bool
	transformer           Transformer
}

// New constructs an Orchestrator applying any provided options. Missing
// dependencies are initialised with the built-in implementations so callers can
// start with a single constructor call.
func New(options ...Option) *Orchestrator {
	o := &Orchestrator{
		defaultRenderer: defaultRendererName,
	}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(o)
	}
	o.applyDefaults()
	return o
}

// Request describes the inputs required to render a form from an OpenAPI
// operation.
type Request struct {
	// Source identifies where the OpenAPI document lives. Optional when Document
	// is supplied.
	Source pkgopenapi.Source

	// Document allows callers to bypass the loader when they already have a
	// parsed payload.
	Document *pkgopenapi.Document

	// OperationID selects which OpenAPI operation to render into a form.
	OperationID string

	// Renderer names the renderer to use. If empty, the orchestrator falls back
	// to the configured default renderer.
	Renderer string

	// RenderOptions carries per-request instructions such as method overrides,
	// prefilled values, or server-side errors that renderers can surface. When
	// omitted, renderers receive the zero-value struct.
	RenderOptions render.RenderOptions
}

// Generate executes the loader → parser → model builder → renderer sequence and
// returns the rendered bytes (HTML for the default vanilla renderer).
func (o *Orchestrator) Generate(ctx context.Context, req Request) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("orchestrator: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := o.initialiseErr; err != nil {
		return nil, err
	}
	if !o.defaultsApplied {
		o.applyDefaults()
		if err := o.initialiseErr; err != nil {
			return nil, err
		}
	}

	if req.OperationID == "" {
		return nil, errors.New("orchestrator: operation id is required")
	}

	doc, err := o.resolveDocument(ctx, req)
	if err != nil {
		return nil, err
	}

	operations, err := o.parser.Operations(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: parse operations: %w", err)
	}

	op, ok := operations[req.OperationID]
	if !ok {
		return nil, fmt.Errorf("orchestrator: operation %q not found", req.OperationID)
	}

	form, err := o.builder.Build(op)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: build form model: %w", err)
	}

	o.applyEndpointOverrides(req.OperationID, &form)
	if err := o.applyTransformer(ctx, &form); err != nil {
		return nil, err
	}
	if err := o.applyDecorators(&form); err != nil {
		return nil, err
	}

	renderer, err := o.rendererFor(req.Renderer)
	if err != nil {
		return nil, err
	}

	output, err := renderer.Render(ctx, form, req.RenderOptions)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: render output: %w", err)
	}

	return output, nil
}

func (o *Orchestrator) resolveDocument(ctx context.Context, req Request) (pkgopenapi.Document, error) {
	if req.Document != nil {
		return *req.Document, nil
	}
	if req.Source == nil {
		return pkgopenapi.Document{}, errors.New("orchestrator: source or document is required")
	}
	doc, err := o.loader.Load(ctx, req.Source)
	if err != nil {
		return pkgopenapi.Document{}, fmt.Errorf("orchestrator: load document: %w", err)
	}
	return doc, nil
}

func (o *Orchestrator) rendererFor(name string) (render.Renderer, error) {
	if o.registry == nil {
		return nil, errors.New("orchestrator: renderer registry is nil")
	}

	target := name
	if target == "" {
		target = o.defaultRenderer
	}

	if target != "" {
		renderer, err := o.registry.Get(target)
		if err == nil {
			return renderer, nil
		}
		if name != "" {
			return nil, fmt.Errorf("orchestrator: renderer %q: %w", name, err)
		}
	}

	names := o.registry.List()
	if len(names) == 0 {
		return nil, errors.New("orchestrator: no renderers registered")
	}

	renderer, err := o.registry.Get(names[0])
	if err != nil {
		return nil, fmt.Errorf("orchestrator: renderer %q: %w", names[0], err)
	}
	return renderer, nil
}

func (o *Orchestrator) applyDecorators(form *model.FormModel) error {
	if len(o.decorators) == 0 || form == nil {
		return nil
	}
	for _, decorator := range o.decorators {
		if decorator == nil {
			continue
		}
		if err := decorator.Decorate(form); err != nil {
			return fmt.Errorf("orchestrator: decorate form: %w", err)
		}
	}
	return nil
}

func (o *Orchestrator) applyTransformer(ctx context.Context, form *model.FormModel) error {
	if o.transformer == nil || form == nil {
		return nil
	}
	if err := o.transformer.Transform(ctx, form); err != nil {
		return fmt.Errorf("orchestrator: transform form: %w", err)
	}
	return nil
}

func (o *Orchestrator) applyDefaults() {
	if o.defaultsApplied {
		return
	}

	if o.loader == nil {
		o.loader = internalLoader.New(pkgopenapi.NewLoaderOptions())
	}
	if o.parser == nil {
		o.parser = internalParser.New(pkgopenapi.NewParserOptions())
	}
	if o.builder == nil {
		o.builder = model.NewBuilder()
	}
	if o.registry == nil {
		o.registry = render.NewRegistry()
		renderer, err := vanilla.New()
		if err != nil {
			o.initialiseErr = fmt.Errorf("orchestrator: default renderer: %w", err)
		} else {
			o.registry.MustRegister(renderer)
		}
	}
	if o.defaultRenderer == "" {
		o.defaultRenderer = defaultRendererName
	}

	o.ensureUIDecorator()

	o.defaultsApplied = true
}

func (o *Orchestrator) ensureUIDecorator() {
	if o.uiDecoratorConfigured {
		return
	}
	o.uiDecoratorConfigured = true

	if !o.uiSchemaSpecified && o.uiSchemaFS == nil {
		o.uiSchemaFS = uischema.EmbeddedFS()
	}
	if o.uiSchemaFS == nil {
		return
	}

	store, err := uischema.LoadFS(o.uiSchemaFS)
	if err != nil {
		o.initialiseErr = fmt.Errorf("orchestrator: load ui schema: %w", err)
		return
	}
	if store.Empty() {
		return
	}

	o.decorators = append(o.decorators, uischema.NewDecorator(store))
}
