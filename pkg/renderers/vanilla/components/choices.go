package components

import (
	"bytes"
	"fmt"
	"html"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
)

// Optional native choice components. They are registered under explicit names
// and selected only through component metadata, so the default boolean
// checkbox, select, and input renderers are unchanged. Each component renders
// its own fieldset/legend chrome, description, and error slot; option values
// use the typed enum control codec so false, zero, numbers, and string codes
// survive parsing in Go and normalization in the browser. Nothing is selected
// unless the field declares a default.
const (
	choiceThemePartialKey         = "forms.choice"
	choiceMultipleThemePartialKey = "forms.choice-multiple"
	scaleThemePartialKey          = "forms.scale"
	maxScaleOptions               = 101
	defaultTrueLabel              = "Yes"
	defaultFalseLabel             = "No"
)

type choiceKind string

const (
	choiceSingle   choiceKind = "single"
	choiceMultiple choiceKind = "multiple"
	choiceScale    choiceKind = "scale"
)

type choiceOption struct {
	Value       string
	Label       string
	Description string
	Selected    bool
	Disabled    bool
}

type choiceModel struct {
	Kind        choiceKind
	ControlID   string
	ControlName string
	LegendID    string
	Options     []choiceOption
	Label       string
	Description string
	ErrorText   string
	Invalid     bool
	Required    bool
	Disabled    bool
	Readonly    bool
	LowLabel    string
	HighLabel   string
	// RequiredHint is the visually hidden legend text checkbox groups use in
	// place of aria-required; hosts localize it through config `requiredHint`.
	RequiredHint string
	DataAttrs    string
	WrapperCSS   string
}

func choiceRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	return renderChoiceComponent(buf, field, data, choiceSingle, choiceThemePartialKey)
}

func choiceMultipleRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	return renderChoiceComponent(buf, field, data, choiceMultiple, choiceMultipleThemePartialKey)
}

func scaleRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	return renderChoiceComponent(buf, field, data, choiceScale, scaleThemePartialKey)
}

func renderChoiceComponent(buf *bytes.Buffer, field model.Field, data ComponentData, kind choiceKind, partialKey string) error {
	view, err := buildChoiceModel(field, data, kind)
	if err != nil {
		return err
	}
	if data.Template != nil && data.ThemePartials != nil {
		if candidate := strings.TrimSpace(data.ThemePartials[partialKey]); candidate != "" {
			rendered, err := data.Template.RenderTemplate(candidate, map[string]any{
				"field":      field,
				"config":     data.Config,
				"theme":      data.Theme,
				"style_mode": data.StyleMode,
				"choice":     view,
			})
			if err != nil {
				return fmt.Errorf("components: render theme partial %q: %w", candidate, err)
			}
			buf.WriteString(rendered)
			return nil
		}
	}
	writeChoiceMarkup(buf, view, data.StyleMode)
	return nil
}

func buildChoiceModel(field model.Field, data ComponentData, kind choiceKind) (choiceModel, error) {
	options, err := choiceOptionsFor(field, data.Config, kind)
	if err != nil {
		return choiceModel{}, err
	}
	controlID := componentControlID(field)
	if controlID == "" {
		return choiceModel{}, fmt.Errorf("components: %s requires a field name", kind)
	}
	controlName := componentControlPath(field)
	if kind == choiceMultiple && !strings.HasSuffix(controlName, "[]") {
		controlName += "[]"
	}
	view := choiceModel{
		Kind:        kind,
		ControlID:   controlID,
		ControlName: controlName,
		LegendID:    controlID + "-legend",
		Options:     options,
		Label:       strings.TrimSpace(field.Label),
		Description: strings.TrimSpace(field.Description),
		ErrorText:   strings.TrimSpace(field.Metadata["validation.message"]),
		Invalid:     field.Metadata["validation.state"] == "invalid",
		Required:    field.Required,
		Disabled:    fieldControlDisabled(field),
		Readonly:    fieldControlReadonly(field),
		LowLabel:    configString(data.Config, "lowLabel"),
		HighLabel:   configString(data.Config, "highLabel"),
		DataAttrs:   field.Metadata["__data_attrs"],
		WrapperCSS:  sanitizeClassList(field.UIHints["cssClass"]),
	}
	if view.Label == "" {
		view.Label = field.Name
	}
	if view.RequiredHint = configString(data.Config, "requiredHint"); view.RequiredHint == "" {
		view.RequiredHint = "(required)"
	}
	return view, nil
}

func choiceOptionsFor(field model.Field, config map[string]any, kind choiceKind) ([]choiceOption, error) {
	switch kind {
	case choiceMultiple:
		if field.Type != model.FieldTypeArray {
			return nil, fmt.Errorf("components: %s requires an array field, got %q", NameChoiceMultiple, field.Type)
		}
		source := field
		if len(field.Options) == 0 && len(field.Enum) == 0 && field.Items != nil {
			source = *field.Items
			source.Default = field.Default
		}
		options := typedChoiceOptions(source, field.Default)
		if len(options) == 0 {
			return nil, fmt.Errorf("components: %s requires enum or options on field %q or its items", NameChoiceMultiple, field.Name)
		}
		return options, nil
	case choiceScale:
		if field.Type != model.FieldTypeInteger && field.Type != model.FieldTypeNumber {
			return nil, fmt.Errorf("components: %s requires an integer or number field, got %q", NameScale, field.Type)
		}
		if len(field.Options) > 0 || len(field.Enum) > 0 {
			return typedChoiceOptions(field, field.Default), nil
		}
		return scaleOptionsFromBounds(field, config)
	default:
		if options := typedChoiceOptions(field, field.Default); len(options) > 0 {
			return options, nil
		}
		if field.Type == model.FieldTypeBoolean {
			return booleanChoiceOptions(field, config), nil
		}
		return nil, fmt.Errorf("components: %s requires enum, options, or a boolean field for %q", NameChoice, field.Name)
	}
}

func typedChoiceOptions(source model.Field, defaultValue any) []choiceOption {
	if len(source.Options) > 0 {
		out := make([]choiceOption, 0, len(source.Options))
		for _, option := range source.Options {
			label := strings.TrimSpace(option.Label)
			if label == "" {
				label = fmt.Sprint(option.Value)
			}
			out = append(out, choiceOption{
				Value:       submission.EncodeEnumControlValue(option.Value),
				Label:       label,
				Description: strings.TrimSpace(option.Description),
				Disabled:    option.Disabled,
				Selected:    enumSelected(defaultValue, option.Value),
			})
		}
		return out
	}
	if len(source.Enum) == 0 {
		return nil
	}
	out := make([]choiceOption, 0, len(source.Enum))
	for _, value := range source.Enum {
		out = append(out, choiceOption{
			Value:    submission.EncodeEnumControlValue(value),
			Label:    fmt.Sprint(value),
			Selected: enumSelected(defaultValue, value),
		})
	}
	return out
}

func booleanChoiceOptions(field model.Field, config map[string]any) []choiceOption {
	trueLabel := configString(config, "trueLabel")
	if trueLabel == "" {
		trueLabel = defaultTrueLabel
	}
	falseLabel := configString(config, "falseLabel")
	if falseLabel == "" {
		falseLabel = defaultFalseLabel
	}
	return []choiceOption{
		{Value: submission.EncodeEnumControlValue(true), Label: trueLabel, Selected: enumSelected(field.Default, true)},
		{Value: submission.EncodeEnumControlValue(false), Label: falseLabel, Selected: enumSelected(field.Default, false)},
	}
}

// scaleOptionsFromBounds derives discrete numeric choices from inclusive
// integer min/max validation rules; zero is a valid bound and a valid answer.
func scaleOptionsFromBounds(field model.Field, config map[string]any) ([]choiceOption, error) {
	minValue, hasMin := ruleInteger(field, model.ValidationRuleMin)
	maxValue, hasMax := ruleInteger(field, model.ValidationRuleMax)
	if !hasMin || !hasMax {
		return nil, fmt.Errorf("components: %s requires integer min and max bounds or enum on field %q", NameScale, field.Name)
	}
	if maxValue < minValue {
		return nil, fmt.Errorf("components: %s bounds are inverted on field %q", NameScale, field.Name)
	}
	step := int64(1)
	if raw := configString(config, "step"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			return nil, fmt.Errorf("components: %s step must be a positive integer on field %q", NameScale, field.Name)
		}
		step = parsed
	}
	// Compute the option count without int64 overflow so extreme bounds are
	// diagnosed instead of panicking or wrapping.
	span := new(big.Int).Sub(big.NewInt(maxValue), big.NewInt(minValue))
	count := new(big.Int).Add(new(big.Int).Quo(span, big.NewInt(step)), big.NewInt(1))
	if count.Cmp(big.NewInt(maxScaleOptions)) > 0 {
		return nil, fmt.Errorf("components: %s would render %s options on field %q; declare enum or a larger step", NameScale, count.String(), field.Name)
	}
	total := int(count.Int64())
	out := make([]choiceOption, 0, total)
	for index := range total {
		value := minValue + int64(index)*step
		var typed any = value
		if field.Type == model.FieldTypeNumber {
			typed = float64(value)
		}
		out = append(out, choiceOption{
			Value:    submission.EncodeEnumControlValue(typed),
			Label:    strconv.FormatInt(value, 10),
			Selected: scaleSelected(field.Default, value),
		})
	}
	return out, nil
}

func scaleSelected(defaultValue any, candidate int64) bool {
	if defaultValue == nil {
		return false
	}
	text, ok := formatControlValue(defaultValue)
	if !ok {
		return false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.Trunc(parsed) != parsed {
		return false
	}
	return int64(parsed) == candidate
}

func ruleInteger(field model.Field, kind string) (int64, bool) {
	for _, rule := range field.Validations {
		if rule.Kind != kind || rule.Params == nil {
			continue
		}
		raw := strings.TrimSpace(rule.Params["value"])
		if raw == "" {
			continue
		}
		if exact, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return exact, true
		}
		// Decimal spellings such as "10.0" are accepted when integral and safe.
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.Trunc(parsed) != parsed || math.Abs(parsed) >= 1<<53 {
			return 0, false
		}
		return int64(parsed), true
	}
	return 0, false
}

func configString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	switch value := config[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}

func fieldControlDisabled(field model.Field) bool {
	return field.Disabled ||
		isTruthyHint(field.UIHints["disabled"]) ||
		isTruthyHint(field.Metadata["disabled"]) ||
		isTruthyHint(field.Metadata["prefill.disabled"])
}

func fieldControlReadonly(field model.Field) bool {
	return field.Readonly ||
		isTruthyHint(field.UIHints["readonly"]) ||
		isTruthyHint(field.Metadata["readonly"]) ||
		isTruthyHint(field.Metadata["prefill.readonly"])
}

func writeChoiceMarkup(buf *bytes.Buffer, view choiceModel, styleMode string) {
	styled := styleMode != "unstyled"
	descriptionID := ""
	if view.Description != "" {
		descriptionID = view.ControlID + "-description"
	}
	errorID := view.ControlID + "-error"

	writeChoiceFieldsetOpen(buf, view, styled, descriptionID, errorID)
	writeChoiceLegend(buf, view, styled)
	writeChoiceDescription(buf, view, styled, descriptionID)
	writeScaleEnd(buf, view.Kind, view.LowLabel, "low")
	writeChoiceOptions(buf, view)
	writeScaleEnd(buf, view.Kind, view.HighLabel, "high")
	writeChoiceErrorSlot(buf, view, styled, errorID)
	buf.WriteString("</fieldset>")
}

func writeChoiceFieldsetOpen(buf *bytes.Buffer, view choiceModel, styled bool, descriptionID, errorID string) {
	describedBy := make([]string, 0, 2)
	if descriptionID != "" {
		describedBy = append(describedBy, descriptionID)
	}
	if view.Invalid {
		describedBy = append(describedBy, errorID)
	}
	buf.WriteString(`<fieldset`)
	writeAttr(buf, "id", view.ControlID)
	writeAttr(buf, "class", choiceClass("formgen-choice", view, styled))
	writeAttr(buf, "data-formgen-choice", string(view.Kind))
	writeAttr(buf, "data-field-path", strings.TrimSuffix(view.ControlName, "[]"))
	// Radio groups are radiogroups, which support aria-required/aria-invalid;
	// checkbox groups stay plain groups and convey state through text.
	if view.Kind != choiceMultiple {
		writeAttr(buf, "role", "radiogroup")
	}
	writeAttr(buf, "aria-labelledby", view.LegendID)
	if len(describedBy) > 0 {
		writeAttr(buf, "aria-describedby", strings.Join(describedBy, " "))
	}
	writeChoiceStateAttrs(buf, view)
	if view.DataAttrs != "" {
		buf.WriteString(view.DataAttrs)
	}
	buf.WriteString(">\n")
}

func writeChoiceStateAttrs(buf *bytes.Buffer, view choiceModel) {
	// aria-required and aria-invalid are valid on radiogroup, not on the plain
	// group a checkbox fieldset is; checkbox groups convey both through text.
	radiogroup := view.Kind != choiceMultiple
	if view.Required {
		if radiogroup {
			buf.WriteString(` aria-required="true"`)
		}
		buf.WriteString(` data-formgen-required="true"`)
	}
	if view.Invalid {
		if radiogroup {
			buf.WriteString(` aria-invalid="true"`)
		}
		buf.WriteString(` data-validation-state="invalid"`)
	}
	if view.Disabled {
		buf.WriteString(` disabled aria-disabled="true"`)
	}
	if view.Readonly {
		buf.WriteString(` aria-readonly="true" data-formgen-readonly="true"`)
	}
}

func writeChoiceLegend(buf *bytes.Buffer, view choiceModel, styled bool) {
	buf.WriteString(`  <legend`)
	writeAttr(buf, "id", view.LegendID)
	if styled {
		writeAttr(buf, "class", "formgen-choice-legend text-sm font-medium text-gray-700 dark:text-neutral-300")
	} else {
		writeAttr(buf, "class", "formgen-choice-legend")
	}
	buf.WriteString(">")
	buf.WriteString(html.EscapeString(view.Label))
	if view.Required {
		buf.WriteString(`<span class="formgen-choice-required text-red-500 ms-1" aria-hidden="true">*</span>`)
	}
	// Checkbox groups cannot use aria-required, so the legend carries a
	// visually hidden hint the runtime toggles with conditional requiredness.
	if view.Kind == choiceMultiple {
		buf.WriteString(`<span class="sr-only" data-formgen-required-hint="true"`)
		if !view.Required {
			buf.WriteString(" hidden")
		}
		buf.WriteString(">" + html.EscapeString(view.RequiredHint) + "</span>")
	}
	buf.WriteString("</legend>\n")
}

func writeChoiceDescription(buf *bytes.Buffer, view choiceModel, styled bool, descriptionID string) {
	if view.Description == "" {
		return
	}
	buf.WriteString(`  <p data-formgen-chrome="description"`)
	writeAttr(buf, "id", descriptionID)
	if styled {
		writeAttr(buf, "class", "formgen-choice-description text-xs text-gray-500 dark:text-gray-400")
	} else {
		writeAttr(buf, "class", "formgen-choice-description")
	}
	buf.WriteString(">")
	buf.WriteString(html.EscapeString(view.Description))
	buf.WriteString("</p>\n")
}

func writeScaleEnd(buf *bytes.Buffer, kind choiceKind, label, position string) {
	if kind != choiceScale || label == "" {
		return
	}
	fmt.Fprintf(buf, `  <span class="formgen-choice-scale-end formgen-choice-scale-end--%s" data-formgen-scale-end=%q>`, position, position)
	buf.WriteString(html.EscapeString(label))
	buf.WriteString("</span>\n")
}

func writeChoiceOptions(buf *bytes.Buffer, view choiceModel) {
	buf.WriteString(`  <div class="formgen-choice-options" data-formgen-choice-options="true" role="presentation">` + "\n")
	inputType := "radio"
	if view.Kind == choiceMultiple {
		inputType = "checkbox"
	}
	for index, option := range view.Options {
		writeChoiceOption(buf, view, option, index, inputType)
	}
	buf.WriteString("  </div>\n")
}

func writeChoiceOption(buf *bytes.Buffer, view choiceModel, option choiceOption, index int, inputType string) {
	optionID := view.ControlID + "-" + strconv.Itoa(index)
	inactive := option.Disabled || view.Disabled || view.Readonly
	buf.WriteString(`    <label`)
	writeAttr(buf, "class", "formgen-choice-option")
	writeAttr(buf, "for", optionID)
	buf.WriteString(` data-formgen-choice-option="true"`)
	if option.Selected {
		buf.WriteString(` data-selected="true"`)
	}
	if inactive {
		buf.WriteString(` data-disabled="true"`)
	}
	buf.WriteString(">\n      <input")
	writeAttr(buf, "type", inputType)
	writeAttr(buf, "id", optionID)
	writeAttr(buf, "name", view.ControlName)
	writeAttr(buf, "value", option.Value)
	writeAttr(buf, "class", "formgen-choice-input")
	writeAttr(buf, "data-formgen-choice-index", strconv.Itoa(index))
	writeChoiceInputState(buf, view, option, inactive)
	buf.WriteString(">\n      <span class=\"formgen-choice-text\">")
	buf.WriteString(html.EscapeString(option.Label))
	buf.WriteString("</span>")
	if option.Description != "" {
		buf.WriteString(`<span class="formgen-choice-hint">`)
		buf.WriteString(html.EscapeString(option.Description))
		buf.WriteString("</span>")
	}
	buf.WriteString("\n    </label>\n")
}

func writeChoiceInputState(buf *bytes.Buffer, view choiceModel, option choiceOption, inactive bool) {
	if option.Selected {
		buf.WriteString(" checked")
	}
	if inactive {
		buf.WriteString(" disabled")
	}
	if view.Readonly {
		buf.WriteString(` aria-readonly="true"`)
	}
	if view.Required && view.Kind != choiceMultiple {
		buf.WriteString(" required")
	}
	if view.Invalid {
		buf.WriteString(` aria-invalid="true"`)
	}
}

func writeChoiceErrorSlot(buf *bytes.Buffer, view choiceModel, styled bool, errorID string) {
	buf.WriteString(`  <p data-relationship-error="true" role="status" aria-live="polite" aria-atomic="true"`)
	writeAttr(buf, "id", errorID)
	if styled {
		writeAttr(buf, "class", "formgen-error text-sm text-red-600 dark:text-red-400")
	} else {
		writeAttr(buf, "class", "formgen-error")
	}
	if !view.Invalid {
		buf.WriteString(` aria-hidden="true"`)
	}
	buf.WriteString(">")
	if view.Invalid {
		buf.WriteString(html.EscapeString(view.ErrorText))
	}
	buf.WriteString("</p>\n")
}

func choiceClass(base string, view choiceModel, styled bool) string {
	classes := []string{base, base + "--" + string(view.Kind)}
	if view.WrapperCSS != "" {
		classes = append(classes, view.WrapperCSS)
	}
	if styled {
		classes = append(classes, "border-0 p-0 m-0 min-w-0")
	}
	sort.Stable(sort.StringSlice(classes[2:]))
	return strings.Join(classes, " ")
}

func writeAttr(buf *bytes.Buffer, name, value string) {
	if value == "" {
		return
	}
	buf.WriteString(" ")
	buf.WriteString(name)
	buf.WriteString(`="`)
	buf.WriteString(html.EscapeString(value))
	buf.WriteString(`"`)
}
