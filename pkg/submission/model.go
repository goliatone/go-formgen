package submission

import (
	"maps"
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
		next, nextFound, ok := idx.nextField(current, found, segment)
		if !ok {
			return model.Field{}, false
		}
		current = next
		found = nextFound
	}
	if !found {
		return model.Field{}, false
	}
	return current, true
}

func (idx fieldIndex) nextField(current model.Field, found bool, segment pathSegment) (model.Field, bool, bool) {
	if segment.Name != "" {
		field, ok := idx.nextNamedField(current, found, segment.Name)
		return field, true, ok
	}
	if segment.Index != nil || segment.Append {
		field, ok := nextArrayItemField(current, found)
		return field, found, ok
	}
	return current, found, true
}

func (idx fieldIndex) nextNamedField(current model.Field, found bool, name string) (model.Field, bool) {
	if !found {
		field, ok := idx.roots[name]
		return field, ok
	}
	if current.Type == model.FieldTypeArray && current.Items != nil {
		current = *current.Items
	}
	if current.Type != model.FieldTypeObject || IsRawObjectField(current) {
		return model.Field{}, false
	}
	return nestedField(current.Nested, name)
}

func nextArrayItemField(current model.Field, found bool) (model.Field, bool) {
	if !found || current.Type != model.FieldTypeArray {
		return model.Field{}, false
	}
	item := model.Field{Name: current.Name, Type: inferEnumType(current.Enum), Enum: current.Enum}
	if current.Items != nil {
		item = *current.Items
	}
	// A select representing the whole array owns item transport even when
	// Items has no enum. Copy metadata; parsing must not mutate the model.
	mode := nativeEnumMode(current)
	if mode != EnumNative {
		item.Metadata = maps.Clone(item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]string{}
		}
		if mode == EnumControl {
			item.Metadata["submission.enumValues"] = "control"
		} else {
			item.Metadata["submission.enumValues"] = "literal"
		}
	}
	return item, true
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

// nativeEnumMode describes the declared control, not the submitted token's
// spelling. Custom renderers must declare transport when it differs from the
// model's standard choice/text semantics. No renderer dependency is needed.
func nativeEnumMode(field model.Field) EnumValueMode {
	switch strings.TrimSpace(field.Metadata["submission.enumValues"]) {
	case "control":
		return EnumControl
	case "literal":
		return EnumLiteral
	}
	component := strings.TrimSpace(field.UIHints["component"])
	if component == "" {
		component = strings.TrimSpace(field.Metadata["component.name"])
	}
	if component != "" {
		switch component {
		case "select", "choice", "choice-multiple", "scale", "boolean":
			return EnumControl
		case "array", "object":
			return EnumNative
		default:
			return EnumLiteral
		}
	}
	switch widgetHint(field) {
	case "select", "chips", "typeahead", "toggle":
		return EnumControl
	case "textarea", "json-editor", "code-editor", "wysiwyg", "rich-text", "rich_text":
		return EnumLiteral
	}
	if hasNativeChoiceValues(field) {
		return EnumControl
	}
	if field.Type == model.FieldTypeObject || field.Type == model.FieldTypeArray {
		return EnumNative
	}
	return EnumLiteral
}

func hasNativeChoiceValues(field model.Field) bool {
	return len(field.Enum) > 0 || len(field.Options) > 0 || field.Relationship != nil ||
		field.Metadata["relationship.endpoint.url"] != "" || field.UIHints["input"] == "select" ||
		field.UIHints["collectionRenderer"] == "select" || field.UIHints["collectionRenderer"] == "chips" ||
		field.Type == model.FieldTypeBoolean
}
