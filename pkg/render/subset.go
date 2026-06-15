package render

import "github.com/goliatone/go-formgen/pkg/model"

// FieldSubset describes the allowed groups, tags, or sections for partial
// rendering. This is a compatibility alias to the renderer-free model type.
type FieldSubset = model.FieldSubset

// ApplySubset removes fields that do not match the supplied subset filters.
// This compatibility wrapper delegates to the renderer-free model helper.
func ApplySubset(form *model.FormModel, subset FieldSubset) {
	model.ApplySubset(form, subset)
}
