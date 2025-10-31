package model

// FieldType is the simplified enum for form-friendly field kinds.
type FieldType string

const (
	FieldTypeString  FieldType = "string"
	FieldTypeInteger FieldType = "integer"
	FieldTypeNumber  FieldType = "number"
	FieldTypeBoolean FieldType = "boolean"
	FieldTypeArray   FieldType = "array"
	FieldTypeObject  FieldType = "object"
)

const (
	ValidationRuleMin       = "min"
	ValidationRuleMax       = "max"
	ValidationRuleMinLength = "minLength"
	ValidationRuleMaxLength = "maxLength"
	ValidationRulePattern   = "pattern"
)

// ValidationRule represents a single validation constraint applied to a field.
// Use the ValidationRule* constants to reference canonical OpenAPI-derived
// constraints (min/max, minLength/maxLength, pattern). Numeric bounds and length
// limits encode their threshold in Params["value"] while pattern rules preserve
// the original expression in Params["pattern"]. Boolean flags such as
// exclusivity are encoded as string values to keep JSON snapshots stable.
type ValidationRule struct {
	Kind   string            `json:"kind"`
	Params map[string]string `json:"params,omitempty"`
}

// Field models an individual input inside a generated form. Struct fields are
// annotated so renderers can serialise them directly when needed.
type Field struct {
	Name        string            `json:"name"`
	Type        FieldType         `json:"type"`
	Format      string            `json:"format,omitempty"`
	Required    bool              `json:"required"`
	Label       string            `json:"label,omitempty"`
	Placeholder string            `json:"placeholder,omitempty"`
	Description string            `json:"description,omitempty"`
	Default     any               `json:"default,omitempty"`
	Enum        []any             `json:"enum,omitempty"`
	Nested      []Field           `json:"nested,omitempty"`
	Items       *Field            `json:"items,omitempty"`
	Validations []ValidationRule  `json:"validations,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// FormModel is the top-level representation renderers consume, matching the
// README structure in go-form-gen.md:111-158.
type FormModel struct {
	OperationID string            `json:"operationId"`
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Fields      []Field           `json:"fields"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
