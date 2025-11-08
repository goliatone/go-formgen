package orchestrator_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
)

func TestOrchestrator_AppliesTransformer(t *testing.T) {
	baseForm := model.FormModel{
		OperationID: "transformMe",
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString},
		},
	}

	builder := &stubFormBuilder{form: baseForm}
	renderer := &stubRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	transformCalled := false
	transformer := orchestrator.TransformerFunc(func(ctx context.Context, form *model.FormModel) error {
		transformCalled = true
		form.Metadata = map[string]string{"patched": "true"}
		return nil
	})

	orch := orchestrator.New(
		orchestrator.WithModelBuilder(builder),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(renderer.Name()),
		orchestrator.WithParser(stubParser{operation: pkgopenapi.Operation{ID: baseForm.OperationID}}),
		orchestrator.WithUISchemaFS(nil),
		orchestrator.WithSchemaTransformer(transformer),
	)

	_, err := orch.Generate(context.Background(), orchestrator.Request{
		Document:    &pkgopenapi.Document{},
		OperationID: baseForm.OperationID,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !transformCalled {
		t.Fatalf("expected transformer to be invoked")
	}
	if renderer.last.Metadata["patched"] != "true" {
		t.Fatalf("transformer mutation missing: %#v", renderer.last.Metadata)
	}
}

func TestJSONPresetTransformerFromFS(t *testing.T) {
	form := model.FormModel{
		OperationID: "presetForm",
		Fields: []model.Field{
			{
				Name: "title",
				Type: model.FieldTypeString,
			},
			{
				Name: "metrics",
				Type: model.FieldTypeObject,
				Items: &model.Field{
					Name: "metricsItem",
					Type: model.FieldTypeObject,
					Nested: []model.Field{
						{Name: "value", Type: model.FieldTypeNumber},
					},
				},
			},
		},
	}

	path := filepath.Join("testdata")
	fsys := os.DirFS(path)
	transformer, err := orchestrator.NewJSONPresetTransformerFromFS(fsys, "sample_transformer.json")
	if err != nil {
		t.Fatalf("new json transformer: %v", err)
	}

	if err := transformer.Transform(context.Background(), &form); err != nil {
		t.Fatalf("apply transformer: %v", err)
	}

	if form.Metadata["layout.fieldOrder.details"] == "" {
		t.Fatalf("metadata patch missing: %#v", form.Metadata)
	}
	if form.UIHints["layout.title"] != "Transformed Title" {
		t.Fatalf("ui hint missing: %#v", form.UIHints)
	}
	if form.Fields[0].Label != "Custom Title" {
		t.Fatalf("field label not updated: %#v", form.Fields[0])
	}
	if form.Fields[1].Items == nil || len(form.Fields[1].Items.Nested) == 0 {
		t.Fatalf("metrics items missing")
	}
	metric := form.Fields[1].Items.Nested[0]
	if metric.Metadata["unit"] != "ms" {
		t.Fatalf("nested metadata missing: %#v", metric.Metadata)
	}
}

func TestOrchestrator_TransformerErrorAborts(t *testing.T) {
	baseForm := model.FormModel{
		OperationID: "failTransform",
		Fields:      []model.Field{{Name: "title"}},
	}

	builder := &stubFormBuilder{form: baseForm}
	renderer := &stubRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	transformer := orchestrator.TransformerFunc(func(context.Context, *model.FormModel) error {
		return fmt.Errorf("boom")
	})

	orch := orchestrator.New(
		orchestrator.WithModelBuilder(builder),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(renderer.Name()),
		orchestrator.WithParser(stubParser{operation: pkgopenapi.Operation{ID: baseForm.OperationID}}),
		orchestrator.WithUISchemaFS(nil),
		orchestrator.WithSchemaTransformer(transformer),
	)

	_, err := orch.Generate(context.Background(), orchestrator.Request{
		Document:    &pkgopenapi.Document{},
		OperationID: baseForm.OperationID,
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected transformer error, got %v", err)
	}
}
