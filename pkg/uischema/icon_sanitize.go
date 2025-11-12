package uischema

import (
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	iconPolicyOnce sync.Once
	iconPolicy     *bluemonday.Policy
)

func sanitizeIconMarkup(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	policy := iconSanitizer()
	cleaned := strings.TrimSpace(policy.Sanitize(trimmed))
	if cleaned == "" {
		return ""
	}
	return cleaned
}

func iconSanitizer() *bluemonday.Policy {
	iconPolicyOnce.Do(func() {
		policy := bluemonday.StrictPolicy()
		elements := []string{
			"svg", "g", "path", "circle", "rect", "line", "polyline", "polygon",
			"ellipse", "title", "desc", "defs", "use", "clipPath",
		}
		policy.AllowElements(elements...)

		policy.AllowAttrs(
			"xmlns", "viewBox", "width", "height", "fill", "stroke",
			"stroke-width", "stroke-linecap", "stroke-linejoin", "aria-hidden",
			"role", "focusable", "class",
		).OnElements("svg")

		policy.AllowAttrs(
			"href", "xlink:href", "clip-path",
		).OnElements("use")

		for _, el := range []string{"path", "circle", "rect", "line", "polyline", "polygon", "ellipse"} {
			policy.AllowAttrs(
				"d", "cx", "cy", "r", "x", "y", "x1", "y1", "x2", "y2",
				"points", "rx", "ry", "fill", "stroke", "stroke-width",
				"stroke-linecap", "stroke-linejoin", "class",
			).OnElements(el)
		}

		policy.AllowAttrs("id", "clipPathUnits").OnElements("clipPath")
		policy.AllowAttrs("id").OnElements("defs")
		policy.AllowAttrs("id").OnElements("g")

		iconPolicy = policy
	})
	return iconPolicy
}
