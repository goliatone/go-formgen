package vanilla

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/goliatone/formgen/pkg/model"
)

func TestBuildFieldMarkupUsesChromePartials(t *testing.T) {
	renderer := newStubChromeRenderer(map[string]stubChromeResponse{
		chromeLabelTemplate:       {output: "<label>partial-label</label>"},
		chromeDescriptionTemplate: {output: "<p>partial-description</p>"},
		chromeHelpTemplate:        {output: "<p>partial-help</p>"},
	})

	field := model.Field{
		Name:        "title",
		Label:       "Title",
		Description: "Description",
		Required:    true,
		UIHints: map[string]string{
			"helpText": "Help text",
		},
	}

	html := buildFieldMarkup(renderer, field, "input", `<input id="fg-title">`)
	if !strings.Contains(html, "partial-label") {
		t.Fatalf("expected label partial output, got:\n%s", html)
	}
	if !strings.Contains(html, "partial-description") {
		t.Fatalf("expected description partial output, got:\n%s", html)
	}
	if !strings.Contains(html, "partial-help") {
		t.Fatalf("expected help partial output, got:\n%s", html)
	}

	if got := renderer.calls[chromeLabelTemplate]; got != 1 {
		t.Fatalf("expected label partial to be invoked once, got %d", got)
	}
	if got := renderer.calls[chromeDescriptionTemplate]; got != 1 {
		t.Fatalf("expected description partial to be invoked once, got %d", got)
	}
	if got := renderer.calls[chromeHelpTemplate]; got != 1 {
		t.Fatalf("expected help partial to be invoked once, got %d", got)
	}
}

func TestBuildFieldMarkupFallsBackWhenPartialsFail(t *testing.T) {
	renderer := newStubChromeRenderer(map[string]stubChromeResponse{
		chromeLabelTemplate:       {err: errors.New("boom")},
		chromeDescriptionTemplate: {err: errors.New("boom")},
		chromeHelpTemplate:        {err: errors.New("boom")},
	})

	field := model.Field{
		Name:        "title",
		Label:       "Title",
		Description: "Required description",
		Required:    true,
		UIHints: map[string]string{
			"helpText": "Assistive copy",
		},
	}

	html := buildFieldMarkup(renderer, field, "input", `<input id="fg-title">`)
	if !strings.Contains(html, `for="fg-title"`) {
		t.Fatalf("expected fallback label markup with control binding, got:\n%s", html)
	}
	if !strings.Contains(html, `<span class="text-red-500">*</span>`) {
		t.Fatalf("expected required indicator in fallback label, got:\n%s", html)
	}
	if !strings.Contains(html, "Required description") {
		t.Fatalf("expected fallback description markup, got:\n%s", html)
	}
	if !strings.Contains(html, "Assistive copy") {
		t.Fatalf("expected fallback help markup, got:\n%s", html)
	}
	for _, attr := range []string{
		`data-formgen-chrome="label"`,
		`data-formgen-chrome="description"`,
		`data-formgen-chrome="help"`,
	} {
		if !strings.Contains(html, attr) {
			t.Fatalf("expected fallback markup to include %s attribute, got:\n%s", attr, html)
		}
	}
}

func TestBuildFieldMarkupVisuallyHidesLabel(t *testing.T) {
	renderer := newStubChromeRenderer(map[string]stubChromeResponse{
		chromeLabelTemplate:       {err: errors.New("boom")},
		chromeDescriptionTemplate: {output: "<p>partial-description</p>"},
	})

	field := model.Field{
		Name:        "hidden",
		Label:       "Hidden Label",
		Description: "Description",
		UIHints: map[string]string{
			"hideLabel": "true",
			"helpText":  "Help",
		},
	}

	html := buildFieldMarkup(renderer, field, "input", `<input id="fg-hidden">`)
	if !strings.Contains(html, "sr-only") {
		t.Fatalf("expected visually hidden class on label, got:\n%s", html)
	}
	if got := renderer.calls[chromeLabelTemplate]; got != 1 {
		t.Fatalf("expected label partial to run once, got %d invocations", got)
	}
}

func TestBuildFieldMarkupHonoursChromeSkipMetadata(t *testing.T) {
	field := model.Field{
		Name:     "raw",
		Metadata: map[string]string{componentChromeMetadataKey: componentChromeSkipKeyword},
	}
	control := `<input id="fg-raw">`
	html := buildFieldMarkup(nil, field, "input", control)
	if html != control {
		t.Fatalf("expected chrome skip to return control markup only, got: %q", html)
	}
}

type stubChromeTemplateRenderer struct {
	responses map[string]stubChromeResponse
	calls     map[string]int
}

type stubChromeResponse struct {
	output string
	err    error
}

func newStubChromeRenderer(responses map[string]stubChromeResponse) *stubChromeTemplateRenderer {
	return &stubChromeTemplateRenderer{
		responses: responses,
		calls:     make(map[string]int),
	}
}

func (s *stubChromeTemplateRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return s.RenderTemplate(name, data, out...)
}

func (s *stubChromeTemplateRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	s.calls[name]++
	if resp, ok := s.responses[name]; ok {
		return resp.output, resp.err
	}
	return "", nil
}

func (s *stubChromeTemplateRenderer) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return "", nil
}

func TestBuildDataAttributesIncludesBehaviors(t *testing.T) {
	metadata := map[string]string{
		behaviorNamesMetadataKey:  "autoSlug autoResize",
		behaviorConfigMetadataKey: `{"autoSlug":{"source":"title"},"autoResize":{"minRows":5}}`,
	}

	result := buildDataAttributes(metadata)
	if !strings.Contains(result, `data-behavior="autoSlug autoResize"`) {
		t.Fatalf("expected behavior names attribute, got %q", result)
	}
	expectedConfig := `data-behavior-config="{&#34;autoSlug&#34;:{&#34;source&#34;:&#34;title&#34;},&#34;autoResize&#34;:{&#34;minRows&#34;:5}}"` // html-escaped quotes
	if !strings.Contains(result, expectedConfig) {
		t.Fatalf("expected behavior config attribute with escaped payload, got %q", result)
	}
}

func (s *stubChromeTemplateRenderer) RegisterFilter(name string, fn func(any, any) (any, error)) error {
	return nil
}

func (s *stubChromeTemplateRenderer) GlobalContext(data any) error {
	return nil
}
