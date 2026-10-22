package vanilla_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

func codesAndPaths(diagnostics []model.DefinitionDiagnostic) []string {
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.Code+":"+diagnostic.Path)
	}
	return out
}

func TestPreflightReportsUnknownComponentsAndKeepsRenderError(t *testing.T) {
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	form := model.FormModel{Fields: []model.Field{
		{Name: "enabled", Type: model.FieldTypeBoolean, Label: "Enable", Default: true, Metadata: map[string]string{"component.name": "chocie"}},
		{Name: "note", Type: model.FieldTypeString, Label: "Note", Default: "secret-answer", Metadata: map[string]string{"host.widget": "custom", "x-host.flag": "yes"}},
	}}
	diagnostics := renderer.Preflight(form)
	if got := codesAndPaths(diagnostics); strings.Join(got, ",") != "componentUnknown:enabled" {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	if !strings.Contains(diagnostics[0].Message, `"chocie"`) {
		t.Fatalf("diagnostic must name the component: %+v", diagnostics[0])
	}
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "secret-answer") || strings.Contains(diagnostic.Message, "true") {
			t.Fatalf("diagnostic leaked a value: %+v", diagnostic)
		}
	}
	// Legacy behavior: rendering an unregistered explicit component still fails.
	if _, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{}); err == nil || !strings.Contains(err.Error(), "chocie") {
		t.Fatalf("render must keep failing for unknown explicit components, got %v", err)
	}
}

func TestPreflightReportsMalformedConfigAndRejectedFields(t *testing.T) {
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	form := model.FormModel{Fields: []model.Field{
		{Name: "title", Type: model.FieldTypeString, Label: "Title", Metadata: map[string]string{"component.config": `{"bad"`}},
		{Name: "level", Type: model.FieldTypeString, Label: "Level", Default: "hidden-default", Metadata: map[string]string{"component.name": "scale"}},
		{Name: "rows", Type: model.FieldTypeArray, Label: "Rows", Items: &model.Field{Type: model.FieldTypeObject, Nested: []model.Field{
			{Name: "answer", Type: model.FieldTypeString, Label: "Answer", Metadata: map[string]string{"component.name": "radio-cards"}},
		}}},
	}}
	got := codesAndPaths(renderer.Preflight(form))
	want := []string{"componentConfig:title", "componentRender:level", "componentUnknown:rows[].answer"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("diagnostics %v, want %v", got, want)
	}
	for _, diagnostic := range renderer.Preflight(form) {
		if strings.Contains(diagnostic.Message, "hidden-default") {
			t.Fatalf("diagnostic leaked a default value: %+v", diagnostic)
		}
	}
}

func TestPreflightAcceptsRegisteredComponentsAndRenderSemantics(t *testing.T) {
	registry := components.NewDefaultRegistry()
	registry.MustRegister("host-rating", components.Descriptor{Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
		buf.WriteString(`<input type="range" name="` + field.Name + `" data-host-rating="true">`)
		return nil
	}})
	renderer, err := vanilla.New(vanilla.WithComponentRegistry(registry))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	// Registration after construction does not reach the renderer's cloned registry.
	registry.MustRegister("late-component", components.Descriptor{Renderer: func(buf *bytes.Buffer, _ model.Field, _ components.ComponentData) error {
		buf.WriteString("<input>")
		return nil
	}})

	form := model.FormModel{Fields: []model.Field{
		{Name: "enabled", Type: model.FieldTypeBoolean, Label: "Enable", Metadata: map[string]string{"component.name": "choice"}},
		{Name: "rating", Type: model.FieldTypeInteger, Label: "Rating", Metadata: map[string]string{"component.name": "host-rating"}},
		{Name: "level", Type: model.FieldTypeInteger, Label: "Level", Validations: []model.ValidationRule{{Kind: model.ValidationRuleMin, Params: map[string]string{"value": "1"}}, {Kind: model.ValidationRuleMax, Params: map[string]string{"value": "10"}}}, Metadata: map[string]string{"component.name": "scale"}},
		{Name: "colors", Type: model.FieldTypeArray, Label: "Colors", Items: &model.Field{Type: model.FieldTypeString, Enum: []any{"blue", "green"}}, Metadata: map[string]string{"component.name": "choice-multiple"}},
		{Name: "contact", Type: model.FieldTypeObject, Label: "Contact", Nested: []model.Field{{Name: "email", Type: model.FieldTypeString, Label: "Email"}}},
		{Name: "late", Type: model.FieldTypeString, Label: "Late", Metadata: map[string]string{"component.name": "late-component"}},
	}}
	got := codesAndPaths(renderer.Preflight(form))
	if strings.Join(got, ",") != "componentUnknown:late" {
		t.Fatalf("expected only the late registration to fail, got %v", got)
	}

	form.Fields = form.Fields[:5]
	if diagnostics := renderer.Preflight(form); len(diagnostics) != 0 {
		t.Fatalf("supported definition produced diagnostics: %v", diagnostics)
	}
	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{RenderMode: render.RenderModeFields})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(output)
	for _, marker := range []string{`data-formgen-choice="single"`, `data-host-rating="true"`, `data-formgen-choice="scale"`, `data-formgen-choice="multiple"`, `<fieldset id="fg-contact"`} {
		if !strings.Contains(html, marker) {
			t.Fatalf("expected semantic control %q in output:\n%s", marker, html)
		}
	}
}

func TestPreflightRecursesOnlyIntoSharedContainers(t *testing.T) {
	registry := components.NewDefaultRegistry()
	registry.MustRegister("grid-editor", components.Descriptor{Renderer: func(buf *bytes.Buffer, field model.Field, _ components.ComponentData) error {
		buf.WriteString(`<div data-grid-editor="` + field.Name + `"></div>`)
		return nil
	}})
	renderer, err := vanilla.New(vanilla.WithComponentRegistry(registry))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	form := model.FormModel{Fields: []model.Field{{
		Name:     "rows",
		Type:     model.FieldTypeArray,
		Label:    "Rows",
		Metadata: map[string]string{"component.name": "grid-editor"},
		Items: &model.Field{Type: model.FieldTypeObject, Nested: []model.Field{
			{Name: "answer", Type: model.FieldTypeString, Metadata: map[string]string{"component.name": "host-only-cell"}},
		}},
	}}}
	if diagnostics := renderer.Preflight(form); len(diagnostics) != 0 {
		t.Fatalf("host container children must not be resolved by preflight: %v", diagnostics)
	}
	if _, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{}); err != nil {
		t.Fatalf("render must agree with preflight: %v", err)
	}
}

func TestPreflightHonorsUISchemaComponentPrecedence(t *testing.T) {
	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	form := model.FormModel{Fields: []model.Field{{
		Name:     "mode",
		Type:     model.FieldTypeString,
		Label:    "Mode",
		Enum:     []any{"a", "b"},
		UIHints:  map[string]string{"component": "choice"},
		Metadata: map[string]string{"component.name": "not-registered"},
	}}}
	if diagnostics := renderer.Preflight(form); len(diagnostics) != 0 {
		t.Fatalf("explicit UI schema component must win over inline metadata: %v", diagnostics)
	}
}
