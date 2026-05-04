package model

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
)

var (
	uiHintKeys = []string{
		"accordion",
		"badge",
		"cardinality",
		"category",
		"class",
		"cssClass",
		"group",
		"input",
		"helpText",
		"hint",
		"hideLabel",
		"inputType",
		"label",
		"order",
		"placeholder",
		"precision",
		"priority",
		"readonly",
		"repeaterLabel",
		"section",
		"submitLabel",
		"success-message",
		"successMessage",
		"tags",
		"unit",
		"visibilityRule",
		"widget",
	}

	uiHintKeySet = func(keys []string) map[string]struct{} {
		result := make(map[string]struct{}, len(keys))
		for _, key := range keys {
			result[key] = struct{}{}
		}
		return result
	}(uiHintKeys)
)

// AllowedUIHintKeys returns a sorted copy of the recognised UI extension keys.
func AllowedUIHintKeys() []string {
	keys := append([]string(nil), uiHintKeys...)
	sort.Strings(keys)
	return keys
}

// IsAllowedUIHintKey reports whether the supplied key participates in the
// curated UI hint contract.
func IsAllowedUIHintKey(key string) bool {
	_, ok := uiHintKeySet[key]
	return ok
}

// CanonicalizeExtensionValue mirrors the builder's rules for turning extension
// values into renderer-friendly strings. Returns false when the value cannot be
// represented deterministically.
func CanonicalizeExtensionValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return nonEmptyString(v)
	case interface{ String() string }:
		return nonEmptyString(v.String())
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case map[string]any:
		return marshalNonEmpty(v)
	case map[string]string:
		return marshalNonEmpty(v)
	case []any:
		return marshalNonEmpty(v)
	case []string:
		return marshalNonEmpty(v)
	}

	return canonicalizeNumericValue(value)
}

func nonEmptyString(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	return value, true
}

func marshalNonEmpty(value any) (string, bool) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Len() == 0 {
		return "", false
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return string(payload), true
}

func canonicalizeNumericValue(value any) (string, bool) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return "", false
	}

	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflected.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(reflected.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(reflected.Float(), 'f', -1, 64), true
	default:
		return "", false
	}
}
