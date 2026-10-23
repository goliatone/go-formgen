package model_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/schema"
)

func TestEmbeddedContractFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string
		Form    model.FormModel
		Cases   []json.RawMessage
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "formgen.embedded/v1" || fixture.Form.Conditions.Version != model.ConditionsVersion || len(fixture.Cases) < 9 {
		t.Fatal("unsupported contract fixture")
	}
	for _, value := range []any{false, 0, "false"} {
		set := model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "rows[2].answer", ApplicableWhen: &model.Expression{Op: "eq", Path: "rows[2].enabled", Value: value}}}}
		data, err := json.Marshal(set)
		if err != nil {
			t.Fatal(err)
		}
		var got model.ConditionSet
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Rules[0].ApplicableWhen.Value == nil {
			t.Fatalf("lost present value %v", value)
		}
	}
}

// Keep the browser collection-shape regressions aligned with the public builder.
func TestConditionRuleCollectionShape(t *testing.T) {
	tests := []struct {
		name    string
		rules   any
		missing bool
		invalid bool
	}{
		{name: "object", rules: map[string]any{}, invalid: true},
		{name: "string", rules: "rules", invalid: true},
		{name: "number", rules: 1, invalid: true},
		{name: "boolean", rules: false, invalid: true},
		{name: "null", rules: nil},
		{name: "missing", missing: true},
		{name: "empty", rules: []any{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conditions := map[string]any{"version": model.ConditionsVersion}
			if !test.missing {
				conditions["rules"] = test.rules
			}
			_, err := model.NewBuilder().Build(schema.Form{ID: "shape", Method: "POST", Endpoint: "/", Schema: schema.Schema{
				Type: "object", Properties: map[string]schema.Schema{"answer": {Type: "string"}},
				Extensions: map[string]any{"x-formgen": map[string]any{"conditions": conditions}},
			}})
			if (err != nil) != test.invalid {
				t.Fatalf("error = %v; invalid = %v", err, test.invalid)
			}
		})
	}
}

func TestConditionDefinitionShapeParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/definition-shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version string
		Cases   []struct {
			Name       string
			Conditions any
			Invalid    bool
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != "formgen.condition-shapes/v1" {
		t.Fatal("unexpected fixture version")
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			_, err := model.NewBuilder().Build(schema.Form{ID: "shape", Method: "POST", Endpoint: "/", Schema: schema.Schema{
				Type: "object", Properties: map[string]schema.Schema{"answer": {Type: "string"}, "gate": {Type: "boolean"}},
				Extensions: map[string]any{"x-formgen": map[string]any{"conditions": test.Conditions}},
			}})
			if (err != nil) != test.Invalid {
				t.Fatalf("error = %v; invalid = %v", err, test.Invalid)
			}
		})
	}
}
