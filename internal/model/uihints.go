package model

import (
	"encoding/json"
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
		"input",
		"helpText",
		"hint",
		"hideLabel",
		"inputType",
		"label",
		"placeholder",
		"precision",
		"priority",
		"repeaterLabel",
		"section",
		"submitLabel",
		"success-message",
		"successMessage",
		"unit",
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
		if v == "" {
			return "", false
		}
		return v, true
	case interface{ String() string }:
		s := v.String()
		if s == "" {
			return "", false
		}
		return s, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case int:
		return strconv.Itoa(v), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case map[string]any:
		if len(v) == 0 {
			return "", false
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(payload), true
	case map[string]string:
		if len(v) == 0 {
			return "", false
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(payload), true
	case []any:
		if len(v) == 0 {
			return "", false
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(payload), true
	case []string:
		if len(v) == 0 {
			return "", false
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(payload), true
	default:
		return "", false
	}
}
