package orchestrator

import (
	"context"
	"testing"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/render"
	theme "github.com/goliatone/go-theme"
)

func TestOrchestrator_PassesThemeConfigToRenderer(t *testing.T) {
	t.Helper()

	manifest := &theme.Manifest{
		Name:    "acme",
		Version: "1.0.0",
		Tokens: map[string]string{
			"brand": "#123456",
		},
	}

	selection := &theme.Selection{
		Theme:    "acme",
		Variant:  "custom-variant",
		Manifest: manifest,
	}

	selector := &stubThemeSelector{selection: selection}

	renderer := &captureRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	orch := New(
		WithParser(stubParser{operations: map[string]pkgopenapi.Operation{
			"create": pkgopenapi.MustNewOperation("create", "POST", "/items", pkgopenapi.Schema{}, nil),
		}}),
		WithModelBuilder(stubBuilder{form: pkgmodel.FormModel{OperationID: "create"}}),
		WithRegistry(registry),
		WithDefaultRenderer(renderer.Name()),
		WithThemeSelector(selector),
		WithUISchemaFS(nil),
	)

	doc := pkgopenapi.MustNewDocument(stubSource{}, []byte("{}"))
	_, err := orch.Generate(context.Background(), Request{
		Document:     &doc,
		OperationID:  "create",
		Renderer:     renderer.Name(),
		ThemeName:    "custom-theme",
		ThemeVariant: "custom-variant",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(selector.calls) != 1 {
		t.Fatalf("expected selector called once, got %d", len(selector.calls))
	}
	if selector.calls[0].name != "custom-theme" || selector.calls[0].variant != "custom-variant" {
		t.Fatalf("unexpected selector args: %+v", selector.calls[0])
	}

	if renderer.options.Theme == nil {
		t.Fatalf("expected theme config passed to renderer")
	}
	if renderer.options.Theme.Theme != selection.Theme {
		t.Fatalf("theme name mismatch: want %s, got %s", selection.Theme, renderer.options.Theme.Theme)
	}
	if renderer.options.Theme.Variant != selection.Variant {
		t.Fatalf("theme variant mismatch: want %s, got %s", selection.Variant, renderer.options.Theme.Variant)
	}
	if renderer.options.Theme.AssetURL == nil {
		t.Fatalf("expected AssetURL resolver present")
	}
	if got := renderer.options.Theme.Partials["forms.input"]; got != defaultThemeFallbacks()["forms.input"] {
		t.Fatalf("partials not merged with fallbacks: want %s, got %s", defaultThemeFallbacks()["forms.input"], got)
	}
	if renderer.options.Theme.Tokens["brand"] != manifest.Tokens["brand"] {
		t.Fatalf("tokens not propagated")
	}
	if renderer.options.Theme.CSSVars["--brand"] != manifest.Tokens["brand"] {
		t.Fatalf("css vars not derived from tokens")
	}
}

func TestOrchestrator_WithThemeProviderUsesDefaults(t *testing.T) {
	t.Helper()

	manifest := &theme.Manifest{
		Name:    "acme",
		Version: "1.0.0",
		Tokens: map[string]string{
			"brand": "#123456",
		},
		Templates: map[string]string{
			"forms.input": "themes/acme/input.tmpl",
		},
		Assets: theme.Assets{
			Prefix: "/assets/themes/acme",
			Files: map[string]string{
				"preact.stylesheet": "theme.css",
			},
		},
		Variants: map[string]theme.Variant{
			"dark": {
				Tokens: map[string]string{
					"brand": "#654321",
				},
				Templates: map[string]string{
					"forms.checkbox": "themes/acme/dark/checkbox.tmpl",
				},
				Assets: theme.Assets{
					Files: map[string]string{
						"preact.vendor": "vendor.dark.js",
					},
				},
			},
		},
	}

	provider := theme.NewRegistry()
	if err := provider.Register(manifest); err != nil {
		t.Fatalf("register manifest: %v", err)
	}

	renderer := &captureRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	orch := New(
		WithParser(stubParser{operations: map[string]pkgopenapi.Operation{
			"create": pkgopenapi.MustNewOperation("create", "POST", "/items", pkgopenapi.Schema{}, nil),
		}}),
		WithModelBuilder(stubBuilder{form: pkgmodel.FormModel{OperationID: "create"}}),
		WithRegistry(registry),
		WithDefaultRenderer(renderer.Name()),
		WithThemeProvider(provider, "acme", "dark"),
		WithUISchemaFS(nil),
	)

	doc := pkgopenapi.MustNewDocument(stubSource{}, []byte("{}"))
	_, err := orch.Generate(context.Background(), Request{
		Document:    &doc,
		OperationID: "create",
		Renderer:    renderer.Name(),
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	cfg := renderer.options.Theme
	if cfg == nil {
		t.Fatalf("expected theme config passed to renderer")
	}
	if cfg.Theme != "acme" {
		t.Fatalf("theme name mismatch: want acme, got %s", cfg.Theme)
	}
	if cfg.Variant != "dark" {
		t.Fatalf("theme variant mismatch: want dark, got %s", cfg.Variant)
	}
	if cfg.Partials["forms.input"] != "themes/acme/input.tmpl" {
		t.Fatalf("expected base template override, got %s", cfg.Partials["forms.input"])
	}
	if cfg.Partials["forms.checkbox"] != "themes/acme/dark/checkbox.tmpl" {
		t.Fatalf("expected variant template override, got %s", cfg.Partials["forms.checkbox"])
	}
	if cfg.Partials["forms.textarea"] != defaultThemeFallbacks()["forms.textarea"] {
		t.Fatalf("fallback partial not applied for textarea")
	}
	if cfg.Tokens["brand"] != "#654321" {
		t.Fatalf("tokens not merged with variant override, got %s", cfg.Tokens["brand"])
	}
	if cfg.CSSVars["--brand"] != "#654321" {
		t.Fatalf("css vars not derived from variant tokens, got %s", cfg.CSSVars["--brand"])
	}
	if cfg.AssetURL == nil {
		t.Fatalf("expected AssetURL resolver present")
	}
	if got := cfg.AssetURL("preact.vendor"); got != "/assets/themes/acme/vendor.dark.js" {
		t.Fatalf("unexpected vendor asset url: %s", got)
	}
	if got := cfg.AssetURL("preact.stylesheet"); got != "/assets/themes/acme/theme.css" {
		t.Fatalf("unexpected stylesheet asset url: %s", got)
	}
}

type stubSource struct{}

func (stubSource) Kind() pkgopenapi.SourceKind { return pkgopenapi.SourceKindFile }
func (stubSource) Location() string            { return "stub" }

type stubParser struct {
	operations map[string]pkgopenapi.Operation
	err        error
}

func (s stubParser) Operations(_ context.Context, _ pkgopenapi.Document) (map[string]pkgopenapi.Operation, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.operations, nil
}

type stubBuilder struct {
	form pkgmodel.FormModel
	err  error
}

func (s stubBuilder) Build(pkgopenapi.Operation) (pkgmodel.FormModel, error) {
	if s.err != nil {
		return pkgmodel.FormModel{}, s.err
	}
	return s.form, nil
}

func (s stubBuilder) Decorate(_ *pkgmodel.FormModel) error {
	return nil
}

type captureRenderer struct {
	options render.RenderOptions
}

func (r *captureRenderer) Name() string {
	return "capture"
}

func (r *captureRenderer) ContentType() string {
	return "text/plain"
}

func (r *captureRenderer) Render(_ context.Context, form pkgmodel.FormModel, opts render.RenderOptions) ([]byte, error) {
	r.options = opts
	return []byte(form.OperationID), nil
}

type selectorCall struct {
	name    string
	variant string
}

type stubThemeSelector struct {
	selection *theme.Selection
	err       error
	calls     []selectorCall
}

func (s *stubThemeSelector) Select(name, variant string, _ ...theme.QueryOption) (*theme.Selection, error) {
	s.calls = append(s.calls, selectorCall{name: name, variant: variant})
	return s.selection, s.err
}
