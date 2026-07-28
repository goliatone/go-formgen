package render

import "testing"

func TestFormSemanticTokenSpecsReturnsDefensiveCopy(t *testing.T) {
	first := FormSemanticTokenSpecs()
	first["form.control.background"] = SemanticTokenSpec{Constraint: "changed"}

	second := FormSemanticTokenSpecs()
	if got := second["form.control.background"].Constraint; got != "color" {
		t.Fatalf("expected registry to remain unchanged, got %q", got)
	}
	if got := second["form.control.background"].Fallback; got != "color.surface.default" {
		t.Fatalf("unexpected portable fallback %q", got)
	}
}

func TestThemeConfigResolveSemanticToken(t *testing.T) {
	cfg := &ThemeConfig{
		SemanticTokens: map[string]string{
			"form.control.background": "#ffffff",
			"color.text.primary":      "#111827",
		},
	}

	resolved, ok := cfg.ResolveSemanticToken("form.control.background")
	if !ok {
		t.Fatal("expected package token resolution")
	}
	if resolved.Token != "form.control.background" || resolved.Value != "#ffffff" {
		t.Fatalf("unexpected package resolution: %+v", resolved)
	}

	resolved, ok = cfg.ResolveSemanticToken("form.control.text")
	if !ok {
		t.Fatal("expected portable fallback resolution")
	}
	if resolved.Token != "color.text.primary" || resolved.Value != "#111827" {
		t.Fatalf("unexpected portable resolution: %+v", resolved)
	}

	if _, found := cfg.ResolveSemanticToken("form.control.radius"); found {
		t.Fatal("expected omitted token to preserve the renderer default")
	}
	if _, found := cfg.ResolveSemanticToken("unknown.token"); found {
		t.Fatal("expected unknown token to remain unsupported")
	}

	expression, resolved, ok := cfg.SemanticCSSValue("form.control.text")
	if !ok {
		t.Fatal("expected semantic CSS fallback")
	}
	if want := "var(--form-control-text, var(--color-text-primary))"; expression != want {
		t.Fatalf("unexpected semantic CSS expression: want %q, got %q", want, expression)
	}
	if resolved.Token != "color.text.primary" {
		t.Fatalf("expected portable token to supply the value, got %+v", resolved)
	}
}

func TestThemeCSSVariablesStyle(t *testing.T) {
	safe := &ThemeConfig{
		CSSVars: map[string]string{
			"--ignored-order": "legacy",
		},
		SafeCSSVarsInline: "--form-control-text:#111827;--color-text-primary:#222222;",
	}
	if got, want := ThemeCSSVariablesStyle(safe), ":root {\n--form-control-text:#111827;--color-text-primary:#222222;\n}"; got != want {
		t.Fatalf("safe projection style mismatch:\nwant %q\ngot  %q", want, got)
	}

	legacy := &ThemeConfig{
		CSSVars: map[string]string{
			"--z": "last",
			"--a": "first",
		},
	}
	if got, want := ThemeCSSVariablesStyle(legacy), ":root {\n--a: first;\n--z: last;\n}"; got != want {
		t.Fatalf("legacy style mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestThemeDiagnosticsForConsumer(t *testing.T) {
	cfg := &ThemeConfig{
		SemanticTokens: map[string]string{
			"form.control.background": "#ffffff",
			"color.text.primary":      "#111827",
		},
		Diagnostics: []ThemeTokenDiagnostic{{
			Token:  "form.control.background",
			Status: "resolved",
		}},
	}

	diagnostics := ThemeDiagnosticsForConsumer(cfg, "go-formgen/test", []string{"color.text.primary"})
	if len(diagnostics) != 3 {
		t.Fatalf("expected projection plus two consumption diagnostics, got %+v", diagnostics)
	}
	if diagnostics[1].Token != "color.text.primary" || diagnostics[1].Status != "consumed" {
		t.Fatalf("expected sorted consumed diagnostic, got %+v", diagnostics[1])
	}
	if diagnostics[2].Token != "form.control.background" || diagnostics[2].Status != "unused" {
		t.Fatalf("expected sorted unused diagnostic, got %+v", diagnostics[2])
	}
}
