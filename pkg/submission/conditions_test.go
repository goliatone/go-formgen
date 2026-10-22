package submission

import (
	"encoding/json"
	"github.com/goliatone/go-formgen/pkg/conditions"
	"github.com/goliatone/go-formgen/pkg/model"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestSharedEmbeddedConformance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Form              model.FormModel
		InvalidConditions []struct {
			ID         string
			Definition model.ConditionSet
		}
		Cases []struct {
			ID     string
			Raw    map[string]any
			Issues []string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.InvalidConditions {
		t.Run(test.ID, func(t *testing.T) {
			form := fixture.Form
			form.Conditions = &test.Definition
			if p, ds := conditions.Compile(form); p != nil || len(ds) == 0 {
				t.Fatalf("invalid definition accepted: %+v", test)
			}
		})
	}
	for _, test := range fixture.Cases {
		t.Run(test.ID, func(t *testing.T) {
			result := ValidateConditional(fixture.Form, ParseMap(fixture.Form, test.Raw), InactiveReject)
			var got []string
			for _, issue := range result.Issues {
				got = append(got, string(issue.Code)+":"+issue.Path)
			}
			sort.Strings(got)
			sort.Strings(test.Issues)
			if len(got) != len(test.Issues) {
				t.Fatalf("got %v want %v", got, test.Issues)
			}
			for i := range got {
				if got[i] != test.Issues[i] {
					t.Fatalf("got %v want %v", got, test.Issues)
				}
			}
			if test.ID == "C04" && result.Values["level"] != int64(4) {
				t.Fatalf("untyped level: %+v", result.Values)
			}
			if test.ID == "C07" {
				normalized := ValidateConditional(fixture.Form, ParseMap(fixture.Form, test.Raw), InactiveNormalize)
				if len(normalized.Issues) != 0 {
					t.Fatal(normalized)
				}
				if _, exists := normalized.Values["level"]; exists {
					t.Fatal("stale value survived")
				}
			}
		})
	}
}
func TestConditionalPreservesIssuesAndMissingNestedRequiredness(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "future", Type: model.FieldTypeInteger, Required: true}, {Name: "profile", Type: model.FieldTypeObject, Nested: []model.Field{{Name: "answer", Type: model.FieldTypeString}}}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "profile.answer", RequiredWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}}}}
	parsed := ParseMap(form, map[string]any{"enabled": true, "future": "bad", "unknown": "host"})
	got := ValidateConditional(form, parsed, InactiveReject)
	codes := map[string]bool{}
	for _, issue := range got.Issues {
		codes[string(issue.Code)+":"+issue.Path] = true
	}
	for _, want := range []string{"type:future", "unknownField:unknown", "required:future", "required:profile.answer"} {
		if !codes[want] {
			t.Fatalf("missing %s in %+v", want, got)
		}
	}
	form.Conditions.Rules[0].RequiredWhen.Op = "unsupported"
	got = ValidateConditional(form, parsed, InactiveReject)
	if got.Valid() || len(got.Values) != 0 || len(got.Issues) < len(parsed.Issues)+1 {
		t.Fatal(got)
	}
}

func TestConditionalArraysValidateOnlyApplicableMembers(t *testing.T) {
	item := model.Field{Type: model.FieldTypeBoolean, Required: true}
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "answers", Type: model.FieldTypeArray, Items: &item, Enum: []any{false, true}, Validations: []model.ValidationRule{{Kind: model.ValidationRuleMinItems, Params: map[string]string{"value": "1"}}, {Kind: model.ValidationRuleMaxItems, Params: map[string]string{"value": "1"}}, {Kind: model.ValidationRuleUniqueItems}}}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "answers[0]", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}, {Path: "answers[1]", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}}}}
	for _, test := range []struct {
		name    string
		answers []any
		want    IssueCode
	}{
		{"inactive sole item", []any{true}, CodeMinItems},
		{"inactive duplicate placeholders", []any{true, false, true}, ""},
		{"inactive null placeholders", []any{nil, nil, true}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed := ParseMap(form, map[string]any{"enabled": false, "answers": test.answers})
			result := ValidateConditional(form, parsed, InactiveNormalize)
			if test.want == "" {
				if len(result.Issues) != 0 {
					t.Fatal(result.Issues)
				}
			} else if len(result.Issues) != 1 || result.Issues[0].Code != test.want {
				t.Fatal(result.Issues)
			}
		})
	}
}

func TestConditionalRawObjectProjectionsOwnTypedContainers(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "object", Type: model.FieldTypeObject}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "object", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}, OnInapplicable: "retain"}}}}
	for _, enabled := range []bool{false, true} {
		original := map[string]any{"codes": []string{"original"}, "groups": map[string][]string{"team": {"original"}}, "rows": []map[string]any{{"answer": "original"}}, "large": int64(9007199254740993)}
		parsed := ParseMap(form, map[string]any{"enabled": enabled, "object": original})
		result := ValidateConditional(form, parsed, InactiveNormalize)
		if len(result.Issues) != 0 {
			t.Fatal(result.Issues)
		}
		value := result.Values["object"]
		if !enabled {
			value = result.Retained["object"]
		}
		snapshot := value.(map[string]any)
		snapshot["codes"].([]string)[0] = "changed"
		snapshot["groups"].(map[string][]string)["team"][0] = "changed"
		snapshot["rows"].([]map[string]any)[0]["answer"] = "changed"
		if original["codes"].([]string)[0] != "original" || original["groups"].(map[string][]string)["team"][0] != "original" || original["rows"].([]map[string]any)[0]["answer"] != "original" {
			t.Fatalf("snapshot aliases input: %+v", original)
		}
		if snapshot["large"] != int64(9007199254740993) {
			t.Fatalf("large integer changed: %v", snapshot["large"])
		}
		next := ValidateConditional(form, parsed, InactiveNormalize)
		nextValue := next.Values["object"]
		if !enabled {
			nextValue = next.Retained["object"]
		}
		if nextValue.(map[string]any)["codes"].([]string)[0] != "original" {
			t.Fatal("evaluations share snapshot state")
		}
		original["codes"].([]string)[0] = "source changed"
		if snapshot["codes"].([]string)[0] != "changed" || nextValue.(map[string]any)["codes"].([]string)[0] != "original" {
			t.Fatal("source mutation changed a snapshot")
		}
	}
}

func TestConditionalRowsPreserveExistingRequiredProperties(t *testing.T) {
	item := model.Field{Type: model.FieldTypeObject, Nested: []model.Field{{Name: "answer", Type: model.FieldTypeString, Required: true}}}
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "rows", Type: model.FieldTypeArray, Items: &item}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "rows[2].answer", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}}}}
	for _, raw := range []map[string]any{{"enabled": true}, {"enabled": true, "rows": []any{}}, {"enabled": true, "rows": []any{map[string]any{"answer": "ok"}}}} {
		result := ValidateConditional(form, ParseMap(form, raw), InactiveReject)
		if len(result.Issues) != 0 {
			t.Fatalf("nonexistent row became required: %+v", result)
		}
	}
	result := ValidateConditional(form, ParseMap(form, map[string]any{"enabled": true, "rows": []any{map[string]any{"answer": "ok"}, map[string]any{"answer": "ok"}, map[string]any{}}}), InactiveReject)
	if len(result.Issues) != 1 || result.Issues[0].Code != CodeRequired || result.Issues[0].Path != "rows[2].answer" {
		t.Fatal(result)
	}
}

func TestConditionalArrayHolesPreserveReferencesAndReentry(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "answers", Type: model.FieldTypeArray, Items: &model.Field{Type: model.FieldTypeBoolean, Required: true}, Validations: []model.ValidationRule{{Kind: model.ValidationRuleMinItems, Params: map[string]string{"value": "1"}}}}, {Name: "followup", Type: model.FieldTypeString}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "answers[0]", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}, {Path: "followup", ApplicableWhen: &model.Expression{Op: "eq", Path: "answers[1]", Value: true}}}}}
	result := ValidateConditional(form, ParseMap(form, map[string]any{"enabled": false, "answers": []any{true, true}, "followup": "kept"}), InactiveNormalize)
	if len(result.Issues) != 0 || !result.Applicable["followup"] || result.Values["followup"] != "kept" {
		t.Fatal(result)
	}
	if answers := result.Values["answers"].([]any); len(answers) != 2 || answers[0] != nil || answers[1] != true {
		t.Fatal(answers)
	}
	// A projected null slot is structural; the strict server accepts it as no answer.
	repeated := ValidateConditional(form, ParseMap(form, map[string]any(result.Values)), InactiveReject)
	if len(repeated.Issues) != 0 {
		t.Fatal(repeated)
	}
	result.Values["enabled"] = true
	reentered := ValidateConditional(form, ParseMap(form, map[string]any(result.Values)), InactiveReject)
	if len(reentered.Issues) != 1 || reentered.Issues[0].Code != CodeRequired || reentered.Issues[0].Path != "answers[0]" {
		t.Fatal(reentered)
	}
	// False and zero are supplied inactive answers, not structural holes.
	for _, value := range []any{false, 0} {
		rejected := ValidateConditional(form, Result{Values: Values{"enabled": false, "answers": []any{value, true}}}, InactiveReject)
		if len(rejected.Issues) != 1 || rejected.Issues[0].Code != CodeInactiveInput || rejected.Issues[0].Path != "answers[0]" {
			t.Fatal(rejected)
		}
	}
}

func TestConditionalArrayIssuesKeepOriginalIndexes(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{{Name: "enabled", Type: model.FieldTypeBoolean}, {Name: "answers", Type: model.FieldTypeArray, Enum: []any{"blue"}, Items: &model.Field{Type: model.FieldTypeString, Enum: []any{"blue"}}}}, Conditions: &model.ConditionSet{Version: model.ConditionsVersion, Rules: []model.ConditionRule{{Path: "answers[0]", ApplicableWhen: &model.Expression{Op: "eq", Path: "enabled", Value: true}}}}}
	result := ValidateConditional(form, ParseMap(form, map[string]any{"enabled": false, "answers": []any{"red", "red"}}), InactiveNormalize)
	if len(result.Issues) != 2 {
		t.Fatal(result)
	}
	for _, issue := range result.Issues {
		if issue.Code != CodeEnum || issue.Path != "answers[1]" || issue.RendererPath != "answers" {
			t.Fatal(issue)
		}
	}
}

func TestSharedIndexedConformance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/embedded/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Indexed struct {
			Form  model.FormModel
			Cases []struct {
				ID     string
				Raw    map[string]any
				Policy string
				Issues []string
				Values map[string]any
			}
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Indexed.Cases {
		t.Run(test.ID, func(t *testing.T) {
			policy := InactiveReject
			if test.Policy == "normalize" {
				policy = InactiveNormalize
			}
			result := ValidateConditional(fixture.Indexed.Form, ParseMap(fixture.Indexed.Form, test.Raw), policy)
			got := make([]string, 0, len(result.Issues))
			for _, issue := range result.Issues {
				got = append(got, string(issue.Code)+":"+issue.Path)
			}
			sort.Strings(got)
			sort.Strings(test.Issues)
			if !reflect.DeepEqual(got, test.Issues) || !reflect.DeepEqual(map[string]any(result.Values), test.Values) {
				t.Fatalf("issues=%v want=%v values=%+v want=%+v", got, test.Issues, result.Values, test.Values)
			}
		})
	}
}
