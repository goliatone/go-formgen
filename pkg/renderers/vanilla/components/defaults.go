package components

import (
	"bytes"
	"fmt"
	"html"
	"maps"
	"reflect"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
)

const (
	templatePrefix = "templates/components/"
	runtimeScript  = "/runtime/formgen-relationships.min.js"
	runtimeInit    = `(function(){function init(){var api=window.FormgenRelationships;if(api&&typeof api.initRelationships==="function"){try{api.initRelationships();}catch(e){}}}if(typeof document==="undefined"){return;}if(document.readyState==="loading"){window.addEventListener("DOMContentLoaded",init);}else{init();}})();`
)

// NewDefaultRegistry constructs a registry pre-populated with the built-in
// components used by the vanilla renderer.
func NewDefaultRegistry() *Registry {
	registry := New()

	registry.MustRegister(NameInput, Descriptor{
		Renderer: templateComponentRenderer("forms.input", templatePrefix+"input.tmpl"),
	})
	registry.MustRegister(NameTextarea, Descriptor{
		Renderer: templateComponentRenderer("forms.textarea", templatePrefix+"textarea.tmpl"),
	})
	registry.MustRegister(NameSelect, Descriptor{
		Renderer: templateComponentRenderer("forms.select", templatePrefix+"select.tmpl"),
	})
	registry.MustRegister(NameBoolean, Descriptor{
		Renderer: templateComponentRenderer("forms.checkbox", templatePrefix+"boolean.tmpl"),
	})
	registry.MustRegister(NameObject, Descriptor{
		Renderer: objectRenderer,
	})
	registry.MustRegister(NameArray, Descriptor{
		Renderer: arrayRenderer,
		Scripts: []Script{
			{Src: runtimeScript, Defer: true},
			{Inline: runtimeInit},
		},
	})
	registry.MustRegister(NameDatetimeRange, Descriptor{
		Renderer: datetimeRangeRenderer,
	})
	registry.MustRegister(NameMediaPicker, Descriptor{
		Renderer: templateComponentRenderer("forms.media-picker", templatePrefix+"media_picker.tmpl"),
		Scripts: []Script{
			{Src: runtimeScript, Defer: true},
			{Inline: runtimeInit},
		},
	})
	registry.MustRegister(NameWysiwyg, Descriptor{
		Renderer: templateComponentRenderer("forms.wysiwyg", templatePrefix+"wysiwyg.tmpl"),
		Scripts: []Script{
			{Src: runtimeScript, Defer: true},
			{Inline: runtimeInit},
		},
	})
	registry.MustRegister(NameFileUploader, Descriptor{
		Renderer: templateComponentRenderer("forms.file-uploader", templatePrefix+"file_uploader.tmpl"),
		Scripts: []Script{
			{Src: runtimeScript, Defer: true},
			{Inline: runtimeInit},
		},
	})
	registry.MustRegister(NameJSONEditor, jsonEditorDescriptor())

	return registry
}

func templateComponentRenderer(partialKey, templateName string) Renderer {
	return func(buf *bytes.Buffer, field model.Field, data ComponentData) error {
		if data.Template == nil {
			return fmt.Errorf("components: template renderer not configured for %q", templateName)
		}

		resolvedTemplate := templateName
		if data.ThemePartials != nil {
			if candidate := strings.TrimSpace(data.ThemePartials[partialKey]); candidate != "" {
				resolvedTemplate = candidate
			}
		}

		payload := map[string]any{
			"field":        field,
			"config":       data.Config,
			"theme":        data.Theme,
			"style_mode":   data.StyleMode,
			"enum_options": enumOptions(field),
		}
		rendered, err := data.Template.RenderTemplate(resolvedTemplate, payload)
		if err != nil {
			return fmt.Errorf("components: render template %q: %w", templateName, err)
		}
		buf.WriteString(rendered)
		return nil
	}
}

type enumOption struct {
	Value    string
	Label    string
	Selected bool
}

func enumOptions(field model.Field) []enumOption {
	if len(field.Enum) == 0 {
		return nil
	}
	out := make([]enumOption, 0, len(field.Enum))
	for _, value := range field.Enum {
		out = append(out, enumOption{
			Value:    submission.EncodeEnumControlValue(value),
			Label:    fmt.Sprint(value),
			Selected: enumSelected(field.Default, value),
		})
	}
	return out
}

func enumSelected(defaultValue, candidate any) bool {
	switch defaults := defaultValue.(type) {
	case []any:
		for _, value := range defaults {
			if reflect.DeepEqual(value, candidate) {
				return true
			}
		}
	case []string:
		for _, value := range defaults {
			if fmt.Sprint(candidate) == value {
				return true
			}
		}
	default:
		return reflect.DeepEqual(defaultValue, candidate)
	}
	return false
}

func objectRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	labelID := objectLabelID(field)

	writeObjectStart(&builder, field, labelID)
	writeObjectCopy(&builder, field, labelID)
	if err := writeObjectChildren(&builder, field, data); err != nil {
		return err
	}
	builder.WriteString(`</fieldset>`)
	buf.WriteString(builder.String())
	return nil
}

func objectClasses(field model.Field) []string {
	classes := []string{
		"space-y-4",
		"p-4",
		"border",
		"border-gray-200",
		"rounded-lg",
		"dark:border-gray-700",
	}
	if strings.TrimSpace(field.UIHints["accordion"]) == "true" {
		classes = append(classes, "border-s-4", "border-s-blue-600")
	}
	for _, key := range []string{"cssClass", "class"} {
		if extra := sanitizeClassList(field.UIHints[key]); extra != "" {
			classes = append(classes, extra)
		}
	}
	return classes
}

func objectLabelID(field model.Field) string {
	if strings.TrimSpace(field.Label) == "" {
		return ""
	}
	return componentLabelID(field)
}

func writeObjectStart(builder *strings.Builder, field model.Field, labelID string) {
	builder.WriteString(`<fieldset`)
	if id := componentControlID(field); id != "" {
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(id))
		builder.WriteString(`"`)
	}
	builder.WriteString(` class="`)
	builder.WriteString(html.EscapeString(strings.Join(objectClasses(field), " ")))
	builder.WriteString(`"`)
	writeRelationshipAttributes(builder, field.Relationship)
	if labelID != "" {
		builder.WriteString(` aria-labelledby="`)
		builder.WriteString(html.EscapeString(labelID))
		builder.WriteString(`"`)
	}
	builder.WriteString(`>`)
}

func writeObjectCopy(builder *strings.Builder, field model.Field, labelID string) {
	if label := strings.TrimSpace(field.Label); label != "" {
		builder.WriteString(`<legend`)
		if labelID != "" {
			builder.WriteString(` id="`)
			builder.WriteString(html.EscapeString(labelID))
			builder.WriteString(`"`)
		}
		builder.WriteString(` class="text-sm font-semibold text-gray-900 dark:text-white">`)
		builder.WriteString(html.EscapeString(label))
		builder.WriteString(`</legend>`)
	}
	if desc := strings.TrimSpace(field.Description); desc != "" {
		builder.WriteString(`<p class="text-xs text-gray-500 dark:text-gray-400">`)
		builder.WriteString(html.EscapeString(desc))
		builder.WriteString(`</p>`)
	}
	if hint := strings.TrimSpace(field.UIHints["helpText"]); hint != "" {
		builder.WriteString(`<p class="text-xs text-gray-600 dark:text-gray-300">`)
		builder.WriteString(html.EscapeString(hint))
		builder.WriteString(`</p>`)
	}
}

func writeObjectChildren(builder *strings.Builder, field model.Field, data ComponentData) error {
	if data.RenderChild != nil {
		builder.WriteString(`<div class="space-y-4">`)
		for _, nested := range field.Nested {
			child, err := data.RenderChild(nested)
			if err != nil {
				return err
			}
			builder.WriteString(child)
		}
		builder.WriteString(`</div>`)
	}
	return nil
}

func arrayRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	label := strings.TrimSpace(field.Label)
	labelID := ""
	if label != "" {
		labelID = componentLabelID(field)
	}
	builder.WriteString(`<div`)
	if id := componentControlID(field); id != "" {
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(id))
		builder.WriteString(`"`)
	}
	builder.WriteString(` class="space-y-3`)
	if field.UIHints != nil {
		if extra := sanitizeClassList(field.UIHints["cssClass"]); extra != "" {
			builder.WriteByte(' ')
			builder.WriteString(html.EscapeString(extra))
		} else if extra := sanitizeClassList(field.UIHints["class"]); extra != "" {
			builder.WriteByte(' ')
			builder.WriteString(html.EscapeString(extra))
		}
	}
	builder.WriteString(`"`)
	writeRelationshipAttributes(&builder, field.Relationship)
	builder.WriteString(` role="group"`)
	if labelID != "" {
		builder.WriteString(` aria-labelledby="`)
		builder.WriteString(html.EscapeString(labelID))
		builder.WriteString(`"`)
	}
	builder.WriteString(`>`)

	if label != "" {
		builder.WriteString(`<div`)
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(labelID))
		builder.WriteString(`"`)
		builder.WriteString(` class="text-sm font-medium text-gray-900 dark:text-white">`)
		builder.WriteString(html.EscapeString(label))
		builder.WriteString(`</div>`)
	}
	if desc := strings.TrimSpace(field.Description); desc != "" {
		builder.WriteString(`<p class="text-xs text-gray-500 dark:text-gray-400">`)
		builder.WriteString(html.EscapeString(desc))
		builder.WriteString(`</p>`)
	}
	if hint := strings.TrimSpace(field.UIHints["helpText"]); hint != "" {
		builder.WriteString(`<p class="text-xs text-gray-600 dark:text-gray-300">`)
		builder.WriteString(html.EscapeString(hint))
		builder.WriteString(`</p>`)
	}

	if field.Items != nil && data.RenderChild != nil {
		if err := writeArrayItems(&builder, field, data); err != nil {
			return err
		}
	} else {
		builder.WriteString(`<p class="text-sm text-gray-500 dark:text-gray-400">Array field `)
		builder.WriteString(html.EscapeString(field.Name))
		builder.WriteString(` requires item definition.</p>`)
	}

	builder.WriteString(`</div>`)
	buf.WriteString(builder.String())
	return nil
}

func writeArrayItems(builder *strings.Builder, field model.Field, data ComponentData) error {
	cardinality := arrayCardinality(field)
	repeatable := cardinality == "many"
	itemValues := coerceSlice(field.Default)
	builder.WriteString(`<div class="space-y-3"`)
	if cardinality != "" {
		builder.WriteString(` data-relationship-collection="`)
		builder.WriteString(html.EscapeString(cardinality))
		builder.WriteString(`"`)
	}
	if repeatable {
		writeArrayRepeaterAttributes(builder, field, len(itemValues))
	}
	builder.WriteString(`>`)
	if err := writeArrayItemFields(builder, field, data, itemValues, repeatable); err != nil {
		return err
	}
	if repeatable {
		if err := writeArrayPrototypeTemplate(builder, field, data, len(itemValues)); err != nil {
			return err
		}
	}
	builder.WriteString(`</div>`)
	if repeatable {
		writeArrayAddButton(builder, field)
	}
	return nil
}

func writeArrayRepeaterAttributes(builder *strings.Builder, field model.Field, prototypeIndex int) {
	builder.WriteString(` data-formgen-array-items="true"`)
	if path := componentControlPath(field); path != "" {
		builder.WriteString(` data-formgen-array-name="`)
		builder.WriteString(html.EscapeString(path))
		builder.WriteString(`"`)
	}
	builder.WriteString(` data-formgen-array-next-index="`)
	builder.WriteString(html.EscapeString(fmt.Sprint(prototypeIndex)))
	builder.WriteString(`"`)
	if path := arrayItemControlPath(field, prototypeIndex); path != "" {
		builder.WriteString(` data-formgen-array-prototype-path="`)
		builder.WriteString(html.EscapeString(path))
		builder.WriteString(`"`)
		builder.WriteString(` data-formgen-array-prototype-id-prefix="`)
		builder.WriteString(html.EscapeString(controlIDFromPath(path)))
		builder.WriteString(`"`)
	}
}

func writeArrayPrototypeTemplate(builder *strings.Builder, field model.Field, data ComponentData, prototypeIndex int) error {
	child, err := renderArrayTemplatePrototypeItem(field, data, prototypeIndex)
	if err != nil {
		return err
	}
	builder.WriteString(`<template data-formgen-array-prototype="true">`)
	builder.WriteString(child)
	builder.WriteString(`</template>`)
	return nil
}

func writeArrayItemFields(builder *strings.Builder, field model.Field, data ComponentData, itemValues []any, repeatable bool) error {
	if len(itemValues) == 0 {
		if repeatable {
			return nil
		}
		child, err := renderArrayPrototypeItem(field, data, 0)
		if err != nil {
			return err
		}
		builder.WriteString(child)
		return nil
	}
	for idx, value := range itemValues {
		item := cloneField(*field.Items)
		if path := arrayItemControlPath(field, idx); path != "" {
			applyControlPath(&item, path)
		}
		child, err := data.RenderChild(WithFieldValue(item, value))
		if err != nil {
			return err
		}
		builder.WriteString(child)
	}
	return nil
}

func renderArrayPrototypeItem(field model.Field, data ComponentData, idx int) (string, error) {
	item := cloneField(*field.Items)
	if path := arrayItemControlPath(field, idx); path != "" {
		applyPrototypeControlPath(&item, path)
	}
	return data.RenderChild(item)
}

func renderArrayTemplatePrototypeItem(field model.Field, data ComponentData, idx int) (string, error) {
	item := cloneField(*field.Items)
	if path := arrayItemControlPath(field, idx); path != "" {
		applyControlPath(&item, path)
		markPrototypeControlDisabled(&item)
	}
	return data.RenderChild(item)
}

func writeArrayAddButton(builder *strings.Builder, field model.Field) {
	builder.WriteString(`<button type="button" class="py-3 px-4 inline-flex items-center gap-x-2 text-sm font-medium rounded-lg border border-gray-200 bg-white text-gray-800 shadow-sm hover:bg-gray-50 disabled:opacity-50 disabled:pointer-events-none dark:bg-slate-900 dark:border-gray-700 dark:text-white dark:hover:bg-gray-800" data-formgen-array-action="add" data-relationship-action="add">`)
	if label := strings.TrimSpace(field.UIHints["addText"]); label != "" {
		builder.WriteString(html.EscapeString(label))
	} else if label := strings.TrimSpace(field.UIHints["repeaterLabel"]); label != "" {
		builder.WriteString(`Add `)
		builder.WriteString(html.EscapeString(label))
	} else if field.Label != "" {
		builder.WriteString(`Add `)
		builder.WriteString(html.EscapeString(field.Label))
	} else {
		builder.WriteString("Add item")
	}
	builder.WriteString(`</button>`)
}

func arrayCardinality(field model.Field) string {
	if field.UIHints == nil {
		return ""
	}
	return strings.TrimSpace(field.UIHints["cardinality"])
}

func datetimeRangeRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	builder.WriteString(`<div`)
	if id := componentControlID(field); id != "" {
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(id))
		builder.WriteString(`"`)
	}
	builder.WriteString(` class="space-y-3`)
	if field.UIHints != nil {
		if extra := sanitizeClassList(field.UIHints["cssClass"]); extra != "" {
			builder.WriteByte(' ')
			builder.WriteString(html.EscapeString(extra))
		} else if extra := sanitizeClassList(field.UIHints["class"]); extra != "" {
			builder.WriteByte(' ')
			builder.WriteString(html.EscapeString(extra))
		}
	}
	builder.WriteString(`"`)
	writeRelationshipAttributes(&builder, field.Relationship)
	builder.WriteString(` role="group"`)
	builder.WriteString(`>`)

	if label := strings.TrimSpace(field.Label); label != "" {
		builder.WriteString(`<div class="text-sm font-medium text-gray-900 dark:text-white">`)
		builder.WriteString(html.EscapeString(label))
		builder.WriteString(`</div>`)
	}
	if desc := strings.TrimSpace(field.Description); desc != "" {
		builder.WriteString(`<p class="text-xs text-gray-500 dark:text-gray-400">`)
		builder.WriteString(html.EscapeString(desc))
		builder.WriteString(`</p>`)
	}
	if hint := strings.TrimSpace(field.UIHints["helpText"]); hint != "" {
		builder.WriteString(`<p class="text-xs text-gray-600 dark:text-gray-300">`)
		builder.WriteString(html.EscapeString(hint))
		builder.WriteString(`</p>`)
	}
	if len(field.Nested) == 0 || data.RenderChild == nil {
		builder.WriteString(`<p class="text-sm text-red-600 dark:text-red-400">`)
		builder.WriteString(`Datetime range "`)
		builder.WriteString(html.EscapeString(field.Name))
		builder.WriteString(`" requires nested start/end fields.`)
		builder.WriteString(`</p>`)
	} else {
		builder.WriteString(`<div class="grid gap-3 sm:grid-cols-2">`)
		for _, nested := range field.Nested {
			child, err := data.RenderChild(nested)
			if err != nil {
				return err
			}
			builder.WriteString(child)
		}
		builder.WriteString(`</div>`)
	}
	builder.WriteString(`</div>`)
	buf.WriteString(builder.String())
	return nil
}

func writeRelationshipAttributes(builder *strings.Builder, rel *model.Relationship) {
	if rel == nil {
		return
	}
	if rel.Kind != "" {
		builder.WriteString(` data-relationship-type="`)
		builder.WriteString(html.EscapeString(string(rel.Kind)))
		builder.WriteString(`"`)
	}
	if rel.Target != "" {
		builder.WriteString(` data-relationship-target="`)
		builder.WriteString(html.EscapeString(rel.Target))
		builder.WriteString(`"`)
	}
	if rel.ForeignKey != "" {
		builder.WriteString(` data-relationship-foreign-key="`)
		builder.WriteString(html.EscapeString(rel.ForeignKey))
		builder.WriteString(`"`)
	}
	if rel.Cardinality != "" {
		builder.WriteString(` data-relationship-cardinality="`)
		builder.WriteString(html.EscapeString(rel.Cardinality))
		builder.WriteString(`"`)
	}
	if rel.Inverse != "" {
		builder.WriteString(` data-relationship-inverse="`)
		builder.WriteString(html.EscapeString(rel.Inverse))
		builder.WriteString(`"`)
	}
}

const (
	controlIDMetadataKey   = "control.id"
	controlNameMetadataKey = "control.name"
	controlOmitNameKey     = "control.omitName"
	controlPathMetadataKey = "control.path"
)

func componentControlID(field model.Field) string {
	if field.Metadata != nil {
		if id := strings.TrimSpace(field.Metadata[controlIDMetadataKey]); id != "" {
			return id
		}
	}
	trimmed := strings.TrimSpace(field.Name)
	if trimmed == "" {
		return ""
	}
	return "fg-" + trimmed
}

func componentControlPath(field model.Field) string {
	if field.Metadata != nil {
		if path := strings.TrimSpace(field.Metadata[controlPathMetadataKey]); path != "" {
			return path
		}
		if name := strings.TrimSpace(field.Metadata[controlNameMetadataKey]); name != "" {
			return name
		}
	}
	return strings.TrimSpace(field.Name)
}

func componentLabelID(field model.Field) string {
	controlID := componentControlID(field)
	if controlID == "" {
		return ""
	}
	return controlID + "-label"
}

func sanitizeClassList(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	tokens := strings.Fields(value)
	keep := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "fg-") {
			continue
		}
		keep = append(keep, token)
	}
	return strings.Join(keep, " ")
}

func coerceSlice(value any) []any {
	if value == nil {
		return nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

func ApplyArrayItemValue(field model.Field, value any, applyValue FieldValueApplier) model.Field {
	if shouldApplyDirectItemValue(field) {
		return applyDirectItemValue(field, value, applyValue)
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(field.Nested) > 0 {
			field.Nested = applyValuesToNested(field.Nested, typed, applyValue)
			return field
		}
	case map[string]string:
		if len(field.Nested) > 0 {
			coerced := make(map[string]any, len(typed))
			for key, val := range typed {
				coerced[key] = val
			}
			field.Nested = applyValuesToNested(field.Nested, coerced, applyValue)
			return field
		}
	}
	return applyDirectItemValue(field, value, applyValue)
}

func shouldApplyDirectItemValue(field model.Field) bool {
	return field.Relationship != nil || field.Type == model.FieldTypeArray || len(field.Nested) == 0
}

func applyDirectItemValue(field model.Field, value any, applyValue FieldValueApplier) model.Field {
	if applyValue != nil {
		return applyValue(field, value)
	}
	field.Default = value
	return field
}

func applyValuesToNested(fields []model.Field, values map[string]any, applyValue FieldValueApplier) []model.Field {
	if len(fields) == 0 || len(values) == 0 {
		return fields
	}
	for i := range fields {
		if value, ok := values[fields[i].Name]; ok {
			fields[i] = ApplyArrayItemValue(fields[i], value, applyValue)
		}
	}
	return fields
}

func arrayItemControlPath(field model.Field, idx int) string {
	path := componentControlPath(field)
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s[%d]", path, idx)
}

func applyControlPath(field *model.Field, path string) {
	if field == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if field.Metadata == nil {
		field.Metadata = make(map[string]string, 3)
	}
	if strings.TrimSpace(field.Metadata[controlPathMetadataKey]) == "" {
		field.Metadata[controlPathMetadataKey] = path
	}
	if strings.TrimSpace(field.Metadata[controlNameMetadataKey]) == "" {
		field.Metadata[controlNameMetadataKey] = path
	}
	if strings.TrimSpace(field.Metadata[controlIDMetadataKey]) == "" {
		field.Metadata[controlIDMetadataKey] = controlIDFromPath(path)
	}
	if len(field.Nested) > 0 {
		for i := range field.Nested {
			childPath := joinControlPath(path, field.Nested[i].Name)
			applyControlPath(&field.Nested[i], childPath)
		}
	}
}

func applyPrototypeControlPath(field *model.Field, path string) {
	applyControlPath(field, path)
	markPrototypeControlSuppressed(field)
}

func markPrototypeControlSuppressed(field *model.Field) {
	if field == nil {
		return
	}
	if field.Metadata == nil {
		field.Metadata = make(map[string]string, 2)
	}
	field.Metadata[controlOmitNameKey] = "true"
	field.Metadata["disabled"] = "true"
	field.Disabled = true
	for i := range field.Nested {
		markPrototypeControlSuppressed(&field.Nested[i])
	}
	if field.Items != nil {
		markPrototypeControlSuppressed(field.Items)
	}
}

func markPrototypeControlDisabled(field *model.Field) {
	if field == nil {
		return
	}
	if field.Metadata == nil {
		field.Metadata = make(map[string]string, 1)
	}
	field.Metadata["disabled"] = "true"
	field.Disabled = true
	for i := range field.Nested {
		markPrototypeControlDisabled(&field.Nested[i])
	}
	if field.Items != nil {
		markPrototypeControlDisabled(field.Items)
	}
}

func joinControlPath(parentPath, name string) string {
	parentPath = strings.TrimSpace(parentPath)
	name = strings.TrimSpace(name)
	if parentPath == "" {
		return name
	}
	if name == "" {
		return parentPath
	}
	return parentPath + "." + name
}

func controlIDFromPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "[]", ".item")
	return "fg-" + sanitizeID(path)
}

func sanitizeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func cloneField(field model.Field) model.Field {
	cloned := field
	if len(field.Enum) > 0 {
		cloned.Enum = append([]any(nil), field.Enum...)
	}
	if len(field.Validations) > 0 {
		cloned.Validations = append([]model.ValidationRule(nil), field.Validations...)
	}
	if field.Metadata != nil {
		cloned.Metadata = make(map[string]string, len(field.Metadata))
		maps.Copy(cloned.Metadata, field.Metadata)
	}
	if field.UIHints != nil {
		cloned.UIHints = make(map[string]string, len(field.UIHints))
		maps.Copy(cloned.UIHints, field.UIHints)
	}
	if field.Relationship != nil {
		rel := *field.Relationship
		cloned.Relationship = &rel
	}
	if field.Items != nil {
		item := cloneField(*field.Items)
		cloned.Items = &item
	}
	if len(field.Nested) > 0 {
		cloned.Nested = make([]model.Field, len(field.Nested))
		for i := range field.Nested {
			cloned.Nested[i] = cloneField(field.Nested[i])
		}
	}
	if len(field.OneOf) > 0 {
		cloned.OneOf = make([]model.Field, len(field.OneOf))
		for i := range field.OneOf {
			cloned.OneOf[i] = cloneField(field.OneOf[i])
		}
	}
	return cloned
}
