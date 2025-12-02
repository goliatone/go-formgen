package preact_test

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/preact"
	"github.com/goliatone/go-formgen/pkg/testsupport"
	"github.com/goliatone/go-formgen/pkg/widgets"
	theme "github.com/goliatone/go-theme"
)

func TestRenderer_RenderContract(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := preact.New()
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Theme: testThemeConfig(),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(string(output), `"uiHints"`) {
		t.Fatalf("expected uiHints block in output")
	}

	goldenPath := filepath.Join("testdata", "form_output.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderer_RenderWithAssetURLPrefix(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := preact.New(preact.WithAssetURLPrefix("/static/formgen"))
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Theme: testThemeConfig(),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(string(output), `"uiHints"`) {
		t.Fatalf("expected uiHints block in prefixed output")
	}

	goldenPath := filepath.Join("testdata", "form_output_prefixed.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("prefixed output mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderer_RenderWithCustomAssetBundle(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	customAssets := fstest.MapFS{
		"bundle/vendor.js": &fstest.MapFile{Data: []byte("// vendor")},
		"bundle/app.js":    &fstest.MapFile{Data: []byte("// app")},
		"styles/app.css":   &fstest.MapFile{Data: []byte("/* css */")},
	}

	renderer, err := preact.New(
		preact.WithAssetsFS(customAssets),
		preact.WithAssetPaths(preact.AssetPaths{
			VendorScript: "bundle/vendor.js",
			AppScript:    "bundle/app.js",
			Stylesheet:   "styles/app.css",
		}),
		preact.WithAssetURLPrefix("https://cdn.example.com/formgen"),
	)
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Theme: testThemeConfig(),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(string(output), `"uiHints"`) {
		t.Fatalf("expected uiHints block in custom asset output")
	}

	goldenPath := filepath.Join("testdata", "form_output_cdn.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("custom asset output mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderer_JSONEditorPayload(t *testing.T) {
	renderer, err := preact.New()
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	form := model.FormModel{
		OperationID: "jsonEditor",
		Endpoint:    "/json",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:        "settings",
				Type:        model.FieldTypeObject,
				Description: "Runtime configuration",
				Default: map[string]any{
					"alpha": "beta",
				},
				Metadata: map[string]string{
					"widget": widgets.WidgetJSONEditor,
				},
				UIHints: map[string]string{
					"widget":     widgets.WidgetJSONEditor,
					"schemaHint": "Runtime config",
				},
			},
		},
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	payload := extractPreactPayload(t, output)
	var parsed struct {
		Fields []struct {
			Name     string            `json:"name"`
			Metadata map[string]string `json:"metadata"`
			UIHints  map[string]string `json:"uiHints"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(parsed.Fields) != 1 {
		t.Fatalf("expected single field, got %d", len(parsed.Fields))
	}

	field := parsed.Fields[0]
	if field.Metadata["component.name"] != "json_editor" {
		t.Fatalf("component metadata not applied, got %q", field.Metadata["component.name"])
	}
	if field.UIHints["widget"] != widgets.WidgetJSONEditor {
		t.Fatalf("widget hint not preserved, got %q", field.UIHints["widget"])
	}
}

func TestRenderer_WithTemplateRenderer(t *testing.T) {
	t.Helper()

	stub := &stubTemplateRenderer{
		renderTemplateFunc: func(name string, data any, out ...io.Writer) (string, error) {
			if name != "templates/page.tmpl" {
				t.Fatalf("unexpected template name: %s", name)
			}
			payload, ok := data.(map[string]any)
			if !ok {
				t.Fatalf("expected map payload, got %T", data)
			}
			if _, ok := payload["form"]; !ok {
				t.Fatalf("form not provided to template")
			}
			if payload["form_json"] == "" {
				t.Fatalf("form_json should be provided")
			}
			return "preact-custom", nil
		},
	}

	renderer, err := preact.New(preact.WithTemplateRenderer(stub))
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	out, err := renderer.Render(testsupport.Context(), testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json")), render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(out) != "preact-custom" {
		t.Fatalf("unexpected output: %s", out)
	}
	if !stub.called {
		t.Fatalf("expected render template to be called")
	}
}

func TestRenderer_RenderWithProvenance(t *testing.T) {
	t.Helper()

	renderer, err := preact.New()
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	form := model.FormModel{
		OperationID: "withProvenance",
		Endpoint:    "/items",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "name",
				Type:  model.FieldTypeString,
				Label: "Name",
			},
		},
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Values: map[string]any{
			"name": render.ValueWithProvenance{
				Value:      "prefilled",
				Provenance: "tenant default",
				Readonly:   true,
				Disabled:   true,
			},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	payload := extractPreactPayload(t, output)
	var got model.FormModel
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(got.Fields))
	}
	field := got.Fields[0]
	if field.Default != "prefilled" {
		t.Fatalf("expected default value to be prefilled, got %v", field.Default)
	}
	if field.Metadata["prefill.provenance"] != "tenant default" {
		t.Fatalf("expected provenance metadata, got %q", field.Metadata["prefill.provenance"])
	}
	if field.Metadata["prefill.readonly"] != "true" {
		t.Fatalf("expected readonly metadata, got %q", field.Metadata["prefill.readonly"])
	}
	if !field.Readonly {
		t.Fatalf("expected readonly flag to be set")
	}
	if field.UIHints["readonly"] != "true" {
		t.Fatalf("expected readonly uiHint to be set")
	}
	if !field.Disabled {
		t.Fatalf("expected field to be disabled")
	}
	if field.Metadata["prefill.disabled"] != "true" {
		t.Fatalf("expected disabled metadata, got %q", field.Metadata["prefill.disabled"])
	}
	if field.Metadata["disabled"] != "true" {
		t.Fatalf("expected disabled flag to be set, got %q", field.Metadata["disabled"])
	}
}

func TestNew_WithTemplatesFSMissingTemplate(t *testing.T) {
	_, err := preact.New(preact.WithTemplatesFS(fstest.MapFS{}))
	if err == nil {
		t.Fatalf("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "template") {
		t.Fatalf("expected template error, got %v", err)
	}
}

func TestNew_WithAssetsFSMissingFiles(t *testing.T) {
	custom := fstest.MapFS{
		"bundle/app.js": &fstest.MapFile{Data: []byte("// app")},
	}
	_, err := preact.New(
		preact.WithAssetsFS(custom),
		preact.WithAssetPaths(preact.AssetPaths{
			VendorScript: "bundle/vendor.js",
			AppScript:    "bundle/app.js",
			Stylesheet:   "styles/app.css",
		}),
	)
	if err == nil {
		t.Fatalf("expected error for missing assets")
	}
	if !strings.Contains(err.Error(), "vendor script") {
		t.Fatalf("expected vendor script error, got %v", err)
	}
}

type stubTemplateRenderer struct {
	called             bool
	renderTemplateFunc func(name string, data any, out ...io.Writer) (string, error)
}

func (s *stubTemplateRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return s.RenderTemplate(name, data, out...)
}

func (s *stubTemplateRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	s.called = true
	if s.renderTemplateFunc != nil {
		return s.renderTemplateFunc(name, data, out...)
	}
	return "", nil
}

func (s *stubTemplateRenderer) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return "", nil
}

func (s *stubTemplateRenderer) RegisterFilter(name string, fn func(input any, param any) (any, error)) error {
	return nil
}

func (s *stubTemplateRenderer) GlobalContext(data any) error {
	return nil
}

func extractPreactPayload(t *testing.T, html []byte) []byte {
	t.Helper()

	const scriptPrefix = `<script id="formgen-preact-data" type="application/json">`

	start := strings.Index(string(html), scriptPrefix)
	if start == -1 {
		t.Fatalf("payload script not found")
	}
	start += len(scriptPrefix)
	end := strings.Index(string(html[start:]), "</script>")
	if end == -1 {
		t.Fatalf("payload script closing tag not found")
	}
	return []byte(strings.TrimSpace(string(html[start : start+end])))
}

func testThemeConfig() *theme.RendererConfig {
	return &theme.RendererConfig{
		Theme:   "acme",
		Variant: "dark",
		Tokens: map[string]string{
			"brand": "#123456",
		},
		CSSVars: map[string]string{
			"--brand": "#123456",
		},
		AssetURL: func(key string) string {
			switch key {
			case "preact.vendor":
				return "theme/vendor.js"
			case "preact.app":
				return "theme/app.js"
			case "preact.stylesheet":
				return "theme/theme.css"
			default:
				return ""
			}
		},
	}
}
