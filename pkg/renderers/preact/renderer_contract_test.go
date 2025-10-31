package preact_test

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goliatone/formgen/pkg/renderers/preact"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestRenderer_RenderContract(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := preact.New()
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form)
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

	output, err := renderer.Render(testsupport.Context(), form)
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

	output, err := renderer.Render(testsupport.Context(), form)
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

	out, err := renderer.Render(testsupport.Context(), testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json")))
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
