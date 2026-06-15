package submission_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
)

func TestParseJSONKeepsTypedValues(t *testing.T) {
	form := testForm()

	result, err := submission.ParseJSON(form, []byte(`{
		"title":"Hello",
		"count":2,
		"published":true,
		"owner":{"email":"a@example.com"},
		"settings":{"theme":"dark"}
	}`))
	if err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %+v", result.Issues)
	}

	want := submission.Values{
		"title":     "Hello",
		"count":     int64(2),
		"published": true,
		"owner":     map[string]any{"email": "a@example.com"},
		"settings":  map[string]any{"theme": "dark"},
	}
	if diff := cmp.Diff(want, result.Values); diff != "" {
		t.Fatalf("values mismatch (-want +got):\n%s", diff)
	}
}

func TestParseJSONRejectsTrailingContent(t *testing.T) {
	form := testForm()

	result, err := submission.ParseJSON(form, []byte(`{"title":"Hello"} trailing`))
	if err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodeInvalidJSON {
		t.Fatalf("expected invalidJSON issue, got %+v", result.Issues)
	}
	if len(result.Values) != 0 {
		t.Fatalf("invalid JSON should not preserve values, got %+v", result.Values)
	}
}

func TestParseValuesNormalizesArraysNestedObjectsAndHiddenFields(t *testing.T) {
	form := testForm()
	values := url.Values{
		"title":         {"Hello"},
		"count":         {"42"},
		"published":     {"on"},
		"tags":          {"alpha", "beta"},
		"items[0].name": {"First"},
		"items.1.name":  {"Second"},
		"_csrf":         {"token"},
	}

	result := submission.ParseValues(form, values, submission.WithUnknownFields(submission.UnknownPreserve))
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %+v", result.Issues)
	}

	want := submission.Values{
		"title":     "Hello",
		"count":     int64(42),
		"published": true,
		"tags":      []any{"alpha", "beta"},
		"items": []any{
			map[string]any{"name": "First"},
			map[string]any{"name": "Second"},
		},
		"_csrf": "token",
	}
	if diff := cmp.Diff(want, result.Values); diff != "" {
		t.Fatalf("values mismatch (-want +got):\n%s", diff)
	}
}

func TestParseValuesBracketArraySyntax(t *testing.T) {
	form := testForm()

	result := submission.ParseValues(form, url.Values{
		"tags[]": {"alpha", "beta"},
	})
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %+v", result.Issues)
	}

	want := []any{"alpha", "beta"}
	if diff := cmp.Diff(want, result.Values["tags"]); diff != "" {
		t.Fatalf("tags mismatch (-want +got):\n%s", diff)
	}
}

func TestParseValuesRepeatedScalarReportsConflict(t *testing.T) {
	form := testForm()

	result := submission.ParseValues(form, url.Values{
		"title": {"First", "Second"},
	})

	if result.Values["title"] != "First" {
		t.Fatalf("expected first usable scalar value to be retained, got %+v", result.Values)
	}
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodePathConflict {
		t.Fatalf("expected path conflict for repeated scalar, got %+v", result.Issues)
	}
}

func TestParseValuesUnknownFieldPolicies(t *testing.T) {
	form := testForm()

	result := submission.ParseValues(form, url.Values{"extra": {"value"}})
	if len(result.Issues) != 1 || result.Issues[0].Code != submission.CodeUnknownField {
		t.Fatalf("expected unknown field issue, got %+v", result.Issues)
	}
	if _, ok := result.Values["extra"]; ok {
		t.Fatalf("default policy should not preserve unknown values: %+v", result.Values)
	}

	result = submission.ParseValues(form, url.Values{"extra": {"value"}}, submission.WithUnknownFields(submission.UnknownPreserve))
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected preserve issues: %+v", result.Issues)
	}
	if result.Values["extra"] != "value" {
		t.Fatalf("expected preserved unknown value, got %+v", result.Values)
	}

	result = submission.ParseValues(form, url.Values{"extra": {"value"}}, submission.WithUnknownFields(submission.UnknownIgnore))
	if len(result.Issues) != 0 || len(result.Values) != 0 {
		t.Fatalf("expected ignored unknown value, got values=%+v issues=%+v", result.Values, result.Issues)
	}
}

func TestParseValuesConflictReportsIssueWithoutDroppingUsableValues(t *testing.T) {
	form := testForm()

	result := submission.ParseValues(form, url.Values{
		"owner":       {"scalar"},
		"owner.email": {"a@example.com"},
	})
	if len(result.Issues) == 0 || result.Issues[0].Code != submission.CodeObject {
		t.Fatalf("expected conflict/coercion issue, got %+v", result.Issues)
	}
	if _, ok := result.Values["owner"]; !ok {
		t.Fatalf("expected usable owner value to be retained")
	}
}

func TestParseRequestSupportsFormURLEncodedAndMultipart(t *testing.T) {
	form := testForm()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("title=Hello&count=3"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result, err := submission.ParseRequest(form, req)
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if result.Values["count"] != int64(3) {
		t.Fatalf("expected coerced count, got %+v", result.Values)
	}
}

func testForm() model.FormModel {
	return model.FormModel{
		Fields: []model.Field{
			{Name: "title", Type: model.FieldTypeString, Required: true},
			{Name: "count", Type: model.FieldTypeInteger},
			{Name: "published", Type: model.FieldTypeBoolean},
			{
				Name: "tags",
				Type: model.FieldTypeArray,
				Items: &model.Field{
					Name: "tag",
					Type: model.FieldTypeString,
				},
			},
			{
				Name: "items",
				Type: model.FieldTypeArray,
				Items: &model.Field{
					Name: "item",
					Type: model.FieldTypeObject,
					Nested: []model.Field{
						{Name: "name", Type: model.FieldTypeString},
					},
				},
			},
			{
				Name: "owner",
				Type: model.FieldTypeObject,
				Nested: []model.Field{
					{Name: "email", Type: model.FieldTypeString},
				},
			},
			{Name: "settings", Type: model.FieldTypeObject},
		},
	}
}
