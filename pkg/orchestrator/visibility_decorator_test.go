package orchestrator

import (
	"context"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/visibility"
	visibilityexpr "github.com/goliatone/go-formgen/pkg/visibility/expr"
)

type stubEvaluator struct {
	visible map[string]bool
}

func (s stubEvaluator) Eval(fieldPath, rule string, ctx visibility.Context) (bool, error) {
	if s.visible == nil {
		return true, nil
	}
	value, ok := s.visible[fieldPath]
	if !ok {
		return true, nil
	}
	return value, nil
}

func TestApplyVisibility_RemovesHiddenFields(t *testing.T) {
	t.Parallel()

	form := model.FormModel{
		Fields: []model.Field{
			{
				Name: "name",
				Type: model.FieldTypeString,
			},
			{
				Name: "settings",
				Type: model.FieldTypeObject,
				Nested: []model.Field{
					{
						Name: "enabled",
						Type: model.FieldTypeBoolean,
					},
					{
						Name:     "threshold",
						Type:     model.FieldTypeNumber,
						Metadata: map[string]string{"visibilityRule": "enabled == true"},
					},
				},
			},
		},
	}

	evaluator := stubEvaluator{
		visible: map[string]bool{
			"settings.threshold": false,
		},
	}

	if err := applyVisibility(&form, evaluator, visibility.Context{}); err != nil {
		t.Fatalf("apply visibility: %v", err)
	}

	if len(form.Fields) != 2 {
		t.Fatalf("expected 2 top-level fields, got %d", len(form.Fields))
	}
	settings := form.Fields[1]
	if len(settings.Nested) != 1 || settings.Nested[0].Name != "enabled" {
		t.Fatalf("expected threshold to be removed, got %+v", settings.Nested)
	}
}

func TestOrchestrator_VisibilityEvaluatorIntegration(t *testing.T) {
	t.Parallel()

	form := model.FormModel{
		OperationID: "op",
		Endpoint:    "/op",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "keep", Type: model.FieldTypeString},
			{Name: "hide", Type: model.FieldTypeString, Metadata: map[string]string{"visibilityRule": "hide-me"}},
		},
	}

	evaluator := stubEvaluator{
		visible: map[string]bool{
			"hide": false,
		},
	}

	builder := visibilityBuilder{form: form}
	parser := visibilityParser{form: form}
	renderer := &visibilityRecordingRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	orch := New(
		WithModelBuilder(builder),
		WithParser(parser),
		WithRegistry(registry),
		WithDefaultRenderer(renderer.Name()),
		WithVisibilityEvaluator(evaluator),
	)

	_, err := orch.Generate(context.Background(), Request{
		Document:      &openapi.Document{},
		OperationID:   "op",
		RenderOptions: render.RenderOptions{},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if renderer.lastForm.OperationID != "op" {
		t.Fatalf("renderer did not receive form")
	}
	if len(renderer.lastForm.Fields) != 1 || renderer.lastForm.Fields[0].Name != "keep" {
		t.Fatalf("expected hidden field to be removed, got %+v", renderer.lastForm.Fields)
	}
}

func TestOrchestrator_BuiltInVisibilityEvaluatorIntegration(t *testing.T) {
	t.Parallel()

	form := model.FormModel{
		OperationID: "op",
		Endpoint:    "/op",
		Method:      "POST",
		Fields: []model.Field{
			{Name: "enabled", Type: model.FieldTypeBoolean},
			{Name: "threshold", Type: model.FieldTypeNumber, Metadata: map[string]string{"visibilityRule": "enabled == true"}},
		},
	}

	builder := visibilityBuilder{form: form}
	parser := visibilityParser{form: form}
	renderer := &visibilityRecordingRenderer{}
	registry := render.NewRegistry()
	registry.MustRegister(renderer)

	orch := New(
		WithModelBuilder(builder),
		WithParser(parser),
		WithRegistry(registry),
		WithDefaultRenderer(renderer.Name()),
		WithVisibilityEvaluator(visibilityexpr.New()),
	)

	_, err := orch.Generate(context.Background(), Request{
		Document:    &openapi.Document{},
		OperationID: "op",
		RenderOptions: render.RenderOptions{
			Values: map[string]any{"enabled": false},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(renderer.lastForm.Fields) != 1 || renderer.lastForm.Fields[0].Name != "enabled" {
		t.Fatalf("expected threshold to be removed, got %+v", renderer.lastForm.Fields)
	}
}

type visibilityBuilder struct {
	form model.FormModel
}

func (b visibilityBuilder) Build(schema.Form) (model.FormModel, error) {
	return b.form, nil
}

func (b visibilityBuilder) Decorate(*model.FormModel) error { return nil }

type visibilityParser struct {
	form model.FormModel
}

func (p visibilityParser) Operations(context.Context, openapi.Document) (map[string]openapi.Operation, error) {
	op := openapi.MustNewOperation(p.form.OperationID, p.form.Method, p.form.Endpoint, openapi.Schema{}, nil)
	return map[string]openapi.Operation{op.ID: op}, nil
}

type visibilityRecordingRenderer struct {
	lastForm model.FormModel
}

func (r *visibilityRecordingRenderer) Name() string        { return "recording" }
func (r *visibilityRecordingRenderer) ContentType() string { return "text/html" }
func (r *visibilityRecordingRenderer) Render(_ context.Context, form model.FormModel, _ render.RenderOptions) ([]byte, error) {
	r.lastForm = form
	return []byte("ok"), nil
}
