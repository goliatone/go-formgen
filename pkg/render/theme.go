package render

import (
	"sort"
	"strings"
)

// ThemeTokenDiagnostic mirrors the renderer-neutral diagnostic vocabulary
// produced by go-theme without introducing that dependency into pkg/render.
type ThemeTokenDiagnostic struct {
	Token      string `json:"token"`
	Canonical  string `json:"canonical,omitempty"`
	Variable   string `json:"variable,omitempty"`
	Constraint string `json:"constraint,omitempty"`
	Status     string `json:"status"`
	Consumer   string `json:"consumer,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

const (
	// FormContainerMaxWidthToken controls the maximum width of a rendered form.
	FormContainerMaxWidthToken = "form.container.max-width"
	// LegacyContainerMaxWidthToken is the compatibility alias used by the
	// original form stylesheet.
	LegacyContainerMaxWidthToken = "container-max-width"
)

// SemanticTokenSpec declares the constraint, portable fallback, and legacy
// aliases for one go-formgen-owned semantic token.
type SemanticTokenSpec struct {
	Constraint string
	Fallback   string
	Aliases    []string
}

// SemanticTokenResolution identifies the selected token and safe value.
type SemanticTokenResolution struct {
	Token string
	Value string
}

var formSemanticTokenSpecs = map[string]SemanticTokenSpec{
	FormContainerMaxWidthToken:         {Constraint: "nonnegative-length", Aliases: []string{LegacyContainerMaxWidthToken}},
	"form.control.background":          {Constraint: "color", Fallback: "color.surface.default"},
	"form.control.text":                {Constraint: "color", Fallback: "color.text.primary"},
	"form.control.border":              {Constraint: "color", Fallback: "color.border.default"},
	"form.control.border-focus":        {Constraint: "color", Fallback: "color.focus.ring"},
	"form.control.placeholder":         {Constraint: "color", Fallback: "color.text.secondary"},
	"form.control.disabled-background": {Constraint: "color", Fallback: "color.surface.subtle"},
	"form.control.disabled-text":       {Constraint: "color", Fallback: "color.text.secondary"},
	"form.control.invalid-border":      {Constraint: "color", Fallback: "color.status.danger"},
	"form.control.height":              {Constraint: "nonnegative-length", Fallback: "size.control.height"},
	"form.control.radius":              {Constraint: "nonnegative-length", Fallback: "radius.control"},
	"form.label.text":                  {Constraint: "color", Fallback: "color.text.primary"},
	"form.help.text":                   {Constraint: "color", Fallback: "color.text.secondary"},
	"form.error.text":                  {Constraint: "color", Fallback: "color.status.danger"},
}

// FormSemanticTokenSpecs returns a defensive copy of go-formgen's semantic
// token registry. The opt-in defaults package adapts this registry to
// go-theme, while renderers use the same fallback definitions.
func FormSemanticTokenSpecs() map[string]SemanticTokenSpec {
	out := make(map[string]SemanticTokenSpec, len(formSemanticTokenSpecs))
	for token, spec := range formSemanticTokenSpecs {
		spec.Aliases = append([]string(nil), spec.Aliases...)
		out[token] = spec
	}
	return out
}

// ResolveSemanticToken resolves a form token through its portable fallback.
// SemanticTokens contains only values that passed the opt-in projection
// contract; raw legacy Tokens remain available to templates separately.
func (c *ThemeConfig) ResolveSemanticToken(token string) (SemanticTokenResolution, bool) {
	if c == nil || len(c.SemanticTokens) == 0 {
		return SemanticTokenResolution{}, false
	}
	token = strings.TrimSpace(token)
	spec, supported := formSemanticTokenSpecs[token]
	if !supported {
		return SemanticTokenResolution{}, false
	}
	if value := strings.TrimSpace(c.SemanticTokens[token]); value != "" {
		return SemanticTokenResolution{Token: token, Value: value}, true
	}
	if fallback := strings.TrimSpace(spec.Fallback); fallback != "" {
		if value := strings.TrimSpace(c.SemanticTokens[fallback]); value != "" {
			return SemanticTokenResolution{Token: fallback, Value: value}, true
		}
	}
	return SemanticTokenResolution{}, false
}

// SemanticCSSValue returns the safe CSS variable fallback expression for a
// supported form or portable token and identifies the token that supplied the
// value. No current-default literal is emitted: absence leaves the renderer's
// existing classes and stylesheet behavior untouched.
func (c *ThemeConfig) SemanticCSSValue(token string) (string, SemanticTokenResolution, bool) {
	token = strings.TrimSpace(token)
	if token == "" || c == nil {
		return "", SemanticTokenResolution{}, false
	}
	if spec, isFormToken := formSemanticTokenSpecs[token]; isFormToken {
		resolved, ok := c.ResolveSemanticToken(token)
		if !ok {
			return "", SemanticTokenResolution{}, false
		}
		expression := "var(" + semanticCSSVariable(token)
		if fallback := strings.TrimSpace(spec.Fallback); fallback != "" {
			expression += ", var(" + semanticCSSVariable(fallback) + ")"
		}
		expression += ")"
		return expression, resolved, true
	}
	value := strings.TrimSpace(c.SemanticTokens[token])
	if value == "" {
		return "", SemanticTokenResolution{}, false
	}
	return "var(" + semanticCSSVariable(token) + ")", SemanticTokenResolution{
		Token: token,
		Value: value,
	}, true
}

// ThemeDiagnosticsForConsumer appends deterministic package consumption
// outcomes to the projection/support diagnostics carried by ThemeConfig.
func ThemeDiagnosticsForConsumer(cfg *ThemeConfig, consumer string, consumedTokens []string) []ThemeTokenDiagnostic {
	if cfg == nil {
		return nil
	}
	out := append([]ThemeTokenDiagnostic(nil), cfg.Diagnostics...)
	if len(cfg.SemanticTokens) == 0 {
		return out
	}
	consumer = strings.TrimSpace(consumer)
	consumed := make(map[string]struct{}, len(consumedTokens))
	for _, token := range consumedTokens {
		token = strings.TrimSpace(token)
		if token != "" {
			consumed[token] = struct{}{}
		}
	}
	keys := make([]string, 0, len(cfg.SemanticTokens))
	for token := range cfg.SemanticTokens {
		keys = append(keys, token)
	}
	sort.Strings(keys)
	for _, token := range keys {
		status := "unused"
		if _, ok := consumed[token]; ok {
			status = "consumed"
		}
		out = append(out, ThemeTokenDiagnostic{
			Token:     token,
			Canonical: token,
			Variable:  semanticCSSVariable(token),
			Status:    status,
			Consumer:  consumer,
		})
	}
	return out
}

func semanticCSSVariable(token string) string {
	replacer := strings.NewReplacer(".", "-", "_", "-")
	return "--" + replacer.Replace(strings.TrimSpace(token))
}

// ThemeCSSVariablesStyle returns the renderer-independent style block for a
// theme. SafeCSSVarsInline is emitted by go-theme in lexical input-token order.
// Direct legacy ThemeConfig values retain their historical sorted-map output.
func ThemeCSSVariablesStyle(cfg *ThemeConfig) string {
	if cfg == nil {
		return ""
	}
	if inline := strings.TrimSpace(cfg.SafeCSSVarsInline); inline != "" {
		return ":root {\n" + inline + "\n}"
	}
	if len(cfg.CSSVars) == 0 {
		return ""
	}

	keys := make([]string, 0, len(cfg.CSSVars))
	for key := range cfg.CSSVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	out.WriteString(":root {\n")
	for _, key := range keys {
		out.WriteString(key)
		out.WriteString(": ")
		out.WriteString(cfg.CSSVars[key])
		out.WriteString(";\n")
	}
	out.WriteString("}")
	return out.String()
}
