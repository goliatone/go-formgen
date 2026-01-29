package vanilla

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"os"
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
// either simple field names or dot-paths for nested fields.
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
			for key, fn := range cfg.templateFuncs {
				templateFuncs[key] = fn
			}
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

	componentStyles, componentScripts := componentRenderer.assets()
	stylesheets := append([]string(nil), r.stylesheets...)
	if len(componentStyles) > 0 {
		stylesheets = append(stylesheets, componentStyles...)
	}
	stylesheets = resolveAssets(stylesheets, assetResolver)
	for idx := range componentScripts {
		componentScripts[idx] = resolveScriptAsset(componentScripts[idx], assetResolver)
	}
	componentScriptPayload := scriptPayloads(componentScripts)
	templateTheme := buildTemplateThemeContext(themeCtx, assetResolver)
	responsiveGridStyles := ""
	if layout.HasResponsiveGrid {
		responsiveGridStyles = strings.TrimSpace(responsiveGridCSS)
	}

	// When OmitAssets is set, skip stylesheets, inline styles, and scripts to
	// avoid duplication when the form is embedded in a parent page.
	inlineStyles := r.inlineStyle
	if renderOptions.OmitAssets {
		stylesheets = nil
		inlineStyles = ""
		responsiveGridStyles = ""
		componentScriptPayload = nil
		templateTheme = nil
	}

	result, err := r.templates.RenderTemplate("templates/form.tmpl", map[string]any{
		"locale":                 renderOptions.Locale,
		"form":                   decorated,
		"layout":                 layout,
		"actions":                actions,
		"stylesheets":            stylesheets,
		"inline_styles":          inlineStyles,
		"responsive_grid_styles": responsiveGridStyles,
		"component_scripts":      componentScriptPayload,
		"theme":                  templateTheme,
		"top_padding":            strings.Repeat("\n", topPadding),
		"render_options": map[string]any{
			"method_attr":     templateOptions.MethodAttr,
			"method_override": templateOptions.MethodOverride,
			"form_errors":     templateOptions.FormErrors,
			"hidden_fields":   templateOptions.HiddenFields,
			"locale":          renderOptions.Locale,
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
	for key, value := range in {
		out[key] = value
	}
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
	result := make(map[string]prefillValue)
	var walk func(prefix string, value any, meta prefillValue)

	walk = func(prefix string, value any, meta prefillValue) {
		switch typed := value.(type) {
		case map[string]any:
			for key, val := range typed {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				next := joinPath(prefix, key)
				walk(next, val, meta)
			}
			if prefix != "" && (meta.provenance != "" || meta.readonly || meta.disabled) {
				if _, exists := result[prefix]; !exists {
					result[prefix] = meta
				}
			}
		case map[string]string:
			for key, val := range typed {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				next := joinPath(prefix, key)
				result[next] = prefillValue{
					value:      val,
					provenance: meta.provenance,
					readonly:   meta.readonly,
					disabled:   meta.disabled,
				}
			}
		case render.ValueWithProvenance:
			meta.provenance = typed.Provenance
			meta.readonly = typed.Readonly
			meta.disabled = typed.Disabled
			walk(prefix, typed.Value, meta)
		case *render.ValueWithProvenance:
			meta.provenance = typed.Provenance
			meta.readonly = typed.Readonly
			meta.disabled = typed.Disabled
			walk(prefix, typed.Value, meta)
		default:
			if prefix != "" {
				meta.value = typed
				result[prefix] = meta
			}
		}
	}

	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		walk(key, value, prefillValue{})
	}
	return result
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
		if fields[i].Items != nil && len(fields[i].Nested) == 0 {
			// Array items render inside specialised components. Carry values for
			// relationship-backed arrays via metadata on the parent field.
			if _, ok := values[path]; ok {
				continue
			}
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
	case field.Type == model.FieldTypeBoolean:
		if boolValue, ok := toBool(value); ok {
			field.Default = boolValue
		}
	case len(field.Enum) > 0:
		if scalar, ok := stringifyScalar(value); ok {
			field.Default = scalar
		}
	case field.Type == model.FieldTypeString || field.Type == model.FieldTypeInteger || field.Type == model.FieldTypeNumber || field.Type == model.FieldTypeArray:
		if scalar, ok := stringifyScalar(value); ok {
			field.Default = scalar
		}
	default:
		if scalar, ok := stringifyScalar(value); ok {
			field.Default = scalar
		}
	}
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
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	default:
		return fmt.Sprintf("%v", value), true
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
	}
	return fields
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
		for _, field := range form.Fields {
			rendered, err := renderer.render(field, field.Name)
			if err != nil {
				return layoutContext{}, err
			}
			if strings.TrimSpace(rendered) == "" {
				continue
			}
			attrs, responsive := gridWrapperAttributes(field, ctx.GridColumns)
			if responsive {
				ctx.HasResponsiveGrid = true
			}
			ctx.Unsectioned = append(ctx.Unsectioned, renderedField{
				HTML:  rendered,
				Style: attrs,
			})
		}
		return ctx, nil
	}

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

	sectionOrders := parseFieldOrderMetadata(form.Metadata)
	sectionOutputs := make(map[string][]renderedSectionField, len(index))
	fallbackCounter := 0

	// Collect all fields that have section assignments, including nested fields
	sectioned := collectSectionedFields(form.Fields, "")
	if len(sectioned) > 0 {
		// We have nested fields with section metadata - render them individually
		for _, sf := range sectioned {
			rendered, err := renderer.render(sf.field, sf.path)
			if err != nil {
				return layoutContext{}, err
			}
			if strings.TrimSpace(rendered) == "" {
				continue
			}
			attrs, responsive := gridWrapperAttributes(sf.field, ctx.GridColumns)
			if responsive {
				ctx.HasResponsiveGrid = true
			}
			item := renderedField{
				HTML:  rendered,
				Style: attrs,
			}
			if _, ok := index[sf.sectionID]; ok {
				fallbackCounter++
				sectionOutputs[sf.sectionID] = append(sectionOutputs[sf.sectionID], renderedSectionField{
					path:     sf.path,
					field:    item,
					fallback: fallbackCounter,
				})
			} else {
				ctx.Unsectioned = append(ctx.Unsectioned, item)
			}
		}
	} else {
		// Fall back to top-level field iteration
		for _, field := range form.Fields {
			rendered, err := renderer.render(field, field.Name)
			if err != nil {
				return layoutContext{}, err
			}
			if strings.TrimSpace(rendered) == "" {
				continue
			}
			attrs, responsive := gridWrapperAttributes(field, ctx.GridColumns)
			if responsive {
				ctx.HasResponsiveGrid = true
			}
			item := renderedField{
				HTML:  rendered,
				Style: attrs,
			}
			if sectionID := stringFromMap(field.Metadata, layoutSectionFieldKey); sectionID != "" {
				if _, ok := index[sectionID]; ok {
					fallbackCounter++
					sectionOutputs[sectionID] = append(sectionOutputs[sectionID], renderedSectionField{
						path:     joinPath("", field.Name),
						field:    item,
						fallback: fallbackCounter,
					})
					continue
				}
			}
			ctx.Unsectioned = append(ctx.Unsectioned, item)
		}
	}

	for id, group := range index {
		order := sectionOrders[id]
		group.Fields = orderRenderedFields(sectionOutputs[id], order)
	}

	return ctx, nil
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
	span := columns
	if field.UIHints != nil {
		if raw := strings.TrimSpace(field.UIHints[fieldLayoutSpanHintKey]); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil && value > 0 {
				span = value
			}
		}
	}
	start := ""
	row := ""
	if field.UIHints != nil {
		start = strings.TrimSpace(field.UIHints[fieldLayoutStartHintKey])
		row = strings.TrimSpace(field.UIHints[fieldLayoutRowHintKey])
	}

	parts := make([]string, 0, 12)
	parts = append(parts, fmt.Sprintf("grid-column: span %d / span %d", span, span))
	if start != "" {
		parts = append(parts, fmt.Sprintf("grid-column-start: %s", start))
	}
	if row != "" {
		parts = append(parts, fmt.Sprintf("grid-row: %s", row))
	}

	breakpointParts := make([]string, 0, len(responsiveGridBreakpoints))
	responsive := false
	if field.UIHints != nil {
		for _, breakpoint := range responsiveGridBreakpoints {
			if raw := strings.TrimSpace(field.UIHints[fieldLayoutSpanHintKey+"."+breakpoint]); raw != "" {
				if value, err := strconv.Atoi(raw); err == nil && value > 0 {
					responsive = true
					breakpointParts = append(breakpointParts, fmt.Sprintf("--fg-span-%s: %d", breakpoint, value))
				}
			}
			if raw := strings.TrimSpace(field.UIHints[fieldLayoutStartHintKey+"."+breakpoint]); raw != "" {
				if value, err := strconv.Atoi(raw); err == nil && value > 0 {
					responsive = true
					breakpointParts = append(breakpointParts, fmt.Sprintf("--fg-start-%s: %d", breakpoint, value))
				}
			}
			if raw := strings.TrimSpace(field.UIHints[fieldLayoutRowHintKey+"."+breakpoint]); raw != "" {
				if value, err := strconv.Atoi(raw); err == nil && value > 0 {
					responsive = true
					breakpointParts = append(breakpointParts, fmt.Sprintf("--fg-row-%s: %d", breakpoint, value))
				}
			}
		}
	}

	if !responsive {
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

func resolveComponentName(field model.Field) string {
	if field.UIHints != nil {
		if name := strings.TrimSpace(field.UIHints["component"]); name != "" {
			return name
		}
	}
	if field.Metadata != nil {
		if name := strings.TrimSpace(field.Metadata[componentNameMetadataKey]); name != "" {
			return name
		}
	}

	switch strings.TrimSpace(strings.ToLower(widgetHint(field))) {
	case "textarea":
		return "textarea"
	case widgets.WidgetJSONEditor:
		return "json_editor"
	case widgets.WidgetToggle:
		return "boolean"
	case widgets.WidgetSelect, widgets.WidgetChips:
		return "select"
	case widgets.WidgetCodeEditor:
		return "textarea"
	case "wysiwyg", "rich-text", "rich_text":
		return "wysiwyg"
	case "file_uploader":
		return "file_uploader"
	case "datetime-range", "datetime_range":
		return "datetime-range"
	}

	hint := func(key string) string {
		if field.UIHints == nil {
			return ""
		}
		return strings.TrimSpace(field.UIHints[key])
	}

	switch {
	case field.Type == model.FieldTypeObject || hint("input") == "subform":
		return "object"
	case field.Type == model.FieldTypeArray || hint("input") == "collection":
		renderer := hint("collectionRenderer")
		if renderer == "select" || renderer == "chips" {
			return "select"
		}
		return "array"
	case field.Type == model.FieldTypeBoolean:
		return "boolean"
	case len(field.Enum) > 0:
		return "select"
	case hint("widget") == "textarea":
		return "textarea"
	case hint("input") == "select":
		return "select"
	case field.Relationship != nil:
		return "select"
	default:
		return "input"
	}
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
	for key, value := range src {
		cloned[key] = value
	}
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
		value := metadata[key]
		switch {
		case strings.HasPrefix(key, "relationship.endpoint."):
			suffix := strings.TrimPrefix(key, "relationship.endpoint.")
			switch {
			case strings.HasPrefix(suffix, "auth."):
				parts := strings.SplitN(strings.TrimPrefix(suffix, "auth."), ".", 2)
				if len(parts) == 0 || parts[0] == "" {
					continue
				}
				attr := "data-auth-" + toKebab(strings.Join(parts, "."))
				attrs[attr] = value
			case suffix == "refreshOn":
				attrs["data-endpoint-refresh-on"] = value
			default:
				attr := "data-endpoint-" + toKebab(suffix)
				attrs[attr] = value
			}
		case key == "icon":
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				attrs["data-icon"] = trimmed
			}
		case key == "icon.source":
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				attrs["data-icon-source"] = trimmed
			}
		case key == "icon.raw":
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				attrs["data-icon-raw"] = trimmed
			}
		case strings.HasPrefix(key, "behavior."):
			if key == behaviorNamesMetadataKey || key == behaviorConfigMetadataKey {
				continue
			}
			suffix := strings.TrimPrefix(key, "behavior.")
			suffix = strings.TrimSpace(suffix)
			if suffix == "" {
				continue
			}
			attr := "data-behavior-" + toKebab(suffix)
			attrs[attr] = value
		case strings.HasPrefix(key, "validation."):
			suffix := strings.TrimPrefix(key, "validation.")
			suffix = strings.TrimSpace(suffix)
			if suffix == "" || value == "" {
				continue
			}
			attr := "data-validation-" + toKebab(suffix)
			attrs[attr] = value
		}
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
