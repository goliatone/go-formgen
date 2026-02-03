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
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/uischema"
	"github.com/goliatone/go-formgen/pkg/visibility"
	"github.com/goliatone/go-formgen/pkg/widgets"
	theme "github.com/goliatone/go-theme"
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

// WithThemeSelector injects a go-theme selector used to resolve theme/variant
// combinations into renderer-friendly configuration.
func WithThemeSelector(selector theme.ThemeSelector) Option {
	return func(o *Orchestrator) {
		o.themeSelector = selector
	}
}

// WithThemeProvider builds a go-theme selector from a ThemeProvider and configures
// the orchestrator to resolve renderer configuration (partials, tokens, assets)
// using the supplied defaults when theme inputs are omitted.
func WithThemeProvider(provider theme.ThemeProvider, defaultTheme, defaultVariant string) Option {
	return func(o *Orchestrator) {
		if provider == nil {
			return
		}
		o.themeSelector = theme.Selector{
			Registry:       provider,
			DefaultTheme:   strings.TrimSpace(defaultTheme),
			DefaultVariant: strings.TrimSpace(defaultVariant),
		}
	}
}

// WithThemeFallbacks supplies fallback partial paths passed to
// Selection.RendererTheme so renderers receive a resolved partial map even when
// the manifest omits a template key.
func WithThemeFallbacks(fallbacks map[string]string) Option {
	return func(o *Orchestrator) {
		if len(fallbacks) == 0 {
			return
		}
		o.themeFallbacks = cloneStringMap(fallbacks)
	}
}

func defaultThemeFallbacks() map[string]string {
	return map[string]string{
		"forms.form":          "templates/form.tmpl",
		"forms.input":         "templates/components/input.tmpl",
		"forms.select":        "templates/components/select.tmpl",
		"forms.checkbox":      "templates/components/boolean.tmpl",
		"forms.radio":         "templates/components/boolean.tmpl",
		"forms.textarea":      "templates/components/textarea.tmpl",
		"forms.wysiwyg":       "templates/components/wysiwyg.tmpl",
		"forms.json-editor":   "templates/components/json_editor.tmpl",
		"forms.file-uploader": "templates/components/file_uploader.tmpl",
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

// Orchestrator coordinates the full pipeline from OpenAPI document to rendered
// output. It applies sensible defaults (vanilla renderer, embedded templates)
// while remaining open to dependency injection for advanced callers.
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
	themeSelector            theme.ThemeSelector
	themeFallbacks           map[string]string
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

// Request describes the inputs required to render a form from an OpenAPI
// operation.
type Request struct {
	// Source identifies where the schema document lives. Optional when Document
	// or SchemaDocument is supplied.
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

	adapter, err := o.resolveAdapter(ctx, req)
	if err != nil {
		return nil, err
	}

	doc, err := o.resolveSchemaDocument(ctx, req, adapter)
	if err != nil {
		return nil, err
	}

	ir, err := adapter.Normalize(ctx, doc, req.NormalizeOptions)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: normalize schema: %w", err)
	}
	form, ok := ir.Form(req.OperationID)
	if !ok {
		available, listErr := adapter.Forms(ctx, ir)
		if listErr != nil {
			return nil, fmt.Errorf("orchestrator: form %q not found (list forms: %w)", req.OperationID, listErr)
		}
		return nil, fmt.Errorf("orchestrator: form %q not found (available: %s)", req.OperationID, formatFormRefs(available))
	}

	formModel, err := o.builder.Build(form)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: build form model: %w", err)
	}

	o.applyEndpointOverrides(req.OperationID, &formModel)
	if err := o.applyTransformer(ctx, &formModel); err != nil {
		return nil, err
	}
	if err := o.applyDecorators(&formModel); err != nil {
		return nil, err
	}
	render.ApplySubset(&formModel, req.RenderOptions.Subset)
	if err := applyVisibility(&formModel, o.visibilityEvaluator, visibilityContext(req.RenderOptions)); err != nil {
		return nil, err
	}

	renderOptions := req.RenderOptions
	render.LocalizeFormModel(&formModel, renderOptions)
	mappedErrors := render.MapErrorPayload(formModel, renderOptions.Errors)
	renderOptions.Errors = mappedErrors.Fields
	renderOptions.FormErrors = render.MergeFormErrors(renderOptions.FormErrors, mappedErrors.Form...)
	if renderOptions.Theme == nil {
		themeConfig, err := o.resolveTheme(req.ThemeName, req.ThemeVariant)
		if err != nil {
			return nil, err
		}
		renderOptions.Theme = themeConfig
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

func (o *Orchestrator) resolveTheme(themeName, variant string) (*theme.RendererConfig, error) {
	if o.themeSelector == nil {
		return nil, nil
	}
	selection, err := o.themeSelector.Select(themeName, variant)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: select theme: %w", err)
	}
	if selection == nil {
		return nil, fmt.Errorf("orchestrator: theme selector returned nil selection")
	}
	cfg := selection.RendererTheme(o.themeFallbacks)
	return &cfg, nil
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
	if o.defaultAdapter == "" {
		o.defaultAdapter = pkgopenapi.DefaultAdapterName
	}
	if o.widgetRegistry == nil {
		o.widgetRegistry = widgets.NewRegistry()
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

	if o.themeSelector != nil && len(o.themeFallbacks) == 0 {
		o.themeFallbacks = defaultThemeFallbacks()
	}

	if o.widgetRegistry != nil {
		o.decorators = append([]model.Decorator{o.widgetRegistry}, o.decorators...)
	}

	o.ensureUIDecorator()
	o.ensureUIDecoratorOrder()

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
