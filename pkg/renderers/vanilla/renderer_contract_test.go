package vanilla_test

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
	"github.com/goliatone/formgen/pkg/testsupport"
	theme "github.com/goliatone/go-theme"
)

func TestRenderer_RenderContract(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Theme: testThemeConfig(),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
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

func TestRenderer_RenderWithDefaultStyles(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := vanilla.New(vanilla.WithDefaultStyles(), vanilla.WithStylesheet("/assets/custom.css"))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "form_output_with_styles.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("styled output mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderer_WithTemplateRenderer(t *testing.T) {
	t.Helper()

	stub := &stubTemplateRenderer{
		renderTemplateFunc: func(name string, data any, out ...io.Writer) (string, error) {
			if name == "templates/form.tmpl" {
				return "custom-output", nil
			}
			return "<component />", nil
		},
	}

	renderer, err := vanilla.New(vanilla.WithTemplateRenderer(stub))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	out, err := renderer.Render(testsupport.Context(), testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json")), render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(out) != "custom-output" {
		t.Fatalf("unexpected output: %s", out)
	}
	if !stub.called {
		t.Fatalf("expected render template to be called")
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

func TestRenderer_RenderPrefilledForm(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	options := render.RenderOptions{
		Method: "PATCH",
		Values: map[string]any{
			"title":               "Existing article title",
			"slug":                "existing-article-title",
			"summary":             "Updated teaser copy for the story.",
			"tenant_id":           "garden",
			"status":              "scheduled",
			"read_time_minutes":   7,
			"author_id":           "11111111-1111-4111-8111-111111111111",
			"manager_id":          "88888888-8888-4888-8888-888888888888",
			"category_id":         "55555555-5555-4555-8555-555555555555",
			"tags":                []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"},
			"related_article_ids": []string{"rel-001", "rel-002"},
			"published_at":        "2024-03-01T10:00:00Z",
			"cta.headline":        "Ready to dig deeper?",
			"cta.url":             "https://example.com/cta",
			"cta.button_text":     "Explore guides",
			"seo.title":           "Existing article title | Northwind Editorial",
			"seo.description":     "Updated description for SEO block.",
		},
		Errors: map[string][]string{
			"slug":                {"Slug already taken"},
			"manager_id":          {"Manager must belong to the selected author"},
			"tags":                {"Select at least one tag", "Tags must match the tenant"},
			"title":               {"Title cannot be empty"},
			"related_article_ids": {"Replace duplicate related articles"},
		},
	}

	output, err := renderer.Render(testsupport.Context(), form, options)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "form_output_prefilled.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("prefilled output mismatch (-want +got):\n%s", diff)
	}
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
			if key == "" {
				return ""
			}
			return "/themes/acme/" + key
		},
	}
}
