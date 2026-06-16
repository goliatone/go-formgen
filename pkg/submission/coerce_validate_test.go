package submission_test

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
	"github.com/goliatone/go-formgen/pkg/widgets"
)

func TestEnumHelpersRoundTripTypedValues(t *testing.T) {
	values := []any{"draft", int64(2), uint64(18446744073709551615), 3.5, true, "__fg_enum_v1:not-real"}
	for _, value := range values {
		encoded := submission.EncodeEnumValue(value)
		decoded, ok := submission.DecodeEnumValue(encoded)
		if !ok {
			t.Fatalf("expected encoded value for %v", value)
		}
		if diff := cmp.Diff(value, decoded); diff != "" {
			t.Fatalf("decoded mismatch for %v (-want +got):\n%s", value, diff)
		}
	}

	plain, ok := submission.DecodeEnumValue("__fg_enum_v1:not-real")
	if ok || plain != "__fg_enum_v1:not-real" {
		t.Fatalf("invalid encoded-looking string should remain plain, got value=%v ok=%v", plain, ok)
	}
}

func TestEnumControlValuePreservesPlainStringsAndEncodesCollisions(t *testing.T) {
	if got := submission.EncodeEnumControlValue("draft"); got != "draft" {
		t.Fatalf("plain string control value = %q", got)
	}
	colliding := "__fg_enum_v1:not-real"
	if got := submission.EncodeEnumControlValue(colliding); got == colliding {
		t.Fatalf("encoded-looking string should be encoded to avoid collisions")
	}
	if got := submission.EncodeEnumControlValue(true); got == "true" {
		t.Fatalf("boolean control value should be encoded, got %q", got)
	}
}

func TestCoercionCoversEmptyStringsCheckboxBooleansAndIntegerBounds(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
		{Name: "optional_count", Type: model.FieldTypeInteger},
		{Name: "enabled", Type: model.FieldTypeBoolean},
		{Name: "count", Type: model.FieldTypeInteger},
	}}

	result := submission.ParseValues(form, url.Values{
		"optional_count": {""},
		"enabled":        {"on"},
		"count":          {"9223372036854775808"},
	})

	if result.Values["optional_count"] != nil {
		t.Fatalf("expected optional non-string empty to become nil, got %+v", result.Values)
	}
	if result.Values["enabled"] != true {
		t.Fatalf("expected checkbox on to become true, got %+v", result.Values)
	}
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodeType {
		t.Fatalf("expected integer overflow type issue, got %+v", result.Issues)
	}
}

func TestEmptyPreserveKeepsNonStringScalarEmptyValues(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
		{Name: "optional_count", Type: model.FieldTypeInteger},
		{Name: "enabled", Type: model.FieldTypeBoolean},
	}}

	result := submission.ParseValues(form, url.Values{
		"optional_count": {""},
		"enabled":        {""},
	}, submission.WithEmptyStrings(submission.EmptyPreserve))

	if len(result.Issues) != 0 {
		t.Fatalf("empty preserve should not emit parse issues, got %+v", result.Issues)
	}
	if result.Values["optional_count"] != "" || result.Values["enabled"] != "" {
		t.Fatalf("expected empty strings to be preserved, got %+v", result.Values)
	}
}

func TestEncodedEnumValuesCoerceToTypedScalars(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
		{Name: "level", Type: model.FieldTypeInteger, Enum: []any{float64(1), float64(2)}},
		{Name: "enabled", Type: model.FieldTypeBoolean, Enum: []any{true, false}},
		{Name: "flags", Type: model.FieldTypeArray, Enum: []any{true, false}},
	}}

	result := submission.ParseValues(form, url.Values{
		"level":   {submission.EncodeEnumValue(int64(2))},
		"enabled": {submission.EncodeEnumValue(true)},
		"flags[]": {submission.EncodeEnumValue(true), submission.EncodeEnumValue(false)},
	})
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected parse issues: %+v", result.Issues)
	}
	if result.Values["level"] != int64(2) {
		t.Fatalf("expected typed integer enum, got %+v", result.Values["level"])
	}
	if result.Values["enabled"] != true {
		t.Fatalf("expected typed boolean enum, got %+v", result.Values["enabled"])
	}
	if diff := cmp.Diff([]any{true, false}, result.Values["flags"]); diff != "" {
		t.Fatalf("typed array enum mismatch (-want +got):\n%s", diff)
	}

	if issues := submission.Validate(form, result.Values); len(issues) != 0 {
		t.Fatalf("unexpected enum validation issues: %+v", issues)
	}
}

func TestValidateEnumUsesExactNumericComparison(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
		{Name: "large", Type: model.FieldTypeInteger, Enum: []any{int64(9007199254740992)}},
	}}

	issues := submission.Validate(form, submission.Values{"large": int64(9007199254740993)})
	if len(issues) != 1 || issues[0].Code != submission.CodeEnum {
		t.Fatalf("expected enum issue for distinct large integers, got %+v", issues)
	}

	issues = submission.Validate(form, submission.Values{"large": int64(9007199254740992)})
	if len(issues) != 0 {
		t.Fatalf("expected exact integer enum match, got %+v", issues)
	}
}

func TestValidateProducesDeterministicIssueCodes(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
		{Name: "title", Type: model.FieldTypeString, Required: true},
		{
			Name: "count",
			Type: model.FieldTypeInteger,
			Validations: []model.ValidationRule{
				{Kind: model.ValidationRuleMin, Params: map[string]string{"value": "2"}},
				{Kind: model.ValidationRuleMax, Params: map[string]string{"value": "5"}},
			},
		},
		{
			Name: "slug",
			Type: model.FieldTypeString,
			Validations: []model.ValidationRule{
				{Kind: model.ValidationRuleMinLength, Params: map[string]string{"value": "3"}},
				{Kind: model.ValidationRulePattern, Params: map[string]string{"pattern": "^[a-z]+$"}},
			},
		},
		{Name: "status", Type: model.FieldTypeString, Enum: []any{"draft", "published"}},
		{
			Name: "items",
			Type: model.FieldTypeArray,
			Items: &model.Field{
				Name: "item",
				Type: model.FieldTypeObject,
				Nested: []model.Field{
					{Name: "name", Type: model.FieldTypeString, Required: true},
				},
			},
		},
	}}
	values := submission.Values{
		"count":  int64(1),
		"slug":   "A",
		"status": "archived",
		"items":  []any{map[string]any{}},
	}

	issues := submission.Validate(form, values)
	got := issueCodes(issues)
	want := []submission.IssueCode{
		submission.CodeRequired,
		submission.CodeMin,
		submission.CodeMinLength,
		submission.CodePattern,
		submission.CodeEnum,
		submission.CodeRequired,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("issue codes mismatch (-want +got):\n%s", diff)
	}
	if issues[len(issues)-1].Path != "items[0].name" {
		t.Fatalf("expected nested array path, got %+v", issues[len(issues)-1])
	}
}

func TestValidateProducesArrayCardinalityIssueCodes(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
		{
			Name: "columns",
			Type: model.FieldTypeArray,
			Validations: []model.ValidationRule{
				{Kind: model.ValidationRuleMinItems, Params: map[string]string{"value": "2"}},
				{Kind: model.ValidationRuleMaxItems, Params: map[string]string{"value": "3"}},
			},
			Items: &model.Field{Name: "column", Type: model.FieldTypeString},
		},
		{
			Name: "matrix",
			Type: model.FieldTypeArray,
			Items: &model.Field{
				Name: "row",
				Type: model.FieldTypeArray,
				Validations: []model.ValidationRule{
					{Kind: model.ValidationRuleMinItems, Params: map[string]string{"value": "2"}},
				},
				Items: &model.Field{Name: "cell", Type: model.FieldTypeString},
			},
		},
	}}

	issues := submission.Validate(form, submission.Values{
		"columns": []any{"one"},
		"matrix":  []any{[]any{"one"}},
	})
	got := issueCodes(issues)
	want := []submission.IssueCode{submission.CodeMinItems, submission.CodeMinItems}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("issue codes mismatch (-want +got):\n%s", diff)
	}
	if issues[1].Path != "matrix[0]" {
		t.Fatalf("expected nested array path matrix[0], got %+v", issues[1])
	}

	issues = submission.Validate(form, submission.Values{
		"columns": []any{"one", "two", "three", "four"},
		"matrix":  []any{[]any{"one", "two"}},
	})
	if len(issues) != 1 || issues[0].Code != submission.CodeMaxItems {
		t.Fatalf("expected one maxItems issue, got %+v", issues)
	}
}

func TestRawObjectDetectionAndParsing(t *testing.T) {
	raw := model.Field{Name: "settings", Type: model.FieldTypeObject}
	if !submission.IsRawObjectField(raw) {
		t.Fatalf("schemaless object should be raw")
	}
	nested := model.Field{
		Name: "owner",
		Type: model.FieldTypeObject,
		Nested: []model.Field{
			{Name: "email", Type: model.FieldTypeString},
		},
	}
	if submission.IsRawObjectField(nested) {
		t.Fatalf("nested object controls should not be raw")
	}
	widget := model.Field{
		Name: "config",
		Type: model.FieldTypeObject,
		Metadata: map[string]string{
			"widget": widgets.WidgetJSONEditor,
		},
	}
	if !submission.IsRawObjectField(widget) {
		t.Fatalf("json editor widget should be raw")
	}

	form := model.FormModel{Fields: []model.Field{raw}}
	result := submission.ParseValues(form, url.Values{"settings": {`{"theme":"dark"}`}})
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected raw object issues: %+v", result.Issues)
	}
	if diff := cmp.Diff(map[string]any{"theme": "dark"}, result.Values["settings"]); diff != "" {
		t.Fatalf("raw object mismatch (-want +got):\n%s", diff)
	}

	result = submission.ParseValues(form, url.Values{"settings": {`{"theme"`}})
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodeInvalidJSON {
		t.Fatalf("expected invalidJSON issue, got %+v", result.Issues)
	}

	result = submission.ParseValues(form, url.Values{"settings": {`{"theme":"dark"} trailing`}})
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodeInvalidJSON {
		t.Fatalf("expected invalidJSON issue for trailing content, got %+v", result.Issues)
	}

	result = submission.ParseValues(form, url.Values{"settings": {`["theme"]`}})
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodeObject {
		t.Fatalf("expected object issue for valid non-object JSON, got %+v", result.Issues)
	}
}

func issueCodes(issues []submission.Issue) []submission.IssueCode {
	out := make([]submission.IssueCode, len(issues))
	for i, issue := range issues {
		out[i] = issue.Code
	}
	return out
}
