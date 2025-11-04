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
		Renderer: templateComponentRenderer(templatePrefix + "input.tmpl"),
	})
	registry.MustRegister("textarea", Descriptor{
		Renderer: templateComponentRenderer(templatePrefix + "textarea.tmpl"),
	})
	registry.MustRegister("select", Descriptor{
		Renderer: templateComponentRenderer(templatePrefix + "select.tmpl"),
	})
	registry.MustRegister("boolean", Descriptor{
		Renderer: templateComponentRenderer(templatePrefix + "boolean.tmpl"),
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

	return registry
}

func templateComponentRenderer(templateName string) Renderer {
	return func(buf *bytes.Buffer, field model.Field, data ComponentData) error {
		if data.Template == nil {
			return fmt.Errorf("components: template renderer not configured for %q", templateName)
		}

		payload := map[string]any{
			"field":  field,
			"config": data.Config,
		}
		rendered, err := data.Template.RenderTemplate(templateName, payload)
		if err != nil {
			return fmt.Errorf("components: render template %q: %w", templateName, err)
		}
		buf.WriteString(rendered)
		return nil
	}
}

func objectRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	builder.WriteString(`<fieldset class="grid gap-4 rounded-lg border border-gray-200 p-4`)
	if strings.TrimSpace(field.UIHints["accordion"]) == "true" {
		builder.WriteString(` border-l-4 border-l-blue-600`)
	}
	builder.WriteString(`">`)

	if label := strings.TrimSpace(field.Label); label != "" {
		builder.WriteString(`<legend class="text-sm font-medium">`)
		builder.WriteString(html.EscapeString(label))
		builder.WriteString(`</legend>`)
	}

	if data.RenderChild != nil {
		for _, nested := range field.Nested {
			html, err := data.RenderChild(nested)
			if err != nil {
				return err
			}
			builder.WriteString(html)
		}
	}

	builder.WriteString(`</fieldset>`)
	buf.WriteString(builder.String())
	return nil
}

func arrayRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	builder.WriteString(`<div class="grid gap-3`)
	if cls := strings.TrimSpace(field.UIHints["cssClass"]); cls != "" {
		builder.WriteByte(' ')
		builder.WriteString(html.EscapeString(cls))
	}
	builder.WriteString(`">`)

	if label := strings.TrimSpace(field.UIHints["repeaterLabel"]); label != "" {
		builder.WriteString(`<div class="text-sm font-medium">`)
		builder.WriteString(html.EscapeString(label))
		builder.WriteString(`</div>`)
	}

	if field.Items != nil {
		cardinality := strings.TrimSpace(field.UIHints["cardinality"])
		builder.WriteString(`<div class="grid gap-2"`)
		if cardinality != "" {
			builder.WriteString(` data-relationship-collection="`)
			builder.WriteString(html.EscapeString(cardinality))
			builder.WriteString(`"`)
		}
		builder.WriteString(`>`)

		if data.RenderChild != nil {
			html, err := data.RenderChild(*field.Items)
			if err != nil {
				return err
			}
			builder.WriteString(html)
		}

		if cardinality == "many" {
			builder.WriteString(`<button type="button" class="px-4 py-2 text-sm font-medium rounded-md bg-gray-100 text-gray-700 hover:bg-gray-200" data-relationship-action="add">Add `)
			if label := strings.TrimSpace(field.UIHints["repeaterLabel"]); label != "" {
				builder.WriteString(html.EscapeString(label))
			} else if field.Label != "" {
				builder.WriteString(html.EscapeString(field.Label))
			} else {
				builder.WriteString("item")
			}
			builder.WriteString(`</button>`)
		}

		builder.WriteString(`</div>`)
	} else {
		builder.WriteString(`<p class="text-sm text-gray-500">Array field `)
		builder.WriteString(html.EscapeString(field.Name))
		builder.WriteString(` requires item definition.</p>`)
	}

	builder.WriteString(`</div>`)
	buf.WriteString(builder.String())
	return nil
}

func datetimeRangeRenderer(buf *bytes.Buffer, field model.Field, data ComponentData) error {
	var builder strings.Builder
	builder.WriteString(`<div class="grid gap-4">`)
	if len(field.Nested) == 0 {
		builder.WriteString(`<p class="text-sm text-gray-500">Datetime range "`)
		builder.WriteString(html.EscapeString(field.Name))
		builder.WriteString(`" requires nested start/end fields.</p>`)
	} else if data.RenderChild != nil {
		for _, nested := range field.Nested {
			html, err := data.RenderChild(nested)
			if err != nil {
				return err
			}
			builder.WriteString(html)
		}
	}
	builder.WriteString(`</div>`)
	buf.WriteString(builder.String())
	return nil
}
