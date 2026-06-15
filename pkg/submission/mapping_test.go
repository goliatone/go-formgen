package submission_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/submission"
)

func TestIssuesMapToRendererCompatiblePaths(t *testing.T) {
	form := model.FormModel{Fields: []model.Field{
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
	}}
	issues := []submission.Issue{
		{Code: submission.CodeRequired, Path: "items[0].name", Message: "Name is required"},
	}

	fields, formErrors := submission.IssuesToFieldAndFormErrors(form, issues)
	want := map[string][]string{"items.name": {"Name is required"}}
	if diff := cmp.Diff(want, fields); diff != "" {
		t.Fatalf("field errors mismatch (-want +got):\n%s", diff)
	}
	if len(formErrors) != 0 {
		t.Fatalf("expected no form errors, got %v", formErrors)
	}
}

func TestRendererPathDropsIndexes(t *testing.T) {
	if got := submission.RendererPath("items[12].owner.email"); got != "items.owner.email" {
		t.Fatalf("RendererPath() = %q", got)
	}
}
