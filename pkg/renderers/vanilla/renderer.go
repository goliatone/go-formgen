package vanilla

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/goliatone/formgen/pkg/model"
	rendertemplate "github.com/goliatone/formgen/pkg/render/template"
	gotemplate "github.com/goliatone/formgen/pkg/render/template/gotemplate"
	"github.com/goliatone/formgen/pkg/renderers/vanilla/components"
)

type Option func(*config)

type config struct {
	templateFS         fs.FS
	templateRenderer   rendertemplate.TemplateRenderer
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

const (
	dataAttributesMetadataKey  = "__data_attrs"
	layoutSectionsMetadataKey  = "layout.sections"
	layoutSectionFieldKey      = "layout.section"
	layoutActionsMetadataKey   = "actions"
	layoutGridColumnsHintKey   = "layout.gridColumns"
	layoutGutterHintKey        = "layout.gutter"
	fieldLayoutSpanHintKey     = "layout.span"
	fieldLayoutStartHintKey    = "layout.start"
	fieldLayoutRowHintKey      = "layout.row"
	componentNameMetadataKey   = "component.name"
	componentConfigMetadataKey = "component.config"
	defaultGridColumns         = 12
)

type layoutContext struct {
	GridColumns      int             `json:"gridColumns"`
	GridColumnsValue string          `json:"gridColumnsValue"`
	Gutter           string          `json:"gutter"`
	Sections         []sectionGroup  `json:"sections"`
	Unsectioned      []renderedField `json:"unsectioned"`
}

type sectionGroup struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Fieldset    bool            `json:"fieldset"`
	Fields      []renderedField `json:"fields"`
}

type renderedField struct {
	HTML  string `json:"html"`
	Style string `json:"style"`
}

type sectionMeta struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Order       int               `json:"order"`
	Fieldset    bool              `json:"fieldset"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	UIHints     map[string]string `json:"uiHints,omitempty"`
}

type actionButton struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Href  string `json:"href,omitempty"`
	Type  string `json:"type,omitempty"`
	Icon  string `json:"icon,omitempty"`
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
		engine, err := gotemplate.New(
			gotemplate.WithFS(cfg.templateFS),
			gotemplate.WithExtension(".tmpl"),
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

func (r *Renderer) Render(_ context.Context, form model.FormModel) ([]byte, error) {
	if r.templates == nil {
		return nil, fmt.Errorf("vanilla renderer: template renderer is nil")
	}

	decorated := decorateFormModel(form)
	componentRenderer := newComponentRenderer(r.templates, r.components, r.overrides)
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

	result, err := r.templates.RenderTemplate("templates/form.tmpl", map[string]any{
		"form":              decorated,
		"layout":            layout,
		"actions":           actions,
		"stylesheets":       stylesheets,
		"inline_styles":     r.inlineStyle,
		"component_scripts": componentScripts,
	})
	if err != nil {
		return nil, fmt.Errorf("vanilla renderer: render template: %w", err)
	}
	return []byte(result), nil
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
			ctx.Unsectioned = append(ctx.Unsectioned, renderedField{
				HTML:  rendered,
				Style: gridStyleAttribute(field, ctx.GridColumns),
			})
		}
		return ctx, nil
	}

	ctx.Sections = make([]sectionGroup, len(metas))
	index := make(map[string]*sectionGroup, len(metas))
	for i, meta := range metas {
		ctx.Sections[i] = sectionGroup{
			ID:          meta.ID,
			Title:       meta.Title,
			Description: meta.Description,
			Fieldset:    meta.Fieldset,
		}
		index[meta.ID] = &ctx.Sections[i]
	}

	for _, field := range form.Fields {
		rendered, err := renderer.render(field, field.Name)
		if err != nil {
			return layoutContext{}, err
		}
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		item := renderedField{
			HTML:  rendered,
			Style: gridStyleAttribute(field, ctx.GridColumns),
		}
		if sectionID := stringFromMap(field.Metadata, layoutSectionFieldKey); sectionID != "" {
			if target, ok := index[sectionID]; ok {
				target.Fields = append(target.Fields, item)
				continue
			}
		}
		ctx.Unsectioned = append(ctx.Unsectioned, item)
	}

	return ctx, nil
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
	return actions
}

func gridStyleAttribute(field model.Field, columns int) string {
	span := ""
	if field.UIHints != nil {
		span = strings.TrimSpace(field.UIHints[fieldLayoutSpanHintKey])
	}
	if span == "" {
		span = strconv.Itoa(columns)
	}
	start := ""
	row := ""
	if field.UIHints != nil {
		start = strings.TrimSpace(field.UIHints[fieldLayoutStartHintKey])
		row = strings.TrimSpace(field.UIHints[fieldLayoutRowHintKey])
	}

	parts := make([]string, 0, 3)
	if span != "" {
		parts = append(parts, fmt.Sprintf("--fg-grid-span: %s", span))
	}
	if start != "" {
		parts = append(parts, fmt.Sprintf("--fg-grid-start: %s", start))
	}
	if row != "" {
		parts = append(parts, fmt.Sprintf("--fg-grid-row: %s", row))
	}
	if len(parts) == 0 {
		return ""
	}
	return ` style="` + strings.Join(parts, "; ") + `"`
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
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := metadata[key]
		if strings.HasPrefix(key, "relationship.endpoint.") {
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
		builder.WriteString(attrs[name])
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
