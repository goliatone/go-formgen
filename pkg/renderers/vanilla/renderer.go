package vanilla

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/goliatone/formgen/pkg/model"
	rendertemplate "github.com/goliatone/formgen/pkg/render/template"
	gotemplate "github.com/goliatone/formgen/pkg/render/template/gotemplate"
)

type Option func(*config)

type config struct {
	templateFS       fs.FS
	templateRenderer rendertemplate.TemplateRenderer
	inlineStyles     string
	stylesheets      []string
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

type Renderer struct {
	templates   rendertemplate.TemplateRenderer
	inlineStyle string
	stylesheets []string
}

const dataAttributesMetadataKey = "__data_attrs"

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

	return &Renderer{
		templates:   renderer,
		inlineStyle: cfg.inlineStyles,
		stylesheets: append([]string(nil), cfg.stylesheets...),
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

	result, err := r.templates.RenderTemplate("templates/form.tmpl", map[string]any{
		"form":          decorated,
		"stylesheets":   r.stylesheets,
		"inline_styles": r.inlineStyle,
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
	if attrs := buildDataAttributes(field.Metadata); attrs != "" {
		metadata := cloneMetadata(field.Metadata)
		if metadata == nil {
			metadata = make(map[string]string, 1)
		}
		metadata[dataAttributesMetadataKey] = attrs
		field.Metadata = metadata
	} else if len(field.Metadata) > 0 {
		if _, ok := field.Metadata[dataAttributesMetadataKey]; ok {
			metadata := cloneMetadata(field.Metadata)
			delete(metadata, dataAttributesMetadataKey)
			field.Metadata = metadata
		}
	}

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
