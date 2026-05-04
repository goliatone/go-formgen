package vanilla

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"maps"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	rendertemplate "github.com/goliatone/go-formgen/pkg/render/template"
	gotemplate "github.com/goliatone/go-formgen/pkg/render/template/gotemplate"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
	"github.com/goliatone/go-formgen/pkg/widgets"
	theme "github.com/goliatone/go-theme"
)

type Option func(*config)

type config struct {
	templateFS         fs.FS
	templateRenderer   rendertemplate.TemplateRenderer
	templateFuncs      map[string]any
	inlineStyles       string
	stylesheets        []string
	componentRegistry  *components.Registry
	componentOverrides map[string]string
}

// WithTemplatesFS supplies an alternate template bundle via fs.FS.
func WithTemplatesFS(files fs.FS) Option {
	return func(cfg *config) {
		cfg.templateFS = files
	}
}

// WithTemplatesDir loads templates from a directory on disk.
func WithTemplatesDir(path string) Option {
	return func(cfg *config) {
		if path == "" {
			return
		}
		cfg.templateFS = os.DirFS(path)
	}
}

// WithTemplateRenderer injects a custom template renderer implementation.
func WithTemplateRenderer(renderer rendertemplate.TemplateRenderer) Option {
	return func(cfg *config) {
		if renderer != nil {
			cfg.templateRenderer = renderer
		}
	}
}

// WithTemplateFuncs registers template helper functions on the built-in
// go-template engine. It is a generic injection point for helpers such as i18n
// translation, formatting, or other UI utilities.
func WithTemplateFuncs(funcs map[string]any) Option {
	return func(cfg *config) {
		if len(funcs) == 0 {
			return
		}
		if cfg.templateFuncs == nil {
			cfg.templateFuncs = make(map[string]any, len(funcs))
		}
		for name, fn := range funcs {
			if strings.TrimSpace(name) == "" || fn == nil {
				continue
			}
			cfg.templateFuncs[name] = fn
		}
	}
}

// WithDefaultStyles injects the bundled CSS into the rendered form so the
// output looks polished during development. Downstream consumers can skip this
// option or call WithoutStyles for unstyled markup.
func WithDefaultStyles() Option {
	return func(cfg *config) {
		cfg.inlineStyles = strings.TrimSpace(defaultStylesheet())
	}
}

// WithInlineStyles allows callers to provide custom inline CSS that will be
// emitted in a <style> block above the rendered form.
func WithInlineStyles(css string) Option {
	return func(cfg *config) {
		if trimmed := strings.TrimSpace(css); trimmed != "" {
			cfg.inlineStyles = trimmed
		}
	}
}

// WithStylesheet appends a <link rel="stylesheet"> tag that references the
// provided path.
func WithStylesheet(path string) Option {
	return func(cfg *config) {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			cfg.stylesheets = append(cfg.stylesheets, trimmed)
		}
	}
}

// WithoutStyles disables any inline styles or external stylesheets that have
// been configured so far.
func WithoutStyles() Option {
	return func(cfg *config) {
		cfg.inlineStyles = ""
		cfg.stylesheets = nil
	}
}

// WithComponentRegistry injects a custom component registry used to resolve
// field renderers. When nil, the renderer falls back to the default registry.
func WithComponentRegistry(reg *components.Registry) Option {
	return func(cfg *config) {
		if reg != nil {
			cfg.componentRegistry = reg
		}
	}
}

// WithComponentOverrides specifies component names for particular fields using
// either simple field names or dot-paths for nested fields. Component names
// should use the canonical constants in the components package.
func WithComponentOverrides(overrides map[string]string) Option {
	return func(cfg *config) {
		if len(overrides) == 0 {
			return
		}
		if cfg.componentOverrides == nil {
			cfg.componentOverrides = make(map[string]string, len(overrides))
		}
		for key, value := range overrides {
			cfg.componentOverrides[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
}

// WithComponentOverride assigns a component name to a specific field path.
func WithComponentOverride(path, component string) Option {
	return WithComponentOverrides(map[string]string{
		path: component,
	})
}

type Renderer struct {
	templates   rendertemplate.TemplateRenderer
	inlineStyle string
	stylesheets []string
	components  *components.Registry
	overrides   map[string]string
}

type templateRenderOptions struct {
	MethodAttr     string
	MethodOverride string
	FormErrors     []string
	HiddenFields   []render.HiddenField
}

type rendererTheme struct {
	Name         string            `json:"name"`
	Variant      string            `json:"variant"`
	Partials     map[string]string `json:"partials,omitempty"`
	Tokens       map[string]string `json:"tokens,omitempty"`
	CSSVars      map[string]string `json:"cssVars,omitempty"`
	CSSVarsStyle string            `json:"css_vars_style,omitempty"`
	JSON         string            `json:"json,omitempty"`
}

const (
	dataAttributesMetadataKey  = "__data_attrs"
	layoutSectionsMetadataKey  = "layout.sections"
	layoutSectionFieldKey      = "layout.section"
	layoutFieldOrderPrefix     = "layout.fieldOrder."
	layoutActionsMetadataKey   = "actions"
	layoutGridColumnsHintKey   = "layout.gridColumns"
	layoutGutterHintKey        = "layout.gutter"
	fieldLayoutSpanHintKey     = "layout.span"
	fieldLayoutStartHintKey    = "layout.start"
	fieldLayoutRowHintKey      = "layout.row"
	componentNameMetadataKey   = "component.name"
	componentConfigMetadataKey = "component.config"
	componentChromeMetadataKey = "component.chrome"
	controlIDMetadataKey       = "control.id"
	behaviorNamesMetadataKey   = "behavior.names"
	behaviorConfigMetadataKey  = "behavior.config"
	defaultGridColumns         = 12
)

type layoutContext struct {
	GridColumns       int             `json:"gridColumns"`
	GridColumnsValue  string          `json:"gridColumnsValue"`
	Gutter            string          `json:"gutter"`
	HasResponsiveGrid bool            `json:"hasResponsiveGrid,omitempty"`
	Sections          []sectionGroup  `json:"sections"`
	Unsectioned       []renderedField `json:"unsectioned"`
}

type sectionGroup struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	TitleKey       string          `json:"titleKey,omitempty"`
	Description    string          `json:"description"`
	DescriptionKey string          `json:"descriptionKey,omitempty"`
	Fieldset       bool            `json:"fieldset"`
	Fields         []renderedField `json:"fields"`
}

type renderedField struct {
	HTML  string `json:"html"`
	Style string `json:"style"`
}

type renderedSectionField struct {
	path     string
	field    renderedField
	fallback int
}

type sectionedField struct {
	field     model.Field
	path      string
	sectionID string
}

type sectionMeta struct {
	ID             string            `json:"id"`
	Title          string            `json:"title"`
	TitleKey       string            `json:"titleKey,omitempty"`
	Description    string            `json:"description"`
	DescriptionKey string            `json:"descriptionKey,omitempty"`
	Order          int               `json:"order"`
	Fieldset       bool              `json:"fieldset"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	UIHints        map[string]string `json:"uiHints,omitempty"`
}

type actionButton struct {
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	LabelKey string `json:"labelKey,omitempty"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Icon     string `json:"icon,omitempty"`
}

// New constructs the vanilla renderer applying any provided options.
func New(options ...Option) (*Renderer, error) {
	cfg := config{templateFS: TemplatesFS()}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}

	if cfg.templateFS == nil {
		cfg.templateFS = TemplatesFS()
	}

	renderer := cfg.templateRenderer
	if renderer == nil {
		var templateFuncs map[string]any
		if len(cfg.templateFuncs) > 0 {
			templateFuncs = make(map[string]any, len(cfg.templateFuncs))
			maps.Copy(templateFuncs, cfg.templateFuncs)
		}

		options := []gotemplate.Option{
			gotemplate.WithFS(cfg.templateFS),
			gotemplate.WithExtension(".tmpl"),
		}
		if len(templateFuncs) > 0 {
			options = append(options, gotemplate.WithTemplateFunc(templateFuncs))
		}

		engine, err := gotemplate.New(
			options...,
		)
		if err != nil {
			return nil, fmt.Errorf("vanilla renderer: configure template renderer: %w", err)
		}
		renderer = engine
	}

	registry := cfg.componentRegistry
	if registry == nil {
		registry = components.NewDefaultRegistry()
	} else {
		registry = registry.Clone()
	}

	return &Renderer{
		templates:   renderer,
		inlineStyle: cfg.inlineStyles,
		stylesheets: append([]string(nil), cfg.stylesheets...),
		components:  registry,
		overrides:   cloneStringMap(cfg.componentOverrides),
	}, nil
}

func (r *Renderer) Name() string {
	return "vanilla"
}

func (r *Renderer) ContentType() string {
	return "text/html; charset=utf-8"
}

func (r *Renderer) Render(_ context.Context, form model.FormModel, renderOptions render.RenderOptions) ([]byte, error) {
	if r.templates == nil {
		return nil, fmt.Errorf("vanilla renderer: template renderer is nil")
	}

	render.ApplySubset(&form, renderOptions.Subset)
	render.LocalizeFormModel(&form, renderOptions)

	topPadding := renderOptions.TopPadding
	if topPadding == 0 {
		topPadding = 3
	}

	templateOptions := prepareRenderContext(&form, renderOptions)
	decorated := decorateFormModel(form)
	themeCtx := buildThemeContext(renderOptions.Theme)
	assetResolver := themeAssetResolver(renderOptions.Theme)

	componentRenderer := newComponentRenderer(r.templates, r.components, r.overrides, themeCtx, assetResolver)
	layout, err := buildLayoutContext(decorated, componentRenderer)
	if err != nil {
		return nil, fmt.Errorf("vanilla renderer: build layout: %w", err)
	}
	actions := parseActions(decorated.Metadata)
	assets := r.renderAssets(componentRenderer, renderOptions, layout, assetResolver)
	formTemplateName := formTemplateName(renderOptions.Theme)
	chromeClasses := chromeClassMap(renderOptions.ChromeClasses)

	result, err := r.templates.RenderTemplate(formTemplateName, map[string]any{
		"locale":                 renderOptions.Locale,
		"form":                   decorated,
		"layout":                 layout,
		"actions":                actions,
		"stylesheets":            assets.stylesheets,
		"inline_styles":          assets.inlineStyles,
		"responsive_grid_styles": assets.responsiveGridStyles,
		"component_scripts":      assets.componentScripts,
		"theme":                  assets.templateTheme,
		"top_padding":            strings.Repeat("\n", topPadding),
		"default_form_class":     DefaultFormClass,
		"default_header_class":   DefaultHeaderClass,
		"default_section_class":  DefaultSectionClass,
		"default_fieldset_class": DefaultFieldsetClass,
		"default_actions_class":  DefaultActionsClass,
		"default_errors_class":   DefaultErrorsClass,
		"default_grid_class":     DefaultGridClass,
		"render_options": map[string]any{
			"method_attr":     templateOptions.MethodAttr,
			"method_override": templateOptions.MethodOverride,
			"form_errors":     templateOptions.FormErrors,
			"hidden_fields":   templateOptions.HiddenFields,
			"locale":          renderOptions.Locale,
			"chrome_classes":  chromeClasses,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vanilla renderer: render template: %w", err)
	}
	if (renderOptions.Theme == nil || strings.TrimSpace(renderOptions.Theme.Theme) == "") && strings.Contains(result, "<form") {
		result += "\n\n"
	}
	return []byte(result), nil
}

type renderAssetBundle struct {
	stylesheets          []string
	inlineStyles         string
	responsiveGridStyles string
	componentScripts     []map[string]any
	templateTheme        map[string]any
}

func (r *Renderer) renderAssets(componentRenderer *componentRenderer, renderOptions render.RenderOptions, layout layoutContext, assetResolver func(string) string) renderAssetBundle {
	componentStyles, componentScripts := componentRenderer.assets()
	stylesheets := append([]string(nil), r.stylesheets...)
	stylesheets = append(stylesheets, componentStyles...)
	for idx := range componentScripts {
		componentScripts[idx] = resolveScriptAsset(componentScripts[idx], assetResolver)
	}
	assets := renderAssetBundle{
		stylesheets:      resolveAssets(stylesheets, assetResolver),
		inlineStyles:     r.inlineStyle,
		componentScripts: scriptPayloads(componentScripts),
		templateTheme:    buildTemplateThemeContext(buildThemeContext(renderOptions.Theme), assetResolver),
	}
	if layout.HasResponsiveGrid {
		assets.responsiveGridStyles = strings.TrimSpace(responsiveGridCSS)
	}
	if renderOptions.OmitAssets {
		return renderAssetBundle{}
	}
	return assets
}

func formTemplateName(cfg *theme.RendererConfig) string {
	if cfg != nil {
		if candidate := strings.TrimSpace(cfg.Partials["forms.form"]); candidate != "" {
			return candidate
		}
	}
	return "templates/form.tmpl"
}

func chromeClassMap(classes *render.ChromeClasses) map[string]string {
	chromeClasses := map[string]string{}
	if classes == nil {
		return chromeClasses
	}
	chromeClasses["form"] = strings.TrimSpace(classes.Form)
	chromeClasses["header"] = strings.TrimSpace(classes.Header)
	chromeClasses["section"] = strings.TrimSpace(classes.Section)
	chromeClasses["fieldset"] = strings.TrimSpace(classes.Fieldset)
	chromeClasses["actions"] = strings.TrimSpace(classes.Actions)
	chromeClasses["errors"] = strings.TrimSpace(classes.Errors)
	chromeClasses["grid"] = strings.TrimSpace(classes.Grid)
	return chromeClasses
}

func buildThemeContext(cfg *theme.RendererConfig) rendererTheme {
	if cfg == nil {
		return rendererTheme{}
	}
	ctx := rendererTheme{
		Name:     cfg.Theme,
		Variant:  cfg.Variant,
		Partials: copyStringMap(cfg.Partials),
		Tokens:   copyStringMap(cfg.Tokens),
		CSSVars:  copyStringMap(cfg.CSSVars),
	}
	ctx.CSSVarsStyle = cssVarsStyle(ctx.CSSVars)
	ctx.JSON = themeJSON(ctx)
	return ctx
}

func buildTemplateThemeContext(ctx rendererTheme, resolver func(string) string) map[string]any {
	return map[string]any{
		"name":           ctx.Name,
		"variant":        ctx.Variant,
		"partials":       ctx.Partials,
		"tokens":         ctx.Tokens,
		"cssVars":        ctx.CSSVars,
		"css_vars_style": ctx.CSSVarsStyle,
		"json":           ctx.JSON,
		"assetURL": func(key any) string {
			trimmed := strings.TrimSpace(anyToString(key))
			if trimmed == "" {
				return ""
			}
			if resolver == nil {
				return ""
			}
			if isAbsoluteAsset(trimmed) {
				return trimmed
			}
			if resolved := strings.TrimSpace(resolver(trimmed)); resolved != "" {
				return resolved
			}
			return trimmed
		},
	}
}

func themeAssetResolver(cfg *theme.RendererConfig) func(string) string {
	if cfg == nil {
		return nil
	}
	return cfg.AssetURL
}

func anyToString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(value)
	}
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func cssVarsStyle(vars map[string]string) string {
	if len(vars) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(":root {\n")
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(vars[key])
		b.WriteString(";\n")
	}
	b.WriteString("}")
	return b.String()
}

func themeJSON(cfg rendererTheme) string {
	payload := struct {
		Name    string            `json:"name,omitempty"`
		Variant string            `json:"variant,omitempty"`
		Tokens  map[string]string `json:"tokens,omitempty"`
		CSSVars map[string]string `json:"cssVars,omitempty"`
	}{
		Name:    cfg.Name,
		Variant: cfg.Variant,
		Tokens:  cfg.Tokens,
		CSSVars: cfg.CSSVars,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func resolveAssets(paths []string, resolver func(string) string) []string {
	if resolver == nil || len(paths) == 0 {
		return paths
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		if isAbsoluteAsset(trimmed) {
			out = append(out, trimmed)
			continue
		}
		resolved := strings.TrimSpace(resolver(trimmed))
		if resolved == "" {
			resolved = trimmed
		}
		out = append(out, resolved)
	}
	return out
}

func resolveScriptAsset(script components.Script, resolver func(string) string) components.Script {
	if resolver == nil {
		return script
	}
	if strings.TrimSpace(script.Src) == "" || isAbsoluteAsset(script.Src) {
		return script
	}
	if resolved := strings.TrimSpace(resolver(script.Src)); resolved != "" {
		script.Src = resolved
	}
	return script
}

func isAbsoluteAsset(path string) bool {
	trimmed := strings.TrimSpace(path)
	return strings.HasPrefix(trimmed, "/") ||
		strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "//")
}

func scriptPayloads(scripts []components.Script) []map[string]any {
	if len(scripts) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(scripts))
	for _, script := range scripts {
		out = append(out, map[string]any{
			"src":    script.Src,
			"type":   script.Type,
			"inline": script.Inline,
			"async":  script.Async,
			"defer":  script.Defer,
			"module": script.Module,
			"attrs":  script.Attrs,
		})
	}
	return out
}

func prepareRenderContext(form *model.FormModel, options render.RenderOptions) templateRenderOptions {
	ctx := templateRenderOptions{
		MethodAttr:     "post",
		MethodOverride: "",
	}
	if form == nil {
		ctx.FormErrors = render.MergeFormErrors(options.FormErrors)
		ctx.HiddenFields = render.SortedHiddenFields(options.HiddenFields)
		return ctx
	}

	applyMethodOverride(form, &ctx, options.Method)
	applyPrefillValues(form, options.Values)

	mapped := render.MapErrorPayload(*form, options.Errors)
	applyServerErrors(form, mapped.Fields)
	ctx.FormErrors = render.MergeFormErrors(options.FormErrors, mapped.Form...)
	ctx.HiddenFields = render.SortedHiddenFields(options.HiddenFields)

	return ctx
}

func applyMethodOverride(form *model.FormModel, ctx *templateRenderOptions, override string) {
	target := strings.TrimSpace(override)
	if target == "" && form != nil {
		target = strings.TrimSpace(form.Method)
	}
	if target == "" {
		target = "POST"
	}

	canonical := strings.ToUpper(target)
	methodAttr := strings.ToLower(canonical)
	methodOverride := ""

	if canonical != "GET" && canonical != "POST" {
		methodAttr = "post"
		methodOverride = canonical
	}

	if ctx != nil {
		ctx.MethodAttr = methodAttr
		ctx.MethodOverride = methodOverride
	}
	if form != nil {
		form.Method = canonical
	}
}

func applyPrefillValues(form *model.FormModel, values map[string]any) {
	if form == nil || len(values) == 0 {
		return
	}

	flattened := flattenPrefillValues(values)
	if len(flattened) == 0 {
		return
	}

	form.Fields = applyValuesToFields(form.Fields, flattened, "")
}

type prefillValue struct {
	value      any
	provenance string
	readonly   bool
	disabled   bool
}

func flattenPrefillValues(values map[string]any) map[string]prefillValue {
	flattener := prefillFlattener{result: make(map[string]prefillValue)}
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		flattener.walk(key, value, prefillValue{})
	}
	return flattener.result
}

type prefillFlattener struct {
	result map[string]prefillValue
}

func (f prefillFlattener) walk(prefix string, value any, meta prefillValue) {
	switch typed := value.(type) {
	case map[string]any:
		f.walkAnyMap(prefix, typed, meta)
	case map[string]string:
		f.walkStringMap(prefix, typed, meta)
	case render.ValueWithProvenance:
		f.walk(prefix, typed.Value, prefillMetaFromValue(typed, meta))
	case *render.ValueWithProvenance:
		f.walk(prefix, typed.Value, prefillMetaFromValue(*typed, meta))
	default:
		if prefix != "" {
			meta.value = typed
			f.result[prefix] = meta
		}
	}
}

func (f prefillFlattener) walkAnyMap(prefix string, values map[string]any, meta prefillValue) {
	for key, val := range values {
		if key = strings.TrimSpace(key); key != "" {
			f.walk(joinPath(prefix, key), val, meta)
		}
	}
	if prefix != "" && (meta.provenance != "" || meta.readonly || meta.disabled) {
		if _, exists := f.result[prefix]; !exists {
			f.result[prefix] = meta
		}
	}
}

func (f prefillFlattener) walkStringMap(prefix string, values map[string]string, meta prefillValue) {
	for key, val := range values {
		if key = strings.TrimSpace(key); key != "" {
			next := joinPath(prefix, key)
			f.result[next] = prefillValue{value: val, provenance: meta.provenance, readonly: meta.readonly, disabled: meta.disabled}
		}
	}
}

func prefillMetaFromValue(value render.ValueWithProvenance, meta prefillValue) prefillValue {
	meta.provenance = value.Provenance
	meta.readonly = value.Readonly
	meta.disabled = value.Disabled
	return meta
}

func applyValuesToFields(fields []model.Field, values map[string]prefillValue, parentPath string) []model.Field {
	if len(fields) == 0 {
		return fields
	}

	for i := range fields {
		path := joinPath(parentPath, fields[i].Name)
		if value, ok := values[path]; ok {
			if value.value != nil {
				assignFieldValue(&fields[i], value.value)
			}
			applyValueProvenance(&fields[i], value)
		}
		if len(fields[i].Nested) > 0 {
			fields[i].Nested = applyValuesToFields(fields[i].Nested, values, path)
		}
	}

	return fields
}

func applyValueProvenance(field *model.Field, value prefillValue) {
	if field == nil {
		return
	}

	if value.provenance != "" {
		if field.Metadata == nil {
			field.Metadata = make(map[string]string, 2)
		}
		field.Metadata["prefill.provenance"] = value.provenance
	}
	if value.readonly {
		if field.Metadata == nil {
			field.Metadata = make(map[string]string, 1)
		}
		field.Metadata["readonly"] = "true"
		field.Metadata["prefill.readonly"] = "true"
		if field.UIHints == nil {
			field.UIHints = make(map[string]string, 1)
		}
		field.UIHints["readonly"] = "true"
		field.Readonly = true
	}
	if value.disabled {
		if field.Metadata == nil {
			field.Metadata = make(map[string]string, 1)
		}
		field.Metadata["disabled"] = "true"
		field.Metadata["prefill.disabled"] = "true"
		field.Disabled = true
	}
}

func assignFieldValue(field *model.Field, value any) {
	if field == nil || value == nil {
		return
	}

	switch {
	case field.Relationship != nil:
		applyRelationshipCurrent(field, value)
		return
	case field.Type == model.FieldTypeArray:
		if applyArrayValue(field, value) {
			return
		}
	case field.Type == model.FieldTypeBoolean:
		if boolValue, ok := toBool(value); ok {
			field.Default = boolValue
		}
	case len(field.Enum) > 0 || isScalarFieldType(field.Type):
		if scalar, ok := stringifyScalar(value); ok {
			field.Default = scalar
		}
	default:
		if scalar, ok := stringifyScalar(value); ok {
			field.Default = scalar
		}
	}
}

func isScalarFieldType(fieldType model.FieldType) bool {
	return fieldType == model.FieldTypeString || fieldType == model.FieldTypeInteger || fieldType == model.FieldTypeNumber
}

func applyArrayValue(field *model.Field, value any) bool {
	values, ok := toAnySlice(value)
	if !ok {
		field.Default = []any{value}
		return true
	}
	if len(values) == 0 {
		field.Default = []any{}
		return true
	}
	if field != nil && field.Items != nil {
		switch field.Items.Type {
		case model.FieldTypeObject, model.FieldTypeArray:
			field.Default = values
			return true
		}
		if len(field.Items.Nested) > 0 {
			field.Default = values
			return true
		}
	}
	field.Default = normalizeArrayValues(field, values)
	return true
}

func normalizeArrayValues(field *model.Field, values []any) []any {
	if len(values) == 0 {
		return nil
	}
	itemType := model.FieldTypeString
	if field != nil && field.Items != nil {
		itemType = field.Items.Type
	}
	out := make([]any, 0, len(values))
	for _, value := range values {
		switch itemType {
		case model.FieldTypeString:
			if str, ok := stringifyScalar(value); ok {
				out = append(out, str)
			}
		case model.FieldTypeInteger, model.FieldTypeNumber:
			if num, ok := toFloat(value); ok {
				out = append(out, num)
			}
		case model.FieldTypeBoolean:
			if b, ok := toBool(value); ok {
				out = append(out, b)
			}
		default:
			out = append(out, value)
		}
	}
	return out
}

func toAnySlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	length := rv.Len()
	out := make([]any, length)
	for i := range length {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}

func applyRelationshipCurrent(field *model.Field, value any) {
	payload := relationshipCurrentPayload(value)
	if payload == "" {
		return
	}
	if field.Metadata == nil {
		field.Metadata = make(map[string]string, 1)
	}
	field.Metadata["relationship.current"] = payload
}

func relationshipCurrentPayload(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case fmt.Stringer:
		return typed.String()
	case []string:
		return marshalStringSlice(typed)
	case []any:
		coll := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := stringifyScalar(item); ok && str != "" {
				coll = append(coll, str)
				continue
			}
			if str := extractRelationshipValue(item); str != "" {
				coll = append(coll, str)
			}
		}
		return marshalStringSlice(coll)
	case map[string]any:
		return extractRelationshipValue(typed)
	case map[string]string:
		return extractRelationshipValue(typed)
	default:
		if str, ok := stringifyScalar(value); ok {
			return str
		}
	}
	return ""
}

func extractRelationshipValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"value", "id", "slug"} {
			if raw, ok := typed[key]; ok {
				if str, ok := stringifyScalar(raw); ok {
					return str
				}
			}
		}
	case map[string]string:
		for _, key := range []string{"value", "id", "slug"} {
			if raw, ok := typed[key]; ok && raw != "" {
				return raw
			}
		}
	}
	return ""
}

func marshalStringSlice(values []string) string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		clean = append(clean, value)
	}
	if len(clean) == 0 {
		return ""
	}
	payload, err := json.Marshal(clean)
	if err != nil {
		return ""
	}
	return string(payload)
}

func stringifyScalar(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		return typed, true
	case json.Number:
		return typed.String(), true
	case fmt.Stringer:
		return typed.String(), true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	}
	if scalar, ok := stringifyReflectScalar(value); ok {
		return scalar, true
	}
	return fmt.Sprintf("%v", value), true
}

func stringifyReflectScalar(value any) (string, bool) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return "", false
	}
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflected.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(reflected.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(reflected.Float(), 'f', -1, 64), true
	default:
		return "", false
	}
}

func toBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case uint:
		return typed != 0, true
	case uint64:
		return typed != 0, true
	}
	return false, false
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func applyServerErrors(form *model.FormModel, errors map[string][]string) {
	if form == nil || len(errors) == 0 {
		return
	}

	trimmed := make(map[string][]string, len(errors))
	for key, values := range errors {
		key = strings.TrimSpace(key)
		if key == "" || len(values) == 0 {
			continue
		}
		filtered := make([]string, 0, len(values))
		for _, message := range values {
			message = strings.TrimSpace(message)
			if message != "" {
				filtered = append(filtered, message)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		trimmed[key] = filtered
	}
	if len(trimmed) == 0 {
		return
	}

	form.Fields = applyErrorsToFields(form.Fields, trimmed, "")
}

func applyErrorsToFields(fields []model.Field, errors map[string][]string, parentPath string) []model.Field {
	if len(fields) == 0 {
		return fields
	}

	for i := range fields {
		path := joinPath(parentPath, fields[i].Name)
		if messages, ok := errors[path]; ok {
			setFieldError(&fields[i], messages)
		}
		if len(fields[i].Nested) > 0 {
			fields[i].Nested = applyErrorsToFields(fields[i].Nested, errors, path)
		}
		if fields[i].Items != nil {
			item := applyErrorsToItem(*fields[i].Items, errors, path)
			fields[i].Items = &item
		}
	}
	return fields
}

func applyErrorsToItem(item model.Field, errors map[string][]string, parentPath string) model.Field {
	if name := strings.TrimSpace(item.Name); name != "" {
		itemPath := joinPath(parentPath, name)
		if messages, ok := errors[itemPath]; ok {
			setFieldError(&item, messages)
		}
	}
	if len(item.Nested) > 0 {
		item.Nested = applyErrorsToFields(item.Nested, errors, parentPath)
	}
	if item.Items != nil {
		nested := applyErrorsToItem(*item.Items, errors, parentPath)
		item.Items = &nested
	}
	return item
}

func setFieldError(field *model.Field, messages []string) {
	if field == nil || len(messages) == 0 {
		return
	}
	if field.Metadata == nil {
		field.Metadata = make(map[string]string, 2)
	}
	field.Metadata["validation.state"] = "invalid"
	field.Metadata["validation.message"] = strings.Join(messages, "; ")
}

func decorateFormModel(form model.FormModel) model.FormModel {
	if len(form.Fields) == 0 {
		return form
	}
	form.Fields = decorateFields(form.Fields)
	return form
}

func buildLayoutContext(form model.FormModel, renderer *componentRenderer) (layoutContext, error) {
	columns := gridColumnsFromHints(form.UIHints)
	ctx := layoutContext{
		GridColumns:      columns,
		GridColumnsValue: strconv.Itoa(columns),
		Gutter:           stringFromMap(form.UIHints, layoutGutterHintKey),
	}

	metas := parseSectionsMetadata(stringFromMap(form.Metadata, layoutSectionsMetadataKey))
	if len(metas) == 0 {
		fields, responsive, err := renderUnsectionedFields(form.Fields, renderer, ctx.GridColumns)
		if err != nil {
			return layoutContext{}, err
		}
		ctx.Unsectioned = fields
		ctx.HasResponsiveGrid = responsive
		return ctx, nil
	}

	index := initialiseSections(&ctx, metas)

	sectionOrders := parseFieldOrderMetadata(form.Metadata)
	sectionOutputs := make(map[string][]renderedSectionField, len(index))
	fallbackCounter := 0

	sectioned := collectSectionedFields(form.Fields, "")
	if len(sectioned) > 0 {
		responsive, err := collectSectionedOutputs(sectioned, renderer, ctx.GridColumns, index, sectionOutputs, &fallbackCounter, &ctx)
		if err != nil {
			return layoutContext{}, err
		}
		ctx.HasResponsiveGrid = ctx.HasResponsiveGrid || responsive
	} else {
		responsive, err := collectTopLevelSectionOutputs(form.Fields, renderer, ctx.GridColumns, index, sectionOutputs, &fallbackCounter, &ctx)
		if err != nil {
			return layoutContext{}, err
		}
		ctx.HasResponsiveGrid = ctx.HasResponsiveGrid || responsive
	}

	for id, group := range index {
		order := sectionOrders[id]
		group.Fields = orderRenderedFields(sectionOutputs[id], order)
	}

	return ctx, nil
}

func renderUnsectionedFields(fields []model.Field, renderer *componentRenderer, columns int) ([]renderedField, bool, error) {
	var out []renderedField
	responsiveGrid := false
	for _, field := range fields {
		item, responsive, ok, err := renderLayoutField(renderer, field, field.Name, columns)
		if err != nil {
			return nil, false, err
		}
		if ok {
			responsiveGrid = responsiveGrid || responsive
			out = append(out, item)
		}
	}
	return out, responsiveGrid, nil
}

func initialiseSections(ctx *layoutContext, metas []sectionMeta) map[string]*sectionGroup {
	ctx.Sections = make([]sectionGroup, len(metas))
	index := make(map[string]*sectionGroup, len(metas))
	for i, meta := range metas {
		ctx.Sections[i] = sectionGroup{
			ID:             meta.ID,
			Title:          meta.Title,
			TitleKey:       meta.TitleKey,
			Description:    meta.Description,
			DescriptionKey: meta.DescriptionKey,
			Fieldset:       meta.Fieldset,
		}
		index[meta.ID] = &ctx.Sections[i]
	}
	return index
}

func collectSectionedOutputs(sectioned []sectionedField, renderer *componentRenderer, columns int, index map[string]*sectionGroup, outputs map[string][]renderedSectionField, fallbackCounter *int, ctx *layoutContext) (bool, error) {
	responsiveGrid := false
	for _, sf := range sectioned {
		item, responsive, ok, err := renderLayoutField(renderer, sf.field, sf.path, columns)
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		responsiveGrid = responsiveGrid || responsive
		if _, ok := index[sf.sectionID]; ok {
			*fallbackCounter = *fallbackCounter + 1
			outputs[sf.sectionID] = append(outputs[sf.sectionID], renderedSectionField{path: sf.path, field: item, fallback: *fallbackCounter})
			continue
		}
		ctx.Unsectioned = append(ctx.Unsectioned, item)
	}
	return responsiveGrid, nil
}

func collectTopLevelSectionOutputs(fields []model.Field, renderer *componentRenderer, columns int, index map[string]*sectionGroup, outputs map[string][]renderedSectionField, fallbackCounter *int, ctx *layoutContext) (bool, error) {
	responsiveGrid := false
	for _, field := range fields {
		item, responsive, ok, err := renderLayoutField(renderer, field, field.Name, columns)
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		responsiveGrid = responsiveGrid || responsive
		if sectionID := stringFromMap(field.Metadata, layoutSectionFieldKey); sectionID != "" {
			if _, ok := index[sectionID]; ok {
				*fallbackCounter = *fallbackCounter + 1
				outputs[sectionID] = append(outputs[sectionID], renderedSectionField{path: field.Name, field: item, fallback: *fallbackCounter})
				continue
			}
		}
		ctx.Unsectioned = append(ctx.Unsectioned, item)
	}
	return responsiveGrid, nil
}

func renderLayoutField(renderer *componentRenderer, field model.Field, path string, columns int) (renderedField, bool, bool, error) {
	rendered, err := renderer.render(field, path)
	if err != nil {
		return renderedField{}, false, false, err
	}
	if strings.TrimSpace(rendered) == "" {
		return renderedField{}, false, false, nil
	}
	attrs, responsive := gridWrapperAttributes(field, columns)
	return renderedField{HTML: rendered, Style: attrs}, responsive, true, nil
}

func collectSectionedFields(fields []model.Field, parentPath string) []sectionedField {
	return collectSectionedFieldsInternal(fields, parentPath, parentPath != "")
}

func collectSectionedFieldsInternal(fields []model.Field, parentPath string, nested bool) []sectionedField {
	var result []sectionedField
	for _, field := range fields {
		path := field.Name
		if parentPath != "" {
			path = parentPath + "." + field.Name
		}

		// Only collect layout entries for top-level fields. Nested fields render
		// within their parent components (objects/arrays) to avoid duplication.
		if !nested {
			if sectionID := stringFromMap(field.Metadata, layoutSectionFieldKey); sectionID != "" {
				result = append(result, sectionedField{
					field:     field,
					path:      path,
					sectionID: sectionID,
				})
			}
		}

		if len(field.Nested) > 0 {
			child := collectSectionedFieldsInternal(field.Nested, path, true)
			if len(child) > 0 {
				result = append(result, child...)
			}
		}

		if field.Items != nil && len(field.Items.Nested) > 0 {
			itemPath := path + ".items"
			child := collectSectionedFieldsInternal(field.Items.Nested, itemPath, true)
			if len(child) > 0 {
				result = append(result, child...)
			}
		}
	}
	return result
}

func parseFieldOrderMetadata(metadata map[string]string) map[string][]string {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string][]string)
	for key, raw := range metadata {
		if !strings.HasPrefix(key, layoutFieldOrderPrefix) {
			continue
		}
		sectionID := strings.TrimSpace(strings.TrimPrefix(key, layoutFieldOrderPrefix))
		if sectionID == "" || strings.TrimSpace(raw) == "" {
			continue
		}
		var order []string
		if err := json.Unmarshal([]byte(raw), &order); err != nil {
			continue
		}
		filtered := make([]string, 0, len(order))
		for _, entry := range order {
			if trimmed := strings.TrimSpace(entry); trimmed != "" {
				filtered = append(filtered, trimmed)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		result[sectionID] = filtered
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func orderRenderedFields(entries []renderedSectionField, order []string) []renderedField {
	if len(entries) == 0 {
		return nil
	}
	if len(order) == 0 {
		out := make([]renderedField, len(entries))
		for idx, entry := range entries {
			out[idx] = entry.field
		}
		return out
	}

	result := make([]renderedField, 0, len(entries))
	lookup := make(map[string]renderedSectionField, len(entries))
	used := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		lookup[entry.path] = entry
	}

	for _, token := range order {
		path := strings.TrimSpace(token)
		if path == "" {
			continue
		}
		entry, ok := lookup[path]
		if !ok {
			continue
		}
		if _, exists := used[path]; exists {
			continue
		}
		result = append(result, entry.field)
		used[path] = struct{}{}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].fallback < entries[j].fallback
	})
	for _, entry := range entries {
		if _, exists := used[entry.path]; exists {
			continue
		}
		result = append(result, entry.field)
	}
	return result
}

func parseSectionsMetadata(raw string) []sectionMeta {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var metas []sectionMeta
	if err := json.Unmarshal([]byte(raw), &metas); err != nil {
		return nil
	}
	sort.SliceStable(metas, func(i, j int) bool {
		if metas[i].Order != metas[j].Order {
			return metas[i].Order < metas[j].Order
		}
		return metas[i].ID < metas[j].ID
	})
	return metas
}

func parseActions(metadata map[string]string) []actionButton {
	raw := stringFromMap(metadata, layoutActionsMetadataKey)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var actions []actionButton
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return nil
	}
	for i := range actions {
		actions[i].Type = normalizeActionType(actions[i].Type)
	}
	return actions
}

func normalizeActionType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "submit":
		return "submit"
	case "button":
		return "button"
	case "reset":
		return "reset"
	default:
		return "submit"
	}
}

var responsiveGridBreakpoints = []string{"sm", "md", "lg", "xl", "2xl"}

const responsiveGridCSS = `
@media (min-width: 640px) {
  .fg-grid-responsive {
    grid-column: span var(--fg-span-sm, var(--fg-span, 12)) / span var(--fg-span-sm, var(--fg-span, 12)) !important;
    grid-column-start: var(--fg-start-sm, var(--fg-start, auto)) !important;
    grid-row: var(--fg-row-sm, var(--fg-row, auto)) !important;
  }
}

@media (min-width: 768px) {
  .fg-grid-responsive {
    grid-column: span var(--fg-span-md, var(--fg-span, 12)) / span var(--fg-span-md, var(--fg-span, 12)) !important;
    grid-column-start: var(--fg-start-md, var(--fg-start, auto)) !important;
    grid-row: var(--fg-row-md, var(--fg-row, auto)) !important;
  }
}

@media (min-width: 1024px) {
  .fg-grid-responsive {
    grid-column: span var(--fg-span-lg, var(--fg-span, 12)) / span var(--fg-span-lg, var(--fg-span, 12)) !important;
    grid-column-start: var(--fg-start-lg, var(--fg-start, auto)) !important;
    grid-row: var(--fg-row-lg, var(--fg-row, auto)) !important;
  }
}

@media (min-width: 1280px) {
  .fg-grid-responsive {
    grid-column: span var(--fg-span-xl, var(--fg-span, 12)) / span var(--fg-span-xl, var(--fg-span, 12)) !important;
    grid-column-start: var(--fg-start-xl, var(--fg-start, auto)) !important;
    grid-row: var(--fg-row-xl, var(--fg-row, auto)) !important;
  }
}

@media (min-width: 1536px) {
  .fg-grid-responsive {
    grid-column: span var(--fg-span-2xl, var(--fg-span, 12)) / span var(--fg-span-2xl, var(--fg-span, 12)) !important;
    grid-column-start: var(--fg-start-2xl, var(--fg-start, auto)) !important;
    grid-row: var(--fg-row-2xl, var(--fg-row, auto)) !important;
  }
}
`

func gridWrapperAttributes(field model.Field, columns int) (string, bool) {
	span := intHintOrDefault(field.UIHints, fieldLayoutSpanHintKey, columns)
	start := strings.TrimSpace(field.UIHints[fieldLayoutStartHintKey])
	row := strings.TrimSpace(field.UIHints[fieldLayoutRowHintKey])

	parts := make([]string, 0, 12)
	parts = append(parts, fmt.Sprintf("grid-column: span %d / span %d", span, span))
	if start != "" {
		parts = append(parts, fmt.Sprintf("grid-column-start: %s", start))
	}
	if row != "" {
		parts = append(parts, fmt.Sprintf("grid-row: %s", row))
	}

	breakpointParts := responsiveGridParts(field.UIHints)
	if len(breakpointParts) == 0 {
		return ` style="` + strings.Join(parts, "; ") + `"`, false
	}

	parts = append(parts, fmt.Sprintf("--fg-span: %d", span))
	if start != "" {
		parts = append(parts, fmt.Sprintf("--fg-start: %s", start))
	} else {
		parts = append(parts, "--fg-start: auto")
	}
	if row != "" {
		parts = append(parts, fmt.Sprintf("--fg-row: %s", row))
	} else {
		parts = append(parts, "--fg-row: auto")
	}
	parts = append(parts, breakpointParts...)

	return ` class="fg-grid-responsive" style="` + strings.Join(parts, "; ") + `"`, true
}

func responsiveGridParts(hints map[string]string) []string {
	parts := make([]string, 0, len(responsiveGridBreakpoints))
	for _, breakpoint := range responsiveGridBreakpoints {
		appendResponsiveGridPart(&parts, hints, fieldLayoutSpanHintKey, "--fg-span-", breakpoint)
		appendResponsiveGridPart(&parts, hints, fieldLayoutStartHintKey, "--fg-start-", breakpoint)
		appendResponsiveGridPart(&parts, hints, fieldLayoutRowHintKey, "--fg-row-", breakpoint)
	}
	return parts
}

func appendResponsiveGridPart(parts *[]string, hints map[string]string, keyPrefix, cssPrefix, breakpoint string) {
	value := intHintOrDefault(hints, keyPrefix+"."+breakpoint, 0)
	if value > 0 {
		*parts = append(*parts, fmt.Sprintf("%s%s: %d", cssPrefix, breakpoint, value))
	}
}

func intHintOrDefault(hints map[string]string, key string, fallback int) int {
	raw := strings.TrimSpace(hints[key])
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func gridColumnsFromHints(hints map[string]string) int {
	if hints == nil {
		return defaultGridColumns
	}
	raw := hints[layoutGridColumnsHintKey]
	if raw == "" {
		return defaultGridColumns
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultGridColumns
	}
	return value
}

// resolveComponentName maps widget hints to canonical component names.
// Component overrides (UIHints["component"] or Metadata["component.name"]) should
// supply canonical names from components.* constants. Widget hints accept a
// limited alias set (case-insensitive): textarea, json-editor, toggle, select,
// chips, code-editor, wysiwyg, rich-text, rich_text, file_uploader,
// media-picker, media_picker, datetime-range, datetime_range.
func resolveComponentName(field model.Field) string {
	if name := explicitComponentName(field); name != "" {
		return name
	}
	if name := componentNameFromWidget(widgetHint(field)); name != "" {
		return name
	}

	if field.Type == model.FieldTypeObject && field.Relationship == nil && len(field.Nested) == 0 {
		return components.NameJSONEditor
	}
	if name := componentNameFromFieldType(field); name != "" {
		return name
	}
	return componentNameFromHints(field)
}

func componentNameFromFieldType(field model.Field) string {
	switch field.Type {
	case model.FieldTypeObject:
		return components.NameObject
	case model.FieldTypeArray:
		renderer := uiHint(field, "collectionRenderer")
		if renderer == components.NameSelect || renderer == widgets.WidgetChips {
			return components.NameSelect
		}
		return components.NameArray
	case model.FieldTypeBoolean:
		return components.NameBoolean
	default:
		return ""
	}
}

func componentNameFromHints(field model.Field) string {
	switch {
	case uiHint(field, "input") == "subform":
		return components.NameObject
	case uiHint(field, "input") == "collection":
		return components.NameArray
	case len(field.Enum) > 0:
		return components.NameSelect
	case uiHint(field, "widget") == components.NameTextarea:
		return components.NameTextarea
	case uiHint(field, "input") == components.NameSelect:
		return components.NameSelect
	case field.Relationship != nil:
		return components.NameSelect
	default:
		return components.NameInput
	}
}

func explicitComponentName(field model.Field) string {
	if name := strings.TrimSpace(field.UIHints["component"]); name != "" {
		return name
	}
	return strings.TrimSpace(field.Metadata[componentNameMetadataKey])
}

var widgetComponentAliases = map[string]string{
	components.NameTextarea:      components.NameTextarea,
	widgets.WidgetJSONEditor:     components.NameJSONEditor,
	widgets.WidgetToggle:         components.NameBoolean,
	widgets.WidgetSelect:         components.NameSelect,
	widgets.WidgetChips:          components.NameSelect,
	widgets.WidgetCodeEditor:     components.NameTextarea,
	components.NameWysiwyg:       components.NameWysiwyg,
	"rich-text":                  components.NameWysiwyg,
	"rich_text":                  components.NameWysiwyg,
	"media-picker":               components.NameMediaPicker,
	components.NameMediaPicker:   components.NameMediaPicker,
	components.NameFileUploader:  components.NameFileUploader,
	components.NameDatetimeRange: components.NameDatetimeRange,
	"datetime_range":             components.NameDatetimeRange,
}

func componentNameFromWidget(widget string) string {
	return widgetComponentAliases[strings.TrimSpace(strings.ToLower(widget))]
}

func uiHint(field model.Field, key string) string {
	return strings.TrimSpace(field.UIHints[key])
}

func stringFromMap(values map[string]string, key string) string {
	if values == nil {
		return ""
	}
	return values[key]
}

func widgetHint(field model.Field) string {
	if field.Metadata != nil {
		if widget := strings.TrimSpace(field.Metadata["admin.widget"]); widget != "" {
			return widget
		}
		if widget := strings.TrimSpace(field.Metadata["widget"]); widget != "" {
			return widget
		}
	}
	if field.UIHints != nil {
		if widget := strings.TrimSpace(field.UIHints["widget"]); widget != "" {
			return widget
		}
	}
	return ""
}

func decorateFields(fields []model.Field) []model.Field {
	if len(fields) == 0 {
		return fields
	}
	decorated := make([]model.Field, len(fields))
	for i, field := range fields {
		decorated[i] = decorateField(field)
	}
	return decorated
}

func decorateField(field model.Field) model.Field {
	metadata := cloneMetadata(field.Metadata)

	metadata = appendValidationMetadata(field, metadata)

	if attrs := buildDataAttributes(metadata); attrs != "" {
		if metadata == nil {
			metadata = make(map[string]string, 1)
		}
		metadata[dataAttributesMetadataKey] = attrs
	} else if metadata != nil {
		delete(metadata, dataAttributesMetadataKey)
	}

	componentName := resolveComponentName(field)
	if componentName != "" {
		if metadata == nil {
			metadata = make(map[string]string, 1)
		}
		metadata[componentNameMetadataKey] = componentName
	}

	field.Metadata = metadata

	if field.Items != nil {
		item := decorateField(*field.Items)
		field.Items = &item
	}
	if len(field.Nested) > 0 {
		field.Nested = decorateFields(field.Nested)
	}
	return field
}

func cloneMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func buildDataAttributes(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}

	attrs := make(map[string]string)

	if names := strings.TrimSpace(metadata[behaviorNamesMetadataKey]); names != "" {
		attrs["data-behavior"] = names
	}
	if config := strings.TrimSpace(metadata[behaviorConfigMetadataKey]); config != "" {
		attrs["data-behavior-config"] = config
	}

	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		addMetadataDataAttribute(attrs, key, metadata[key])
	}

	if current, ok := metadata["relationship.current"]; ok && current != "" {
		attrs["data-relationship-current"] = current
	}

	if len(attrs) == 0 {
		return ""
	}

	attrKeys := make([]string, 0, len(attrs))
	for name := range attrs {
		attrKeys = append(attrKeys, name)
	}
	sort.Strings(attrKeys)

	var builder strings.Builder
	for _, name := range attrKeys {
		builder.WriteByte(' ')
		builder.WriteString(name)
		builder.WriteString(`="`)
		builder.WriteString(html.EscapeString(attrs[name]))
		builder.WriteByte('"')
	}
	return builder.String()
}

func addMetadataDataAttribute(attrs map[string]string, key, value string) {
	switch {
	case strings.HasPrefix(key, "relationship.endpoint."):
		addEndpointDataAttribute(attrs, key, value)
	case key == "icon":
		addTrimmedAttribute(attrs, "data-icon", value)
	case key == "icon.source":
		addTrimmedAttribute(attrs, "data-icon-source", value)
	case key == "icon.raw":
		addTrimmedAttribute(attrs, "data-icon-raw", value)
	case strings.HasPrefix(key, "behavior."):
		addBehaviorDataAttribute(attrs, key, value)
	case strings.HasPrefix(key, "validation."):
		addPrefixedDataAttribute(attrs, "validation.", "data-validation-", key, value)
	}
}

func addEndpointDataAttribute(attrs map[string]string, key, value string) {
	suffix := strings.TrimPrefix(key, "relationship.endpoint.")
	if after, ok := strings.CutPrefix(suffix, "auth."); ok {
		authKey := after
		if authKey != "" {
			attrs["data-auth-"+toKebab(authKey)] = value
		}
		return
	}
	if suffix == "refreshOn" {
		attrs["data-endpoint-refresh-on"] = value
		return
	}
	attrs["data-endpoint-"+toKebab(suffix)] = value
}

func addBehaviorDataAttribute(attrs map[string]string, key, value string) {
	if key == behaviorNamesMetadataKey || key == behaviorConfigMetadataKey {
		return
	}
	addPrefixedDataAttribute(attrs, "behavior.", "data-behavior-", key, value)
}

func addPrefixedDataAttribute(attrs map[string]string, prefix, attrPrefix, key, value string) {
	suffix := strings.TrimSpace(strings.TrimPrefix(key, prefix))
	if suffix == "" || value == "" {
		return
	}
	attrs[attrPrefix+toKebab(suffix)] = value
}

func addTrimmedAttribute(attrs map[string]string, key, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		attrs[key] = trimmed
	}
}

func toKebab(input string) string {
	if input == "" {
		return ""
	}
	var builder strings.Builder
	var last rune
	for i, r := range input {
		switch {
		case r == '.':
			builder.WriteByte('-')
			last = '-'
		case unicode.IsUpper(r):
			if i > 0 && last != '-' {
				builder.WriteByte('-')
			}
			lower := unicode.ToLower(r)
			builder.WriteRune(lower)
			last = lower
		default:
			lower := unicode.ToLower(r)
			builder.WriteRune(lower)
			last = lower
		}
	}
	return builder.String()
}

func appendValidationMetadata(field model.Field, metadata map[string]string) map[string]string {
	hasValidations := len(field.Validations) > 0
	label := strings.TrimSpace(field.Label)

	if !hasValidations && !field.Required && label == "" {
		return metadata
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}

	if hasValidations {
		if payload, err := json.Marshal(field.Validations); err == nil && len(payload) > 0 {
			metadata["validation.rules"] = string(payload)
		}
	}

	if field.Required {
		metadata["validation.required"] = "true"
	}

	if label != "" {
		metadata["validation.label"] = label
	}

	return metadata
}
