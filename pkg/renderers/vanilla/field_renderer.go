package vanilla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render/template"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
)

type componentRenderer struct {
	templates template.TemplateRenderer
	registry  *components.Registry
	overrides map[string]string

	usedComponents map[string]struct{}
	theme          rendererTheme
	templateTheme  map[string]any
	assetResolver  func(string) string
}

const (
	chromeTemplatePrefix       = "templates/components/chrome/"
	chromeLabelTemplate        = chromeTemplatePrefix + "_label.tmpl"
	chromeDescriptionTemplate  = chromeTemplatePrefix + "_description.tmpl"
	chromeHelpTemplate         = chromeTemplatePrefix + "_help.tmpl"
	controlIDPrefix            = "fg-"
	descriptionIDSuffix        = "-description"
	helpIDSuffix               = "-help"
	componentChromeSkipKeyword = "skip"
)

func newComponentRenderer(templates template.TemplateRenderer, registry *components.Registry, overrides map[string]string, theme rendererTheme, assetResolver func(string) string) *componentRenderer {
	if registry == nil {
		registry = components.NewDefaultRegistry()
	}
	return &componentRenderer{
		templates:      templates,
		registry:       registry,
		overrides:      cloneStringMap(overrides),
		usedComponents: make(map[string]struct{}),
		theme:          theme,
		templateTheme:  buildTemplateThemeContext(theme, assetResolver),
		assetResolver:  assetResolver,
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
		componentName = components.NameInput
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
		Template:      r.templates,
		Config:        config,
		RenderChild:   r.childRenderer(path),
		ThemePartials: r.theme.Partials,
		Theme:         r.templateTheme,
	}

	var control bytes.Buffer
	if err := descriptor.Renderer(&control, field, data); err != nil {
		return "", fmt.Errorf("render component %q for field %q: %w", componentName, path, err)
	}

	r.usedComponents[componentName] = struct{}{}

	return buildFieldMarkup(r.templates, field, componentName, control.String()), nil
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
	styles, scripts := r.registry.Assets(names)
	return resolveAssetDependencies(styles, scripts, r.assetResolver)
}

func resolveAssetDependencies(styles []string, scripts []components.Script, resolver func(string) string) ([]string, []components.Script) {
	if resolver == nil {
		return styles, scripts
	}
	resolvedStyles := resolveAssets(styles, resolver)
	resolvedScripts := make([]components.Script, len(scripts))
	for i, script := range scripts {
		resolvedScripts[i] = resolveScriptAsset(script, resolver)
	}
	return resolvedStyles, resolvedScripts
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

func buildFieldMarkup(templates template.TemplateRenderer, field model.Field, componentName, control string) string {
	if strings.TrimSpace(control) == "" {
		return ""
	}
	if shouldSkipChrome(field) {
		return control
	}

	var builder strings.Builder
	builder.Grow(len(control) + 256)

	builder.WriteString(`<div class="flex flex-col gap-2`)
	if extra := sanitizedWrapperClass(field); extra != "" {
		builder.WriteByte(' ')
		builder.WriteString(html.EscapeString(extra))
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
	if prov := strings.TrimSpace(stringFromMap(field.Metadata, "prefill.provenance")); prov != "" {
		builder.WriteString(` data-prefill-provenance="`)
		builder.WriteString(html.EscapeString(prov))
		builder.WriteString(`"`)
	}
	if strings.TrimSpace(stringFromMap(field.Metadata, "prefill.readonly")) == "true" {
		builder.WriteString(` data-prefill-readonly="true"`)
	}
	if strings.TrimSpace(stringFromMap(field.Metadata, "prefill.disabled")) == "true" {
		builder.WriteString(` data-prefill-disabled="true"`)
	}

	builder.WriteString(">\n")

	context := buildChromeContext(field, componentName)

	skipChrome := componentHandlesChrome(componentName)

	if !skipChrome && shouldRenderLabel(field) {
		label := renderChromePartial(templates, chromeLabelTemplate, field, context, fallbackLabelMarkup)
		writeIndentedBlock(&builder, label)
	}

	writeIndentedBlock(&builder, control)

	if !skipChrome && !componentHandlesDescription(componentName) && strings.TrimSpace(field.Description) != "" {
		description := renderChromePartial(templates, chromeDescriptionTemplate, field, context, fallbackDescriptionMarkup)
		writeIndentedBlock(&builder, description)
	}

	if !skipChrome && strings.TrimSpace(stringFromMap(field.UIHints, "helpText")) != "" {
		help := renderChromePartial(templates, chromeHelpTemplate, field, context, fallbackHelpMarkup)
		writeIndentedBlock(&builder, help)
	}

	if !skipChrome {
		builder.WriteString(`    <p data-relationship-error="true" role="status" aria-live="polite" aria-atomic="true" class="formgen-error text-sm text-red-600 dark:text-red-400">`)
		if message := fieldErrorMessage(field); message != "" {
			builder.WriteString(html.EscapeString(message))
		}
		builder.WriteString(`</p>`)
		builder.WriteByte('\n')
	}

	builder.WriteString("</div>\n")
	return builder.String()
}

func renderChromePartial(renderer template.TemplateRenderer, templateName string, field model.Field, context map[string]any, fallback func(model.Field, map[string]any) string) string {
	if renderer != nil {
		payload := map[string]any{
			"field":   field,
			"context": context,
		}
		if rendered, err := renderer.RenderTemplate(templateName, payload); err == nil {
			return strings.TrimSpace(rendered)
		}
	}
	if fallback == nil {
		return ""
	}
	return strings.TrimSpace(fallback(field, context))
}

func buildChromeContext(field model.Field, componentName string) map[string]any {
	controlID := fieldControlID(field)
	context := map[string]any{
		"controlID": controlID,
	}
	if isLabelVisuallyHidden(field) {
		context["visuallyHiddenLabel"] = true
	}

	if labelID := fieldLabelID(field); labelID != "" {
		context["labelID"] = labelID
	}
	if labelSupportsFor(componentName) && controlID != "" {
		context["labelTarget"] = controlID
	}

	if strings.TrimSpace(field.Description) != "" && controlID != "" {
		context["descriptionID"] = controlID + descriptionIDSuffix
	}
	if strings.TrimSpace(stringFromMap(field.UIHints, "helpText")) != "" && controlID != "" {
		context["helpID"] = controlID + helpIDSuffix
	}

	return context
}

func fieldControlID(field model.Field) string {
	if id := strings.TrimSpace(stringFromMap(field.Metadata, controlIDMetadataKey)); id != "" {
		return id
	}
	return buildControlID(field.Name)
}

func fieldLabelID(field model.Field) string {
	controlID := fieldControlID(field)
	if controlID == "" {
		return ""
	}
	return controlID + "-label"
}

func buildControlID(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return controlIDPrefix + trimmed
}

func writeIndentedBlock(builder *strings.Builder, block string) {
	if strings.TrimSpace(block) == "" {
		return
	}
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
}

func fallbackLabelMarkup(field model.Field, context map[string]any) string {
	if strings.TrimSpace(field.Label) == "" {
		return ""
	}
	labelID, _ := context["labelID"].(string)
	labelTarget, _ := context["labelTarget"].(string)
	hidden, _ := context["visuallyHiddenLabel"].(bool)

	var builder strings.Builder
	builder.WriteString(`<label data-formgen-chrome="label"`)
	if labelID != "" {
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(labelID))
		builder.WriteString(`"`)
	}
	if labelTarget != "" {
		builder.WriteString(` for="`)
		builder.WriteString(html.EscapeString(labelTarget))
		builder.WriteString(`"`)
	}
	builder.WriteString(` class="text-sm font-medium text-gray-900 dark:text-white inline-flex items-center gap-1`)
	if hidden {
		builder.WriteString(` sr-only`)
	}
	builder.WriteString(`">`)
	builder.WriteString(html.EscapeString(field.Label))
	if field.Required {
		builder.WriteString(`<span class="text-red-500">*</span>`)
	}
	builder.WriteString(`</label>`)
	return builder.String()
}

func fallbackDescriptionMarkup(field model.Field, _ map[string]any) string {
	if desc := strings.TrimSpace(field.Description); desc != "" {
		var builder strings.Builder
		builder.WriteString(`<p data-formgen-chrome="description" class="text-xs text-gray-500 dark:text-gray-400">`)
		builder.WriteString(html.EscapeString(desc))
		builder.WriteString(`</p>`)
		return builder.String()
	}
	return ""
}

func fallbackHelpMarkup(field model.Field, _ map[string]any) string {
	if hint := strings.TrimSpace(stringFromMap(field.UIHints, "helpText")); hint != "" {
		var builder strings.Builder
		builder.WriteString(`<p data-formgen-chrome="help" class="text-xs text-gray-600 dark:text-gray-300">`)
		builder.WriteString(html.EscapeString(hint))
		builder.WriteString(`</p>`)
		return builder.String()
	}
	return ""
}

func shouldSkipChrome(field model.Field) bool {
	value := strings.TrimSpace(strings.ToLower(stringFromMap(field.Metadata, componentChromeMetadataKey)))
	return value == componentChromeSkipKeyword
}

func sanitizedWrapperClass(field model.Field) string {
	if field.UIHints == nil {
		return ""
	}
	if value := sanitizeClassList(field.UIHints["cssClass"]); value != "" {
		return value
	}
	if value := sanitizeClassList(field.UIHints["class"]); value != "" {
		return value
	}
	return ""
}

func shouldRenderLabel(field model.Field) bool {
	if strings.TrimSpace(field.Label) == "" {
		return false
	}
	if strings.TrimSpace(field.UIHints["inputType"]) == "hidden" {
		return false
	}
	return true
}

func isLabelVisuallyHidden(field model.Field) bool {
	return strings.TrimSpace(field.UIHints["hideLabel"]) == "true"
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

func fieldErrorMessage(field model.Field) string {
	if len(field.Metadata) == 0 {
		return ""
	}
	return strings.TrimSpace(field.Metadata["validation.message"])
}
