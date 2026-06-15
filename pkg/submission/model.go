package submission

import (
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/widgets"
)

type fieldIndex struct {
	roots map[string]model.Field
}

func newFieldIndex(form model.FormModel) fieldIndex {
	idx := fieldIndex{roots: make(map[string]model.Field, len(form.Fields))}
	for _, field := range form.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		idx.roots[name] = field
	}
	return idx
}

func (idx fieldIndex) fieldFor(segments []pathSegment) (model.Field, bool) {
	var current model.Field
	found := false
	for _, segment := range segments {
		switch {
		case segment.Name != "":
			if !found {
				field, ok := idx.roots[segment.Name]
				if !ok {
					return model.Field{}, false
				}
				current = field
				found = true
				continue
			}
			if current.Type == model.FieldTypeArray && current.Items != nil {
				current = *current.Items
			}
			if current.Type != model.FieldTypeObject || IsRawObjectField(current) {
				return model.Field{}, false
			}
			child, ok := nestedField(current.Nested, segment.Name)
			if !ok {
				return model.Field{}, false
			}
			current = child
		case segment.Index != nil || segment.Append:
			if !found || current.Type != model.FieldTypeArray {
				return model.Field{}, false
			}
			if current.Items != nil {
				current = *current.Items
			} else {
				current = model.Field{Name: current.Name, Type: inferEnumType(current.Enum), Enum: current.Enum}
			}
		}
	}
	if !found {
		return model.Field{}, false
	}
	return current, true
}

func inferEnumType(values []any) model.FieldType {
	for _, value := range values {
		switch value.(type) {
		case bool:
			return model.FieldTypeBoolean
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32:
			return model.FieldTypeInteger
		case float32, float64:
			return model.FieldTypeNumber
		}
	}
	return model.FieldTypeString
}

func nestedField(fields []model.Field, name string) (model.Field, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return model.Field{}, false
}

func topLevelField(idx fieldIndex, name string) (model.Field, bool) {
	field, ok := idx.roots[name]
	return field, ok
}

// IsRawObjectField reports whether an object field should submit as one JSON
// textarea value rather than nested child controls.
func IsRawObjectField(field model.Field) bool {
	if field.Type != model.FieldTypeObject || field.Relationship != nil {
		return false
	}
	if explicitRawObject(field.Metadata) || explicitRawObject(field.UIHints) {
		return true
	}
	if componentHint(field) == "json_editor" || widgetHint(field) == widgets.WidgetJSONEditor {
		return true
	}
	return len(field.Nested) == 0
}

func explicitRawObject(values map[string]string) bool {
	if len(values) == 0 {
		return false
	}
	for _, key := range []string{"submission.rawObject", "rawObject", "json.rawObject"} {
		if strings.EqualFold(strings.TrimSpace(values[key]), "true") {
			return true
		}
	}
	return false
}

func componentHint(field model.Field) string {
	if field.Metadata != nil {
		if name := strings.TrimSpace(field.Metadata["component.name"]); name != "" {
			return strings.ToLower(name)
		}
	}
	if field.UIHints != nil {
		if name := strings.TrimSpace(field.UIHints["component"]); name != "" {
			return strings.ToLower(name)
		}
	}
	return ""
}

func widgetHint(field model.Field) string {
	if field.Metadata != nil {
		if widget := strings.TrimSpace(field.Metadata["admin.widget"]); widget != "" {
			return strings.ToLower(widget)
		}
		if widget := strings.TrimSpace(field.Metadata["widget"]); widget != "" {
			return strings.ToLower(widget)
		}
	}
	if field.UIHints != nil {
		if widget := strings.TrimSpace(field.UIHints["widget"]); widget != "" {
			return strings.ToLower(widget)
		}
	}
	return ""
}
