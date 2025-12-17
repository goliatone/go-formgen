package vanilla

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/widgets"
)

func TestResolveComponentNameHonorsWidgetHint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   model.Field
		want string
	}{
		{
			name: "textarea widget maps to textarea",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": "textarea",
				},
			},
			want: "textarea",
		},
		{
			name: "code editor widget maps to textarea",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": widgets.WidgetCodeEditor,
				},
			},
			want: "textarea",
		},
		{
			name: "select widget maps to select",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": widgets.WidgetSelect,
				},
			},
			want: "select",
		},
		{
			name: "chips widget maps to select",
			in: model.Field{
				Type: model.FieldTypeArray,
				Metadata: map[string]string{
					"widget": widgets.WidgetChips,
				},
			},
			want: "select",
		},
		{
			name: "toggle widget maps to boolean",
			in: model.Field{
				Type: model.FieldTypeBoolean,
				Metadata: map[string]string{
					"widget": widgets.WidgetToggle,
				},
			},
			want: "boolean",
		},
		{
			name: "json editor widget maps to json_editor",
			in: model.Field{
				Type: model.FieldTypeObject,
				Metadata: map[string]string{
					"widget": widgets.WidgetJSONEditor,
				},
			},
			want: "json_editor",
		},
		{
			name: "wysiwyg widget maps to wysiwyg component",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": "wysiwyg",
				},
			},
			want: "wysiwyg",
		},
		{
			name: "file_uploader widget maps to file_uploader component",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": "file_uploader",
				},
			},
			want: "file_uploader",
		},
		{
			name: "datetime-range widget maps to datetime-range component",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": "datetime-range",
				},
			},
			want: "datetime-range",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveComponentName(tc.in); got != tc.want {
				t.Fatalf("resolveComponentName() = %q, want %q", got, tc.want)
			}
		})
	}
}

