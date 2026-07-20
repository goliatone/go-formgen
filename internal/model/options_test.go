package model

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestBuilderPreservesRichOptionsFromFormgenExtensions(t *testing.T) {
	form, err := New(Options{}).Build(schema.Form{
		ID:       "rich-options",
		Method:   "POST",
		Endpoint: "/rich-options",
		Schema: schema.Schema{
			Type: "object",
			Properties: map[string]schema.Schema{
				"mode": {
					Type: "string",
					Extensions: map[string]any{"x-formgen": map[string]any{
						"options": []any{
							map[string]any{"value": "safe", "label": "Safe", "description": "Recommended", "disabled": true, "metadata": map[string]any{"tier": "default"}},
						},
					}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(form.Fields) != 1 || len(form.Fields[0].Options) != 1 {
		t.Fatalf("options = %#v", form.Fields)
	}
	option := form.Fields[0].Options[0]
	if option.Value != "safe" || option.Label != "Safe" || option.Description != "Recommended" || !option.Disabled || option.Metadata["tier"] != "default" {
		t.Fatalf("option = %#v", option)
	}
}
