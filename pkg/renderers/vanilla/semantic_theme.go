package vanilla

import (
	"strings"

	"github.com/goliatone/go-formgen/pkg/render"
)

const vanillaThemeConsumer = "go-formgen/vanilla"

type semanticProperty struct {
	name  string
	token string
}

func semanticThemeCSS(cfg *render.ThemeConfig, mode render.RenderMode) (string, []string) {
	if cfg == nil || len(cfg.SemanticTokens) == 0 {
		return "", nil
	}

	var css strings.Builder
	consumed := map[string]struct{}{}
	writeSemanticBaseRules(&css, cfg, consumed, mode != render.RenderModeFields)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, textarea)::placeholder`,
		[]semanticProperty{{name: "color", token: "form.control.placeholder"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input:not([type="hidden"]), select, textarea, [contenteditable="true"]):focus`,
		[]semanticProperty{
			{name: "border-color", token: "form.control.border-focus"},
			{name: "outline-color", token: "form.control.border-focus"},
		},
	)
	writeSemanticFocusRing(&css, cfg, consumed)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, select, textarea, [contenteditable="true"])[aria-invalid="true"]`,
		[]semanticProperty{{name: "border-color", token: "form.control.invalid-border"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, select, textarea, button):disabled`,
		[]semanticProperty{
			{name: "background-color", token: "form.control.disabled-background"},
			{name: "color", token: "form.control.disabled-text"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, textarea)[readonly]`,
		[]semanticProperty{
			{name: "background-color", token: "form.control.disabled-background"},
			{name: "color", token: "form.control.disabled-text"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] [data-formgen-chrome="label"]`,
		[]semanticProperty{
			{name: "color", token: "form.label.text"},
			{name: "font-size", token: "font.size.label"},
			{name: "font-weight", token: "font.weight.emphasis"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where([data-formgen-chrome="description"], [data-formgen-chrome="help"])`,
		[]semanticProperty{{name: "color", token: "form.help.text"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(.formgen-error, .formgen-errors, [role="alert"])`,
		[]semanticProperty{{name: "color", token: "form.error.text"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] [data-formgen-action="primary"]`,
		[]semanticProperty{
			{name: "background-color", token: "color.action.primary"},
			{name: "color", token: "color.text.inverse"},
			{name: "border-radius", token: "radius.control"},
			{name: "transition-duration", token: "motion.duration.fast"},
			{name: "transition-timing-function", token: "motion.easing.standard"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] [data-formgen-action="primary"]:hover`,
		[]semanticProperty{{name: "background-color", token: "color.action.primary-hover"}},
	)
	writeSemanticLoadingRule(&css, cfg, consumed)
	writeSemanticResponsiveRule(&css, cfg, consumed)

	tokens := make([]string, 0, len(consumed))
	for token := range consumed {
		tokens = append(tokens, token)
	}
	return strings.TrimSpace(css.String()), tokens
}

func writeSemanticBaseRules(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}, includeForm bool) {
	if includeForm {
		writeSemanticRule(css, cfg, consumed,
			`form[data-formgen-semantic="true"]`,
			[]semanticProperty{
				{name: "max-width", token: render.FormContainerMaxWidthToken},
			},
		)
	}
	writeSemanticRule(css, cfg, consumed,
		`[data-formgen-semantic="true"]`,
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
		`[data-formgen-semantic="true"] :where(input:not([type="hidden"]), select, textarea, [contenteditable="true"])`,
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
	css.WriteString(`[data-formgen-semantic="true"] :where(input:not([type="hidden"]), select, textarea, [contenteditable="true"]):focus{box-shadow:0 0 0 3px `)
	css.WriteString(value)
	css.WriteString("}\n")
	consumed[resolution.Token] = struct{}{}
}

func writeSemanticLoadingRule(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}) {
	writeSemanticRule(css, cfg, consumed,
		`[data-formgen-semantic="true"]:where([aria-busy="true"], [data-formgen-loading="true"]) :where(input, select, textarea, button, [data-fg-chip-root], [data-fg-typeahead-root]), [data-formgen-semantic="true"] :where(input, select, textarea, button, [data-fg-chip-root], [data-fg-typeahead-root])[data-state="loading"]`,
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
	css.WriteString(`@media (max-width:640px){[data-formgen-semantic="true"] [data-formgen-layout="grid"]{gap:`)
	css.WriteString(value)
	css.WriteString("}}\n")
	consumed[resolution.Token] = struct{}{}
}
