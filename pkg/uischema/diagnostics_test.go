package uischema

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestUISchemaMetadataDiagnosticsAndPrecedence(t *testing.T) {
	raw := []byte(`{"operations":{"edit":{"form":{"title":"Valid form title"},"fields":{"answer":{"component":"cards","label":"UI label","x-formgen":{"label":"Inline extension","host.custom":"private"}}}}}}`)
	got := DiagnoseDefinition(raw)
	if len(got) != 1 || got[0].Code != "metadataConflict" {
		t.Fatal(got)
	}
	store, err := LoadFS(fstest.MapFS{"schema.json": {Data: raw}})
	if err != nil {
		t.Fatal(err)
	}
	form := model.FormModel{OperationID: "edit", Fields: []model.Field{{Name: "answer", Type: model.FieldTypeString, Label: "Schema label", Metadata: map[string]string{"component.name": "old"}}}}
	if err := NewDecorator(store).Decorate(&form); err != nil {
		t.Fatal(err)
	}
	if form.Fields[0].Label != "UI label" || form.Fields[0].UIHints["component"] != "cards" {
		t.Fatal("copy precedence changed")
	}
	raw = []byte(`{"operations":{"edit":{"fields":{"answer":{"component.name":"private","title":"private","componentOptions":[],"x-formgen":{"component":"private"},"host":{"value":"private"}}}}}}`)
	got = DiagnoseDefinition(raw)
	if len(got) != 4 {
		t.Fatal(got)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "private") {
		t.Fatal("payload in diagnostics")
	}
	if got := DiagnoseDefinition([]byte(`{"operations":{"edit":{"fields":{"answer":{"component":"cards","label":"UI","componentOptions":{},"x-host":{"component":"anything"}}}}}}`)); len(got) != 0 {
		t.Fatal(got)
	}
}
