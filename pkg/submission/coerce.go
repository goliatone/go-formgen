package submission

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

// CoerceValue normalizes one submitted value according to the supplied field.
func CoerceValue(field model.Field, value any, opts Options, path string) (any, []Issue) {
	if str, ok := value.(string); ok {
		if decoded, encoded := DecodeEnumValue(str); encoded {
			value = decoded
		}
	}

	if str, ok := value.(string); ok && str == "" {
		if opts.EmptyStrings == EmptyPreserve {
			return str, nil
		}
		if field.Type != model.FieldTypeString {
			return nil, nil
		}
		return "", nil
	}

	switch field.Type {
	case model.FieldTypeString:
		return coerceString(value), nil
	case model.FieldTypeInteger:
		return coerceInteger(value, path)
	case model.FieldTypeNumber:
		return coerceNumber(value, path)
	case model.FieldTypeBoolean:
		return coerceBoolean(value, path)
	case model.FieldTypeArray:
		return coerceArray(field, value, opts, path)
	case model.FieldTypeObject:
		return coerceObject(field, value, opts, path)
	default:
		return value, nil
	}
}

func coerceString(value any) any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func coerceInteger(value any, path string) (any, []Issue) {
	if value == nil {
		return nil, nil
	}
	if i, ok := signedInteger(value); ok {
		return i, nil
	}
	if i, ok := unsignedInteger(value); ok {
		return i, nil
	}
	if i, ok := integralFloat(value); ok {
		return i, nil
	}
	if i, ok := integralJSONNumber(value); ok {
		return i, nil
	}
	if i, ok := integralString(value); ok {
		return i, nil
	}
	message := "expected integer"
	if u, ok := value.(uint64); ok && u > math.MaxInt64 {
		message = "integer is out of range"
	}
	return nil, []Issue{issue(CodeType, path, message, value)}
}

func signedInteger(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

func unsignedInteger(value any) (int64, bool) {
	var u uint64
	switch v := value.(type) {
	case uint:
		u = uint64(v)
	case uint8:
		u = uint64(v)
	case uint16:
		u = uint64(v)
	case uint32:
		u = uint64(v)
	case uint64:
		u = v
	default:
		return 0, false
	}
	if u > math.MaxInt64 {
		return 0, false
	}
	return int64(u), true
}

func integralFloat(value any) (int64, bool) {
	switch v := value.(type) {
	case float32:
		f := float64(v)
		if math.Trunc(f) != f || f > math.MaxInt64 || f < math.MinInt64 {
			return 0, false
		}
		return int64(f), true
	case float64:
		if math.Trunc(v) != v || v > math.MaxInt64 || v < math.MinInt64 {
			return 0, false
		}
		return int64(v), true
	default:
		return 0, false
	}
}

func integralJSONNumber(value any) (int64, bool) {
	v, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	if i, err := v.Int64(); err == nil {
		return i, true
	}
	f, err := v.Float64()
	if err != nil || math.Trunc(f) != f || f > math.MaxInt64 || f < math.MinInt64 {
		return 0, false
	}
	return int64(f), true
}

func integralString(value any) (int64, bool) {
	v, ok := value.(string)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return i, err == nil
}

func coerceNumber(value any, path string) (any, []Issue) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint, uint8, uint16, uint32:
		f, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f, nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return nil, []Issue{issue(CodeType, path, "expected number", value)}
		}
		return f, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nil, []Issue{issue(CodeType, path, "expected number", value)}
		}
		return f, nil
	default:
		return nil, []Issue{issue(CodeType, path, "expected number", value)}
	}
}

func coerceBoolean(value any, path string) (any, []Issue) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes":
			return true, nil
		case "false", "0", "off", "no":
			return false, nil
		default:
			return nil, []Issue{issue(CodeType, path, "expected boolean", value)}
		}
	default:
		return nil, []Issue{issue(CodeType, path, "expected boolean", value)}
	}
}

func coerceArray(field model.Field, value any, opts Options, path string) (any, []Issue) {
	if value == nil {
		return nil, nil
	}
	var values []any
	switch v := value.(type) {
	case []any:
		values = v
	case []string:
		values = make([]any, len(v))
		for i, item := range v {
			values[i] = item
		}
	default:
		values = []any{v}
	}
	itemField := model.Field{Name: field.Name, Type: inferEnumType(field.Enum), Enum: field.Enum}
	if field.Items != nil {
		itemField = *field.Items
	}
	out := make([]any, 0, len(values))
	var issues []Issue
	for i, item := range values {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		coerced, itemIssues := CoerceValue(itemField, item, opts, itemPath)
		issues = append(issues, itemIssues...)
		out = append(out, coerced)
	}
	return out, issues
}

func coerceObject(field model.Field, value any, opts Options, path string) (any, []Issue) {
	if value == nil {
		return nil, nil
	}
	if IsRawObjectField(field) {
		switch v := value.(type) {
		case map[string]any:
			return v, nil
		case string:
			var decoded any
			if err := strictDecodeJSON(strings.NewReader(v), &decoded); err != nil {
				return nil, []Issue{issue(CodeInvalidJSON, path, "invalid JSON object", value)}
			}
			obj, ok := decoded.(map[string]any)
			if !ok {
				return nil, []Issue{issue(CodeObject, path, "expected object", value)}
			}
			return obj, nil
		default:
			return nil, []Issue{issue(CodeObject, path, "expected object", value)}
		}
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, []Issue{issue(CodeObject, path, "expected object", value)}
	}
	out := make(map[string]any, len(obj))
	var issues []Issue
	for key, item := range obj {
		child, ok := nestedField(field.Nested, key)
		if !ok {
			switch opts.UnknownFields {
			case UnknownPreserve:
				out[key] = item
			case UnknownIssue:
				issues = append(issues, issue(CodeUnknownField, joinPath(path, key), fmt.Sprintf("unknown field %q", joinPath(path, key)), item))
			}
			continue
		}
		coerced, childIssues := CoerceValue(child, item, opts, joinPath(path, key))
		issues = append(issues, childIssues...)
		out[key] = coerced
	}
	return out, issues
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}
