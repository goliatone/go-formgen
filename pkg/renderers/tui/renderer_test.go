package tui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/render"
)

type stubDriver struct {
	inputs       []string
	selectIdx    []int
	multiIdx     [][]int
	confirm      []bool
	textAreas    []string
	passwords    []string
	infoMessages []string
	inputPos     int
	selectPos    int
	multiPos     int
	confirmPos   int
	textPos      int
	passPos      int
}

func (s *stubDriver) Input(_ context.Context, _ InputConfig) (string, error) {
	if s.inputPos >= len(s.inputs) {
		return "", errors.New("no input scripted")
	}
	val := s.inputs[s.inputPos]
	s.inputPos++
	return val, nil
}

func (s *stubDriver) Password(_ context.Context, _ InputConfig) (string, error) {
	if s.passPos >= len(s.passwords) {
		return "", errors.New("no password scripted")
	}
	val := s.passwords[s.passPos]
	s.passPos++
	return val, nil
}

func (s *stubDriver) Confirm(_ context.Context, _ ConfirmConfig) (bool, error) {
	if s.confirmPos >= len(s.confirm) {
		return false, errors.New("no confirm scripted")
	}
	val := s.confirm[s.confirmPos]
	s.confirmPos++
	return val, nil
}

func (s *stubDriver) Select(_ context.Context, _ SelectConfig) (int, error) {
	if s.selectPos >= len(s.selectIdx) {
		return -1, errors.New("no select scripted")
	}
	val := s.selectIdx[s.selectPos]
	s.selectPos++
	return val, nil
}

func (s *stubDriver) MultiSelect(_ context.Context, _ SelectConfig) ([]int, error) {
	if s.multiPos >= len(s.multiIdx) {
		return nil, errors.New("no multiselect scripted")
	}
	val := s.multiIdx[s.multiPos]
	s.multiPos++
	return val, nil
}

func (s *stubDriver) TextArea(_ context.Context, _ TextAreaConfig) (string, error) {
	if s.textPos >= len(s.textAreas) {
		return "", errors.New("no textarea scripted")
	}
	val := s.textAreas[s.textPos]
	s.textPos++
	return val, nil
}

func (s *stubDriver) Repeat(_ context.Context, _ RepeatConfig) ([][]byte, error) {
	return nil, ErrRepeatUnsupported
}

func (s *stubDriver) Info(_ context.Context, msg string) error {
	s.infoMessages = append(s.infoMessages, msg)
	return nil
}

func TestRender_StringAndEnum(t *testing.T) {
	driver := &stubDriver{
		inputs:    []string{"hello"},
		selectIdx: []int{1},
	}
	r, err := New(WithPromptDriver(driver))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		Fields: []model.Field{
			{
				Name:  "title",
				Type:  model.FieldTypeString,
				Label: "Title",
			},
			{
				Name:  "status",
				Type:  model.FieldTypeString,
				Label: "Status",
				Enum:  []any{"draft", "published"},
			},
		},
	}

	out, err := r.Render(context.Background(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["title"] != "hello" || payload["status"] != "published" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestRender_NumberValidation(t *testing.T) {
	driver := &stubDriver{
		inputs: []string{"-1", "10"},
	}
	r, err := New(WithPromptDriver(driver))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	minVal := map[string]string{"value": "0"}
	form := model.FormModel{
		Fields: []model.Field{
			{
				Name:     "count",
				Type:     model.FieldTypeInteger,
				Label:    "Count",
				Required: true,
				Validations: []model.ValidationRule{
					{Kind: model.ValidationRuleMin, Params: minVal},
				},
			},
		},
	}

	_, err = r.Render(context.Background(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(driver.infoMessages) == 0 {
		t.Fatalf("expected validation message for first invalid input")
	}
}

func TestRender_RelationshipOptionsFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"1","label":"One"},{"id":"2","label":"Two"}]`))
	}))
	defer server.Close()

	driver := &stubDriver{
		selectIdx: []int{1}, // pick "Two"
	}
	r, err := New(
		WithPromptDriver(driver),
		WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		Fields: []model.Field{
			{
				Name:  "author_id",
				Label: "Author",
				Type:  model.FieldTypeString,
				Relationship: &model.Relationship{
					Kind:        model.RelationshipBelongsTo,
					Cardinality: "one",
				},
				Metadata: map[string]string{
					"relationship.endpoint.url":        server.URL,
					"relationship.endpoint.labelField": "label",
					"relationship.endpoint.valueField": "id",
				},
			},
		},
	}

	out, err := r.Render(context.Background(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["author_id"] != "2" {
		t.Fatalf("expected author_id 2, got %+v", payload)
	}
}

func TestRender_RelationshipManualFallback(t *testing.T) {
	driver := &stubDriver{
		inputs: []string{"abc-123"},
	}
	r, err := New(WithPromptDriver(driver))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		Fields: []model.Field{
			{
				Name:  "tag_id",
				Label: "Tag",
				Type:  model.FieldTypeString,
				Relationship: &model.Relationship{
					Kind:        model.RelationshipBelongsTo,
					Cardinality: "one",
				},
			},
		},
	}

	out, err := r.Render(context.Background(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["tag_id"] != "abc-123" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestRender_FormURLEncodedOutput(t *testing.T) {
	driver := &stubDriver{
		inputs: []string{"hello"},
	}
	r, err := New(
		WithPromptDriver(driver),
		WithOutputFormat(OutputFormatFormURLEncoded),
	)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString, Label: "Title"},
		},
	}

	out, err := r.Render(context.Background(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(out) != "title=hello" {
		t.Fatalf("expected form output, got %s", string(out))
	}
}

func TestRender_ArrayAndObject(t *testing.T) {
	driver := &stubDriver{
		inputs:  []string{"first", "user@example.com"},
		confirm: []bool{false}, // add another? -> no
	}
	r, err := New(WithPromptDriver(driver))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		Fields: []model.Field{
			{
				Name:     "tags",
				Type:     model.FieldTypeArray,
				Required: true,
				Items: &model.Field{
					Type:  model.FieldTypeString,
					Label: "Tag",
				},
			},
			{
				Name:  "author",
				Type:  model.FieldTypeObject,
				Label: "Author",
				Nested: []model.Field{
					{Name: "email", Type: model.FieldTypeString, Label: "Email"},
				},
			},
		},
	}

	out, err := r.Render(context.Background(), form, render.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tags, ok := payload["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "first" {
		t.Fatalf("unexpected tags: %#v", payload["tags"])
	}
	author, _ := payload["author"].(map[string]any)
	if author["email"] != "user@example.com" {
		t.Fatalf("unexpected author: %#v", payload["author"])
	}
}

func TestRender_Abort(t *testing.T) {
	r, err := New(WithPromptDriver(abortDriver{}))
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	form := model.FormModel{
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString},
		},
	}

	if _, err := r.Render(context.Background(), form, render.RenderOptions{}); !errors.Is(err, ErrAborted) {
		t.Fatalf("expected ErrAborted, got %v", err)
	}
}

// abortDriver short-circuits prompts to simulate user abort.
type abortDriver struct{}

func (abortDriver) Input(context.Context, InputConfig) (string, error)       { return "", ErrAborted }
func (abortDriver) Password(context.Context, InputConfig) (string, error)    { return "", ErrAborted }
func (abortDriver) Confirm(context.Context, ConfirmConfig) (bool, error)     { return false, ErrAborted }
func (abortDriver) Select(context.Context, SelectConfig) (int, error)        { return -1, ErrAborted }
func (abortDriver) MultiSelect(context.Context, SelectConfig) ([]int, error) { return nil, ErrAborted }
func (abortDriver) TextArea(context.Context, TextAreaConfig) (string, error) { return "", ErrAborted }
func (abortDriver) Repeat(context.Context, RepeatConfig) ([][]byte, error)   { return nil, ErrAborted }
func (abortDriver) Info(context.Context, string) error                       { return ErrAborted }
