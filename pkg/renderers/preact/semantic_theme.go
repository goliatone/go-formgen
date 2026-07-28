package preact

import (
	"strings"

	"github.com/goliatone/go-formgen/pkg/render"
)

const preactThemeConsumer = "go-formgen/preact"

type semanticProperty struct {
	name  string
	token string
}

func semanticThemeCSS(cfg *render.ThemeConfig) (string, []string) {
	if cfg == nil || len(cfg.SemanticTokens) == 0 {
		return "", nil
	}

	var css strings.Builder
	consumed := map[string]struct{}{}
	writeSemanticBaseRules(&css, cfg, consumed)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] :where(input.fg-preact-input, textarea.fg-preact-input)::placeholder`,
		[]semanticProperty{{name: "color", token: "form.control.placeholder"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] .fg-preact-input:focus`,
		[]semanticProperty{
			{name: "border-color", token: "form.control.border-focus"},
			{name: "outline-color", token: "form.control.border-focus"},
		},
	)
	writeSemanticFocusRing(&css, cfg, consumed)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] .fg-preact-input[aria-invalid="true"]`,
		[]semanticProperty{{name: "border-color", token: "form.control.invalid-border"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] :where(.fg-preact-input:disabled, [data-fg-chip-root][aria-disabled="true"], [data-fg-typeahead-root][aria-disabled="true"])`,
		[]semanticProperty{
			{name: "background-color", token: "form.control.disabled-background"},
			{name: "color", token: "form.control.disabled-text"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] :where(input.fg-preact-input, textarea.fg-preact-input, select.fg-preact-input, [data-fg-chip-root], [data-fg-typeahead-root])[aria-readonly="true"]`,
		[]semanticProperty{
			{name: "background-color", token: "form.control.disabled-background"},
			{name: "color", token: "form.control.disabled-text"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] .fg-preact-label`,
		[]semanticProperty{
			{name: "color", token: "form.label.text"},
			{name: "font-size", token: "font.size.label"},
			{name: "font-weight", token: "font.weight.emphasis"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] :where(.fg-preact-description, .fg-preact-help)`,
		[]semanticProperty{{name: "color", token: "form.help.text"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] :where(.fg-preact-error, .fg-preact-form-errors, [role="alert"])`,
		[]semanticProperty{{name: "color", token: "form.error.text"}},
	)
	writeSemanticLoadingRule(&css, cfg, consumed)
	writeSemanticResponsiveRule(&css, cfg, consumed)

	tokens := make([]string, 0, len(consumed))
	for token := range consumed {
		tokens = append(tokens, token)
	}
	return strings.TrimSpace(css.String()), tokens
}

func writeSemanticBaseRules(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}) {
	writeSemanticRule(css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] > :where(.fg-preact, .fg-preact-form, .fg-preact-fields)`,
		[]semanticProperty{
			{name: "background-color", token: "color.surface.default"},
			{name: "color", token: "color.text.primary"},
			{name: "border-color", token: "color.border.default"},
			{name: "border-radius", token: "radius.surface"},
			{name: "padding", token: "space.surface"},
			{name: "font-family", token: "font.family.body"},
			{name: "font-size", token: "font.size.body"},
			{name: "font-weight", token: "font.weight.body"},
			{name: "line-height", token: "line.height.body"},
			{name: "letter-spacing", token: "letter.spacing.body"},
		},
	)
	writeSemanticRule(css, cfg, consumed,
		`#formgen-preact-root[data-formgen-semantic="true"] .fg-preact-input`,
		[]semanticProperty{
			{name: "background-color", token: "form.control.background"},
			{name: "color", token: "form.control.text"},
			{name: "border-color", token: "form.control.border"},
			{name: "border-radius", token: "form.control.radius"},
			{name: "min-height", token: "form.control.height"},
			{name: "padding-inline", token: "space.control.inline"},
			{name: "padding-block", token: "space.control.block"},
			{name: "font-family", token: "font.family.body"},
			{name: "font-size", token: "font.size.body"},
			{name: "font-weight", token: "font.weight.body"},
			{name: "line-height", token: "line.height.body"},
			{name: "letter-spacing", token: "letter.spacing.body"},
			{name: "transition-duration", token: "motion.duration.fast"},
			{name: "transition-timing-function", token: "motion.easing.standard"},
		},
	)
}

func writeSemanticRule(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}, selector string, properties []semanticProperty) {
	declarations := make([]string, 0, len(properties))
	for _, property := range properties {
		value, resolution, ok := cfg.SemanticCSSValue(property.token)
		if !ok {
			continue
		}
		declarations = append(declarations, property.name+":"+value)
		consumed[resolution.Token] = struct{}{}
	}
	if len(declarations) == 0 {
		return
	}
	css.WriteString(selector)
	css.WriteByte('{')
	css.WriteString(strings.Join(declarations, ";"))
	css.WriteString("}\n")
}

func writeSemanticFocusRing(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}) {
	value, resolution, ok := cfg.SemanticCSSValue("form.control.border-focus")
	if !ok {
		return
	}
	css.WriteString(`#formgen-preact-root[data-formgen-semantic="true"] .fg-preact-input:focus{box-shadow:0 0 0 3px `)
	css.WriteString(value)
	css.WriteString("}\n")
	consumed[resolution.Token] = struct{}{}
}

func writeSemanticLoadingRule(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}) {
	writeSemanticRule(css, cfg, consumed,
		`:where(#formgen-preact-root[data-formgen-semantic="true"][aria-busy="true"] :where(.fg-preact-input, [data-fg-chip-root], [data-fg-typeahead-root]), #formgen-preact-root[data-formgen-semantic="true"] :where(.fg-preact-input, [data-fg-chip-root], [data-fg-typeahead-root])[data-state="loading"])`,
		[]semanticProperty{
			{name: "background-color", token: "form.control.disabled-background"},
			{name: "color", token: "form.control.disabled-text"},
		},
	)
}

func writeSemanticResponsiveRule(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}) {
	value, resolution, ok := cfg.SemanticCSSValue("space.stack")
	if !ok {
		return
	}
	css.WriteString(`@media (max-width:640px){#formgen-preact-root[data-formgen-semantic="true"] :where(.fg-preact-form, .fg-preact-fields, .fg-preact-section-fields){gap:`)
	css.WriteString(value)
	css.WriteString("}}\n")
	consumed[resolution.Token] = struct{}{}
}
