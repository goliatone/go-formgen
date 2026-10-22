package conditions_test

import (
	"reflect"
	"testing"

	"github.com/goliatone/go-formgen/pkg/conditions"
	"github.com/goliatone/go-formgen/pkg/model"
)

func leaf(op, path string, value any) *model.Expression {
	return &model.Expression{Op: op, Path: path, Value: value}
}
func base() model.FormModel {
	return model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "a", Type: model.FieldTypeInteger}, {Name: "b", Type: model.FieldTypeString}, {Name: "c", Type: model.FieldTypeBoolean}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion}}
}
func TestEffectiveChainsAndIndependentRetainedValues(t *testing.T) {
	form := base()
	form.Conditions.Rules = []model.ConditionRule{
		{Path: "b", ApplicableWhen: leaf("present", "a", nil), OnInapplicable: "retain"},
		{Path: "a", ApplicableWhen: leaf("eq", "enabled", true)},
		{Path: "c", ApplicableWhen: &model.Expression{Op: "or", Args: []model.Expression{{Op: "eq", Path: "b", Value: "yes"}, {Op: "and", Args: []model.Expression{{Op: "eq", Path: "enabled", Value: true}, {Op: "eq", Path: "a", Value: 0}}}}}},
	}
	p, ds := conditions.Compile(form)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	input := map[string]any{"enabled": false, "a": int64(0), "b": "yes", "c": true}
	got := p.Evaluate(input)
	if len(got.Values) != 1 || got.Applicable["b"] || got.Applicable["c"] || got.Retained["b"] != "yes" {
		t.Fatalf("%+v", got)
	}
	if len(input) != 4 {
		t.Fatal("mutated input")
	}
	// Mutation of the authoritative source after compilation cannot alter the plan.
	form.Conditions.Rules[1].ApplicableWhen.Value = false
	if p.Evaluate(input).Applicable["a"] {
		t.Fatal("plan aliases definition")
	}
	input["enabled"] = true
	got = p.Evaluate(input)
	if !got.Applicable["a"] || !got.Applicable["c"] {
		t.Fatal(got)
	}
}
func TestPresenceAndTypedEquality(t *testing.T) {
	for _, test := range []struct {
		value         any
		present, want bool
	}{{nil, true, false}, {false, true, true}, {0, true, true}, {"", true, true}, {"false", true, true}, {false, false, false}} {
		form := base()
		form.Conditions.Rules = []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("present", "enabled", nil)}}
		p, ds := conditions.Compile(form)
		if len(ds) != 0 {
			t.Fatal(ds)
		}
		input := map[string]any{}
		if test.present {
			input["enabled"] = test.value
		}
		if got := p.Evaluate(input).Applicable["a"]; got != test.want {
			t.Fatalf("%+v: %v", test, got)
		}
	}
	form := base()
	form.Conditions.Rules = []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("eq", "enabled", false)}}
	p, _ := conditions.Compile(form)
	if p.Evaluate(map[string]any{"enabled": "false"}).Applicable["a"] || p.Evaluate(nil).Applicable["a"] || !p.Evaluate(map[string]any{"enabled": false}).Applicable["a"] {
		t.Fatal("equality inferred false")
	}
}
func TestParentAndFixedIndexedConditions(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject, Nested: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "answer", Type: model.FieldTypeString}}}
	form := base()
	form.Fields = append(form.Fields, model.Field{Name: "rows", Type: model.FieldTypeArray, Items: &item}, model.Field{Name: "profile", Type: model.FieldTypeObject, Nested: []model.Field{{Name: "answer", Type: model.FieldTypeString}}})
	form.Conditions.Rules = []model.ConditionRule{
		{Path: "rows[1].answer", ApplicableWhen: leaf("eq", "rows[1].enabled", true)},
		{Path: "profile.answer", ApplicableWhen: leaf("present", "enabled", nil)},
		{Path: "profile", ApplicableWhen: leaf("eq", "enabled", true)},
	}
	p, ds := conditions.Compile(form)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	input := map[string]any{"enabled": false, "profile": map[string]any{"answer": "secret"}, "rows": []any{map[string]any{"answer": "keep"}, map[string]any{"enabled": false, "answer": "drop"}}}
	got := p.Evaluate(input)
	if _, ok := got.Values["profile"]; ok || got.Applicable["profile.answer"] {
		t.Fatal(got)
	}
	rows := got.Values["rows"].([]any)
	if _, ok := rows[1].(map[string]any)["answer"]; ok || rows[0].(map[string]any)["answer"] != "keep" {
		t.Fatal(got)
	}
	if reflect.DeepEqual(got.Values, input) {
		t.Fatal("projection failed")
	}
}
func TestInvalidDefinitions(t *testing.T) {
	cases := []model.ConditionSet{
		{Version: "future"},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "missing", ApplicableWhen: leaf("present", "enabled", nil)}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("eq", "missing", true)}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("js", "enabled", true)}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("eq", "enabled", nil)}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: &model.Expression{Op: "and"}}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("present", "a", nil)}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("present", "b", nil)}, {Path: "b", ApplicableWhen: leaf("present", "a", nil)}}},
		{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("present", "enabled", nil), OnInapplicable: "guess"}}},
	}
	for i, set := range cases {
		form := base()
		form.Conditions = &set
		p, ds := conditions.Compile(form)
		if p != nil || len(ds) == 0 {
			t.Fatalf("case %d: %+v", i, ds)
		}
	}
}

func TestRetainedCopiesAndClearReentry(t *testing.T) {
	form := base()
	form.Fields = append(form.Fields, model.Field{Name: "object", Type: model.FieldTypeObject, Nested: []model.Field{{Name: "answer", Type: model.FieldTypeString}}})
	form.Conditions.Rules = []model.ConditionRule{{Path: "a", ApplicableWhen: leaf("eq", "enabled", true)}, {Path: "object", ApplicableWhen: leaf("eq", "enabled", true), OnInapplicable: "retain"}}
	p, ds := conditions.Compile(form)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	input := map[string]any{"enabled": false, "a": int64(4), "object": map[string]any{"answer": "saved"}}
	projected := p.Evaluate(input)
	projected.Retained["object"].(map[string]any)["answer"] = "mutated"
	if input["object"].(map[string]any)["answer"] != "saved" {
		t.Fatal("retained aliases input")
	}
	projected.Values["enabled"] = true
	next := p.Evaluate(projected.Values)
	if !next.Applicable["a"] {
		t.Fatal("reentry inactive")
	}
	if _, exists := next.Values["a"]; exists {
		t.Fatal("clear restored stale input")
	}
}

func TestIndexedConditionsOnlyApplyToExistingRows(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject, Nested: []model.Field{{Name: "answer", Type: model.FieldTypeString, Required: true}}}
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "rows", Type: model.FieldTypeArray, Items: &item}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "rows[2].answer", ApplicableWhen: leaf("eq", "enabled", true)}}}}
	p, diagnostics := conditions.Compile(form)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	for _, rows := range []any{nil, []any{}, []any{map[string]any{}}} {
		got := p.Evaluate(map[string]any{"enabled": true, "rows": rows})
		if got.Applicable["rows[2].answer"] || got.Required["rows[2].answer"] {
			t.Fatalf("phantom row: %+v", got)
		}
	}
	got := p.Evaluate(map[string]any{"enabled": true, "rows": []any{map[string]any{}, map[string]any{}, map[string]any{}}})
	if !got.Applicable["rows[2].answer"] || !got.Required["rows[2].answer"] {
		t.Fatalf("existing row lost requiredness: %+v", got)
	}
}
