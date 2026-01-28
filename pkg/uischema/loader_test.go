package uischema_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goliatone/go-formgen/pkg/uischema"
)

func TestLoadFS_JSON(t *testing.T) {
	store := loadStore(t, "basic")
	if store.Empty() {
		t.Fatalf("expected store to contain operations")
	}

	op, ok := store.Operation("createArticle")
	if !ok {
		t.Fatalf("operation createArticle not found")
	}

	if got := len(op.Fields); got != 5 {
		t.Fatalf("expected 5 fields, got %d", got)
	}

	slugCfg, ok := op.Fields["slug"]
	if !ok {
		t.Fatalf("slug field missing")
	}
	if slugCfg.Behaviors == nil {
		t.Fatalf("slug behaviors not parsed: %#v", slugCfg)
	}
	if _, ok := slugCfg.Behaviors["autoSlug"]; !ok {
		t.Fatalf("slug behaviors missing autoSlug entry: %#v", slugCfg.Behaviors)
	}

	eventCfg, ok := op.Fields["event_id"]
	if !ok {
		t.Fatalf("event_id field missing")
	}
	if eventCfg.Component != "event-select" {
		t.Fatalf("component mismatch: %s", eventCfg.Component)
	}
	if eventCfg.ComponentOptions["placeholder"] != "Search events" {
		t.Fatalf("component options not parsed: %#v", eventCfg.ComponentOptions)
	}
	if eventCfg.OriginalPath != "event_id" {
		t.Fatalf("original path mismatch: %s", eventCfg.OriginalPath)
	}

	if op.Form.Layout.GridColumns != 12 {
		t.Fatalf("grid columns mismatch: %d", op.Form.Layout.GridColumns)
	}
	if op.Form.Metadata["hero"] != "session" {
		t.Fatalf("form metadata merge failed: %#v", op.Form.Metadata)
	}
	if op.Form.UIHints["theme"] != "garden" {
		t.Fatalf("form ui hints merge failed: %#v", op.Form.UIHints)
	}
}

func TestLoadFS_YAML(t *testing.T) {
	store := loadStore(t, "nested")
	op, ok := store.Operation("updateManager")
	if !ok {
		t.Fatalf("operation updateManager not found")
	}

	if _, ok := op.Fields["tags.items.id"]; !ok {
		t.Fatalf("expected tags.items.id path after normalisation: %#v", op.Fields)
	}
	if op.Form.Layout.GridColumns != 6 {
		t.Fatalf("grid columns mismatch: %d", op.Form.Layout.GridColumns)
	}
}

func TestLoadFS_DuplicateFieldPath(t *testing.T) {
	_, err := uischema.LoadFS(subDirFS(t, "invalid_duplicate"))
	if err == nil {
		t.Fatalf("expected duplicate path error")
	}
}

func TestLoadFS_FieldOrderPresets(t *testing.T) {
	store := loadStore(t, "ordering")
	op, ok := store.Operation("orderedExample")
	if !ok {
		t.Fatalf("operation orderedExample not found")
	}

	if got := len(op.FieldOrderPresets); got != 1 {
		t.Fatalf("expected 1 fieldOrder preset, got %d", got)
	}

	preset := op.FieldOrderPresets["audited"]
	if len(preset) != 6 || preset[3] != "*" {
		t.Fatalf("unexpected preset contents: %#v", preset)
	}

	var primary, extras uischema.SectionConfig
	for _, section := range op.Sections {
		switch section.ID {
		case "primary":
			primary = section
		case "extras":
			extras = section
		}
	}
	if !primary.OrderPreset.Defined() || primary.OrderPreset.Reference() != "audited" {
		t.Fatalf("primary section should reference audited preset: %#v", primary.OrderPreset)
	}
	inline := extras.OrderPreset.Inline()
	if len(inline) != 3 || inline[0] != "address.street" || inline[1] != "*" {
		t.Fatalf("extras section should carry inline preset, got %#v", inline)
	}
}

func TestLoadFS_I18nKeys(t *testing.T) {
	store := loadStore(t, "i18n_keys")
	op, ok := store.Operation("createThing")
	if !ok {
		t.Fatalf("operation createThing not found")
	}

	if op.Form.TitleKey != "forms.createThing.title" {
		t.Fatalf("expected form titleKey, got %q", op.Form.TitleKey)
	}
	if op.Form.SubtitleKey != "forms.createThing.subtitle" {
		t.Fatalf("expected form subtitleKey, got %q", op.Form.SubtitleKey)
	}
	if len(op.Form.Actions) != 1 || op.Form.Actions[0].LabelKey != "actions.save" {
		t.Fatalf("expected action labelKey, got %#v", op.Form.Actions)
	}

	if len(op.Sections) != 1 || op.Sections[0].TitleKey != "sections.main.title" || op.Sections[0].DescriptionKey != "sections.main.description" {
		t.Fatalf("expected section i18n keys, got %#v", op.Sections)
	}

	nameCfg, ok := op.Fields["name"]
	if !ok {
		t.Fatalf("name field missing")
	}
	if nameCfg.LabelKey != "fields.thing.name" || nameCfg.PlaceholderKey != "fields.thing.name.placeholder" || nameCfg.HelpTextKey != "fields.thing.name.help" {
		t.Fatalf("expected field i18n keys, got %#v", nameCfg)
	}
}

func TestNormalizeFieldPath(t *testing.T) {
	cases := map[string]string{
		"tags[].id":           "tags.items.id",
		"manager.address":     "manager.address",
		" settings.name ":     "settings.name",
		"tags[0].label":       "tags.items.label",
		"nested[].items[].id": "nested.items.items.id",
	}

	for input, want := range cases {
		if got := uischema.NormalizeFieldPath(input); got != want {
			t.Fatalf("normalize %q: want %q got %q", input, want, got)
		}
	}
}

func loadStore(t *testing.T, subdir string) *uischema.Store {
	t.Helper()
	store, err := uischema.LoadFS(subDirFS(t, subdir))
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	return store
}

func subDirFS(t *testing.T, subdir string) fs.FS {
	t.Helper()
	base := os.DirFS(testdataRoot())
	fsys, err := fs.Sub(base, subdir)
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}
	return fsys
}

func testdataRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "testdata"
	}
	return filepath.Join(filepath.Dir(filename), "testdata")
}
