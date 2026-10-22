package jsonschema

import (
	"context"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/schema"
	"github.com/goliatone/go-formgen/pkg/submission"
)

func TestConditionsCompileThroughJSONSchema(t *testing.T) {
	raw := `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"conditions","type":"object","properties":{"enabled":{"type":"boolean"},"answer":{"type":"string"}},"x-formgen":{"conditions":{"version":"formgen.conditions/v1","rules":[{"path":"answer","applicableWhen":{"op":"eq","path":"enabled","value":true},"requiredWhen":{"op":"present","path":"enabled"}}]}}}`
	adapter := NewAdapter(failingLoader{})
	ir, err := adapter.Normalize(context.Background(), MustNewDocument(SourceFromFS("root.json"), []byte(raw)), schema.NormalizeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := ir.Form("conditions.edit")
	form, err := model.NewBuilder().Build(definition)
	if err != nil {
		t.Fatal(err)
	}
	if form.Conditions == nil || len(form.Fields) != 2 {
		t.Fatalf("lost authoritative definition: %+v", form)
	}
	parsed := submission.ParseMap(form, map[string]any{"enabled": true})
	result := submission.ValidateConditional(form, parsed, submission.InactiveReject)
	if len(result.Issues) != 1 || result.Issues[0].Path != "answer" || result.Issues[0].Code != submission.CodeRequired {
		t.Fatal(result)
	}
	definition.Schema.Extensions["x-formgen"] = map[string]any{"conditions": map[string]any{"version": model.ConditionsVersion, "rules": []any{map[string]any{"path": "answer", "applicableWhen": map[string]any{"op": "present", "path": "answer"}}}}}
	if _, err := model.NewBuilder().Build(definition); err == nil || !strings.Contains(err.Error(), "conditionCycle") {
		t.Fatalf("cycle: %v", err)
	}
	definition.Schema.Extensions["x-formgen"] = map[string]any{"conditions": map[string]any{"version": model.ConditionsVersion, "invented": "private-literal"}}
	if _, err := model.NewBuilder().Build(definition); err == nil || strings.Contains(err.Error(), "private-literal") {
		t.Fatalf("shape diagnostics: %v", err)
	}
}
