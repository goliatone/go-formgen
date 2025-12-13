package render_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
)

type stubTranslator map[string]string

func (t stubTranslator) Translate(_ string, key string, _ ...any) (string, error) {
	if msg, ok := t[key]; ok {
		return msg, nil
	}
	return "", errors.New("missing translation")
}

func TestLocalizeFormModel_UsesKeysAndFallbacks(t *testing.T) {
	form := model.FormModel{
		OperationID: "createThing",
		UIHints: map[string]string{
			"layout.title":    "Create Thing",
			"layout.titleKey": "forms.createThing.title",
		},
		Metadata: map[string]string{
			"actions":         `[{"kind":"primary","label":"Save","labelKey":"actions.save","type":"submit"}]`,
			"layout.sections": `[{"id":"main","title":"","titleKey":"sections.main.title","description":"Main fields","descriptionKey":"sections.main.description","order":0,"fieldset":true}]`,
		},
		Fields: []model.Field{
			{
				Name:        "name",
				Label:       "Name",
				Placeholder: "Enter name",
				UIHints: map[string]string{
					"labelKey":       "fields.thing.name",
					"placeholderKey": "fields.thing.name.placeholder",
					"helpText":       "Used for display",
					"helpTextKey":    "fields.thing.name.help",
				},
			},
		},
	}

	render.LocalizeFormModel(&form, render.RenderOptions{
		Locale:     "es",
		Translator: stubTranslator{"fields.thing.name": "Nombre"},
	})

	if form.UIHints["layout.title"] != "Create Thing" {
		t.Fatalf("expected layout.title to fall back when missing, got %q", form.UIHints["layout.title"])
	}
	if form.Fields[0].Label != "Nombre" {
		t.Fatalf("expected translated field label, got %q", form.Fields[0].Label)
	}

	var actions []map[string]any
	if err := json.Unmarshal([]byte(form.Metadata["actions"]), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v", err)
	}
	if actions[0]["label"] != "Save" {
		t.Fatalf("expected actions label to fall back, got %#v", actions[0])
	}

	var sections []map[string]any
	if err := json.Unmarshal([]byte(form.Metadata["layout.sections"]), &sections); err != nil {
		t.Fatalf("unmarshal sections: %v", err)
	}
	if sections[0]["title"] != "sections.main.title" {
		t.Fatalf("expected section title to default to key when no fallback, got %#v", sections[0]["title"])
	}
	if sections[0]["description"] != "Main fields" {
		t.Fatalf("expected section description to fall back, got %#v", sections[0]["description"])
	}
}
