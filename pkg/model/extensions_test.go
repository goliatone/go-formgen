package model_test

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestParseUIExtensions(t *testing.T) {
	extensions := map[string]any{
		"x-formgen": map[string]any{
			"label":       "Display Name",
			"placeholder": "Enter name",
			"forms": []any{
				map[string]any{"id": "example.edit"},
			},
		},
		"x-admin": map[string]any{
			"visibility-rule": "enabled == true",
			"widget":          "select",
		},
	}

	metadata, hints := model.ParseUIExtensions(extensions)

	if got := metadata["label"]; got != "Display Name" {
		t.Fatalf("expected label metadata, got %q", got)
	}
	if got := metadata["placeholder"]; got != "Enter name" {
		t.Fatalf("expected placeholder metadata, got %q", got)
	}
	if _, ok := metadata["forms"]; ok {
		t.Fatalf("expected forms extension to be ignored")
	}
	if got := metadata["admin.visibilityRule"]; got != "enabled == true" {
		t.Fatalf("expected admin.visibilityRule metadata, got %q", got)
	}
	if got := metadata["visibilityRule"]; got != "enabled == true" {
		t.Fatalf("expected visibilityRule metadata, got %q", got)
	}
	if got := metadata["admin.widget"]; got != "select" {
		t.Fatalf("expected admin.widget metadata, got %q", got)
	}
	if got := metadata["widget"]; got != "select" {
		t.Fatalf("expected widget metadata, got %q", got)
	}

	if got := hints["label"]; got != "Display Name" {
		t.Fatalf("expected label uiHint, got %q", got)
	}
	if got := hints["placeholder"]; got != "Enter name" {
		t.Fatalf("expected placeholder uiHint, got %q", got)
	}
	if got := hints["visibilityRule"]; got != "enabled == true" {
		t.Fatalf("expected visibilityRule uiHint, got %q", got)
	}
	if got := hints["widget"]; got != "select" {
		t.Fatalf("expected widget uiHint, got %q", got)
	}
}
