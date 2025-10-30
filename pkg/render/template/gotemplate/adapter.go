package gotemplate

import (
	"io"
	"io/fs"

	gotemplatepkg "github.com/goliatone/go-template"

	"github.com/goliatone/formgen/pkg/render/template"
)

// Option configures the go-template adapter before construction.
type Option func(*config)

type config struct {
	engineOptions []gotemplatepkg.Option
}

// WithBaseDir configures the underlying engine to load templates from a base
// directory on disk.
func WithBaseDir(dir string) Option {
	return func(cfg *config) {
		cfg.engineOptions = append(cfg.engineOptions, gotemplatepkg.WithBaseDir(dir))
	}
}

// WithFS configures the underlying engine to load templates from an fs.FS.
func WithFS(files fs.FS) Option {
	return func(cfg *config) {
		cfg.engineOptions = append(cfg.engineOptions, gotemplatepkg.WithFS(files))
	}
}

// WithExtension overrides the default template extension used by the engine.
func WithExtension(ext string) Option {
	return func(cfg *config) {
		cfg.engineOptions = append(cfg.engineOptions, gotemplatepkg.WithExtension(ext))
	}
}

// WithTemplateFunc registers helper functions or filters when the engine loads.
func WithTemplateFunc(funcs map[string]any) Option {
	return func(cfg *config) {
		cfg.engineOptions = append(cfg.engineOptions, gotemplatepkg.WithTemplateFunc(funcs))
	}
}

// WithGlobalData seeds global context values available to every template.
func WithGlobalData(data map[string]any) Option {
	return func(cfg *config) {
		cfg.engineOptions = append(cfg.engineOptions, gotemplatepkg.WithGlobalData(data))
	}
}

// WithGoTemplateOptions allows callers to pass raw go-template options.
func WithGoTemplateOptions(options ...gotemplatepkg.Option) Option {
	return func(cfg *config) {
		cfg.engineOptions = append(cfg.engineOptions, options...)
	}
}

// Engine adapts github.com/goliatone/go-template so it satisfies the
// template.TemplateRenderer contract defined in go-form-gen.md:443-460.
type Engine struct {
	renderer *gotemplatepkg.Engine
}

// Ensure Engine implements the TemplateRenderer interface.
var _ template.TemplateRenderer = (*Engine)(nil)

// New constructs an Engine using the provided configuration options.
func New(options ...Option) (*Engine, error) {
	cfg := &config{}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(cfg)
	}

	renderer, err := gotemplatepkg.NewRenderer(cfg.engineOptions...)
	if err != nil {
		return nil, err
	}

	return &Engine{renderer: renderer}, nil
}

// Render delegates to the wrapped engine.
func (e *Engine) Render(name string, data any, out ...io.Writer) (string, error) {
	return e.renderer.Render(name, data, out...)
}

// RenderTemplate delegates to the wrapped engine.
func (e *Engine) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	return e.renderer.RenderTemplate(name, data, out...)
}

// RenderString delegates to the wrapped engine.
func (e *Engine) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return e.renderer.RenderString(templateContent, data, out...)
}

// RegisterFilter registers template filters on the wrapped engine.
func (e *Engine) RegisterFilter(name string, fn func(input any, param any) (any, error)) error {
	return e.renderer.RegisterFilter(name, fn)
}

// GlobalContext seeds global data on the wrapped engine.
func (e *Engine) GlobalContext(data any) error {
	return e.renderer.GlobalContext(data)
}
