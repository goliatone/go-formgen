package vanilla_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/jsonschema"
	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

const minimalJSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "widget",
  "type": "object",
  "properties": {
    "name": { "type": "string" }
  },
  "required": ["name"]
}`

type memoryLoader struct {
	docs map[string][]byte
}

func (m memoryLoader) Load(_ context.Context, src jsonschema.Source) (schema.Document, error) {
	raw, ok := m.docs[src.Location()]
	if !ok {
		return schema.Document{}, fmt.Errorf("missing document %q", src.Location())
	}
	return schema.NewDocument(src, raw)
}

func buildFormFromJSONSchema(t *testing.T, raw string) model.FormModel {
	t.Helper()

	src := schema.SourceFromFS("schema.json")
	payload := []byte(raw)
	loader := memoryLoader{docs: map[string][]byte{src.Location(): payload}}
	adapter := jsonschema.NewAdapter(loader)
	doc := schema.MustNewDocument(src, payload)

	ir, err := adapter.Normalize(context.Background(), doc, schema.NormalizeOptions{})
	if err != nil {
		t.Fatalf("normalize schema: %v", err)
	}

	refs := ir.FormRefs()
	if len(refs) == 0 {
		t.Fatal("expected at least one form ref")
	}

	form, ok := ir.Form(refs[0].ID)
	if !ok {
		t.Fatalf("expected form %q to exist", refs[0].ID)
	}

	builder := model.NewBuilder()
	formModel, err := builder.Build(form)
	if err != nil {
		t.Fatalf("build form model: %v", err)
	}
	return formModel
}

func TestRenderer_FormClassOverride(t *testing.T) {
	form := buildFormFromJSONSchema(t, minimalJSONSchema)

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		ChromeClasses: &render.ChromeClasses{
			Form: "space-y-6",
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	got := string(output)
	if !strings.Contains(got, `<form class="space-y-6"`) {
		t.Fatalf("expected form class override, got: %s", got)
	}
	if strings.Contains(got, `<form class="`+vanilla.DefaultFormClass+`"`) {
		t.Fatalf("expected default form class to be replaced")
	}
}

func TestRenderer_FormClassDefault(t *testing.T) {
	form := buildFormFromJSONSchema(t, minimalJSONSchema)

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	output, err := renderer.Render(testsupport.Context(), form, render.RenderOptions{
		ChromeClasses: &render.ChromeClasses{},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(string(output), `<form class="`+vanilla.DefaultFormClass+`"`) {
		t.Fatalf("expected default form class to be preserved")
	}
}
