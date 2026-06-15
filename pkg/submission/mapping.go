package submission

import (
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"github.com/goliatone/go-formgen/pkg/render"
)

// IssuesToErrorPayload converts issues to a go-errors compatible payload keyed
// by precise submission paths.
func IssuesToErrorPayload(issues []Issue) map[string][]string {
	if len(issues) == 0 {
		return nil
	}
	payload := make(map[string][]string)
	for _, item := range issues {
		message := strings.TrimSpace(item.Message)
		if message == "" {
			message = string(item.Code)
		}
		path := strings.TrimSpace(item.Path)
		if path == "" {
			path = "form"
		}
		payload[path] = append(payload[path], message)
	}
	return payload
}

// IssuesToRenderErrors maps submission issues to the renderer-compatible field
// and form error shape used by render.RenderOptions.
func IssuesToRenderErrors(form model.FormModel, issues []Issue) render.ErrorMapping {
	return render.MapErrorPayload(form, IssuesToErrorPayload(issues))
}

// IssuesToFieldAndFormErrors returns field and form errors for direct
// RenderOptions assignment.
func IssuesToFieldAndFormErrors(form model.FormModel, issues []Issue) (map[string][]string, []string) {
	mapping := IssuesToRenderErrors(form, issues)
	return mapping.Fields, mapping.Form
}
