// Package defaults provides renderer-facing orchestrator helpers. Importing
// this package intentionally opts into renderer and theme dependencies; import
// pkg/orchestrator directly for headless FormModel builds.
package defaults

import (
	"context"
	"fmt"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/orchestrator"
	"github.com/goliatone/go-formgen/pkg/render"
	jsonrenderer "github.com/goliatone/go-formgen/pkg/renderers/json"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	theme "github.com/goliatone/go-theme"
)

const DefaultRendererName = "vanilla"

// New constructs an orchestrator with the default vanilla renderer registered.
func New(options ...orchestrator.Option) *orchestrator.Orchestrator {
	opts := append([]orchestrator.Option{}, options...)
	opts = append(opts, WithVanillaRenderer())
	opts = append(opts, WithJSONRenderer())
	return orchestrator.New(opts...)
}

// WithVanillaRenderer registers the built-in vanilla renderer and makes it the
// default renderer when a request does not name one.
func WithVanillaRenderer(options ...vanilla.Option) orchestrator.Option {
	return func(o *orchestrator.Orchestrator) {
		orchestrator.WithRendererFactory(func() (render.Renderer, error) {
			renderer, err := vanilla.New(options...)
			if err != nil {
				return nil, fmt.Errorf("orchestrator defaults: vanilla renderer: %w", err)
			}
			return renderer, nil
		})(o)
		orchestrator.WithDefaultRenderer(DefaultRendererName)(o)
	}
}

// WithJSONRenderer registers the built-in descriptor renderer.
func WithJSONRenderer(options ...jsonrenderer.Option) orchestrator.Option {
	return func(o *orchestrator.Orchestrator) {
		orchestrator.WithRenderer(jsonrenderer.New(options...))(o)
	}
}

// WithThemeSelector injects a go-theme selector used to resolve theme/variant
// combinations into renderer-friendly configuration.
func WithThemeSelector(selector theme.ThemeSelector) orchestrator.Option {
	return orchestrator.WithRenderOptionsResolver(themeResolver(selector, nil))
}

// WithThemeProvider builds a go-theme selector from a ThemeProvider and
// configures renderer theme resolution using the supplied defaults when request
// theme inputs are omitted.
func WithThemeProvider(provider theme.ThemeProvider, defaultTheme, defaultVariant string) orchestrator.Option {
	if provider == nil {
		return func(*orchestrator.Orchestrator) {}
	}
	selector := theme.Selector{
		Registry:       provider,
		DefaultTheme:   strings.TrimSpace(defaultTheme),
		DefaultVariant: strings.TrimSpace(defaultVariant),
	}
	return WithThemeSelector(selector)
}

// WithThemeFallbacks supplies fallback partial paths passed to
// Selection.RendererTheme so renderers receive a resolved partial map even when
// the manifest omits a template key.
func WithThemeFallbacks(fallbacks map[string]string) orchestrator.Option {
	return orchestrator.WithRenderOptionsResolver(themeResolver(nil, fallbacks))
}

// WithTheme wires a selector and fallback partials in one option. Use this when
// both selector and fallback configuration are needed.
func WithTheme(selector theme.ThemeSelector, fallbacks map[string]string) orchestrator.Option {
	return orchestrator.WithRenderOptionsResolver(themeResolver(selector, fallbacks))
}

// DefaultThemeFallbacks returns the vanilla renderer partial fallback map used
// by compatibility theme helpers.
func DefaultThemeFallbacks() map[string]string {
	return map[string]string{
		"forms.form":          "templates/form.tmpl",
		"forms.input":         "templates/components/input.tmpl",
		"forms.select":        "templates/components/select.tmpl",
		"forms.checkbox":      "templates/components/boolean.tmpl",
		"forms.radio":         "templates/components/boolean.tmpl",
		"forms.textarea":      "templates/components/textarea.tmpl",
		"forms.media-picker":  "templates/components/media_picker.tmpl",
		"forms.wysiwyg":       "templates/components/wysiwyg.tmpl",
		"forms.json-editor":   "templates/components/json_editor.tmpl",
		"forms.file-uploader": "templates/components/file_uploader.tmpl",
	}
}

func themeResolver(selector theme.ThemeSelector, fallbacks map[string]string) orchestrator.RenderOptionsResolver {
	return func(_ context.Context, req orchestrator.Request, _ model.FormModel, options render.RenderOptions) (render.RenderOptions, error) {
		if selector == nil || options.Theme != nil {
			return options, nil
		}
		resolvedFallbacks := fallbacks
		if len(resolvedFallbacks) == 0 {
			resolvedFallbacks = DefaultThemeFallbacks()
		}
		selection, err := selector.Select(req.ThemeName, req.ThemeVariant)
		if err != nil {
			return render.RenderOptions{}, fmt.Errorf("orchestrator defaults: select theme: %w", err)
		}
		if selection == nil {
			return render.RenderOptions{}, fmt.Errorf("orchestrator defaults: theme selector returned nil selection")
		}
		cfg := selection.RendererTheme(resolvedFallbacks)
		options.Theme = &render.ThemeConfig{
			Theme:    cfg.Theme,
			Variant:  cfg.Variant,
			Partials: cfg.Partials,
			Tokens:   cfg.Tokens,
			CSSVars:  cfg.CSSVars,
			AssetURL: cfg.AssetURL,
		}
		return options, nil
	}
}
