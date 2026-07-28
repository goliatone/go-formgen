package preact

import (
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
	css, consumed := semanticThemeCSS(packageTheme)
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
	css, consumed = semanticThemeCSS(portableTheme)
	if !strings.Contains(css, "background-color:var(--form-control-background, var(--color-surface-default))") {
		t.Fatalf("portable semantic fallback missing:\n%s", css)
	}
	if !containsToken(consumed, "color.surface.default") {
		t.Fatalf("portable token not recorded as consumed: %v", consumed)
	}

	if css, consumed := semanticThemeCSS(&render.ThemeConfig{}); css != "" || len(consumed) != 0 {
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
			"color.action.primary":             "#2563eb",
			"color.action.primary-hover":       "#1d4ed8",
		},
	}

	css, consumed := semanticThemeCSS(cfg)
	for _, want := range []string{
		`::placeholder`,
		`.fg-preact-input:focus`,
		`box-shadow:0 0 0 3px`,
		`[aria-invalid="true"]`,
		`.fg-preact-input:disabled`,
		`)[aria-readonly="true"]`,
		`.fg-preact-label`,
		`.fg-preact-help`,
		`[data-state="loading"]`,
		`[aria-busy="true"]`,
		`@media (max-width:640px)`,
		`.fg-preact-fields`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("semantic state %q missing:\n%s", want, css)
		}
	}
	for _, actionToken := range []string{"color.action.primary", "color.action.primary-hover"} {
		if containsToken(consumed, actionToken) || strings.Contains(css, "--"+strings.ReplaceAll(actionToken, ".", "-")) {
			t.Fatalf("preact must not claim unrendered action token %q:\ncss=%s\nconsumed=%v", actionToken, css, consumed)
		}
	}
}

func containsToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}
