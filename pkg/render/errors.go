package render

import (
	"strconv"
	"strings"

	"github.com/goliatone/formgen/pkg/model"
)

// ErrorMapping splits a go-errors compatible payload into field-level and
// form-level messages keyed by the dotted field paths used throughout the
// render pipeline.
type ErrorMapping struct {
	Fields map[string][]string
	Form   []string
}

// MergeFormErrors concatenates and normalises multiple form-level error
// slices, trimming whitespace and removing duplicates while preserving order.
func MergeFormErrors(existing []string, extras ...string) []string {
	combined := make([]string, 0, len(existing)+len(extras))
	combined = append(combined, existing...)
	combined = append(combined, extras...)
	return normalizeMessages(combined)
}

// MapErrorPayload normalises server error payloads (including go-errors style
// JSON pointer paths) into dotted field identifiers that renderers can consume.
// Unknown paths are treated as form-level errors so messages are not lost.
func MapErrorPayload(form model.FormModel, payload map[string][]string) ErrorMapping {
	mapping := ErrorMapping{
		Fields: make(map[string][]string),
	}
	if len(payload) == 0 {
		return mapping
	}

	fieldPaths := make(map[string]struct{})
	collectFieldPaths(form.Fields, "", fieldPaths)

	for rawPath, messages := range payload {
		normalizedMessages := normalizeMessages(messages)
		if len(normalizedMessages) == 0 {
			continue
		}

		mapped, formLevel := mapErrorPath(rawPath, fieldPaths)
		if formLevel || mapped == "" {
			mapping.Form = append(mapping.Form, normalizedMessages...)
			continue
		}
		mapping.Fields[mapped] = append(mapping.Fields[mapped], normalizedMessages...)
	}

	if len(mapping.Fields) == 0 {
		mapping.Fields = nil
	}
	mapping.Form = normalizeMessages(mapping.Form)
	return mapping
}

func normalizeMessages(messages []string) []string {
	if len(messages) == 0 {
		return nil
	}

	out := make([]string, 0, len(messages))
	seen := make(map[string]struct{}, len(messages))

	for _, message := range messages {
		trimmed := strings.TrimSpace(message)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func mapErrorPath(raw string, fieldPaths map[string]struct{}) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if isFormLevelKey(trimmed) {
		return "", true
	}

	segments := parsePathSegments(trimmed)
	if len(segments) == 0 {
		return "", true
	}

	best := ""
	for _, variant := range buildSegmentVariants(segments) {
		if path := longestMatchingPath(variant, fieldPaths); path != "" {
			if len(pathSegments(path)) > len(pathSegments(best)) {
				best = path
			}
		}
	}

	if best != "" {
		return best, false
	}

	return "", true
}

func parsePathSegments(path string) []string {
	if path == "" {
		return nil
	}

	clean := strings.TrimSpace(path)
	clean = strings.TrimPrefix(clean, "#/")
	clean = strings.TrimPrefix(clean, "$/")
	clean = strings.TrimPrefix(clean, "$.")
	for strings.HasPrefix(clean, "#") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, ".") || strings.HasPrefix(clean, "$") {
		clean = strings.TrimPrefix(clean, "#")
		clean = strings.TrimPrefix(clean, "/")
		clean = strings.TrimPrefix(clean, ".")
		clean = strings.TrimPrefix(clean, "$")
	}

	replacer := strings.NewReplacer("[", ".", "]", "", "//", "/")
	clean = replacer.Replace(clean)
	clean = strings.Trim(clean, "./")
	if clean == "" {
		return nil
	}

	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '.' || r == '/'
	})

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := strings.TrimSpace(part)
		if segment == "" {
			continue
		}
		segment = strings.ReplaceAll(segment, "~1", "/")
		segment = strings.ReplaceAll(segment, "~0", "~")
		out = append(out, segment)
	}
	return out
}

func buildSegmentVariants(segments []string) [][]string {
	var variants [][]string
	seen := make(map[string]struct{}, 4)

	appendVariant := func(candidate []string) {
		if len(candidate) == 0 {
			return
		}
		key := strings.Join(candidate, ".")
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		var copyCandidate []string
		copyCandidate = append(copyCandidate, candidate...)
		variants = append(variants, copyCandidate)
	}

	appendVariant(segments)

	noWrappers := dropWrapperSegments(segments)
	appendVariant(noWrappers)
	appendVariant(stripNumericSegments(segments))
	appendVariant(stripNumericSegments(noWrappers))

	return variants
}

func dropWrapperSegments(segments []string) []string {
	if len(segments) == 0 {
		return segments
	}

	wrappers := map[string]struct{}{
		"body":       {},
		"request":    {},
		"payload":    {},
		"data":       {},
		"attributes": {},
	}

	out := segments
	for len(out) > 0 {
		if _, ok := wrappers[strings.ToLower(out[0])]; ok {
			out = out[1:]
			continue
		}
		break
	}
	return out
}

func stripNumericSegments(segments []string) []string {
	if len(segments) == 0 {
		return segments
	}

	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		if _, err := strconv.Atoi(segment); err == nil {
			continue
		}
		out = append(out, segment)
	}
	return out
}

func longestMatchingPath(segments []string, fieldPaths map[string]struct{}) string {
	if len(segments) == 0 || len(fieldPaths) == 0 {
		return ""
	}

	for end := len(segments); end > 0; end-- {
		candidate := strings.Join(segments[:end], ".")
		if _, ok := fieldPaths[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func pathSegments(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

func collectFieldPaths(fields []model.Field, prefix string, dest map[string]struct{}) {
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		path := joinPath(prefix, name)
		dest[path] = struct{}{}

		if len(field.Nested) > 0 {
			collectFieldPaths(field.Nested, path, dest)
		}
		if field.Items != nil {
			collectItemPaths(field.Items, path, dest)
		}
	}
}

func collectItemPaths(item *model.Field, prefix string, dest map[string]struct{}) {
	if item == nil {
		return
	}
	if name := strings.TrimSpace(item.Name); name != "" {
		itemPath := joinPath(prefix, name)
		dest[itemPath] = struct{}{}
	}
	if len(item.Nested) > 0 {
		collectFieldPaths(item.Nested, prefix, dest)
	}
	if item.Items != nil {
		collectItemPaths(item.Items, prefix, dest)
	}
}

func joinPath(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}

func isFormLevelKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "", ".", "/", "#", "$", "form", "base", "__all__", "non_field_errors", "non-field-errors":
		return true
	default:
		return false
	}
}
