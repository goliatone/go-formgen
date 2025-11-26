package components

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
)

const (
	templatePrefix = "templates/components/"
)

// NewDefaultRegistry constructs a registry pre-populated with the built-in
// components used by the vanilla renderer.
func NewDefaultRegistry() *Registry {
	registry := New()

	registry.MustRegister("input", Descriptor{
		Renderer: templateComponentRenderer("forms.input", templatePrefix+"input.tmpl"),
	})
	registry.MustRegister("textarea", Descriptor{
		Renderer: templateComponentRenderer("forms.textarea", templatePrefix+"textarea.tmpl"),
	})
	registry.MustRegister("select", Descriptor{
		Renderer: templateComponentRenderer("forms.select", templatePrefix+"select.tmpl"),
	})
	registry.MustRegister("boolean", Descriptor{
		Renderer: templateComponentRenderer("forms.checkbox", templatePrefix+"boolean.tmpl"),
	})
	registry.MustRegister("object", Descriptor{
		Renderer: objectRenderer,
	})
	registry.MustRegister("array", Descriptor{
		Renderer: arrayRenderer,
	})
	registry.MustRegister("datetime-range", Descriptor{
		Renderer: datetimeRangeRenderer,
	})
	registry.MustRegister("wysiwyg", Descriptor{
		Renderer: templateComponentRenderer("forms.wysiwyg", templatePrefix+"wysiwyg.tmpl"),
	})
	registry.MustRegister("file_uploader", Descriptor{
		Renderer: templateComponentRenderer("forms.file-uploader", templatePrefix+"file_uploader.tmpl"),
	})

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
			"field":  field,
			"config": data.Config,
		}
		rendered, err := data.Template.RenderTemplate(resolvedTemplate, payload)
		if err != nil {
			return fmt.Errorf("components: render template %q: %w", templateName, err)
		}
		buf.WriteString(rendered)
		return nil
	}
}

func objectRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
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
	if field.UIHints != nil {
		if extra := sanitizeClassList(field.UIHints["cssClass"]); extra != "" {
			classes = append(classes, extra)
		}
		if extra := sanitizeClassList(field.UIHints["class"]); extra != "" {
			classes = append(classes, extra)
		}
	}

	builder.WriteString(`<fieldset`)
	if id := componentControlID(field.Name); id != "" {
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(id))
		builder.WriteString(`"`)
	}
	builder.WriteString(` class="`)
	builder.WriteString(html.EscapeString(strings.Join(classes, " ")))
	builder.WriteString(`"`)
	writeRelationshipAttributes(&builder, field.Relationship)
	labelID := ""
	if strings.TrimSpace(field.Label) != "" {
		labelID = componentLabelID(field.Name)
	}
	if labelID != "" {
		builder.WriteString(` aria-labelledby="`)
		builder.WriteString(html.EscapeString(labelID))
		builder.WriteString(`"`)
	}
	builder.WriteString(`>`)

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

	builder.WriteString(`</fieldset>`)
	buf.WriteString(builder.String())
	return nil
}

func arrayRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	label := strings.TrimSpace(field.Label)
	labelID := ""
	if label != "" {
		labelID = componentLabelID(field.Name)
	}
	builder.WriteString(`<div`)
	if id := componentControlID(field.Name); id != "" {
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
		if labelID != "" {
			builder.WriteString(` id="`)
			builder.WriteString(html.EscapeString(labelID))
			builder.WriteString(`"`)
		}
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
		cardinality := strings.TrimSpace(field.UIHints["cardinality"])
		builder.WriteString(`<div class="space-y-3"`)
		if cardinality != "" {
			builder.WriteString(` data-relationship-collection="`)
			builder.WriteString(html.EscapeString(cardinality))
			builder.WriteString(`"`)
		}
		builder.WriteString(`>`)

		child, err := data.RenderChild(*field.Items)
		if err != nil {
			return err
		}
		builder.WriteString(child)
		builder.WriteString(`</div>`)

		if cardinality == "many" {
			builder.WriteString(`<button type="button" class="py-3 px-4 inline-flex items-center gap-x-2 text-sm font-medium rounded-lg border border-gray-200 bg-white text-gray-800 shadow-sm hover:bg-gray-50 disabled:opacity-50 disabled:pointer-events-none dark:bg-slate-900 dark:border-gray-700 dark:text-white dark:hover:bg-gray-800" data-relationship-action="add">`)
			builder.WriteString(`Add `)
			if label := strings.TrimSpace(field.UIHints["repeaterLabel"]); label != "" {
				builder.WriteString(html.EscapeString(label))
			} else if field.Label != "" {
				builder.WriteString(html.EscapeString(field.Label))
			} else {
				builder.WriteString("item")
			}
			builder.WriteString(`</button>`)
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

func datetimeRangeRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	builder.WriteString(`<div`)
	if id := componentControlID(field.Name); id != "" {
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

func componentControlID(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return "fg-" + trimmed
}

func componentLabelID(name string) string {
	controlID := componentControlID(name)
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
