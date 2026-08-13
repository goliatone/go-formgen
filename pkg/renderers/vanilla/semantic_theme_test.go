package vanilla

import (
	"slices"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/render"
)

func TestSemanticThemeCSSUsesPackageAndPortableFallbacks(t *testing.T) {
	packageTheme := &render.ThemeConfig{
		SemanticTokens: map[string]string{
			"form.control.background": "#ffffff",
		},
	}
	css, consumed := semanticThemeCSS(packageTheme, render.RenderModeDocument)
	if !strings.Contains(css, "background-color:var(--form-control-background, var(--color-surface-default))") {
		t.Fatalf("package semantic fallback missing:\n%s", css)
	}
	if !containsToken(consumed, "form.control.background") {
		t.Fatalf("package token not recorded as consumed: %v", consumed)
	}

	portableTheme := &render.ThemeConfig{
		SemanticTokens: map[string]string{
			"color.surface.default": "#f8fafc",
		},
	}
	css, consumed = semanticThemeCSS(portableTheme, render.RenderModeDocument)
	if !strings.Contains(css, "background-color:var(--form-control-background, var(--color-surface-default))") {
		t.Fatalf("portable semantic fallback missing:\n%s", css)
	}
	if !containsToken(consumed, "color.surface.default") {
		t.Fatalf("portable token not recorded as consumed: %v", consumed)
	}

	if css, consumed := semanticThemeCSS(&render.ThemeConfig{}, render.RenderModeDocument); css != "" || len(consumed) != 0 {
		t.Fatalf("empty theme must preserve current defaults, got %q, %v", css, consumed)
	}
}

func TestSemanticThemeCSSCoversStatesAndResponsiveHook(t *testing.T) {
	cfg := &render.ThemeConfig{
		SemanticTokens: map[string]string{
			"form.control.border-focus":        "#2563eb",
			"form.control.invalid-border":      "#dc2626",
			"form.control.disabled-background": "#f1f5f9",
			"form.control.disabled-text":       "#64748b",
			"form.control.placeholder":         "#94a3b8",
			"form.label.text":                  "#111827",
			"form.help.text":                   "#64748b",
			"form.error.text":                  "#dc2626",
			"space.stack":                      "1rem",
			"color.surface.default":            "#ffffff",
			"color.action.primary":             "#2563eb",
		},
	}

	css, _ := semanticThemeCSS(cfg, render.RenderModeDocument)
	for _, want := range []string{
		`::placeholder`,
		`):focus`,
		`box-shadow:0 0 0 3px`,
		`[aria-invalid="true"]`,
		`):disabled`,
		`)[readonly]`,
		`[data-formgen-chrome="label"]`,
		`[data-formgen-chrome="help"]`,
		`[data-formgen-loading="true"]`,
		`[data-state="loading"]`,
		`[data-formgen-action="primary"]`,
		`[data-formgen-layout="grid"]`,
		`@media (max-width:640px)`,
		`[data-formgen-semantic="true"]:where([aria-busy="true"], [data-formgen-loading="true"])`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("semantic state %q missing:\n%s", want, css)
		}
	}
	if strings.Contains(css, ".formgen-grid") || strings.Contains(css, ".formgen-form") {
		t.Fatalf("semantic selectors must use stable data hooks instead of replaceable default classes:\n%s", css)
	}
}

func TestSemanticThemeCSSAppliesContainerWidthOnlyToForms(t *testing.T) {
	cfg := &render.ThemeConfig{SemanticTokens: map[string]string{
		render.FormContainerMaxWidthToken: "100%",
	}}

	for _, mode := range []render.RenderMode{render.RenderModeDocument, render.RenderModeForm} {
		css, consumed := semanticThemeCSS(cfg, mode)
		if !strings.Contains(css, `form[data-formgen-semantic="true"]{max-width:var(--form-container-max-width)}`) {
			t.Fatalf("%s mode form container width rule missing:\n%s", mode, css)
		}
		if !containsToken(consumed, render.FormContainerMaxWidthToken) {
			t.Fatalf("%s mode form container token not recorded as consumed: %v", mode, consumed)
		}
	}

	css, consumed := semanticThemeCSS(cfg, render.RenderModeFields)
	if strings.Contains(css, "max-width") {
		t.Fatalf("fields mode emitted a form-only width rule:\n%s", css)
	}
	if containsToken(consumed, render.FormContainerMaxWidthToken) {
		t.Fatalf("fields mode recorded an unrendered form token as consumed: %v", consumed)
	}
	assertThemeDiagnosticStatus(t, buildThemeContext(cfg, render.RenderModeFields).Diagnostics,
		render.FormContainerMaxWidthToken, vanillaThemeConsumer, "unused")
	assertThemeDiagnosticStatus(t, buildThemeContext(cfg, render.RenderModeDocument).Diagnostics,
		render.FormContainerMaxWidthToken, vanillaThemeConsumer, "consumed")
}

func containsToken(tokens []string, want string) bool {
	return slices.Contains(tokens, want)
}

func assertThemeDiagnosticStatus(t *testing.T, diagnostics []render.ThemeTokenDiagnostic, token, consumer, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Canonical == token && diagnostic.Consumer == consumer {
			if diagnostic.Status != want {
				t.Fatalf("diagnostic status for %s/%s = %q, want %q", token, consumer, diagnostic.Status, want)
			}
			return
		}
	}
	t.Fatalf("diagnostic for %s/%s not found: %+v", token, consumer, diagnostics)
}
