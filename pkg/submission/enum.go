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
	encoded := strings.TrimPrefix(raw, enumPrefix)
	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return raw, false
	}
	var payload enumPayload
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return raw, false
	}
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
		switch value := payload.Value.(type) {
		case json.Number:
			i, err := value.Int64()
			return i, err == nil
		case float64:
			return int64(value), true
		}
	case "uint":
		switch value := payload.Value.(type) {
		case json.Number:
			u, err := strconv.ParseUint(value.String(), 10, 64)
			return u, err == nil
		case float64:
			if value < 0 || value != float64(uint64(value)) {
				return raw, false
			}
			return uint64(value), true
		}
	case "number":
		switch value := payload.Value.(type) {
		case json.Number:
			f, err := value.Float64()
			return f, err == nil
		case float64:
			return value, true
		}
	case "null":
		return nil, true
	}
	return raw, false
}

func enumType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32:
		return "int"
	case uint64:
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
		return int64(v)
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
	switch v := value.(type) {
	case int:
		return big.NewRat(int64(v), 1), true
	case int8:
		return big.NewRat(int64(v), 1), true
	case int16:
		return big.NewRat(int64(v), 1), true
	case int32:
		return big.NewRat(int64(v), 1), true
	case int64:
		return big.NewRat(v, 1), true
	case uint:
		return new(big.Rat).SetInt(new(big.Int).SetUint64(uint64(v))), true
	case uint8:
		return new(big.Rat).SetInt(new(big.Int).SetUint64(uint64(v))), true
	case uint16:
		return new(big.Rat).SetInt(new(big.Int).SetUint64(uint64(v))), true
	case uint32:
		return new(big.Rat).SetInt(new(big.Int).SetUint64(uint64(v))), true
	case uint64:
		return new(big.Rat).SetInt(new(big.Int).SetUint64(v)), true
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
