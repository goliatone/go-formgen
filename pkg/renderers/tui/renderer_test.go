package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/formgen/pkg/model"
)

type stubDriver struct {
	inputs      []string
	selectIdx   []int
	multiIdx    [][]int
	confirm     []bool
	textAreas   []string
	passwords   []string
	infoMessages []string
	inputPos    int
	selectPos   int
	multiPos    int
	confirmPos  int
	textPos     int
	passPos     int
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

	out, err := r.Render(context.Background(), form, modelRenderOpts())
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	got := string(out)
	if got == "" {
		t.Fatalf("expected output, got empty")
	}
	if driver.inputPos != 1 || driver.selectPos != 1 {
		t.Fatalf("prompts not consumed as expected")
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

	_, err = r.Render(context.Background(), form, modelRenderOpts())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(driver.infoMessages) == 0 {
		t.Fatalf("expected validation message for first invalid input")
	}
}

func modelRenderOpts() renderOptionsWrapper {
	return renderOptionsWrapper{}
}

// renderOptionsWrapper exists to avoid importing pkg/render directly in tests.
type renderOptionsWrapper struct{}

