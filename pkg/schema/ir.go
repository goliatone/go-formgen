package schema

import (
	"context"
	"sort"
	"strings"
)

// NormalizeOptions supplies optional hints to adapters during normalization.
type NormalizeOptions struct {
	// ContentTypeSlug provides a fallback slug for form discovery.
	ContentTypeSlug string
	// DefaultFormSuffix overrides the default ".edit" suffix for derived form IDs.
	DefaultFormSuffix string
	// FormID optionally pins normalization to a specific form identifier.
	FormID string
}

// Form describes a normalized form entry extracted from a source document.
type Form struct {
	ID          string
	Method      string
	Endpoint    string
	Summary     string
	Description string
	Schema      Schema
	Responses   map[string]Schema
	Extensions  map[string]any
}

// Schema represents the canonical schema IR consumed by form model builders.
type Schema struct {
	Ref              string
	Type             string
	Format           string
	Title            string
	Description      string
	Default          any
	Enum             []any
	Const            any
	Required         []string
	Properties       map[string]Schema
	Items            *Schema
	OneOf            []Schema
	AnyOf            []Schema
	AllOf            []Schema
	Minimum          *float64
	Maximum          *float64
	ExclusiveMinimum bool
	ExclusiveMaximum bool
	MinLength        *int
	MaxLength        *int
	Pattern          string
	Extensions       map[string]any `json:"Extensions,omitempty"`
}

// SchemaIR is the normalized schema set produced by adapters.
type SchemaIR struct {
	Forms map[string]Form
}

// FormRef provides minimal metadata about an available form.
type FormRef struct {
	ID          string
	Title       string
	Summary     string
	Description string
}

// NewSchemaIR constructs an empty schema IR container.
func NewSchemaIR() SchemaIR {
	return SchemaIR{Forms: make(map[string]Form)}
}

// Form looks up a form by id.
func (ir SchemaIR) Form(id string) (Form, bool) {
	if ir.Forms == nil {
		return Form{}, false
	}
	form, ok := ir.Forms[id]
	return form, ok
}

// FormRefs returns a sorted list of available form references.
func (ir SchemaIR) FormRefs() []FormRef {
	if len(ir.Forms) == 0 {
		return nil
	}
	ids := make([]string, 0, len(ir.Forms))
	for id := range ir.Forms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	refs := make([]FormRef, 0, len(ids))
	for _, id := range ids {
		form := ir.Forms[id]
		refID := form.ID
		if strings.TrimSpace(refID) == "" {
			refID = id
		}
		refs = append(refs, FormRef{
			ID:          refID,
			Title:       strings.TrimSpace(form.Summary),
			Summary:     form.Summary,
			Description: form.Description,
		})
	}
	return refs
}

// FormatAdapter normalizes source documents into the canonical IR.
type FormatAdapter interface {
	Name() string
	Detect(src Source, raw []byte) bool
	Load(ctx context.Context, src Source) (Document, error)
	Normalize(ctx context.Context, doc Document, opts NormalizeOptions) (SchemaIR, error)
	Forms(ctx context.Context, ir SchemaIR) ([]FormRef, error)
}
