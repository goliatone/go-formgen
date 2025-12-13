package render

import (
	"encoding/json"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

const (
	formTitleKeyHint    = "layout.titleKey"
	formSubtitleKeyHint = "layout.subtitleKey"

	fieldLabelKeyHint       = "labelKey"
	fieldDescriptionKeyHint = "descriptionKey"
	fieldPlaceholderKeyHint = "placeholderKey"
	fieldHelpTextKeyHint    = "helpTextKey"

	metadataLayoutSectionsKey = "layout.sections"
	metadataActionsKey        = "actions"
)

// LocalizeFormModel mutates the supplied form model in place, translating any
// configured `*Key` hints in the UI schema into their localized string values.
//
// This is best-effort: malformed metadata payloads are ignored and translation
// failures are routed through opts.OnMissing.
func LocalizeFormModel(form *model.FormModel, opts RenderOptions) {
	if form == nil {
		return
	}

	onMissing := opts.OnMissing
	if onMissing == nil {
		onMissing = missingTranslationDefault
	}

	localizeFormUIHints(form, opts.Locale, opts.Translator, onMissing)
	localizeMetadataActions(form, opts.Locale, opts.Translator, onMissing)
	localizeMetadataSections(form, opts.Locale, opts.Translator, onMissing)

	for i := range form.Fields {
		localizeField(&form.Fields[i], opts.Locale, opts.Translator, onMissing)
	}
}

func localizeFormUIHints(form *model.FormModel, locale string, t Translator, onMissing MissingTranslationHandler) {
	if form == nil || len(form.UIHints) == 0 {
		return
	}

	if key := strings.TrimSpace(form.UIHints[formTitleKeyHint]); key != "" {
		form.UIHints["layout.title"] = translate(locale, key, strings.TrimSpace(form.UIHints["layout.title"]), t, onMissing)
	}
	if key := strings.TrimSpace(form.UIHints[formSubtitleKeyHint]); key != "" {
		form.UIHints["layout.subtitle"] = translate(locale, key, strings.TrimSpace(form.UIHints["layout.subtitle"]), t, onMissing)
	}
}

func localizeMetadataActions(form *model.FormModel, locale string, t Translator, onMissing MissingTranslationHandler) {
	if form == nil || len(form.Metadata) == 0 {
		return
	}
	raw := strings.TrimSpace(form.Metadata[metadataActionsKey])
	if raw == "" {
		return
	}

	var actions []map[string]any
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return
	}

	changed := false
	for i := range actions {
		key := strings.TrimSpace(anyToString(actions[i]["labelKey"]))
		if key == "" {
			continue
		}
		fallback := strings.TrimSpace(anyToString(actions[i]["label"]))
		translated := translate(locale, key, fallback, t, onMissing)
		if translated != fallback {
			actions[i]["label"] = translated
			changed = true
		}
	}

	if !changed {
		return
	}
	payload, err := json.Marshal(actions)
	if err != nil {
		return
	}
	form.Metadata[metadataActionsKey] = string(payload)
}

func localizeMetadataSections(form *model.FormModel, locale string, t Translator, onMissing MissingTranslationHandler) {
	if form == nil || len(form.Metadata) == 0 {
		return
	}
	raw := strings.TrimSpace(form.Metadata[metadataLayoutSectionsKey])
	if raw == "" {
		return
	}

	var sections []map[string]any
	if err := json.Unmarshal([]byte(raw), &sections); err != nil {
		return
	}

	changed := false
	for i := range sections {
		if key := strings.TrimSpace(anyToString(sections[i]["titleKey"])); key != "" {
			fallback := strings.TrimSpace(anyToString(sections[i]["title"]))
			translated := translate(locale, key, fallback, t, onMissing)
			if translated != fallback {
				sections[i]["title"] = translated
				changed = true
			}
		}
		if key := strings.TrimSpace(anyToString(sections[i]["descriptionKey"])); key != "" {
			fallback := strings.TrimSpace(anyToString(sections[i]["description"]))
			translated := translate(locale, key, fallback, t, onMissing)
			if translated != fallback {
				sections[i]["description"] = translated
				changed = true
			}
		}
	}

	if !changed {
		return
	}
	payload, err := json.Marshal(sections)
	if err != nil {
		return
	}
	form.Metadata[metadataLayoutSectionsKey] = string(payload)
}

func localizeField(field *model.Field, locale string, t Translator, onMissing MissingTranslationHandler) {
	if field == nil {
		return
	}

	if key := strings.TrimSpace(mapString(field.UIHints, fieldLabelKeyHint)); key != "" {
		field.Label = translate(locale, key, strings.TrimSpace(field.Label), t, onMissing)
	}
	if key := strings.TrimSpace(mapString(field.UIHints, fieldDescriptionKeyHint)); key != "" {
		field.Description = translate(locale, key, strings.TrimSpace(field.Description), t, onMissing)
	}
	if key := strings.TrimSpace(mapString(field.UIHints, fieldPlaceholderKeyHint)); key != "" {
		field.Placeholder = translate(locale, key, strings.TrimSpace(field.Placeholder), t, onMissing)
	}
	if key := strings.TrimSpace(mapString(field.UIHints, fieldHelpTextKeyHint)); key != "" {
		field.UIHints = ensureMap(field.UIHints)
		field.UIHints["helpText"] = translate(locale, key, strings.TrimSpace(field.UIHints["helpText"]), t, onMissing)
	}

	for i := range field.Nested {
		localizeField(&field.Nested[i], locale, t, onMissing)
	}
	if field.Items != nil {
		localizeField(field.Items, locale, t, onMissing)
	}
}

func translate(locale, key, fallback string, t Translator, onMissing MissingTranslationHandler) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return fallback
	}

	if t == nil {
		if onMissing != nil {
			return onMissing(locale, key, []any{map[string]any{"default": fallback}}, ErrMissingTranslator)
		}
		if strings.TrimSpace(fallback) != "" {
			return fallback
		}
		return key
	}

	result, err := t.Translate(locale, key)
	if err == nil && strings.TrimSpace(result) != "" {
		return result
	}

	if onMissing != nil {
		return onMissing(locale, key, []any{map[string]any{"default": fallback}}, err)
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return key
}

func ensureMap(in map[string]string) map[string]string {
	if in != nil {
		return in
	}
	return make(map[string]string)
}

func mapString(values map[string]string, key string) string {
	if values == nil || key == "" {
		return ""
	}
	return values[key]
}
