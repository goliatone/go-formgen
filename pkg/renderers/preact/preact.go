package preact

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
)

const (
	templatePattern = "templates/*.tmpl"
	templateName    = "templates/page.tmpl"

	defaultVendorScript = "assets/vendor/preact.production.min.js"
	defaultAppScript    = "assets/formgen-preact.min.js"
	defaultStylesheet   = "assets/formgen-preact.min.css"
)

// Option customises the renderer configuration.
type Option func(*config)

type config struct {
	templateFS     fs.FS
	assetsFS       fs.FS
	assetPaths     assetPaths
	assetURLPrefix string
}

type assetPaths struct {
	vendorScript string
	appScript    string
	stylesheet   string
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
	tmpl           *template.Template
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
	if cfg.assetsFS == nil {
		cfg.assetsFS = AssetsFS()
	}

	if err := ensureAssetPaths(cfg.assetPaths); err != nil {
		return nil, err
	}

	tmpl, err := parseTemplates(cfg.templateFS)
	if err != nil {
		return nil, err
	}

	if err := ensureAssets(cfg.assetsFS, cfg.assetPaths); err != nil {
		return nil, err
	}

	return &Renderer{
		tmpl:           tmpl,
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
func (r *Renderer) Render(_ context.Context, form model.FormModel) ([]byte, error) {
	payload, err := json.Marshal(form)
	if err != nil {
		return nil, fmt.Errorf("preact renderer: marshal form model: %w", err)
	}

	view := viewModel{
		Form:     form,
		FormJSON: template.JS(string(payload)),
		Assets:   r.assetURLs(),
	}

	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, templateName, view); err != nil {
		return nil, fmt.Errorf("preact renderer: execute template: %w", err)
	}

	return buf.Bytes(), nil
}

func parseTemplates(store fs.FS) (*template.Template, error) {
	root := template.New("preact")
	tmpl, err := root.ParseFS(store, templatePattern)
	if err != nil {
		return nil, fmt.Errorf("preact renderer: parse templates: %w", err)
	}
	if tmpl.Lookup(templateName) == nil {
		return nil, fmt.Errorf("preact renderer: template %q missing", templateName)
	}
	return tmpl, nil
}

func ensureAssets(store fs.FS, paths assetPaths) error {
	required := map[string]string{
		"vendor script": paths.vendorScript,
		"app script":    paths.appScript,
		"stylesheet":    paths.stylesheet,
	}
	for label, location := range required {
		if _, err := fs.Stat(store, location); err != nil {
			return fmt.Errorf("preact renderer: %s %q not found: %w", label, location, err)
		}
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

func (r *Renderer) assetURLs() assetURLs {
	return assetURLs{
		VendorScript: expandAssetURL(r.assetURLPrefix, r.assetPaths.vendorScript),
		AppScript:    expandAssetURL(r.assetURLPrefix, r.assetPaths.appScript),
		Stylesheet:   expandAssetURL(r.assetURLPrefix, r.assetPaths.stylesheet),
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

type viewModel struct {
	Form     model.FormModel
	FormJSON template.JS
	Assets   assetURLs
}
