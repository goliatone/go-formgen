package render

import (
	"maps"
	"reflect"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

// RedactSensitiveDefaults removes defaults from sensitive fields unless the
// caller explicitly opted into exposing them.
func RedactSensitiveDefaults(form *model.FormModel, includeSensitive bool) {
	if form == nil || includeSensitive {
		return
	}
	form.Fields = cloneAndRedactSensitiveFields(form.Fields)
}

// RedactSensitiveValues returns a copy of values with entries matching sensitive
// fields removed. It accepts nested maps and dotted/bracketed field names.
func RedactSensitiveValues(form model.FormModel, values map[string]any, includeSensitive bool) map[string]any {
	if len(values) == 0 {
		return nil
	}
	if includeSensitive {
		return cloneValueMap(values)
	}
	byPath := make(map[string]model.Field)
	indexFields(byPath, form.Fields, "")
	return redactValueMap(values, byPath)
}

func cloneAndRedactSensitiveFields(fields []model.Field) []model.Field {
	if len(fields) == 0 {
		return nil
	}
	out := make([]model.Field, len(fields))
	for i := range fields {
		out[i] = fields[i]
		out[i].Nested = cloneAndRedactSensitiveFields(out[i].Nested)
		out[i].OneOf = cloneAndRedactSensitiveFields(out[i].OneOf)
		if out[i].Items != nil {
			item := *out[i].Items
			redacted := cloneAndRedactSensitiveFields([]model.Field{item})
			if len(redacted) == 1 {
				out[i].Items = &redacted[0]
			}
		}
		if value, keep := redactDefaultForField(out[i], out[i].Default); keep {
			out[i].Default = value
		} else {
			out[i].Default = nil
		}
	}
	return out
}

func redactDefaultForField(field model.Field, value any) (any, bool) {
	if value == nil {
		return nil, true
	}
	if field.Sensitive {
		return nil, false
	}
	if len(field.Nested) > 0 {
		return redactObjectValue(field.Nested, value)
	}
	if field.Items != nil {
		return redactArrayValue(*field.Items, value)
	}
	if len(field.OneOf) > 0 {
		return redactObjectValue(field.OneOf, value)
	}
	return cloneValue(value), true
}

func redactObjectValue(fields []model.Field, value any) (any, bool) {
	input, ok := valueToStringMap(value)
	if !ok {
		return cloneValue(value), true
	}
	out := cloneValueMap(input)
	for _, field := range fields {
		if field.Name == "" {
			continue
		}
		raw, exists := out[field.Name]
		if !exists {
			continue
		}
		if redacted, keep := redactDefaultForField(field, raw); keep {
			out[field.Name] = redacted
		} else {
			delete(out, field.Name)
		}
	}
	return out, true
}

func redactArrayValue(item model.Field, value any) (any, bool) {
	if item.Sensitive {
		return nil, false
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return redactDefaultForField(item, value)
	}
	out := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		if redacted, keep := redactDefaultForField(item, rv.Index(i).Interface()); keep {
			out = append(out, redacted)
		}
	}
	return out, true
}

func indexFields(out map[string]model.Field, fields []model.Field, parent string) {
	for _, field := range fields {
		path := joinRedactionPath(parent, field.Name)
		if path != "" {
			out[path] = field
		}
		indexFields(out, field.Nested, path)
		indexFields(out, field.OneOf, path)
		if field.Items != nil {
			indexItemFields(out, *field.Items, path)
		}
	}
}

func indexItemFields(out map[string]model.Field, item model.Field, parent string) {
	if item.Name != "" {
		indexFields(out, []model.Field{item}, parent)
		return
	}
	indexFields(out, item.Nested, parent)
	indexFields(out, item.OneOf, parent)
	if item.Items != nil {
		indexItemFields(out, *item.Items, parent)
	}
}

func redactValueMap(values map[string]any, byPath map[string]model.Field) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		path := canonicalRedactionPath(key)
		if field, ok := byPath[path]; ok {
			if redacted, keep := redactDefaultForField(field, value); keep {
				out[key] = redacted
			}
			continue
		}
		if childFields := childFieldsForPath(byPath, path); len(childFields) > 0 {
			if redacted, keep := redactObjectValue(childFields, value); keep {
				out[key] = redacted
			}
			continue
		}
		out[key] = cloneValue(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func childFieldsForPath(byPath map[string]model.Field, path string) []model.Field {
	if path == "" {
		return nil
	}
	prefix := path + "."
	var out []model.Field
	for candidate, field := range byPath {
		if !strings.HasPrefix(candidate, prefix) {
			continue
		}
		rest := strings.TrimPrefix(candidate, prefix)
		if rest == "" || strings.Contains(rest, ".") {
			continue
		}
		out = append(out, field)
	}
	return out
}

func valueToStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			out[key] = val
		}
		return out, true
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	out := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out, true
}

func cloneValueMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneValueMap(typed)
	case map[string]string:
		out := make(map[string]string, len(typed))
		maps.Copy(out, typed)
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneValue(item)
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func joinRedactionPath(parent, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return parent
	}
	if parent == "" {
		return name
	}
	return parent + "." + name
}

func canonicalRedactionPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "[]")
	value = strings.ReplaceAll(value, "[]", "")
	value = strings.ReplaceAll(value, "[", ".")
	value = strings.ReplaceAll(value, "]", "")
	value = strings.TrimPrefix(value, ".")
	for strings.Contains(value, "..") {
		value = strings.ReplaceAll(value, "..", ".")
	}
	return value
}
