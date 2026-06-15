package model

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestBuilderMarksSensitiveFields(t *testing.T) {
	cases := map[string]schema.Schema{
		"passwordFormat": {Type: "string", Format: "password", Default: "secret"},
		"formgenDottedSensitive": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-formgen.sensitive": true},
		},
		"formgenDottedSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-formgen.secret": true},
		},
		"adminDottedSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-admin.secret": true},
		},
		"formgenHyphenSensitive": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-formgen-sensitive": true},
		},
		"formgenHyphenSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-formgen-secret": true},
		},
		"adminHyphenSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-admin-secret": true},
		},
		"cliDottedSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"cli.secret": true},
		},
		"formgenNestedSensitive": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-formgen": map[string]any{"sensitive": true}},
		},
		"formgenNestedSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-formgen": map[string]any{"secret": true}},
		},
		"adminNestedSecret": {
			Type:       "string",
			Default:    "secret",
			Extensions: map[string]any{"x-admin": map[string]any{"secret": true}},
		},
	}

	form := schema.Form{
		ID:       "sensitive",
		Method:   "POST",
		Endpoint: "/sensitive",
		Schema: schema.Schema{
			Type:       "object",
			Properties: cases,
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

	for name := range cases {
		if !got[name] {
			t.Fatalf("expected %s to be sensitive: %#v", name, got)
		}
	}
}
