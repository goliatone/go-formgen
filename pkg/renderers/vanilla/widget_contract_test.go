package vanilla

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
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
					"widget": components.NameTextarea,
				},
			},
			want: components.NameTextarea,
		},
		{
			name: "code editor widget maps to textarea",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": widgets.WidgetCodeEditor,
				},
			},
			want: components.NameTextarea,
		},
		{
			name: "select widget maps to select",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": widgets.WidgetSelect,
				},
			},
			want: components.NameSelect,
		},
		{
			name: "chips widget maps to select",
			in: model.Field{
				Type: model.FieldTypeArray,
				Metadata: map[string]string{
					"widget": widgets.WidgetChips,
				},
			},
			want: components.NameSelect,
		},
		{
			name: "toggle widget maps to boolean",
			in: model.Field{
				Type: model.FieldTypeBoolean,
				Metadata: map[string]string{
					"widget": widgets.WidgetToggle,
				},
			},
			want: components.NameBoolean,
		},
		{
			name: "json editor widget maps to json_editor",
			in: model.Field{
				Type: model.FieldTypeObject,
				Metadata: map[string]string{
					"widget": widgets.WidgetJSONEditor,
				},
			},
			want: components.NameJSONEditor,
		},
		{
			name: "wysiwyg widget maps to wysiwyg component",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": components.NameWysiwyg,
				},
			},
			want: components.NameWysiwyg,
		},
		{
			name: "file_uploader widget maps to file_uploader component",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": components.NameFileUploader,
				},
			},
			want: components.NameFileUploader,
		},
		{
			name: "datetime-range widget maps to datetime-range component",
			in: model.Field{
				Type: model.FieldTypeString,
				Metadata: map[string]string{
					"widget": components.NameDatetimeRange,
				},
			},
			want: components.NameDatetimeRange,
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
