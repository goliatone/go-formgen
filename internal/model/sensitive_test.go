package model

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestBuilderMarksSensitiveFields(t *testing.T) {
	form := schema.Form{
		ID:       "sensitive",
		Method:   "POST",
		Endpoint: "/sensitive",
		Schema: schema.Schema{
			Type:     "object",
			Required: []string{"password"},
			Properties: map[string]schema.Schema{
				"password": {Type: "string", Format: "password", Default: "secret"},
				"apiKey": {
					Type:    "string",
					Default: "key",
					Extensions: map[string]any{
						"x-formgen": map[string]any{"secret": true},
					},
				},
				"token": {
					Type:    "string",
					Default: "token",
					Extensions: map[string]any{
						"x-admin": map[string]any{"secret": true},
					},
				},
				"cli": {
					Type:    "string",
					Default: "cli",
					Extensions: map[string]any{
						"cli.secret": true,
					},
				},
			},
		},
	}
	model, err := New(Options{}).Build(form)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	got := map[string]bool{}
	for _, field := range model.Fields {
		got[field.Name] = field.Sensitive
	}
	for _, name := range []string{"password", "apiKey", "token", "cli"} {
		if !got[name] {
			t.Fatalf("expected %s to be sensitive: %#v", name, got)
		}
	}
}
