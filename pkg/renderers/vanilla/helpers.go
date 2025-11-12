package vanilla

import "strings"

func componentControlID(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return "fg-" + trimmed
}

func componentLabelID(name string) string {
	controlID := componentControlID(name)
	if controlID == "" {
		return ""
	}
	return controlID + "-label"
}

func sanitizeClassList(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	tokens := strings.Fields(value)
	keep := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "fg-") {
			continue
		}
		keep = append(keep, token)
	}
	return strings.Join(keep, " ")
}

func labelSupportsFor(componentName string) bool {
	switch strings.TrimSpace(componentName) {
	case "object", "array", "datetime-range":
		return false
	default:
		return true
	}
}

func componentHandlesChrome(componentName string) bool {
	switch strings.TrimSpace(componentName) {
	case "object", "array", "datetime-range":
		return true
	default:
		return false
	}
}

// componentHandlesDescription returns true if a component handles its own description
// (e.g., uses description as placeholder instead of rendering it separately).
func componentHandlesDescription(componentName string) bool {
	switch strings.TrimSpace(componentName) {
	case "wysiwyg":
		return true
	default:
		return false
	}
}
