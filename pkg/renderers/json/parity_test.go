package json_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	jsonrenderer "github.com/goliatone/go-formgen/pkg/renderers/json"
	"github.com/goliatone/go-formgen/pkg/renderers/preact"
	"github.com/goliatone/go-formgen/pkg/renderers/tui"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

func TestRendererParityPreservesOrderAndValueShape(t *testing.T) {
	form := model.FormModel{
		OperationID: "parity",
		Endpoint:    "/parity",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString, Label: "Title", Default: "Draft"},
			{Name: "published", Type: model.FieldTypeBoolean, Label: "Published", Default: true},
		},
	}
	values := map[string]any{"title": "Runtime", "published": false}

	jsonOut, err := jsonrenderer.New().Render(testsupport.Context(), form, render.RenderOptions{Values: values})
	if err != nil {
		t.Fatalf("json render: %v", err)
	}
	var descriptor struct {
		Form   model.FormModel `json:"form"`
		Values map[string]any  `json:"values"`
	}
	if unmarshalErr := json.Unmarshal(jsonOut, &descriptor); unmarshalErr != nil {
		t.Fatalf("json unmarshal: %v", unmarshalErr)
	}
	assertFieldOrder(t, descriptor.Form.Fields, []string{"title", "published"})
	if descriptor.Values["title"] != "Runtime" || descriptor.Values["published"] != false {
		t.Fatalf("json values changed shape: %#v", descriptor.Values)
	}

	vanillaRenderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("vanilla.New: %v", err)
	}
	vanillaOut, err := vanillaRenderer.Render(testsupport.Context(), form, render.RenderOptions{Values: values})
	if err != nil {
		t.Fatalf("vanilla render: %v", err)
	}
	vanillaHTML := string(vanillaOut)
	if strings.Index(vanillaHTML, `name="title"`) > strings.Index(vanillaHTML, `name="published"`) {
		t.Fatalf("vanilla field order changed:\n%s", vanillaHTML)
	}
	if !strings.Contains(vanillaHTML, `value="Runtime"`) || !strings.Contains(vanillaHTML, `checked`) {
		t.Fatalf("vanilla values changed shape:\n%s", vanillaHTML)
	}

	preactRenderer, err := preact.New()
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}
	preactOut, err := preactRenderer.Render(testsupport.Context(), form, render.RenderOptions{Values: values})
	if err != nil {
		t.Fatalf("preact render: %v", err)
	}
	payload := extractPreactJSON(t, string(preactOut))
	var preactPayload struct {
		Fields []model.Field `json:"fields"`
	}
	if unmarshalErr := json.Unmarshal([]byte(payload), &preactPayload); unmarshalErr != nil {
		t.Fatalf("preact payload unmarshal: %v", unmarshalErr)
	}
	assertFieldOrder(t, preactPayload.Fields, []string{"title", "published"})
	if preactPayload.Fields[0].Default != "Runtime" || preactPayload.Fields[1].Default != false {
		t.Fatalf("preact payload values changed shape: %#v", preactPayload.Fields)
	}

	tuiRenderer, err := tui.New(
		tui.WithPromptDriver(&parityDriver{inputs: []string{"Runtime"}, confirm: []bool{false}}),
	)
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	tuiOut, err := tuiRenderer.Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("tui render: %v", err)
	}
	var tuiValues map[string]any
	if err := json.Unmarshal(tuiOut, &tuiValues); err != nil {
		t.Fatalf("tui unmarshal: %v", err)
	}
	if tuiValues["title"] != "Runtime" || tuiValues["published"] != false {
		t.Fatalf("tui values changed shape: %#v", tuiValues)
	}
}

func assertFieldOrder(t *testing.T, fields []model.Field, want []string) {
	t.Helper()
	if len(fields) != len(want) {
		t.Fatalf("field count mismatch: got %d want %d", len(fields), len(want))
	}
	for i := range want {
		if fields[i].Name != want[i] {
			t.Fatalf("field order mismatch at %d: got %q want %q", i, fields[i].Name, want[i])
		}
	}
}

func extractPreactJSON(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, `<script id="formgen-preact-data" type="application/json">`)
	if start < 0 {
		t.Fatalf("preact payload script missing:\n%s", html)
	}
	start += len(`<script id="formgen-preact-data" type="application/json">`)
	end := strings.Index(html[start:], `</script>`)
	if end < 0 {
		t.Fatalf("preact payload script not closed:\n%s", html)
	}
	return html[start : start+end]
}

type parityDriver struct {
	inputs  []string
	confirm []bool
}

func (d *parityDriver) Input(_ context.Context, _ tui.InputConfig) (string, error) {
	if len(d.inputs) == 0 {
		return "", errors.New("no input scripted")
	}
	value := d.inputs[0]
	d.inputs = d.inputs[1:]
	return value, nil
}

func (d *parityDriver) Password(ctx context.Context, cfg tui.InputConfig) (string, error) {
	return d.Input(ctx, cfg)
}

func (d *parityDriver) Confirm(_ context.Context, _ tui.ConfirmConfig) (bool, error) {
	if len(d.confirm) == 0 {
		return false, errors.New("no confirm scripted")
	}
	value := d.confirm[0]
	d.confirm = d.confirm[1:]
	return value, nil
}

func (d *parityDriver) Select(_ context.Context, _ tui.SelectConfig) (int, error) {
	return 0, errors.New("no select scripted")
}

func (d *parityDriver) MultiSelect(_ context.Context, _ tui.SelectConfig) ([]int, error) {
	return nil, errors.New("no multiselect scripted")
}

func (d *parityDriver) TextArea(ctx context.Context, cfg tui.TextAreaConfig) (string, error) {
	return d.Input(ctx, tui.InputConfig{Message: cfg.Message, Default: cfg.Default, Help: cfg.Help})
}

func (d *parityDriver) Repeat(_ context.Context, _ tui.RepeatConfig) ([][]byte, error) {
	return nil, tui.ErrRepeatUnsupported
}

func (d *parityDriver) Info(_ context.Context, _ string) error {
	return nil
}
