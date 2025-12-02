package widgets

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
)

func TestResolve_ExplicitWidgetWins(t *testing.T) {
	reg := NewRegistry()
	field := model.Field{
		Type: model.FieldTypeBoolean,
		Metadata: map[string]string{
			"admin.widget": "custom-toggle",
		},
	}

	if got, ok := reg.Resolve(field); !ok || got != "custom-toggle" {
		t.Fatalf("expected explicit widget to win, got %q (ok=%v)", got, ok)
	}
}

func TestResolve_Builtins(t *testing.T) {
	reg := NewRegistry()

	cases := []struct {
		name   string
		field  model.Field
		expect string
	}{
		{
			name: "boolean toggle",
			field: model.Field{
				Type: model.FieldTypeBoolean,
			},
			expect: WidgetToggle,
		},
		{
			name: "array chips enum",
			field: model.Field{
				Type: model.FieldTypeArray,
				Enum: []any{"a", "b"},
			},
			expect: WidgetChips,
		},
		{
			name: "select enum",
			field: model.Field{
				Type: model.FieldTypeString,
				Enum: []any{"a"},
			},
			expect: WidgetSelect,
		},
		{
			name: "code editor json format",
			field: model.Field{
				Type:   model.FieldTypeString,
				Format: "json",
			},
			expect: WidgetCodeEditor,
		},
		{
			name: "json editor bare object",
			field: model.Field{
				Type: model.FieldTypeObject,
			},
			expect: WidgetJSONEditor,
		},
		{
			name: "key value editor array object",
			field: model.Field{
				Type: model.FieldTypeArray,
				Items: &model.Field{
					Type: model.FieldTypeObject,
				},
			},
			expect: WidgetKeyValue,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := reg.Resolve(tc.field)
			if !ok {
				t.Fatalf("expected resolution for %s", tc.name)
			}
			if got != tc.expect {
				t.Fatalf("resolve %s: want %q, got %q", tc.name, tc.expect, got)
			}
		})
	}
}

func TestResolve_PriorityOverride(t *testing.T) {
	reg := NewRegistry()
	reg.Register("custom", 999, func(field model.Field) bool {
		return field.Type == model.FieldTypeBoolean
	})

	got, ok := reg.Resolve(model.Field{Type: model.FieldTypeBoolean})
	if !ok || got != "custom" {
		t.Fatalf("priority matcher should win, got %q (ok=%v)", got, ok)
	}
}

func TestDecorator_AppliesWidgetHints(t *testing.T) {
	reg := NewRegistry()

	form := model.FormModel{
		Fields: []model.Field{
			{
				Name: "enabled",
				Type: model.FieldTypeBoolean,
			},
			{
				Name: "tags",
				Type: model.FieldTypeArray,
				Enum: []any{"a"},
			},
		},
	}

	if err := reg.Decorate(&form); err != nil {
		t.Fatalf("decorate: %v", err)
	}

	byName := func(name string) model.Field {
		for _, f := range form.Fields {
			if f.Name == name {
				return f
			}
		}
		t.Fatalf("field %q not found", name)
		return model.Field{}
	}

	enabled := byName("enabled")
	if enabled.UIHints["widget"] != WidgetToggle || enabled.Metadata["widget"] != WidgetToggle {
		t.Fatalf("enabled widget not applied: ui=%q meta=%q", enabled.UIHints["widget"], enabled.Metadata["widget"])
	}

	tags := byName("tags")
	if tags.UIHints["widget"] != WidgetChips || tags.Metadata["widget"] != WidgetChips {
		t.Fatalf("tags widget not applied: ui=%q meta=%q", tags.UIHints["widget"], tags.Metadata["widget"])
	}
}
