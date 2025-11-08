package uischema_test

import (
	"encoding/json"
	"testing"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/uischema"
)

func TestDecorator_Decorate(t *testing.T) {
	store := loadStore(t, "basic")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "createArticle",
		Fields: []pkgmodel.Field{
			{Name: "session_name", Label: "Session Name"},
			{Name: "event_id", Label: "Event"},
			{Name: "session_time", Label: "Session Time"},
			{Name: "notes", Label: "Notes"},
		},
	}

	if err := decorator.Decorate(&form); err != nil {
		t.Fatalf("decorate: %v", err)
	}

	if got := form.UIHints["layout.title"]; got != "Create New Session" {
		t.Fatalf("expected layout.title hint, got %q", got)
	}
	if got := form.UIHints["layout.gridColumns"]; got != "12" {
		t.Fatalf("expected gridColumns 12, got %q", got)
	}

	actionsJSON, ok := form.Metadata["actions"]
	if !ok {
		t.Fatalf("metadata actions missing: %#v", form.Metadata)
	}
	var actions []uischema.ActionConfig
	if err := json.Unmarshal([]byte(actionsJSON), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v", err)
	}
	if len(actions) != 2 || actions[1].Kind != "primary" {
		t.Fatalf("unexpected actions payload: %#v", actions)
	}

	sectionsJSON := form.Metadata["layout.sections"]
	if sectionsJSON == "" {
		t.Fatalf("layout.sections metadata missing")
	}

	sessionName := mustField(t, form.Fields, "session_name")
	if sessionName.UIHints["helpText"] != "Visible to attendees" {
		t.Fatalf("session_name help text mismatch: %#v", sessionName.UIHints)
	}
	if sessionName.Metadata["layout.section"] != "basic-info" {
		t.Fatalf("session_name section metadata missing: %#v", sessionName.Metadata)
	}

	eventField := mustField(t, form.Fields, "event_id")
	if eventField.UIHints["layout.span"] != "6" {
		t.Fatalf("event_id span mismatch: %#v", eventField.UIHints)
	}
	if eventField.Metadata["component.config"] == "" {
		t.Fatalf("event_id component config missing")
	}

	timeField := mustField(t, form.Fields, "session_time")
	if timeField.UIHints["layout.start"] != "7" {
		t.Fatalf("session_time start mismatch: %#v", timeField.UIHints)
	}
	if timeField.Metadata["timezone"] != "UTC" {
		t.Fatalf("session_time metadata merge failed: %#v", timeField.Metadata)
	}

	notesField := mustField(t, form.Fields, "notes")
	if notesField.UIHints["cssClass"] != "fg-field--notes" {
		t.Fatalf("notes css class mismatch: %#v", notesField.UIHints)
	}

	wantOrder := []string{"session_name", "event_id", "session_time", "notes"}
	for idx, name := range wantOrder {
		if form.Fields[idx].Name != name {
			t.Fatalf("field order mismatch at %d: want %s got %s", idx, name, form.Fields[idx].Name)
		}
	}
}

func TestDecorator_FieldOrderPresets(t *testing.T) {
	store := loadStore(t, "ordering")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "orderedExample",
		Fields: []pkgmodel.Field{
			{Name: "id"},
			{Name: "name"},
			{Name: "description"},
			{Name: "created_at"},
			{Name: "updated_at"},
			{
				Name: "address",
				Type: pkgmodel.FieldTypeObject,
				Nested: []pkgmodel.Field{
					{Name: "street"},
					{Name: "city"},
					{Name: "postcode"},
				},
			},
			{Name: "notes"},
		},
	}

	if err := decorator.Decorate(&form); err != nil {
		t.Fatalf("decorate: %v", err)
	}

	assertOrder := func(section string, want []string) {
		t.Helper()
		raw := form.Metadata["layout.fieldOrder."+section]
		if raw == "" {
			t.Fatalf("field order metadata missing for section %s", section)
		}
		var got []string
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("unmarshal %s order: %v", section, err)
		}
		if len(got) != len(want) {
			t.Fatalf("section %s order length mismatch: want %v got %v", section, want, got)
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("section %s order mismatch at %d: want %s got %s", section, idx, want[idx], got[idx])
			}
		}
	}

	assertOrder("primary", []string{"id", "name", "description", "created_at", "updated_at"})
	assertOrder("extras", []string{"notes", "address.street", "address.postcode", "address.city"})

	address := mustField(t, form.Fields, "address")
	if len(address.Nested) != 3 {
		t.Fatalf("address nested count mismatch: %#v", address.Nested)
	}
	if address.Nested[0].Name != "street" || address.Nested[1].Name != "postcode" || address.Nested[2].Name != "city" {
		t.Fatalf("address nested order mismatch: %#v", address.Nested)
	}
}

func TestDecorator_UnknownField(t *testing.T) {
	store := loadStore(t, "invalid_unknown")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "createArticle",
		Fields: []pkgmodel.Field{
			{Name: "session_name"},
		},
	}

	if err := decorator.Decorate(&form); err == nil {
		t.Fatalf("expected unknown field error")
	}
}

func TestDecorator_InvalidGridSpan(t *testing.T) {
	store := loadStore(t, "invalid_span")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "createArticle",
		Fields: []pkgmodel.Field{
			{Name: "session_name"},
		},
	}

	if err := decorator.Decorate(&form); err == nil {
		t.Fatalf("expected grid span validation error")
	}
}

func mustField(t *testing.T, fields []pkgmodel.Field, name string) pkgmodel.Field {
	t.Helper()
	for _, field := range fields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("field %s not found", name)
	return pkgmodel.Field{}
}
