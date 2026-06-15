package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	jsonschemaLoader "github.com/goliatone/go-formgen/internal/jsonschema/loader"
	internalLoader "github.com/goliatone/go-formgen/internal/openapi/loader"
	internalParser "github.com/goliatone/go-formgen/internal/openapi/parser"
	pkgjsonschema "github.com/goliatone/go-formgen/pkg/jsonschema"
	"github.com/goliatone/go-formgen/pkg/model"
	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/uischema"
	"github.com/goliatone/go-formgen/pkg/visibility"
	"github.com/goliatone/go-formgen/pkg/widgets"
)

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

// WithJSONSchemaLoader injects a custom JSON Schema loader.
func WithJSONSchemaLoader(loader pkgjsonschema.Loader) Option {
	return func(o *Orchestrator) {
		o.jsonSchemaLoader = loader
	}
}

// WithJSONSchemaResolverOptions configures resolver options for the default JSON Schema adapter.
func WithJSONSchemaResolverOptions(options pkgjsonschema.ResolveOptions) Option {
	return func(o *Orchestrator) {
		o.jsonSchemaResolveOptions = options
	}
}

// WithAdapterRegistry injects a format adapter registry.
func WithAdapterRegistry(registry *AdapterRegistry) Option {
	return func(o *Orchestrator) {
		if registry == nil {
			return
		}
		o.adapterRegistry = registry
	}
}

// WithFormatAdapter registers a format adapter on the orchestrator registry.
func WithFormatAdapter(adapter FormatAdapter) Option {
	return func(o *Orchestrator) {
		if adapter == nil {
			return
		}
		if o.adapterRegistry == nil {
			o.adapterRegistry = NewAdapterRegistry()
		}
		if err := o.adapterRegistry.Register(adapter); err != nil {
			o.initialiseErr = appendInitialiseError(o.initialiseErr, err)
		}
	}
}

// WithDefaultAdapter overrides the default format adapter name.
func WithDefaultAdapter(name string) Option {
	return func(o *Orchestrator) {
		o.defaultAdapter = strings.TrimSpace(name)
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

// WithRenderer registers a renderer on the orchestrator registry.
func WithRenderer(renderer render.Renderer) Option {
	return func(o *Orchestrator) {
		if renderer == nil {
			return
		}
		if o.registry == nil {
			o.registry = render.NewRegistry()
		}
		if err := o.registry.Register(renderer); err != nil {
			o.initialiseErr = appendInitialiseError(o.initialiseErr, err)
		}
	}
}

// RendererFactory constructs a renderer during orchestrator initialization.
type RendererFactory func() (render.Renderer, error)

// WithRendererFactory registers a renderer returned by the factory.
func WithRendererFactory(factory RendererFactory) Option {
	return func(o *Orchestrator) {
		if factory == nil {
			return
		}
		renderer, err := factory()
		if err != nil {
			o.initialiseErr = appendInitialiseError(o.initialiseErr, err)
			return
		}
		WithRenderer(renderer)(o)
	}
}

// RenderOptionsResolver can enrich renderer options immediately before
// rendering. Renderer-facing packages use this to opt into theme integration
// without making the core orchestrator import theme dependencies.
type RenderOptionsResolver func(context.Context, Request, model.FormModel, render.RenderOptions) (render.RenderOptions, error)

// WithRenderOptionsResolver injects a renderer-option resolver.
func WithRenderOptionsResolver(resolver RenderOptionsResolver) Option {
	return func(o *Orchestrator) {
		if resolver == nil {
			return
		}
		o.renderOptionsResolvers = append(o.renderOptionsResolvers, resolver)
	}
}

// WithWidgetRegistry injects a custom widget registry used to resolve widgets
// before rendering.
func WithWidgetRegistry(registry *widgets.Registry) Option {
	return func(o *Orchestrator) {
		o.widgetRegistry = registry
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

// WithVisibilityEvaluator injects a visibility evaluator to decide whether
// fields guarded by visibility rules should render. When unset, visibility
// rules are ignored.
func WithVisibilityEvaluator(evaluator visibility.Evaluator) Option {
	return func(o *Orchestrator) {
		o.visibilityEvaluator = evaluator
	}
}

// Orchestrator coordinates schema loading, normalization, FormModel building,
// and optional rendering. The core constructor is renderer-free; callers that
// render output must register renderers explicitly or use a compatibility
// helper package that opts into renderer dependencies.
type Orchestrator struct {
	loader                   pkgopenapi.Loader
	parser                   pkgopenapi.Parser
	jsonSchemaLoader         pkgjsonschema.Loader
	jsonSchemaResolveOptions pkgjsonschema.ResolveOptions
	builder                  model.Builder
	adapterRegistry          *AdapterRegistry
	defaultAdapter           string
	registry                 *render.Registry
	defaultRenderer          string
	renderOptionsResolvers   []RenderOptionsResolver
	widgetRegistry           *widgets.Registry
	initialiseErr            error
	defaultsApplied          bool
	endpointOverrides        map[string][]EndpointOverride
	decorators               []model.Decorator
	uiSchemaFS               fs.FS
	uiSchemaSpecified        bool
	uiDecoratorConfigured    bool
	transformer              Transformer
	visibilityEvaluator      visibility.Evaluator
}

// New constructs an Orchestrator applying any provided options. Missing
// dependencies are initialised with the built-in implementations so callers can
// start with a single constructor call.
func New(options ...Option) *Orchestrator {
	o := &Orchestrator{}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(o)
	}
	o.applyDefaults()
	return o
}

// WidgetRegistry returns the widget registry used to decorate form models
// before rendering. Adapters can call this at runtime to register custom
// widgets without rebuilding the orchestrator.
func (o *Orchestrator) WidgetRegistry() *widgets.Registry {
	if o == nil {
		return nil
	}
	if !o.defaultsApplied {
		o.applyDefaults()
	}
	return o.widgetRegistry
}

// RegisterWidget registers a widget matcher on the orchestrator's registry,
// enabling adapters to extend the built-in set without overriding defaults.
func (o *Orchestrator) RegisterWidget(name string, priority int, matcher widgets.Matcher) {
	if reg := o.WidgetRegistry(); reg != nil {
		reg.Register(name, priority, matcher)
	}
}

// SetVisibilityEvaluator replaces the configured visibility evaluator. This is
// useful for adapters that need to attach evaluators after orchestrator
// construction.
func (o *Orchestrator) SetVisibilityEvaluator(evaluator visibility.Evaluator) {
	if o == nil {
		return
	}
	o.visibilityEvaluator = evaluator
}

// BuildRequest describes renderer-free inputs required to build a FormModel.
type BuildRequest struct {
	// Source identifies where the schema document lives. Optional when Document
	// SchemaDocument, or RawJSONSchema is supplied.
	Source pkgopenapi.Source

	// Document allows callers to bypass the loader when they already have a
	// parsed OpenAPI payload.
	Document *pkgopenapi.Document

	// SchemaDocument allows callers to bypass the loader for non-OpenAPI sources.
	SchemaDocument *schema.Document

	// OperationID selects which form to render. OpenAPI adapters map this to an
	// operationId; JSON Schema adapters use derived form IDs.
	OperationID string

	// Format explicitly selects a registered adapter by name.
	Format string

	// NormalizeOptions supplies format-specific normalization hints.
	NormalizeOptions schema.NormalizeOptions

	// RawJSONSchema carries an in-memory JSON Schema document. When set, Format
	// defaults to the JSON Schema adapter and Source is used only as provenance.
	RawJSONSchema []byte

	// Subset restricts the returned model to fields whose group, tags, or
	// section match the supplied tokens. Empty subsets leave the model unchanged.
	Subset model.FieldSubset

	// VisibilityContext carries evaluator-specific inputs such as current form
	// values or feature flags used to decide whether a field belongs in the
	// returned model.
	VisibilityContext visibility.Context
}

// BuildOption customizes convenience BuildFormModel helpers.
type BuildOption func(*BuildRequest)

// WithBuildFormat sets the format adapter used by a convenience build helper.
func WithBuildFormat(format string) BuildOption {
	return func(req *BuildRequest) {
		req.Format = format
	}
}

// WithBuildNormalizeOptions sets adapter normalization options for a
// convenience build helper.
func WithBuildNormalizeOptions(options schema.NormalizeOptions) BuildOption {
	return func(req *BuildRequest) {
		req.NormalizeOptions = options
	}
}

// WithBuildSubset sets the renderer-free field subset for a convenience build
// helper.
func WithBuildSubset(subset model.FieldSubset) BuildOption {
	return func(req *BuildRequest) {
		req.Subset = subset
	}
}

// WithBuildVisibilityContext sets the visibility context for a convenience
// build helper.
func WithBuildVisibilityContext(ctx visibility.Context) BuildOption {
	return func(req *BuildRequest) {
		req.VisibilityContext = ctx
	}
}

// Request describes the inputs required to render a form.
type Request struct {
	// Source identifies where the schema document lives. Optional when Document
	// SchemaDocument, or RawJSONSchema is supplied.
	Source pkgopenapi.Source

	// Document allows callers to bypass the loader when they already have a
	// parsed OpenAPI payload.
	Document *pkgopenapi.Document

	// SchemaDocument allows callers to bypass the loader for non-OpenAPI sources.
	SchemaDocument *schema.Document

	// OperationID selects which form to render. OpenAPI adapters map this to an
	// operationId; JSON Schema adapters use derived form IDs.
	OperationID string

	// Format explicitly selects a registered adapter by name.
	Format string

	// NormalizeOptions supplies format-specific normalization hints.
	NormalizeOptions schema.NormalizeOptions

	// RawJSONSchema carries an in-memory JSON Schema document. When set, Format
	// defaults to the JSON Schema adapter and Source is used only as provenance.
	RawJSONSchema []byte

	// Subset restricts the model before rendering. RenderOptions.Subset remains
	// supported for compatibility; this field is preferred for headless parity.
	Subset model.FieldSubset

	// VisibilityContext carries evaluator-specific inputs used before rendering.
	// RenderOptions.VisibilityContext remains supported for compatibility.
	VisibilityContext visibility.Context

	// Renderer names the renderer to use. If empty, the orchestrator falls back
	// to the configured default renderer.
	Renderer string

	// ThemeName optionally selects a theme by name. When empty, the configured
	// ThemeSelector decides the default.
	ThemeName string

	// ThemeVariant optionally selects a variant (e.g., light/dark). When empty,
	// the ThemeSelector default is used.
	ThemeVariant string

	// RenderOptions carries per-request instructions such as method overrides,
	// prefilled values, or server-side errors that renderers can surface. When
	// omitted, renderers receive the zero-value struct.
	RenderOptions render.RenderOptions
}

// Generate executes the loader → parser → model builder → renderer sequence and
// returns the rendered bytes (HTML for the default vanilla renderer).
func (o *Orchestrator) Generate(ctx context.Context, req Request) ([]byte, error) {
	if err := o.validateGenerateRequest(ctx, req); err != nil {
		return nil, err
	}
	formModel, err := o.BuildFormModel(ctx, buildRequestFromRequest(req))
	if err != nil {
		return nil, err
	}
	renderOptions, err := o.resolveRenderOptions(ctx, req, formModel)
	if err != nil {
		return nil, err
	}
	renderer, err := o.rendererFor(req.Renderer)
	if err != nil {
		return nil, err
	}
	if renderOptions.TopPadding == 0 && renderer.Name() == "vanilla" {
		renderOptions.TopPadding = 5
	}
	output, err := renderer.Render(ctx, formModel, renderOptions)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: render output: %w", err)
	}
	return output, nil
}

// BuildFormModel executes the renderer-free schema loading, normalization, and
// model decoration pipeline.
func (o *Orchestrator) BuildFormModel(ctx context.Context, req BuildRequest) (model.FormModel, error) {
	if err := o.validateBuildRequest(ctx, req); err != nil {
		return model.FormModel{}, err
	}
	formModel, err := o.generateFormModel(ctx, req)
	if err != nil {
		return model.FormModel{}, err
	}
	o.applyEndpointOverrides(req.OperationID, &formModel)
	if pipelineErr := o.applyFormPipeline(ctx, &formModel, req); pipelineErr != nil {
		return model.FormModel{}, pipelineErr
	}
	return formModel, nil
}

// BuildFormModelFromJSONSchemaBytes builds a FormModel from raw JSON Schema
// bytes without requiring a file, URL, or renderer.
func (o *Orchestrator) BuildFormModelFromJSONSchemaBytes(ctx context.Context, raw []byte, operationID string, options ...BuildOption) (model.FormModel, error) {
	req := BuildRequest{
		RawJSONSchema: raw,
		OperationID:   operationID,
		Format:        pkgjsonschema.DefaultAdapterName,
		Source:        schema.SourceFromBytes("jsonschema:raw"),
	}
	for _, opt := range options {
		if opt != nil {
			opt(&req)
		}
	}
	if req.Format == "" {
		req.Format = pkgjsonschema.DefaultAdapterName
	}
	return o.BuildFormModel(ctx, req)
}

// BuildFormModelFromSchemaDocument builds a FormModel from an in-memory schema
// document without rendering.
func (o *Orchestrator) BuildFormModelFromSchemaDocument(ctx context.Context, doc schema.Document, operationID string, options ...BuildOption) (model.FormModel, error) {
	req := BuildRequest{
		SchemaDocument: &doc,
		OperationID:    operationID,
	}
	for _, opt := range options {
		if opt != nil {
			opt(&req)
		}
	}
	return o.BuildFormModel(ctx, req)
}

func (o *Orchestrator) validateGenerateRequest(ctx context.Context, req Request) error {
	return o.validateBuildRequest(ctx, buildRequestFromRequest(req))
}

func (o *Orchestrator) validateBuildRequest(ctx context.Context, req BuildRequest) error {
	if ctx == nil {
		return errors.New("orchestrator: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := o.initialiseErr; err != nil {
		return err
	}
	if !o.defaultsApplied {
		o.applyDefaults()
		if err := o.initialiseErr; err != nil {
			return err
		}
	}
	if req.OperationID == "" {
		return errors.New("orchestrator: operation id is required")
	}
	return nil
}

func buildRequestFromRequest(req Request) BuildRequest {
	build := BuildRequest{
		Source:            req.Source,
		Document:          req.Document,
		SchemaDocument:    req.SchemaDocument,
		OperationID:       req.OperationID,
		Format:            req.Format,
		NormalizeOptions:  req.NormalizeOptions,
		RawJSONSchema:     req.RawJSONSchema,
		Subset:            req.Subset,
		VisibilityContext: req.VisibilityContext,
	}
	if build.Format == "" && len(build.RawJSONSchema) > 0 {
		build.Format = pkgjsonschema.DefaultAdapterName
	}
	if build.Source == nil && len(build.RawJSONSchema) > 0 {
		build.Source = schema.SourceFromBytes("jsonschema:raw")
	}
	if emptySubset(build.Subset) {
		build.Subset = req.RenderOptions.Subset
	}
	if build.VisibilityContext.Values == nil && len(build.VisibilityContext.Extras) == 0 {
		build.VisibilityContext = visibilityContext(req.RenderOptions)
	}
	return build
}

func emptySubset(subset model.FieldSubset) bool {
	return len(subset.Groups) == 0 && len(subset.Tags) == 0 && len(subset.Sections) == 0
}

func (o *Orchestrator) generateFormModel(ctx context.Context, req BuildRequest) (model.FormModel, error) {
	adapter, err := o.resolveAdapter(ctx, req)
	if err != nil {
		return model.FormModel{}, err
	}
	doc, err := o.resolveSchemaDocument(ctx, req, adapter)
	if err != nil {
		return model.FormModel{}, err
	}
	ir, err := adapter.Normalize(ctx, doc, req.NormalizeOptions)
	if err != nil {
		return model.FormModel{}, fmt.Errorf("orchestrator: normalize schema: %w", err)
	}
	form, ok := ir.Form(req.OperationID)
	if !ok {
		return model.FormModel{}, o.formNotFoundError(ctx, adapter, ir, req.OperationID)
	}
	formModel, err := o.builder.Build(form)
	if err != nil {
		return model.FormModel{}, fmt.Errorf("orchestrator: build form model: %w", err)
	}
	return formModel, nil
}

func (o *Orchestrator) formNotFoundError(ctx context.Context, adapter schema.FormatAdapter, ir schema.SchemaIR, operationID string) error {
	available, err := adapter.Forms(ctx, ir)
	if err != nil {
		return fmt.Errorf("orchestrator: form %q not found (list forms: %w)", operationID, err)
	}
	return fmt.Errorf("orchestrator: form %q not found (available: %s)", operationID, formatFormRefs(available))
}

func (o *Orchestrator) applyFormPipeline(ctx context.Context, formModel *model.FormModel, req BuildRequest) error {
	if transformErr := o.applyTransformer(ctx, formModel); transformErr != nil {
		return transformErr
	}
	if decorateErr := o.applyDecorators(formModel); decorateErr != nil {
		return decorateErr
	}
	model.ApplySubset(formModel, req.Subset)
	if err := applyVisibility(formModel, o.visibilityEvaluator, req.VisibilityContext); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) resolveRenderOptions(ctx context.Context, req Request, formModel model.FormModel) (render.RenderOptions, error) {
	renderOptions := req.RenderOptions
	render.LocalizeFormModel(&formModel, renderOptions)
	mappedErrors := render.MapErrorPayload(formModel, renderOptions.Errors)
	renderOptions.Errors = mappedErrors.Fields
	renderOptions.FormErrors = render.MergeFormErrors(renderOptions.FormErrors, mappedErrors.Form...)
	for _, resolver := range o.renderOptionsResolvers {
		if resolver == nil {
			continue
		}
		resolved, err := resolver(ctx, req, formModel, renderOptions)
		if err != nil {
			return render.RenderOptions{}, err
		}
		renderOptions = resolved
	}
	return renderOptions, nil
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

	o.ensureCoreDefaults()
	o.ensureDefaultAdapters()
	o.ensureDecoratorDefaults()
	o.defaultsApplied = true
}

func (o *Orchestrator) ensureCoreDefaults() {
	if o.adapterRegistry == nil {
		o.adapterRegistry = NewAdapterRegistry()
	}
	if o.loader == nil {
		o.loader = internalLoader.New(pkgopenapi.NewLoaderOptions())
	}
	if o.jsonSchemaLoader == nil {
		o.jsonSchemaLoader = jsonschemaLoader.New(pkgjsonschema.NewLoaderOptions())
	}
	if o.parser == nil {
		o.parser = internalParser.New(pkgopenapi.NewParserOptions())
	}
	if o.builder == nil {
		o.builder = model.NewBuilder()
	}
	if o.defaultAdapter == "" {
		o.defaultAdapter = pkgopenapi.DefaultAdapterName
	}
	if o.widgetRegistry == nil {
		o.widgetRegistry = widgets.NewRegistry()
	}
}

func (o *Orchestrator) ensureDefaultAdapters() {
	if o.adapterRegistry != nil && !o.adapterRegistry.Has(pkgopenapi.DefaultAdapterName) {
		adapter := pkgopenapi.NewAdapter(o.loader, o.parser)
		if err := o.adapterRegistry.Register(adapter); err != nil {
			o.initialiseErr = appendInitialiseError(o.initialiseErr, err)
		}
	}
	if o.adapterRegistry != nil && !o.adapterRegistry.Has(pkgjsonschema.DefaultAdapterName) {
		adapter := pkgjsonschema.NewAdapter(o.jsonSchemaLoader, pkgjsonschema.WithResolverOptions(o.jsonSchemaResolveOptions))
		if err := o.adapterRegistry.Register(adapter); err != nil {
			o.initialiseErr = appendInitialiseError(o.initialiseErr, err)
		}
	}
}

func (o *Orchestrator) ensureDecoratorDefaults() {
	if o.widgetRegistry != nil {
		o.decorators = append([]model.Decorator{o.widgetRegistry}, o.decorators...)
	}

	o.ensureUIDecorator()
	o.ensureUIDecoratorOrder()
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

func (o *Orchestrator) ensureUIDecoratorOrder() {
	if len(o.decorators) < 2 {
		return
	}

	var overlays []model.Decorator
	var others []model.Decorator
	for _, decorator := range o.decorators {
		if _, ok := decorator.(*uischema.Decorator); ok {
			overlays = append(overlays, decorator)
			continue
		}
		others = append(others, decorator)
	}
	if len(overlays) == 0 {
		return
	}
	o.decorators = append(others, overlays...)
}
