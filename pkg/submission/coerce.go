package submission

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

// CoerceValue normalizes one submitted value according to the supplied field.
func CoerceValue(field model.Field, value any, opts Options, path string) (any, []Issue) {
	if opts.EnumValues == EnumNative {
		opts.EnumValues = nativeEnumMode(field)
	}
	value = decodeEnumInput(field, value, opts.EnumValues)

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

func decodeEnumInput(field model.Field, value any, mode EnumValueMode) any {
	// Array containers delegate decoding to their items. Decoding the container
	// first would decode a scalar-wrapped string twice.
	if field.Type == model.FieldTypeArray || mode == EnumLiteral || mode == EnumNative {
		return value
	}
	text, ok := value.(string)
	if !ok || (mode != EnumControl && declaredStringValue(field, text)) {
		return value
	}
	if decoded, encoded := DecodeEnumValue(text); encoded {
		return decoded
	}
	return value
}

// A declared literal string can itself start with the codec prefix. Typed JSON
// submissions already carry that value; native controls wrap it one extra time.
func declaredStringValue(field model.Field, value string) bool {
	for _, candidate := range field.Enum {
		if text, ok := candidate.(string); ok && text == value {
			return true
		}
	}
	for _, option := range field.Options {
		if text, ok := option.Value.(string); ok && text == value {
			return true
		}
	}
	return field.Type == model.FieldTypeArray && field.Items != nil && declaredStringValue(*field.Items, value)
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
		if math.Trunc(f) != f || f >= 0x1p63 || f < -0x1p63 {
			return 0, false
		}
		return int64(f), true
	case float64:
		if math.Trunc(v) != v || v >= 0x1p63 || v < -0x1p63 {
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
	// Validate even host-created json.Number values. Canonicalization adjusts
	// the exponent without allocating the represented decimal magnitude.
	if len(v) == 0 || (v[0] != '-' && (v[0] < '0' || v[0] > '9')) || !json.Valid([]byte(v)) {
		return 0, false
	}
	canonical := string(canonicalNumber(v))
	if canonical == "0" {
		return 0, true
	}
	digits, exponent, ok := strings.Cut(canonical, "e")
	if !ok {
		return 0, false
	}
	scale, err := strconv.Atoi(exponent)
	if err != nil || scale < 0 || scale > 19 || len(strings.TrimPrefix(digits, "-")) > 19-scale {
		return 0, false
	}
	i, err := strconv.ParseInt(digits+strings.Repeat("0", scale), 10, 64)
	return i, err == nil
}

func integralString(value any) (int64, bool) {
	v, ok := value.(string)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return i, err == nil
}

// finiteNumber is shared by coercion and direct validation: non-finite values
// are not JSON numbers and must never bypass numeric constraints.
func finiteNumber(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func coerceNumber(value any, path string) (any, []Issue) {
	if value == nil {
		return nil, nil
	}
	var number float64
	var err error
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		number, err = strconv.ParseFloat(fmt.Sprint(v), 64)
	case float32:
		number = float64(v)
	case float64:
		number = v
	case json.Number:
		number, err = v.Float64()
	case string:
		number, err = strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		return nil, []Issue{issue(CodeType, path, "expected number", value)}
	}
	if err != nil || !finiteNumber(number) {
		return nil, []Issue{issue(CodeType, path, "expected finite number", value)}
	}
	return number, nil
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
	values, ok := arrayItems(value)
	if !ok {
		return nil, nil
	}
	itemField, _ := nextArrayItemField(field, true)
	out := make([]any, 0, len(values))
	var issues []Issue
	for i, item := range values {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		itemOpts := opts
		if text, ok := item.(string); ok && opts.EnumValues == EnumAuto && declaredStringValue(field, text) {
			itemOpts.EnumValues = EnumLiteral
		}
		coerced, itemIssues := CoerceValue(itemField, item, itemOpts, itemPath)
		issues = append(issues, itemIssues...)
		out = append(out, coerced)
	}
	return out, issues
}

// arrayItems unwraps the []any shape JSON decoding produces as well as typed
// slices, arrays and pointers to them, so ParseMap parses host containers the
// way ParseJSON parses their encoding. Nil values, slices and pointers are JSON
// null and report ok=false; byte slices stay one item because they encode as a
// single base64 string, while byte arrays encode as JSON arrays and unwrap;
// any other value is a single item.
func arrayItems(value any) ([]any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case []any:
		return v, v != nil
	case []string:
		if v == nil {
			return nil, false
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out, true
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return nil, false
		}
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return []any{rv.Interface()}, true
		}
	case reflect.Array:
	default:
		return []any{rv.Interface()}, true
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
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
