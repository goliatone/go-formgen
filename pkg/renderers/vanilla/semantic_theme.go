package vanilla

import (
	"strings"

	"github.com/goliatone/go-formgen/pkg/render"
)

const vanillaThemeConsumer = "go-formgen/vanilla"

type semanticProperty struct {
	name     string
	themeKey string
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
		[]semanticProperty{{name: "color", themeKey: "form.control.placeholder"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input:not([type="hidden"]), select, textarea, [contenteditable="true"]):focus`,
		[]semanticProperty{
			{name: "border-color", themeKey: "form.control.border-focus"},
			{name: "outline-color", themeKey: "form.control.border-focus"},
		},
	)
	writeSemanticFocusRing(&css, cfg, consumed)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, select, textarea, [contenteditable="true"])[aria-invalid="true"]`,
		[]semanticProperty{{name: "border-color", themeKey: "form.control.invalid-border"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, select, textarea, button):disabled`,
		[]semanticProperty{
			{name: "background-color", themeKey: "form.control.disabled-background"},
			{name: "color", themeKey: "form.control.disabled-text"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input, textarea)[readonly]`,
		[]semanticProperty{
			{name: "background-color", themeKey: "form.control.disabled-background"},
			{name: "color", themeKey: "form.control.disabled-text"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] [data-formgen-chrome="label"]`,
		[]semanticProperty{
			{name: "color", themeKey: "form.label.text"},
			{name: "font-size", themeKey: "font.size.label"},
			{name: "font-weight", themeKey: "font.weight.emphasis"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where([data-formgen-chrome="description"], [data-formgen-chrome="help"])`,
		[]semanticProperty{{name: "color", themeKey: "form.help.text"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(.formgen-error, .formgen-errors, [role="alert"])`,
		[]semanticProperty{{name: "color", themeKey: "form.error.text"}},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] [data-formgen-action="primary"]`,
		[]semanticProperty{
			{name: "background-color", themeKey: "color.action.primary"},
			{name: "color", themeKey: "color.text.inverse"},
			{name: "border-radius", themeKey: "radius.control"},
			{name: "transition-duration", themeKey: "motion.duration.fast"},
			{name: "transition-timing-function", themeKey: "motion.easing.standard"},
		},
	)
	writeSemanticRule(&css, cfg, consumed,
		`[data-formgen-semantic="true"] [data-formgen-action="primary"]:hover`,
		[]semanticProperty{{name: "background-color", themeKey: "color.action.primary-hover"}},
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
				{name: "max-width", themeKey: render.FormContainerMaxWidthToken},
			},
		)
	}
	writeSemanticRule(css, cfg, consumed,
		`[data-formgen-semantic="true"]`,
		[]semanticProperty{
			{name: "background-color", themeKey: "color.surface.default"},
			{name: "color", themeKey: "color.text.primary"},
			{name: "border-color", themeKey: "color.border.default"},
			{name: "border-radius", themeKey: "radius.surface"},
			{name: "padding", themeKey: "space.surface"},
			{name: "font-family", themeKey: "font.family.body"},
			{name: "font-size", themeKey: "font.size.body"},
			{name: "font-weight", themeKey: "font.weight.body"},
			{name: "line-height", themeKey: "line.height.body"},
			{name: "letter-spacing", themeKey: "letter.spacing.body"},
		},
	)
	writeSemanticRule(css, cfg, consumed,
		`[data-formgen-semantic="true"] :where(input:not([type="hidden"]), select, textarea, [contenteditable="true"])`,
		[]semanticProperty{
			{name: "background-color", themeKey: "form.control.background"},
			{name: "color", themeKey: "form.control.text"},
			{name: "border-color", themeKey: "form.control.border"},
			{name: "border-radius", themeKey: "form.control.radius"},
			{name: "min-height", themeKey: "form.control.height"},
			{name: "padding-inline", themeKey: "space.control.inline"},
			{name: "padding-block", themeKey: "space.control.block"},
			{name: "font-family", themeKey: "font.family.body"},
			{name: "font-size", themeKey: "font.size.body"},
			{name: "font-weight", themeKey: "font.weight.body"},
			{name: "line-height", themeKey: "line.height.body"},
			{name: "letter-spacing", themeKey: "letter.spacing.body"},
			{name: "transition-duration", themeKey: "motion.duration.fast"},
			{name: "transition-timing-function", themeKey: "motion.easing.standard"},
		},
	)
}

func writeSemanticRule(css *strings.Builder, cfg *render.ThemeConfig, consumed map[string]struct{}, selector string, properties []semanticProperty) {
	declarations := make([]string, 0, len(properties))
	for _, property := range properties {
		value, resolution, ok := cfg.SemanticCSSValue(property.themeKey)
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
			{name: "background-color", themeKey: "form.control.disabled-background"},
			{name: "color", themeKey: "form.control.disabled-text"},
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
