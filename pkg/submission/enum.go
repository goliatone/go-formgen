package submission

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const enumPrefix = "__fg_enum_v1:"

type enumPayload struct {
	Type  string `json:"t"`
	Value any    `json:"v"`
}

// EncodeEnumValue returns a deterministic, type-tagged form control value.
func EncodeEnumValue(value any) string {
	payload := enumPayload{Type: enumType(value), Value: normalizeEnumValue(value)}
	body, _ := json.Marshal(payload)
	return enumPrefix + base64.RawURLEncoding.EncodeToString(body)
}

// EncodeEnumControlValue returns the value renderers should emit into browser
// form controls. Plain strings stay plain for compatibility unless they collide
// with the reserved encoded-value prefix.
func EncodeEnumControlValue(value any) string {
	if text, ok := value.(string); ok && !strings.HasPrefix(text, enumPrefix) {
		return text
	}
	return EncodeEnumValue(value)
}

// DecodeEnumValue decodes a value emitted by EncodeEnumValue. Plain strings
// that are not valid encoded payloads are returned unchanged.
func DecodeEnumValue(raw string) (any, bool) {
	if !strings.HasPrefix(raw, enumPrefix) {
		return raw, false
	}
	payload, ok := decodeEnumPayload(strings.TrimPrefix(raw, enumPrefix))
	if !ok {
		return raw, false
	}
	value, ok := decodeEnumPayloadValue(payload)
	if !ok {
		return raw, false
	}
	return value, true
}

func decodeEnumPayload(encoded string) (enumPayload, bool) {
	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return enumPayload{}, false
	}
	var payload enumPayload
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return enumPayload{}, false
	}
	return payload, true
}

func decodeEnumPayloadValue(payload enumPayload) (any, bool) {
	switch payload.Type {
	case "string":
		if value, ok := payload.Value.(string); ok {
			return value, true
		}
	case "bool":
		if value, ok := payload.Value.(bool); ok {
			return value, true
		}
	case "int":
		return decodeEnumInt(payload.Value)
	case "uint":
		return decodeEnumUint(payload.Value)
	case "number":
		return decodeEnumNumber(payload.Value)
	case "null":
		return nil, true
	}
	return nil, false
}

func decodeEnumInt(value any) (any, bool) {
	switch v := value.(type) {
	case json.Number:
		i, err := v.Int64()
		return i, err == nil
	case float64:
		return int64(v), true
	default:
		return nil, false
	}
}

func decodeEnumUint(value any) (any, bool) {
	switch v := value.(type) {
	case json.Number:
		u, err := strconv.ParseUint(v.String(), 10, 64)
		return u, err == nil
	case float64:
		if v < 0 || v != float64(uint64(v)) {
			return nil, false
		}
		return uint64(v), true
	default:
		return nil, false
	}
}

func decodeEnumNumber(value any) (any, bool) {
	switch v := value.(type) {
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case float64:
		return v, true
	default:
		return nil, false
	}
}

func enumType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint8, uint16, uint32:
		return "int"
	case uint, uint64:
		return "uint"
	case float32, float64, json.Number:
		return "number"
	case nil:
		return "null"
	default:
		return "string"
	}
}

func normalizeEnumValue(value any) any {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case uint:
		return uint64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case float32:
		return float64(v)
	default:
		return v
	}
}

func enumValueEqual(a, b any) bool {
	if ar, aok := enumNumberRat(a); aok {
		if br, bok := enumNumberRat(b); bok {
			return ar.Cmp(br) == 0
		}
	}
	return fmt.Sprint(normalizeEnumValue(a)) == fmt.Sprint(normalizeEnumValue(b)) && enumType(a) == enumType(b)
}

func enumNumberRat(value any) (*big.Rat, bool) {
	if i, ok := enumSignedInt(value); ok {
		return big.NewRat(i, 1), true
	}
	if u, ok := enumUnsignedInt(value); ok {
		return new(big.Rat).SetInt(new(big.Int).SetUint64(u)), true
	}
	return enumFloatRat(value)
}

func enumSignedInt(value any) (int64, bool) {
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

func enumUnsignedInt(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	default:
		return 0, false
	}
}

func enumFloatRat(value any) (*big.Rat, bool) {
	switch v := value.(type) {
	case float32:
		if rat := new(big.Rat).SetFloat64(float64(v)); rat != nil {
			return rat, true
		}
		return nil, false
	case float64:
		if rat := new(big.Rat).SetFloat64(v); rat != nil {
			return rat, true
		}
		return nil, false
	case json.Number:
		rat, ok := new(big.Rat).SetString(v.String())
		return rat, ok
	default:
		return nil, false
	}
}
