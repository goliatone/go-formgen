package preact

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/render"
	rendertemplate "github.com/goliatone/formgen/pkg/render/template"
	gotemplate "github.com/goliatone/formgen/pkg/render/template/gotemplate"
	theme "github.com/goliatone/go-theme"
)

const (
	templateName = "templates/page.tmpl"

	defaultVendorScript = "assets/vendor/preact.production.min.js"
	defaultAppScript    = "assets/formgen-preact.min.js"
	defaultStylesheet   = "assets/formgen-preact.min.css"

	themeAssetVendorScript = "preact.vendor"
	themeAssetAppScript    = "preact.app"
	themeAssetStylesheet   = "preact.stylesheet"
)

// Option customises the renderer configuration.
type Option func(*config)

type config struct {
	templateFS       fs.FS
	templateRenderer rendertemplate.TemplateRenderer
	assetsFS         fs.FS
	assetPaths       assetPaths
	assetURLPrefix   string
}

type assetPaths struct {
	vendorScript string
	appScript    string
	stylesheet   string
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

var defaultAssetPaths = assetPaths{
	vendorScript: defaultVendorScript,
	appScript:    defaultAppScript,
	stylesheet:   defaultStylesheet,
}

// AssetPaths describes the URLs emitted by the HTML template. Custom bundles
// should set all fields even when overriding a single path.
type AssetPaths struct {
	VendorScript string
	AppScript    string
	Stylesheet   string
}

// WithTemplatesFS supplies an alternate template bundle via fs.FS.
func WithTemplatesFS(files fs.FS) Option {
	return func(cfg *config) {
		if files != nil {
			cfg.templateFS = files
		}
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

// WithAssetsFS overrides the embedded asset bundle (scripts, styles).
func WithAssetsFS(files fs.FS) Option {
	return func(cfg *config) {
		if files != nil {
			cfg.assetsFS = files
		}
	}
}

// WithAssetsDir loads assets from a directory on disk.
func WithAssetsDir(path string) Option {
	return func(cfg *config) {
		if path == "" {
			return
		}
		cfg.assetsFS = os.DirFS(path)
	}
}

// WithAssetPaths customises the relative paths injected into the rendered HTML.
func WithAssetPaths(paths AssetPaths) Option {
	return func(cfg *config) {
		cfg.assetPaths = normalizeAssetPaths(paths)
	}
}

// WithAssetURLPrefix prefixes emitted asset paths (e.g. "/static/formgen").
func WithAssetURLPrefix(prefix string) Option {
	return func(cfg *config) {
		cfg.assetURLPrefix = prefix
	}
}

// Renderer turns a FormModel into a hydrated Preact HTML document.
type Renderer struct {
	templates      rendertemplate.TemplateRenderer
	assetsFS       fs.FS
	assetPaths     assetPaths
	assetURLPrefix string
}

// New constructs a Preact renderer applying any provided options.
func New(options ...Option) (*Renderer, error) {
	cfg := config{
		templateFS: TemplatesFS(),
		assetsFS:   AssetsFS(),
		assetPaths: defaultAssetPaths,
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}

	if cfg.templateFS == nil {
		cfg.templateFS = TemplatesFS()
	}
	if err := ensureTemplate(cfg.templateFS, templateName); err != nil {
		return nil, err
	}
	if cfg.assetsFS == nil {
		cfg.assetsFS = AssetsFS()
	}

	if err := ensureAssetPaths(cfg.assetPaths); err != nil {
		return nil, err
	}

	templateRenderer := cfg.templateRenderer
	if templateRenderer == nil {
		engine, err := gotemplate.New(
			gotemplate.WithFS(cfg.templateFS),
			gotemplate.WithExtension(".tmpl"),
		)
		if err != nil {
			return nil, fmt.Errorf("preact renderer: configure template renderer: %w", err)
		}
		templateRenderer = engine
	}

	if err := ensureAssets(cfg.assetsFS, cfg.assetPaths); err != nil {
		return nil, err
	}

	return &Renderer{
		templates:      templateRenderer,
		assetsFS:       cfg.assetsFS,
		assetPaths:     cfg.assetPaths,
		assetURLPrefix: cfg.assetURLPrefix,
	}, nil
}

// Name identifies the renderer inside the registry.
func (r *Renderer) Name() string {
	return "preact"
}

// ContentType returns the MIME type for generated documents.
func (r *Renderer) ContentType() string {
	return "text/html; charset=utf-8"
}

// Render produces hydrated HTML ready for delivery.
func (r *Renderer) Render(_ context.Context, form model.FormModel, renderOptions render.RenderOptions) ([]byte, error) {
	ordered := toOrderedFormModel(form)
	payload, err := json.Marshal(ordered)
	if err != nil {
		return nil, fmt.Errorf("preact renderer: marshal form model: %w", err)
	}
	if r.templates == nil {
		return nil, fmt.Errorf("preact renderer: template renderer is nil")
	}

	themeCtx := buildThemeContext(renderOptions.Theme)
	assetResolver := themeAssetResolver(renderOptions.Theme)
	urls := r.assetURLs(assetResolver)
	cleanTheme := themeCtx
	data := map[string]any{
		"form":         form,
		"form_json":    string(payload),
		"field_orders": fieldOrderPayload(form.Metadata),
		"assets": map[string]string{
			"vendorScript": urls.VendorScript,
			"appScript":    urls.AppScript,
			"stylesheet":   urls.Stylesheet,
		},
		"theme": cleanTheme,
	}

	rendered, err := r.templates.RenderTemplate(templateName, data)
	if err != nil {
		return nil, fmt.Errorf("preact renderer: render template: %w", err)
	}

	if themeCtx.JSON == "" {
		rendered = strings.Replace(rendered, "\n\n  <script id=\"formgen-preact-data\"", "\n  <script id=\"formgen-preact-data\"", 1)
	}

	return []byte(rendered), nil
}

type orderedFormModel struct {
	OperationID string         `json:"operationId"`
	Endpoint    string         `json:"endpoint"`
	Method      string         `json:"method"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Fields      []orderedField `json:"fields"`
	Metadata    orderedMap     `json:"metadata,omitempty"`
	UIHints     orderedMap     `json:"uiHints,omitempty"`
}

type orderedField struct {
	Name         string              `json:"name"`
	Type         model.FieldType     `json:"type"`
	Format       string              `json:"format,omitempty"`
	Required     bool                `json:"required"`
	Label        string              `json:"label,omitempty"`
	Placeholder  string              `json:"placeholder,omitempty"`
	Description  string              `json:"description,omitempty"`
	Default      any                 `json:"default,omitempty"`
	Enum         []any               `json:"enum,omitempty"`
	Nested       []orderedField      `json:"nested,omitempty"`
	Items        *orderedField       `json:"items,omitempty"`
	Validations  []orderedRule       `json:"validations,omitempty"`
	Metadata     orderedMap          `json:"metadata,omitempty"`
	UIHints      orderedMap          `json:"uiHints,omitempty"`
	Relationship *model.Relationship `json:"relationship,omitempty"`
}

type orderedRule struct {
	Kind   string     `json:"kind"`
	Params orderedMap `json:"params,omitempty"`
}

func toOrderedFormModel(form model.FormModel) orderedFormModel {
	fields := make([]orderedField, len(form.Fields))
	for i, field := range form.Fields {
		fields[i] = toOrderedField(field)
	}

	return orderedFormModel{
		OperationID: form.OperationID,
		Endpoint:    form.Endpoint,
		Method:      form.Method,
		Summary:     form.Summary,
		Description: form.Description,
		Fields:      fields,
		Metadata:    newOrderedMap(form.Metadata),
		UIHints:     newOrderedMap(form.UIHints),
	}
}

func toOrderedField(field model.Field) orderedField {
	nested := make([]orderedField, len(field.Nested))
	for i, f := range field.Nested {
		nested[i] = toOrderedField(f)
	}

	var items *orderedField
	if field.Items != nil {
		v := toOrderedField(*field.Items)
		items = &v
	}

	var rules []orderedRule
	if len(field.Validations) > 0 {
		rules = make([]orderedRule, len(field.Validations))
		for i, rule := range field.Validations {
			rules[i] = orderedRule{
				Kind:   rule.Kind,
				Params: newOrderedMap(rule.Params),
			}
		}
	}

	return orderedField{
		Name:         field.Name,
		Type:         field.Type,
		Format:       field.Format,
		Required:     field.Required,
		Label:        field.Label,
		Placeholder:  field.Placeholder,
		Description:  field.Description,
		Default:      field.Default,
		Enum:         field.Enum,
		Nested:       nested,
		Items:        items,
		Validations:  rules,
		Metadata:     newOrderedMap(field.Metadata),
		UIHints:      newOrderedMap(field.UIHints),
		Relationship: field.Relationship,
	}
}

type orderedMap map[string]string

func newOrderedMap(values map[string]string) orderedMap {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return orderedMap(result)
}

func (m orderedMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return metadataLess(keys[i], keys[j])
	})

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyPayload, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		valuePayload, err := json.Marshal(m[key])
		if err != nil {
			return nil, err
		}
		buf.Write(keyPayload)
		buf.WriteByte(':')
		buf.Write(valuePayload)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

var metadataSpecialOrder = map[string]int{
	"widget":    0,
	"hideLabel": 1,
	"label":     2,
}

func metadataLess(a, b string) bool {
	aAdmin := strings.HasPrefix(a, "admin.")
	bAdmin := strings.HasPrefix(b, "admin.")
	if aAdmin != bAdmin {
		return aAdmin
	}

	aRank, aSpecial := metadataSpecialOrder[a]
	bRank, bSpecial := metadataSpecialOrder[b]
	if aSpecial && bSpecial && aRank != bRank {
		return aRank < bRank
	}
	return a < b
}

func fieldOrderPayload(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	orders := make(map[string][]string)
	const prefix = "layout.fieldOrder."
	for key, raw := range metadata {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		section := strings.TrimSpace(strings.TrimPrefix(key, prefix))
		if section == "" {
			continue
		}
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			continue
		}
		orders[section] = items
	}
	if len(orders) == 0 {
		return ""
	}
	payload, err := json.Marshal(orders)
	if err != nil {
		return ""
	}
	return string(payload)
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

func themeAssetResolver(cfg *theme.RendererConfig) func(string) string {
	if cfg == nil {
		return nil
	}
	return cfg.AssetURL
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

func ensureAssets(store fs.FS, paths assetPaths) error {
	required := []struct {
		label string
		path  string
	}{
		{label: "vendor script", path: paths.vendorScript},
		{label: "app script", path: paths.appScript},
		{label: "stylesheet", path: paths.stylesheet},
	}
	for _, item := range required {
		if _, err := fs.Stat(store, item.path); err != nil {
			return fmt.Errorf("preact renderer: %s %q not found: %w", item.label, item.path, err)
		}
	}
	return nil
}

func ensureTemplate(store fs.FS, name string) error {
	if store == nil {
		return fmt.Errorf("preact renderer: template file system is nil")
	}
	if name == "" {
		return fmt.Errorf("preact renderer: template name required")
	}
	if _, err := fs.Stat(store, name); err != nil {
		return fmt.Errorf("preact renderer: template %q not found: %w", name, err)
	}
	return nil
}

func ensureAssetPaths(paths assetPaths) error {
	if paths.vendorScript == "" {
		return fmt.Errorf("preact renderer: vendor script path required")
	}
	if paths.appScript == "" {
		return fmt.Errorf("preact renderer: app script path required")
	}
	if paths.stylesheet == "" {
		return fmt.Errorf("preact renderer: stylesheet path required")
	}
	return nil
}

func normalizeAssetPaths(paths AssetPaths) assetPaths {
	result := defaultAssetPaths
	if paths.VendorScript != "" {
		result.vendorScript = paths.VendorScript
	}
	if paths.AppScript != "" {
		result.appScript = paths.AppScript
	}
	if paths.Stylesheet != "" {
		result.stylesheet = paths.Stylesheet
	}
	return result
}

type assetURLs struct {
	VendorScript string
	AppScript    string
	Stylesheet   string
}

func (r *Renderer) assetURLs(resolver func(string) string) assetURLs {
	vendor := r.assetPaths.vendorScript
	app := r.assetPaths.appScript
	css := r.assetPaths.stylesheet

	if resolver != nil {
		if resolved := resolver(themeAssetVendorScript); strings.TrimSpace(resolved) != "" {
			vendor = resolved
		}
		if resolved := resolver(themeAssetAppScript); strings.TrimSpace(resolved) != "" {
			app = resolved
		}
		if resolved := resolver(themeAssetStylesheet); strings.TrimSpace(resolved) != "" {
			css = resolved
		}
	}

	return assetURLs{
		VendorScript: expandAssetURL(r.assetURLPrefix, vendor),
		AppScript:    expandAssetURL(r.assetURLPrefix, app),
		Stylesheet:   expandAssetURL(r.assetURLPrefix, css),
	}
}

func expandAssetURL(prefix, name string) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "http://") ||
		strings.HasPrefix(name, "https://") ||
		strings.HasPrefix(name, "//") ||
		strings.HasPrefix(name, "/") {
		return name
	}
	if prefix == "" {
		return name
	}
	p := strings.TrimRight(prefix, "/")
	n := strings.TrimLeft(name, "/")
	if p == "" {
		return n
	}
	return p + "/" + n
}
