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

// RelationshipKind enumerates supported relationship semantics. Keep values in
// sync with docs/adr/RELATIONSHIP_STRUCT_ADR.md.
type RelationshipKind string

const (
	RelationshipBelongsTo RelationshipKind = "belongsTo"
	RelationshipHasOne    RelationshipKind = "hasOne"
	RelationshipHasMany   RelationshipKind = "hasMany"
)

// Relationship encodes typed relationship metadata while Phase 1 dotted keys
// remain for backward compatibility. See docs/adr/RELATIONSHIP_STRUCT_ADR.md
// for the migration strategy.
type Relationship struct {
	Kind        RelationshipKind `json:"kind"`
	Target      string           `json:"target"`
	ForeignKey  string           `json:"foreignKey,omitempty"`
	Cardinality string           `json:"cardinality"`
	Inverse     string           `json:"inverse,omitempty"`
	SourceField string           `json:"sourceField,omitempty"`
}

const (
	ValidationRuleMin         = "min"
	ValidationRuleMax         = "max"
	ValidationRuleMinLength   = "minLength"
	ValidationRuleMaxLength   = "maxLength"
	ValidationRuleMinItems    = "minItems"
	ValidationRuleMaxItems    = "maxItems"
	ValidationRulePattern     = "pattern"
	ValidationRuleUniqueItems = "uniqueItems"
)

// ValidationRule represents a single validation constraint applied to a field.
// Use the ValidationRule* constants to reference canonical OpenAPI-derived
// constraints (min/max, minLength/maxLength, minItems/maxItems, pattern).
// Numeric bounds, length limits, and item limits encode their threshold in
// Params["value"] while pattern rules preserve the original expression in
// Params["pattern"]. Boolean flags such as
// exclusivity are encoded as string values to keep JSON snapshots stable.
type ValidationRule struct {
	Kind   string            `json:"kind"`
	Params map[string]string `json:"params,omitempty"`
}

// Option is a renderer-neutral choice for scalar and multi-value controls.
// Value remains JSON typed while presentation metadata is preserved for rich
// select, chips, and remotely refreshed option widgets.
type Option struct {
	Value       any            `json:"value"`
	Label       string         `json:"label,omitempty"`
	Description string         `json:"description,omitempty"`
	Disabled    bool           `json:"disabled,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Field models an individual input inside a generated form. Struct fields are
// annotated so renderers can serialise them directly when needed.
type Field struct {
	Name         string            `json:"name"`
	Type         FieldType         `json:"type"`
	Format       string            `json:"format,omitempty"`
	Required     bool              `json:"required"`
	Disabled     bool              `json:"disabled,omitempty"`
	Readonly     bool              `json:"readonly,omitempty"`
	Label        string            `json:"label,omitempty"`
	Placeholder  string            `json:"placeholder,omitempty"`
	Description  string            `json:"description,omitempty"`
	Default      any               `json:"default,omitempty"`
	Sensitive    bool              `json:"sensitive,omitempty"`
	Enum         []any             `json:"enum,omitempty"`
	Options      []Option          `json:"options,omitempty"`
	Nested       []Field           `json:"nested,omitempty"`
	Items        *Field            `json:"items,omitempty"`
	OneOf        []Field           `json:"oneOf,omitempty"`
	Validations  []ValidationRule  `json:"validations,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	UIHints      map[string]string `json:"uiHints,omitempty"`
	Relationship *Relationship     `json:"relationship,omitempty"`
}

// FormModel is the top-level representation renderers consume, matching the
// README structure in go-form-gen.md:111-158.
type FormModel struct {
	Conditions  *ConditionSet     `json:"conditions,omitempty"`
	OperationID string            `json:"operationId"`
	Endpoint    string            `json:"endpoint"`
	Method      string            `json:"method"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Fields      []Field           `json:"fields"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	UIHints     map[string]string `json:"uiHints,omitempty"`
}

// ConditionsVersion identifies the opt-in embedded condition contract.
const ConditionsVersion = "formgen.conditions/v1"

// ConditionSet is an authoritative definition. Paths are absolute canonical
// field paths, with explicit indexes; wildcards and relative paths are unsupported.
type ConditionSet struct {
	Version string          `json:"version"`
	Rules   []ConditionRule `json:"rules"`
}

type ConditionRule struct {
	Path           string      `json:"path"`
	ApplicableWhen *Expression `json:"applicableWhen,omitempty"`
	RequiredWhen   *Expression `json:"requiredWhen,omitempty"`
	OnInapplicable string      `json:"onInapplicable,omitempty"` // clear (default) or retain
}

// Expression permits exactly one operator: present, eq, and, or. Equality
// requires a present, non-null value; false and zero remain present.
type Expression struct {
	Op    string       `json:"op"`
	Path  string       `json:"path,omitempty"`
	Value any          `json:"value,omitempty"`
	Args  []Expression `json:"args,omitempty"`
}

// DefinitionDiagnostic contains definition locations and never answer values.
type DefinitionDiagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}
