package vanilla

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"os"

	"github.com/goliatone/formgen/pkg/model"
)

type Option func(*config)

type config struct {
	templateFS fs.FS
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

type Renderer struct {
	tmpl *template.Template
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

	tmpl, err := parseTemplates(cfg.templateFS)
	if err != nil {
		return nil, err
	}

	return &Renderer{tmpl: tmpl}, nil
}

func parseTemplates(files fs.FS) (*template.Template, error) {
	root := template.New("form").Funcs(template.FuncMap{
		"hasEnum": func(field model.Field) bool {
			return len(field.Enum) > 0
		},
	})
	tmpl, err := root.ParseFS(files, "templates/*.tmpl", "templates/fields/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("vanilla renderer: parse templates: %w", err)
	}
	if tmpl.Lookup("templates/form.tmpl") == nil {
		return nil, fmt.Errorf("vanilla renderer: form template missing")
	}
	return tmpl, nil
}

func (r *Renderer) Name() string {
	return "vanilla"
}

func (r *Renderer) ContentType() string {
	return "text/html; charset=utf-8"
}

func (r *Renderer) Render(_ context.Context, form model.FormModel) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, "templates/form.tmpl", viewModel{Form: form}); err != nil {
		return nil, fmt.Errorf("vanilla renderer: execute: %w", err)
	}
	return buf.Bytes(), nil
}

type viewModel struct {
	Form model.FormModel
}
