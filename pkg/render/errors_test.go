package render_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/render"
)

func TestMapErrorPayload_GoErrorsCompatibility(t *testing.T) {
	form := model.FormModel{
		Fields: []model.Field{
			{Name: "name", Type: model.FieldTypeString},
			{
				Name: "owner",
				Type: model.FieldTypeObject,
				Nested: []model.Field{
					{Name: "email", Type: model.FieldTypeString},
					{Name: "phone", Type: model.FieldTypeString},
				},
			},
			{Name: "tags", Type: model.FieldTypeArray},
		},
	}

	payload := map[string][]string{
		"/body/name":                 {"Name is required"},
		"body.owner.email":           {"Email invalid"},
		"$.body.tags[0]":             {"Tags must be unique"},
		"request.payload.owner":      {"Owner missing"},
		"non_field_errors":           {"Form level error"},
		"body/owner/phone/~1number":  {"Phone malformed"},
		"request/body/unknown-field": {"Should fall back to form errors"},
		"":                           {"Unscoped form error"},
	}

	mapped := render.MapErrorPayload(form, payload)

	wantFields := map[string][]string{
		"name":        {"Name is required"},
		"owner.email": {"Email invalid"},
		"tags":        {"Tags must be unique"},
		"owner":       {"Owner missing"},
		"owner.phone": {"Phone malformed"},
	}
	if diff := cmp.Diff(wantFields, mapped.Fields); diff != "" {
		t.Fatalf("field errors mismatch (-want +got):\n%s", diff)
	}

	wantForm := []string{"Form level error", "Should fall back to form errors", "Unscoped form error"}
	if diff := cmp.Diff(wantForm, mapped.Form, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Fatalf("form errors mismatch (-want +got):\n%s", diff)
	}
}

func TestMergeFormErrors(t *testing.T) {
	merged := render.MergeFormErrors([]string{" First ", "Second"}, "Second", "third", "  ")
	want := []string{"First", "Second", "third"}

	if diff := cmp.Diff(want, merged); diff != "" {
		t.Fatalf("merged form errors mismatch (-want +got):\n%s", diff)
	}
}
