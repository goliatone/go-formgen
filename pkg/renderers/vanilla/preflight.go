package vanilla

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
)

// Preflight diagnostic codes. Paths are dotted field paths with `[]` for array
// items (`rows[].answer`); messages name components and causes, never values.
const (
	DiagnosticComponentUnknown = "componentUnknown"
	DiagnosticComponentConfig  = "componentConfig"
	DiagnosticComponentRender  = "componentRender"
)

// Preflight reports renderer-aware definition diagnostics for a compiled form
// against the components registered on this renderer. Call it after component
// registration: the renderer clones its registry at construction, so components
// registered later on the source registry are not visible here or to Render.
//
// Preflight resolves each field's component exactly as Render does (explicit UI
// schema component, inline `component.name`, widget aliases, then field type),
// reports unregistered names, malformed `component.config`, and components that
// reject the field (for example `scale` without integer bounds). It renders
// nothing to a page, leaves headless compilation untouched, ignores host
// metadata namespaces, and never includes answer values. Render keeps failing
// for unregistered explicit components exactly as before.
func (r *Renderer) Preflight(form model.FormModel) []model.DefinitionDiagnostic {
	if r == nil {
		return nil
	}
	decorated := decorateFormModel(form)
	renderer := newComponentRenderer(r.templates, r.components, r.overrides, rendererTheme{}, nil, renderStyleDefault)
	var out []model.DefinitionDiagnostic
	r.preflightFields(renderer, decorated.Fields, "", &out)
	return out
}

func (r *Renderer) preflightFields(renderer *componentRenderer, fields []model.Field, parentPath string, out *[]model.DefinitionDiagnostic) {
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		path := name
		if parentPath != "" {
			path = parentPath + "." + name
		}
		r.preflightField(renderer, field, path, out)
	}
}

func (r *Renderer) preflightField(renderer *componentRenderer, field model.Field, path string, out *[]model.DefinitionDiagnostic) {
	if skipRelationshipSource(field) {
		return
	}
	field = preflightFieldMetadata(field, path)
	componentName := r.preflightComponentName(renderer, field, path)
	descriptor, registered := r.components.Descriptor(componentName)
	if !registered {
		*out = append(*out, model.DefinitionDiagnostic{
			Code:    DiagnosticComponentUnknown,
			Path:    path,
			Message: fmt.Sprintf("component %q is not registered with the vanilla renderer", componentName),
		})
	}
	config, configErr := parseComponentConfig(stringFromMap(field.Metadata, componentConfigMetadataKey))
	if configErr != nil {
		*out = append(*out, model.DefinitionDiagnostic{
			Code:    DiagnosticComponentConfig,
			Path:    path,
			Message: "component.config must be a JSON object",
		})
	}
	if registered && configErr == nil && !preflightContainer(componentName) {
		r.preflightDryRender(renderer, descriptor, componentName, field, path, config, out)
	}
	// Only the object and array components render children through the shared
	// field pipeline; a host component owning a container field renders its
	// children itself (surfaced by the dry render), so recursing would report
	// host-only child component names Render never resolves.
	if preflightContainer(componentName) {
		r.preflightChildren(renderer, field, path, out)
	}
}

func preflightFieldMetadata(field model.Field, path string) model.Field {
	field = applyRenderPathMetadata(field, path)
	if field.Metadata == nil {
		field.Metadata = map[string]string{}
	}
	if strings.TrimSpace(field.Metadata[controlIDMetadataKey]) == "" {
		field.Metadata[controlIDMetadataKey] = buildControlIDFromPath(path)
	}
	return field
}

func (r *Renderer) preflightComponentName(renderer *componentRenderer, field model.Field, path string) string {
	componentName := renderer.overrideFor(path, field.Name)
	if componentName == "" {
		componentName = resolveComponentName(field)
	}
	if componentName == "" {
		componentName = components.NameInput
	}
	return componentName
}

// Containers surface their children's diagnostics through recursion; a dry
// render of the children would duplicate them.
func preflightContainer(componentName string) bool {
	return componentName == components.NameObject || componentName == components.NameArray
}

func (r *Renderer) preflightDryRender(renderer *componentRenderer, descriptor components.Descriptor, componentName string, field model.Field, path string, config map[string]any, out *[]model.DefinitionDiagnostic) {
	data := components.ComponentData{
		Template:    r.templates,
		Config:      config,
		RenderChild: renderer.childRenderer(path),
		Theme:       renderer.templateTheme,
		StyleMode:   string(renderStyleDefault),
	}
	var scratch bytes.Buffer
	if renderErr := descriptor.Renderer(&scratch, field, data); renderErr != nil {
		*out = append(*out, model.DefinitionDiagnostic{
			Code:    DiagnosticComponentRender,
			Path:    path,
			Message: fmt.Sprintf("component %q rejected the field: %s", componentName, strings.TrimPrefix(renderErr.Error(), "components: ")),
		})
	}
}

func (r *Renderer) preflightChildren(renderer *componentRenderer, field model.Field, path string, out *[]model.DefinitionDiagnostic) {
	if len(field.Nested) > 0 {
		r.preflightFields(renderer, field.Nested, path, out)
	}
	if field.Items != nil {
		item := *field.Items
		if strings.TrimSpace(item.Name) == "" {
			item.Name = field.Name
		}
		r.preflightItem(renderer, item, path+"[]", out)
	}
}

func (r *Renderer) preflightItem(renderer *componentRenderer, item model.Field, path string, out *[]model.DefinitionDiagnostic) {
	// Array items render through the array component with reindexed control
	// paths; preflight checks the item definition once under the `[]` path.
	r.preflightField(renderer, item, path, out)
}
