package vanilla_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
	"github.com/goliatone/go-formgen/pkg/submission"
	"github.com/goliatone/go-formgen/pkg/testsupport"
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

func TestRenderer_EncodesEnumOptionValues(t *testing.T) {
	form := model.FormModel{
		OperationID: "enumDemo",
		Endpoint:    "/enum",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "level", Type: model.FieldTypeInteger, Enum: []any{int64(1), int64(2)}},
		},
	}
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	html := string(output)
	encoded := submission.EncodeEnumValue(int64(2))
	if !strings.Contains(html, `value="`+encoded+`"`) {
		t.Fatalf("expected encoded enum option value %q in output:\n%s", encoded, html)
	}
	if !strings.Contains(html, `>2</option>`) {
		t.Fatalf("expected enum label to remain display value in output:\n%s", html)
	}
}

func TestRenderer_RenderContractGutterSm(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))
	if form.UIHints == nil {
		form.UIHints = map[string]string{}
	}
	form.UIHints["layout.gutter"] = "sm"

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

	goldenPath := filepath.Join("testdata", "form_output_gutter_sm.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderer_RenderContractResponsiveGrid(t *testing.T) {
	form := model.FormModel{
		OperationID: "responsiveGrid",
		Endpoint:    "/responsive",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "title",
				Type:  model.FieldTypeString,
				Label: "Title",
				UIHints: map[string]string{
					"layout.span":    "12",
					"layout.span.lg": "6",
				},
			},
			{
				Name:  "summary",
				Type:  model.FieldTypeString,
				Label: "Summary",
				UIHints: map[string]string{
					"layout.span": "12",
				},
			},
		},
	}

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

	goldenPath := filepath.Join("testdata", "form_output_responsive_grid.golden.html")
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

func TestRenderer_RenderModes(t *testing.T) {
	form := model.FormModel{
		OperationID: "embed",
		Endpoint:    "/embed",
		Method:      "POST",
		Fields:      []model.Field{{Name: "title", Type: model.FieldTypeString, Label: "Title"}},
	}
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	formOnly, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		RenderMode: render.RenderModeForm,
	})
	if err != nil {
		t.Fatalf("render form mode: %v", err)
	}
	if count := strings.Count(string(formOnly), "<form"); count != 1 {
		t.Fatalf("form mode should emit one form, got %d:\n%s", count, formOnly)
	}

	fieldsOnly, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		RenderMode:   render.RenderModeFields,
		HiddenFields: map[string]string{"csrf": "token"},
	})
	if err != nil {
		t.Fatalf("render fields mode: %v", err)
	}
	html := string(fieldsOnly)
	if strings.Contains(html, "<form") {
		t.Fatalf("fields mode should not emit form:\n%s", html)
	}
	if !strings.Contains(html, `name="title"`) {
		t.Fatalf("fields mode should emit controls:\n%s", html)
	}
	if strings.Contains(html, `name="csrf"`) || strings.Contains(html, `type="submit"`) {
		t.Fatalf("fields mode should omit hidden fields and actions:\n%s", html)
	}
}

func TestRenderer_UnstyledModeOmitsDefaultClasses(t *testing.T) {
	form := model.FormModel{
		OperationID: "unstyled",
		Endpoint:    "/unstyled",
		Method:      "POST",
		Fields:      []model.Field{{Name: "title", Type: model.FieldTypeString, Label: "Title"}},
	}
	renderer, err := vanilla.New(vanilla.WithDefaultStyles())
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	out, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		StyleMode: render.StyleModeUnstyled,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)
	if strings.Contains(html, vanilla.DefaultFormClass) || strings.Contains(html, "py-3 px-4") {
		t.Fatalf("unstyled mode leaked default classes:\n%s", html)
	}
}

func TestRenderer_RenderContractWysiwygOnlyInjectsRuntime(t *testing.T) {
	form := model.FormModel{
		OperationID: "wysiwygOnly",
		Endpoint:    "/wysiwyg",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "body",
				Type:  model.FieldTypeString,
				Label: "Body",
				UIHints: map[string]string{
					"component": components.NameWysiwyg,
				},
			},
		},
	}

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

	goldenPath := filepath.Join("testdata", "form_output_wysiwyg_only.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("output mismatch (-want +got):\n%s", diff)
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

func TestRenderer_WithTemplateFuncs(t *testing.T) {
	templates := fstest.MapFS{
		"templates/form.tmpl": &fstest.MapFile{
			Data: []byte(`{{ shout(form.operationId) }}`),
		},
	}

	renderer, err := vanilla.New(
		vanilla.WithTemplatesFS(templates),
		vanilla.WithTemplateFuncs(map[string]any{
			"shout": func(value any) string {
				return strings.ToUpper(strings.TrimSpace(fmt.Sprint(value)))
			},
		}),
	)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{OperationID: "demo"}
	out, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.TrimSpace(string(out)) != "DEMO" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRenderer_ThemeAssetURLHelper(t *testing.T) {
	templates := fstest.MapFS{
		"templates/form.tmpl": &fstest.MapFile{
			Data: []byte(`{{ theme.assetURL("logo.svg") }}`),
		},
	}

	renderer, err := vanilla.New(vanilla.WithTemplatesFS(templates))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{OperationID: "demo", Endpoint: "/", Method: "POST"}
	out, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Theme: testThemeConfig(),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "/themes/acme/logo.svg" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRenderer_ThemeAssetURLHelper_NoTheme(t *testing.T) {
	templates := fstest.MapFS{
		"templates/form.tmpl": &fstest.MapFile{
			Data: []byte(`{{ theme.assetURL("logo.svg") }}`),
		},
	}

	renderer, err := vanilla.New(vanilla.WithTemplatesFS(templates))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{OperationID: "demo", Endpoint: "/", Method: "POST"}
	out, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRenderer_ThemeFormTemplateOverride(t *testing.T) {
	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	stub := &stubTemplateRenderer{
		renderTemplateFunc: func(name string, data any, out ...io.Writer) (string, error) {
			switch name {
			case "templates/custom_form.tmpl":
				return "custom-output", nil
			case "templates/form.tmpl":
				return "default-output", nil
			default:
				return "<component />", nil
			}
		},
	}

	renderer, err := vanilla.New(vanilla.WithTemplateRenderer(stub))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	out, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Theme: &render.ThemeConfig{
			Partials: map[string]string{
				"forms.form": "templates/custom_form.tmpl",
			},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "custom-output" {
		t.Fatalf("unexpected output: %q", got)
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
		FormErrors: []string{"Unable to save article", "Please fix the errors below"},
		HiddenFields: render.MergeHiddenFields(nil,
			render.CSRFToken("_csrf", "csrf-token"),
			render.AuthToken("auth_token", "auth-token"),
			render.VersionField("version", 3),
		),
	}

	output, err := renderer.Render(testsupport.Context(), form, options)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "form_output_prefilled.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := strings.TrimSpace(string(testsupport.MustReadGolden(t, goldenPath)))
	got := strings.TrimSpace(string(output))
	if diff := testsupport.CompareGolden(want, got); diff != "" {
		t.Fatalf("prefilled output mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderer_PrefillsNestedArrayRelationshipCurrent(t *testing.T) {
	form := model.FormModel{
		OperationID: "teachingTopicsMenu",
		Endpoint:    "/menus",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "columns",
				Type:  model.FieldTypeArray,
				Label: "Columns",
				Items: &model.Field{
					Type: model.FieldTypeObject,
					Nested: []model.Field{
						{
							Name:  "entries",
							Type:  model.FieldTypeArray,
							Label: "Entries",
							Items: &model.Field{
								Type: model.FieldTypeObject,
								Nested: []model.Field{
									{
										Name:  "topic_id",
										Type:  model.FieldTypeString,
										Label: "Topic",
										Relationship: &model.Relationship{
											Kind:        model.RelationshipBelongsTo,
											Target:      "teaching-topic",
											ForeignKey:  "topic_id",
											Cardinality: "one",
										},
										Metadata: map[string]string{
											"relationship.endpoint.url":          "/admin/api/options/teaching-topic",
											"relationship.endpoint.mode":         "search",
											"relationship.endpoint.hydrateParam": "topic_id",
											"relationship.endpoint.labelField":   "label",
											"relationship.endpoint.valueField":   "value",
										},
									},
									{
										Name:  "topic_slug",
										Type:  model.FieldTypeString,
										Label: "Topic slug",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	const topicID = "7a8ec46f-3024-4585-88be-f6adedf77b28"
	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Values: map[string]any{
			"columns": []any{
				map[string]any{
					"entries": []any{
						map[string]any{
							"topic_id":   topicID,
							"topic_slug": "refuge",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	html := string(output)
	for _, want := range []string{
		`data-endpoint-hydrate-param="topic_id"`,
		`data-relationship-current="` + topicID + `"`,
		`id="fg-columns-0-entries-0-topic_id"`,
		`name="columns[0].entries[0].topic_id"`,
		`id="fg-columns-0-entries-0-topic_slug"`,
		`name="columns[0].entries[0].topic_slug"`,
		`value="refuge"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected rendered HTML to contain %s:\n%s", want, html)
		}
	}
}

func TestRenderer_RepeatableArrayRendersAddButtonAndPrototypeTemplate(t *testing.T) {
	form := model.FormModel{
		OperationID: "teachingTopicsMenu",
		Endpoint:    "/menus",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "columns",
				Type:  model.FieldTypeArray,
				Label: "Columns",
				Items: &model.Field{
					Type: model.FieldTypeObject,
					Nested: []model.Field{
						{Name: "title", Type: model.FieldTypeString, Label: "Title"},
						{
							Name:  "entries",
							Type:  model.FieldTypeArray,
							Label: "Entries",
							UIHints: map[string]string{
								"cardinality": "many",
								"addText":     "Add topic entry",
							},
							Items: &model.Field{
								Type: model.FieldTypeObject,
								Nested: []model.Field{
									{
										Name:  "topic_id",
										Type:  model.FieldTypeString,
										Label: "Topic",
										Relationship: &model.Relationship{
											Kind:        model.RelationshipBelongsTo,
											Target:      "teaching-topic",
											ForeignKey:  "topic_id",
											Cardinality: "one",
										},
										Metadata: map[string]string{
											"relationship.endpoint.url":          "/admin/api/options/teaching-topic",
											"relationship.endpoint.mode":         "search",
											"relationship.endpoint.hydrateParam": "topic_id",
											"relationship.endpoint.labelField":   "label",
											"relationship.endpoint.valueField":   "value",
										},
									},
									{Name: "topic_slug", Type: model.FieldTypeString, Label: "Topic slug"},
								},
							},
						},
					},
				},
			},
		},
	}

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		Values: map[string]any{
			"columns": []any{
				map[string]any{
					"title": "Subjects",
					"entries": []any{
						map[string]any{
							"topic_id":   "topic-refuge-id",
							"topic_slug": "refuge",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	html := string(output)
	for _, want := range []string{
		`data-formgen-array-items="true"`,
		`data-formgen-array-name="columns[0].entries"`,
		`data-formgen-array-next-index="1"`,
		`data-formgen-array-prototype-path="columns[0].entries[1]"`,
		`data-formgen-array-prototype-id-prefix="fg-columns-0-entries-1"`,
		`<template data-formgen-array-prototype="true">`,
		`data-formgen-array-action="add"`,
		`data-relationship-action="add"`,
		`Add topic entry`,
		`name="columns[0].entries[0].topic_id"`,
		`id="fg-columns-0-entries-1-topic_id"`,
		`/runtime/formgen-relationships.min.js`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected rendered HTML to contain %s:\n%s", want, html)
		}
	}

	prototypeTopic := renderedControlTagByID(t, html, "fg-columns-0-entries-1-topic_id")
	if !strings.Contains(prototypeTopic, `name="columns[0].entries[1].topic_id"`) {
		t.Fatalf("template prototype topic control should keep the cloneable name:\n%s", prototypeTopic)
	}
	if !strings.Contains(prototypeTopic, "disabled") {
		t.Fatalf("prototype topic control should be disabled before cloning:\n%s", prototypeTopic)
	}
}

func TestRenderer_EmptyArrayPrototypeDoesNotSubmitValues(t *testing.T) {
	form := model.FormModel{
		OperationID: "arrays",
		Endpoint:    "/arrays",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "keywords",
				Type:  model.FieldTypeArray,
				Label: "Keywords",
				Items: &model.Field{
					Name:     "keyword",
					Type:     model.FieldTypeString,
					Label:    "Keyword",
					Required: true,
				},
			},
			{
				Name:  "contributors",
				Type:  model.FieldTypeArray,
				Label: "Contributors",
				Items: &model.Field{
					Type: model.FieldTypeObject,
					Nested: []model.Field{
						{Name: "name", Type: model.FieldTypeString, Label: "Name", Required: true},
						{Name: "role", Type: model.FieldTypeString, Label: "Role", Required: true},
					},
				},
			},
		},
	}

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	html := string(output)
	for _, want := range []string{
		`id="fg-keywords-0"`,
		`id="fg-contributors-0-name"`,
		`id="fg-contributors-0-role"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected rendered HTML to contain prototype id %s:\n%s", want, html)
		}
	}
	for _, assertion := range []struct {
		id   string
		name string
	}{
		{id: "fg-keywords-0", name: `name="keywords[0]"`},
		{id: "fg-contributors-0-name", name: `name="contributors[0].name"`},
		{id: "fg-contributors-0-role", name: `name="contributors[0].role"`},
	} {
		tag := renderedControlTagByID(t, html, assertion.id)
		if strings.Contains(tag, assertion.name) {
			t.Fatalf("empty array prototype control %s should not contain %s:\n%s", assertion.id, assertion.name, tag)
		}
		if !strings.Contains(tag, "disabled") {
			t.Fatalf("empty array prototype control %s should be disabled:\n%s", assertion.id, tag)
		}
		if !strings.Contains(tag, "required") {
			t.Fatalf("empty array prototype control %s should preserve required semantics for cloned rows:\n%s", assertion.id, tag)
		}
	}
}

func TestRenderer_CustomComponentReceivesUserConfigOnly(t *testing.T) {
	registry := components.NewDefaultRegistry()
	var capturedConfig map[string]any

	registry.MustRegister(components.NameInput, components.Descriptor{
		Renderer: func(buf *bytes.Buffer, field model.Field, data components.ComponentData) error {
			capturedConfig = data.Config
			if _, err := json.Marshal(data.Config); err != nil {
				return fmt.Errorf("component config should be JSON-safe user config: %w", err)
			}
			buf.WriteString(`<input id="custom-`)
			buf.WriteString(field.Name)
			buf.WriteString(`">`)
			return nil
		},
	})

	renderer, err := vanilla.New(vanilla.WithComponentRegistry(registry))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		OperationID: "customConfig",
		Endpoint:    "/custom",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name:  "title",
				Type:  model.FieldTypeString,
				Label: "Title",
				Metadata: map[string]string{
					"component.config": `{"placeholder":"Headline"}`,
				},
			},
		},
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(output), `id="custom-title"`) {
		t.Fatalf("expected custom component output, got:\n%s", output)
	}
	if capturedConfig["placeholder"] != "Headline" {
		t.Fatalf("expected user component config to be preserved, got %#v", capturedConfig)
	}
	if len(capturedConfig) != 1 {
		t.Fatalf("custom component config should not include renderer internals, got %#v", capturedConfig)
	}
}

func renderedControlTagByID(t *testing.T, html, id string) string {
	t.Helper()

	idAttr := `id="` + id + `"`
	idx := strings.Index(html, idAttr)
	if idx < 0 {
		t.Fatalf("rendered HTML missing control id %q:\n%s", id, html)
	}
	start := strings.LastIndex(html[:idx], "<")
	if start < 0 {
		t.Fatalf("could not locate control tag start for id %q", id)
	}
	endRel := strings.Index(html[idx:], ">")
	if endRel < 0 {
		t.Fatalf("could not locate control tag end for id %q", id)
	}
	return html[start : idx+endRel+1]
}

func TestRenderer_RenderWithProvenance(t *testing.T) {
	t.Helper()

	form := model.FormModel{
		OperationID: "article",
		Endpoint:    "/articles",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString, Label: "Title"},
			{Name: "scope", Type: model.FieldTypeString, Label: "Scope"},
		},
	}

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	options := render.RenderOptions{
		Values: map[string]any{
			"title": render.ValueWithProvenance{
				Value:      "Existing title",
				Provenance: "tenant default",
				Disabled:   true,
			},
			"scope": render.ValueWithProvenance{
				Value:      "tenant",
				Provenance: "org policy",
				Readonly:   true,
			},
		},
	}

	output, err := renderer.Render(testsupport.Context(), form, options)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "form_output_provenance.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := strings.TrimSpace(string(testsupport.MustReadGolden(t, goldenPath)))
	got := strings.TrimSpace(string(output))
	if diff := testsupport.CompareGolden(want, got); diff != "" {
		t.Fatalf("provenance output mismatch (-want +got):\n%s", diff)
	}
}

func testThemeConfig() *render.ThemeConfig {
	return &render.ThemeConfig{
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
