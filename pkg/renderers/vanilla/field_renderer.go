package vanilla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/render/template"
	"github.com/goliatone/formgen/pkg/renderers/vanilla/components"
)

type componentRenderer struct {
	templates template.TemplateRenderer
	registry  *components.Registry
	overrides map[string]string

	usedComponents map[string]struct{}
}

func newComponentRenderer(templates template.TemplateRenderer, registry *components.Registry, overrides map[string]string) *componentRenderer {
	if registry == nil {
		registry = components.NewDefaultRegistry()
	}
	return &componentRenderer{
		templates:      templates,
		registry:       registry,
		overrides:      cloneStringMap(overrides),
		usedComponents: make(map[string]struct{}),
	}
}

func (r *componentRenderer) render(field model.Field, path string) (string, error) {
	if skipRelationshipSource(field) {
		return "", nil
	}

	componentName := r.overrideFor(path, field.Name)
	if componentName == "" {
		componentName = resolveComponentName(field)
	}
	if componentName == "" {
		componentName = "input"
	}

	descriptor, ok := r.registry.Descriptor(componentName)
	if !ok {
		return "", fmt.Errorf("component %q not registered for field %q", componentName, path)
	}

	config, err := parseComponentConfig(stringFromMap(field.Metadata, componentConfigMetadataKey))
	if err != nil {
		return "", fmt.Errorf("parse component config for field %q: %w", path, err)
	}

	data := components.ComponentData{
		Template:    r.templates,
		Config:      config,
		RenderChild: r.childRenderer(path),
	}

	var control bytes.Buffer
	if err := descriptor.Renderer(&control, field, data); err != nil {
		return "", fmt.Errorf("render component %q for field %q: %w", componentName, path, err)
	}

	r.usedComponents[componentName] = struct{}{}

	return buildFieldMarkup(field, componentName, control.String()), nil
}

func (r *componentRenderer) childRenderer(parentPath string) func(any) (string, error) {
	return func(value any) (string, error) {
		field, err := coerceField(value)
		if err != nil {
			return "", err
		}
		path := joinPath(parentPath, field.Name)
		return r.render(field, path)
	}
}

func (r *componentRenderer) assets() (stylesheets []string, scripts []components.Script) {
	if r.registry == nil || len(r.usedComponents) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(r.usedComponents))
	for name := range r.usedComponents {
		names = append(names, name)
	}
	slices.Sort(names)
	return r.registry.Assets(names)
}

func (r *componentRenderer) overrideFor(path, name string) string {
	if len(r.overrides) == 0 {
		return ""
	}
	if value := r.overrides[path]; value != "" {
		return value
	}
	return r.overrides[name]
}

func skipRelationshipSource(field model.Field) bool {
	return field.Relationship != nil && strings.TrimSpace(field.Relationship.SourceField) != ""
}

func buildFieldMarkup(field model.Field, componentName, control string) string {
	var builder strings.Builder
	builder.Grow(len(control) + 256)

	builder.WriteString(`<div class="grid gap-2`)
	if cls := field.UIHints["cssClass"]; cls != "" {
		builder.WriteByte(' ')
		builder.WriteString(html.EscapeString(cls))
	} else if cls := field.UIHints["class"]; cls != "" {
		builder.WriteByte(' ')
		builder.WriteString(html.EscapeString(cls))
	}
	builder.WriteString(`"`)

	if componentName != "" {
		builder.WriteString(` data-component="`)
		builder.WriteString(html.EscapeString(componentName))
		builder.WriteString(`"`)
	}

	if config := stringFromMap(field.Metadata, componentConfigMetadataKey); config != "" {
		builder.WriteString(` data-component-config='`)
		builder.WriteString(html.EscapeString(config))
		builder.WriteString(`'`)
	}

	if rel := field.Relationship; rel != nil {
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

	builder.WriteString(">\n")

	if shouldRenderLabel(field) {
		builder.WriteString(`    <label for="fg-`)
		builder.WriteString(html.EscapeString(field.Name))
		builder.WriteString(`" class="text-sm font-medium text-gray-900">`)
		builder.WriteString(html.EscapeString(field.Label))
		if field.Required {
			builder.WriteString(` *`)
		}
		builder.WriteString(`</label>
`)
	}

	if control != "" {
		for _, line := range strings.Split(control, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			builder.WriteString("    ")
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}

	if desc := strings.TrimSpace(field.Description); desc != "" {
		builder.WriteString(`    <small class="text-sm text-gray-500">`)
		builder.WriteString(html.EscapeString(desc))
		builder.WriteString(`</small>
`)
	}

	if hint := strings.TrimSpace(field.UIHints["helpText"]); hint != "" {
		builder.WriteString(`    <small class="text-sm text-gray-600">`)
		builder.WriteString(html.EscapeString(hint))
		builder.WriteString(`</small>
`)
	}

	builder.WriteString("</div>\n")
	return builder.String()
}

func shouldRenderLabel(field model.Field) bool {
	if strings.TrimSpace(field.Label) == "" {
		return false
	}
	return strings.TrimSpace(field.UIHints["hideLabel"]) != "true"
}

func parseComponentConfig(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func coerceField(value any) (model.Field, error) {
	switch v := value.(type) {
	case nil:
		return model.Field{}, fmt.Errorf("nil field value")
	case model.Field:
		return v, nil
	case *model.Field:
		if v == nil {
			return model.Field{}, fmt.Errorf("nil field pointer")
		}
		return *v, nil
	case map[string]any:
		var field model.Field
		payload, err := json.Marshal(v)
		if err != nil {
			return model.Field{}, fmt.Errorf("marshal field map: %w", err)
		}
		if err := json.Unmarshal(payload, &field); err != nil {
			return model.Field{}, fmt.Errorf("unmarshal field map: %w", err)
		}
		return field, nil
	default:
		return model.Field{}, fmt.Errorf("unsupported field type %T", value)
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func joinPath(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}
