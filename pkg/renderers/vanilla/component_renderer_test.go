package vanilla

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render/template/gotemplate"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
	"github.com/goliatone/go-formgen/pkg/testsupport"
	"github.com/goliatone/go-formgen/pkg/widgets"
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

func TestComponentRendererInputTemplatePreservesTypedNumericDefault(t *testing.T) {
	template := &recordingTemplateRenderer{}
	renderer := newComponentRenderer(template, components.NewDefaultRegistry(), nil, rendererTheme{}, nil)
	defaultValue := json.Number("9007199254740993")

	_, err := renderer.render(model.Field{
		Name:    "count",
		Type:    model.FieldTypeInteger,
		Default: defaultValue,
	}, "count")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(template.data) == 0 {
		t.Fatal("expected template payload")
	}
	payload, ok := template.data[0].(map[string]any)
	if !ok {
		t.Fatalf("template payload type = %T, want map[string]any", template.data[0])
	}
	field, ok := payload["field"].(model.Field)
	if !ok {
		t.Fatalf("template field type = %T, want model.Field", payload["field"])
	}
	if got, ok := field.Default.(json.Number); !ok || got.String() != defaultValue.String() {
		t.Fatalf("template field default = %#v (%T), want unchanged json.Number", field.Default, field.Default)
	}
	if got := payload["control_value"]; got != defaultValue.String() {
		t.Fatalf("control_value = %#v, want %q", got, defaultValue)
	}
	if got := payload["has_value"]; got != true {
		t.Fatalf("has_value = %#v, want true", got)
	}
}

func TestComponentRendererUsesMediaPickerThemePartial(t *testing.T) {
	template := &recordingTemplateRenderer{}
	renderer := newComponentRenderer(
		template,
		components.NewDefaultRegistry(),
		nil,
		rendererTheme{Partials: map[string]string{
			"forms.media-picker": "themes/custom/media-picker.tmpl",
		}},
		nil,
	)

	_, err := renderer.render(model.Field{
		Name: "hero",
		Type: model.FieldTypeString,
		Metadata: map[string]string{
			componentNameMetadataKey: components.NameMediaPicker,
		},
	}, "hero")
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if len(template.calls) == 0 {
		t.Fatalf("expected template renderer to be called")
	}
	if got := template.calls[0]; got != "themes/custom/media-picker.tmpl" {
		t.Fatalf("theme partial not applied, got %q", got)
	}
}

func TestJSONEditorComponentRendersPrettyValue(t *testing.T) {
	engine, err := gotemplate.New(
		gotemplate.WithFS(TemplatesFS()),
		gotemplate.WithExtension(".tmpl"),
	)
	if err != nil {
		t.Fatalf("configure template renderer: %v", err)
	}

	renderer := newComponentRenderer(engine, components.NewDefaultRegistry(), nil, rendererTheme{}, nil)

	field := model.Field{
		Name:        "settings",
		Type:        model.FieldTypeObject,
		Description: "Runtime feature flags",
		Default: map[string]any{
			"enabled": true,
			"count":   2,
		},
		Metadata: map[string]string{
			"widget":                   widgets.WidgetJSONEditor,
			componentChromeMetadataKey: componentChromeSkipKeyword,
		},
		UIHints: map[string]string{
			"schemaHint": "Advanced settings",
			"collapsed":  "true",
		},
	}

	html, err := renderer.render(field, "settings")
	if err != nil {
		t.Fatalf("render json editor: %v", err)
	}

	goldenPath := filepath.Join("testdata", "json_editor_component.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, []byte(html)) {
		return
	}

	want := normalizeHTML(string(testsupport.MustReadGolden(t, goldenPath)))
	if diff := testsupport.CompareGolden(want, normalizeHTML(html)); diff != "" {
		t.Fatalf("json editor output mismatch (-want +got):\n%s", diff)
	}
}

func TestComponentRenderer_ComponentTemplateReceivesTheme(t *testing.T) {
	engine, err := gotemplate.New(
		gotemplate.WithFS(fstest.MapFS{
			"templates/components/input.tmpl": &fstest.MapFile{
				Data: []byte(`brand={{ theme.tokens.brand }}`),
			},
		}),
		gotemplate.WithExtension(".tmpl"),
	)
	if err != nil {
		t.Fatalf("configure template renderer: %v", err)
	}

	renderer := newComponentRenderer(engine, components.NewDefaultRegistry(), nil, rendererTheme{
		Tokens: map[string]string{
			"brand": "#123456",
		},
	}, nil)

	html, err := renderer.render(model.Field{Name: "title", Type: model.FieldTypeString}, "title")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "brand=#123456") {
		t.Fatalf("expected theme token in output, got %q", html)
	}
}

func normalizeHTML(input string) string {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n")
}

type recordingTemplateRenderer struct {
	calls []string
	data  []any
}

func (r *recordingTemplateRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return r.RenderTemplate(name, data, out...)
}

func (r *recordingTemplateRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	r.calls = append(r.calls, name)
	r.data = append(r.data, data)
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
