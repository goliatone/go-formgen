package uischema_test

import (
	"encoding/json"
	"testing"

	pkgmodel "github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/uischema"
)

func TestDecorator_Decorate(t *testing.T) {
	store := loadStore(t, "basic")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "createArticle",
		Fields: []pkgmodel.Field{
			{Name: "session_name", Label: "Session Name"},
			{Name: "slug", Label: "Slug"},
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
	if sessionName.UIHints["icon"] != "search" {
		t.Fatalf("session_name icon hint mismatch: %#v", sessionName.UIHints)
	}
	if sessionName.UIHints["iconSource"] != "iconoir" {
		t.Fatalf("session_name icon source mismatch: %#v", sessionName.UIHints)
	}
	if sessionName.UIHints["iconRaw"] == "" {
		t.Fatalf("session_name expected sanitized icon raw markup")
	}
	if sessionName.Metadata["icon"] != "search" || sessionName.Metadata["icon.source"] != "iconoir" {
		t.Fatalf("session_name icon metadata missing: %#v", sessionName.Metadata)
	}
	if sessionName.Metadata["behavior.placeholder"] != "session" {
		t.Fatalf("session_name behavior metadata missing: %#v", sessionName.Metadata)
	}

	slugField := mustField(t, form.Fields, "slug")
	if got := slugField.Metadata["behavior.names"]; got != "autoSlug" {
		t.Fatalf("slug behavior metadata mismatch: %#v", slugField.Metadata)
	}
	if got := slugField.Metadata["behavior.config"]; got != `{"source":"session_name"}` {
		t.Fatalf("slug behavior config mismatch: %s", got)
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
	if got := notesField.Metadata["behavior.names"]; got != "autoResize" {
		t.Fatalf("notes behavior metadata mismatch: %#v", notesField.Metadata)
	}
	if got := notesField.Metadata["behavior.config"]; got != `{"minRows":5}` {
		t.Fatalf("notes behavior config mismatch: %s", got)
	}

	wantOrder := []string{"session_name", "slug", "event_id", "session_time", "notes"}
	for idx, name := range wantOrder {
		if form.Fields[idx].Name != name {
			t.Fatalf("field order mismatch at %d: want %s got %s", idx, name, form.Fields[idx].Name)
		}
	}
}

func TestDecorator_I18nKeys(t *testing.T) {
	store := loadStore(t, "i18n_keys")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "createThing",
		Fields: []pkgmodel.Field{
			{Name: "name", Label: "Name"},
		},
	}

	if err := decorator.Decorate(&form); err != nil {
		t.Fatalf("decorate: %v", err)
	}

	if got := form.UIHints["layout.titleKey"]; got != "forms.createThing.title" {
		t.Fatalf("expected layout.titleKey hint, got %q", got)
	}
	if got := form.UIHints["layout.subtitleKey"]; got != "forms.createThing.subtitle" {
		t.Fatalf("expected layout.subtitleKey hint, got %q", got)
	}

	actionsJSON := form.Metadata["actions"]
	if actionsJSON == "" {
		t.Fatalf("metadata actions missing")
	}
	var actions []map[string]any
	if err := json.Unmarshal([]byte(actionsJSON), &actions); err != nil {
		t.Fatalf("unmarshal actions: %v", err)
	}
	if len(actions) != 1 || actions[0]["labelKey"] != "actions.save" {
		t.Fatalf("unexpected action payload: %#v", actions)
	}

	sectionsJSON := form.Metadata["layout.sections"]
	if sectionsJSON == "" {
		t.Fatalf("layout.sections metadata missing")
	}
	var sections []map[string]any
	if err := json.Unmarshal([]byte(sectionsJSON), &sections); err != nil {
		t.Fatalf("unmarshal sections: %v", err)
	}
	if len(sections) != 1 || sections[0]["titleKey"] != "sections.main.title" || sections[0]["descriptionKey"] != "sections.main.description" {
		t.Fatalf("unexpected sections payload: %#v", sections)
	}

	nameField := mustField(t, form.Fields, "name")
	if got := nameField.UIHints["labelKey"]; got != "fields.thing.name" {
		t.Fatalf("labelKey mismatch: %q", got)
	}
	if got := nameField.UIHints["placeholderKey"]; got != "fields.thing.name.placeholder" {
		t.Fatalf("placeholderKey mismatch: %q", got)
	}
	if got := nameField.UIHints["helpTextKey"]; got != "fields.thing.name.help" {
		t.Fatalf("helpTextKey mismatch: %q", got)
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

func TestDecorator_ResponsiveGridBreakpoints(t *testing.T) {
	store := loadStore(t, "responsive_grid")
	decorator := uischema.NewDecorator(store)

	form := pkgmodel.FormModel{
		OperationID: "responsiveExample",
		Fields: []pkgmodel.Field{
			{Name: "title"},
		},
	}

	if err := decorator.Decorate(&form); err != nil {
		t.Fatalf("decorate: %v", err)
	}

	titleField := mustField(t, form.Fields, "title")
	if got := titleField.UIHints["layout.span"]; got != "12" {
		t.Fatalf("expected base span 12, got %q", got)
	}
	if got := titleField.UIHints["layout.span.lg"]; got != "6" {
		t.Fatalf("expected lg span 6, got %q", got)
	}
	if got := titleField.UIHints["layout.span.xl"]; got != "6" {
		t.Fatalf("expected xl span 6, got %q", got)
	}
	if got := titleField.UIHints["layout.start.xl"]; got != "7" {
		t.Fatalf("expected xl start 7, got %q", got)
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
