package vanilla

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/goliatone/formgen/pkg/model"
	rendertemplate "github.com/goliatone/formgen/pkg/render/template"
	gotemplate "github.com/goliatone/formgen/pkg/render/template/gotemplate"
)

type Option func(*config)

type config struct {
	templateFS       fs.FS
	templateRenderer rendertemplate.TemplateRenderer
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

type Renderer struct {
	templates rendertemplate.TemplateRenderer
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

	return &Renderer{templates: renderer}, nil
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

	result, err := r.templates.RenderTemplate("templates/form.tmpl", map[string]any{
		"form": form,
	})
	if err != nil {
		return nil, fmt.Errorf("vanilla renderer: render template: %w", err)
	}
	return []byte(result), nil
}
