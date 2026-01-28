package orchestrator_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/orchestrator"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestOrchestrator_AppliesUIDecorators(t *testing.T) {
	t.Helper()

	decorator := model.DecoratorFunc(func(form *model.FormModel) error {
		if form.Metadata == nil {
			form.Metadata = make(map[string]string)
		}
		form.Metadata["decorated"] = "true"
		return nil
	})

	baseForm := model.FormModel{
		OperationID: "post-book:create",
		Endpoint:    "/book",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString},
		},
	}

	builder := &stubFormBuilder{form: baseForm}
	renderer := &stubRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	orch := orchestrator.New(
		orchestrator.WithModelBuilder(builder),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(renderer.Name()),
		orchestrator.WithParser(stubParser{operation: pkgopenapi.Operation{ID: baseForm.OperationID, Path: baseForm.Endpoint, Method: baseForm.Method}}),
		orchestrator.WithUISchemaFS(nil),
		orchestrator.WithUIDecorators(decorator),
	)

	output, err := orch.Generate(context.Background(), orchestrator.Request{
		Document:    &pkgopenapi.Document{},
		OperationID: baseForm.OperationID,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if string(output) != "ok" {
		t.Fatalf("unexpected renderer output: %s", output)
	}
	if renderer.last.Metadata["decorated"] != "true" {
		t.Fatalf("decorator not applied: %#v", renderer.last.Metadata)
	}
}

type stubFormBuilder struct {
	form model.FormModel
}

func (s *stubFormBuilder) Build(schema.Form) (model.FormModel, error) {
	return s.form, nil
}

func (s *stubFormBuilder) Decorate(*model.FormModel) error {
	return nil
}

type stubRenderer struct {
	last model.FormModel
}

func (s *stubRenderer) Name() string {
	return "stub"
}

func (s *stubRenderer) ContentType() string {
	return "text/plain"
}

func (s *stubRenderer) Render(_ context.Context, form model.FormModel, _ render.RenderOptions) ([]byte, error) {
	s.last = form
	return []byte("ok"), nil
}

type stubParser struct {
	operation pkgopenapi.Operation
}

func (s stubParser) Operations(context.Context, pkgopenapi.Document) (map[string]pkgopenapi.Operation, error) {
	return map[string]pkgopenapi.Operation{s.operation.ID: s.operation}, nil
}
