package vanilla

import (
	"io"
	"testing"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/renderers/vanilla/components"
)

func TestComponentRendererUnknownComponent(t *testing.T) {
	renderer := newComponentRenderer(nil, components.NewDefaultRegistry(), map[string]string{
		"field": "missing",
	}, rendererTheme{}, nil)

	_, err := renderer.render(model.Field{Name: "field"}, "field")
	if err == nil {
		t.Fatalf("expected error when component is missing")
	}

	if got := err.Error(); got != `component "missing" not registered for field "field"` {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestComponentRendererUsesThemePartial(t *testing.T) {
	template := &recordingTemplateRenderer{}
	renderer := newComponentRenderer(
		template,
		components.NewDefaultRegistry(),
		nil,
		rendererTheme{Partials: map[string]string{
			"forms.input": "themes/custom/input.tmpl",
		}},
		nil,
	)

	_, err := renderer.render(model.Field{Name: "username"}, "username")
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if len(template.calls) == 0 {
		t.Fatalf("expected template renderer to be called")
	}
	if got := template.calls[0]; got != "themes/custom/input.tmpl" {
		t.Fatalf("theme partial not applied, got %q", got)
	}
}

type recordingTemplateRenderer struct {
	calls []string
}

func (r *recordingTemplateRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return r.RenderTemplate(name, data, out...)
}

func (r *recordingTemplateRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	r.calls = append(r.calls, name)
	return "", nil
}

func (r *recordingTemplateRenderer) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return "", nil
}

func (r *recordingTemplateRenderer) RegisterFilter(name string, fn func(input any, param any) (any, error)) error {
	return nil
}

func (r *recordingTemplateRenderer) GlobalContext(data any) error {
	return nil
}
