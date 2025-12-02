package orchestrator

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestRegisterWidget_AllowsRuntimeAdapters(t *testing.T) {
	t.Helper()

	orch := New()

	orch.RegisterWidget("custom-toggle", 200, func(field model.Field) bool {
		return field.Type == model.FieldTypeBoolean
	})

	reg := orch.WidgetRegistry()
	if reg == nil {
		t.Fatalf("widget registry should be initialised")
	}

	form := model.FormModel{
		Fields: []model.Field{
			{Name: "enabled", Type: model.FieldTypeBoolean},
		},
	}

	if err := reg.Decorate(&form); err != nil {
		t.Fatalf("decorate: %v", err)
	}

	field := form.Fields[0]
	if field.UIHints["widget"] != "custom-toggle" {
		t.Fatalf("expected injected widget to win, got %q", field.UIHints["widget"])
	}
	if field.Metadata["widget"] != "custom-toggle" {
		t.Fatalf("expected metadata to reflect injected widget, got %q", field.Metadata["widget"])
	}
}
