package render

import (
	"fmt"
	"sort"
	"strings"
)

// HiddenField represents a hidden form input emitted alongside the visible
// schema. Use the helpers (CSRFToken, AuthToken, VersionField) to add common
// fields without repeating boilerplate.
type HiddenField struct {
	Name  string
	Value string
}

// Hidden returns a HiddenField for an arbitrary name/value pair.
func Hidden(name string, value any) HiddenField {
	return HiddenField{
		Name:  strings.TrimSpace(name),
		Value: fmt.Sprint(value),
	}
}

// CSRFToken constructs a hidden field carrying the provided token. Callers
// supply the input name to match their backend expectations (for example,
// "_csrf" or "csrf_token").
func CSRFToken(name, token string) HiddenField {
	return Hidden(name, token)
}

// AuthToken constructs a hidden field carrying an authentication token or
// session hint.
func AuthToken(name, token string) HiddenField {
	return Hidden(name, token)
}

// VersionField constructs a hidden field used for optimistic locking or
// version-aware submissions (for example, "if-match" or "version").
func VersionField(name string, version any) HiddenField {
	return Hidden(name, version)
}

// MergeHiddenFields returns a copy of base with the provided fields applied.
// Empty names are ignored; later fields win on name collisions.
func MergeHiddenFields(base map[string]string, fields ...HiddenField) map[string]string {
	if len(base) == 0 && len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(fields))
	for key, value := range base {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			out[trimmed] = value
		}
	}
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		out[name] = field.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SortedHiddenFields normalises and sorts hidden fields for deterministic
// rendering. Empty names are dropped.
func SortedHiddenFields(fields map[string]string) []HiddenField {
	if len(fields) == 0 {
		return nil
	}

	clean := make(map[string]string, len(fields))
	for name, value := range fields {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		clean[key] = value
	}
	if len(clean) == 0 {
		return nil
	}

	names := make([]string, 0, len(clean))
	for name := range clean {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]HiddenField, 0, len(names))
	for _, name := range names {
		result = append(result, HiddenField{
			Name:  name,
			Value: clean[name],
		})
	}
	return result
}
