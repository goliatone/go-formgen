package json_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	jsonrenderer "github.com/goliatone/go-formgen/pkg/renderers/json"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

func TestRendererEnvelopeSeparatesDescriptorSections(t *testing.T) {
	form := model.FormModel{
		OperationID: "createSecret",
		Endpoint:    "/vault",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString, Default: "Draft"},
			{Name: "password", Type: model.FieldTypeString, Format: "password", Sensitive: true, Default: "top-secret-token"},
		},
	}

	out, err := jsonrenderer.New().Render(testsupport.Context(), form, render.RenderOptions{
		Values:       map[string]any{"title": "Runtime"},
		Errors:       map[string][]string{"title": {"Required"}},
		FormErrors:   []string{"Cannot save"},
		HiddenFields: map[string]string{"csrf": "token"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), "top-secret-token") {
		t.Fatalf("sensitive default leaked:\n%s", out)
	}
	if form.Fields[1].Default != "top-secret-token" {
		t.Fatalf("renderer mutated source form default")
	}

	var descriptor struct {
		Version      string               `json:"version"`
		Form         model.FormModel      `json:"form"`
		Values       map[string]any       `json:"values"`
		Errors       map[string][]string  `json:"errors"`
		FormErrors   []string             `json:"formErrors"`
		HiddenFields []render.HiddenField `json:"hiddenFields"`
		Metadata     map[string]any       `json:"metadata"`
	}
	if err := json.Unmarshal(out, &descriptor); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if descriptor.Version != "formgen.descriptor/v1" {
		t.Fatalf("unexpected version: %q", descriptor.Version)
	}
	if descriptor.Values["title"] != "Runtime" {
		t.Fatalf("values not preserved: %#v", descriptor.Values)
	}
	if descriptor.Errors["title"][0] != "Required" {
		t.Fatalf("errors not preserved: %#v", descriptor.Errors)
	}
	if descriptor.FormErrors[0] != "Cannot save" {
		t.Fatalf("form errors not preserved: %#v", descriptor.FormErrors)
	}
	if descriptor.HiddenFields[0].Name != "csrf" {
		t.Fatalf("hidden fields not preserved: %#v", descriptor.HiddenFields)
	}
}

func TestRendererWithoutEnvelopeEmitsFormModel(t *testing.T) {
	form := model.FormModel{
		OperationID: "raw",
		Endpoint:    "/raw",
		Method:      "POST",
		Fields:      []model.Field{{Name: "title", Type: model.FieldTypeString}},
	}
	out, err := jsonrenderer.New(jsonrenderer.WithoutEnvelope()).Render(testsupport.Context(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), `"version"`) || !strings.Contains(string(out), `"operationId": "raw"`) {
		t.Fatalf("expected raw form model output:\n%s", out)
	}
}

func TestRendererRedactsSensitiveRenderValuesAndNestedDefaults(t *testing.T) {
	form := model.FormModel{
		OperationID: "nestedSecret",
		Endpoint:    "/secret",
		Method:      "POST",
		Fields: []model.Field{
			{
				Name: "credentials",
				Type: model.FieldTypeObject,
				Default: map[string]any{
					"username": "ada",
					"password": "nested-default-secret",
				},
				Nested: []model.Field{
					{Name: "username", Type: model.FieldTypeString, Default: "ada"},
					{Name: "password", Type: model.FieldTypeString, Format: "password", Sensitive: true, Default: "field-default-secret"},
				},
			},
			{Name: "token", Type: model.FieldTypeString, Sensitive: true, Default: "token-default-secret"},
		},
	}

	out, err := jsonrenderer.New().Render(testsupport.Context(), form, render.RenderOptions{
		Values: map[string]any{
			"credentials": map[string]any{
				"username": "lovelace",
				"password": "nested-runtime-secret",
			},
			"token": "token-runtime-secret",
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, secret := range []string{
		"nested-default-secret",
		"field-default-secret",
		"token-default-secret",
		"nested-runtime-secret",
		"token-runtime-secret",
	} {
		if strings.Contains(string(out), secret) {
			t.Fatalf("sensitive value %q leaked:\n%s", secret, out)
		}
	}

	var descriptor struct {
		Form   model.FormModel `json:"form"`
		Values map[string]any  `json:"values"`
	}
	if err := json.Unmarshal(out, &descriptor); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := descriptor.Values["token"]; ok {
		t.Fatalf("sensitive top-level render value was not removed: %#v", descriptor.Values)
	}
	credentials, ok := descriptor.Values["credentials"].(map[string]any)
	if !ok {
		t.Fatalf("credentials values missing: %#v", descriptor.Values)
	}
	if _, ok := credentials["password"]; ok {
		t.Fatalf("nested sensitive render value was not removed: %#v", credentials)
	}
	defaults, ok := descriptor.Form.Fields[0].Default.(map[string]any)
	if !ok {
		t.Fatalf("credentials default missing: %#v", descriptor.Form.Fields[0].Default)
	}
	if _, ok := defaults["password"]; ok {
		t.Fatalf("nested sensitive default was not removed: %#v", defaults)
	}
	if form.Fields[0].Default.(map[string]any)["password"] != "nested-default-secret" {
		t.Fatalf("renderer mutated source nested default")
	}
}
