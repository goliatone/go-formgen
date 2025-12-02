package model

import internalmodel "github.com/goliatone/go-formgen/internal/model"

// FieldType re-exports the internal FieldType enumeration.
type FieldType = internalmodel.FieldType

const (
	FieldTypeString  = internalmodel.FieldTypeString
	FieldTypeInteger = internalmodel.FieldTypeInteger
	FieldTypeNumber  = internalmodel.FieldTypeNumber
	FieldTypeBoolean = internalmodel.FieldTypeBoolean
	FieldTypeArray   = internalmodel.FieldTypeArray
	FieldTypeObject  = internalmodel.FieldTypeObject
)

// RelationshipKind re-exports the relationship enum defined in
// docs/adr/RELATIONSHIP_STRUCT_ADR.md.
type RelationshipKind = internalmodel.RelationshipKind

const (
	RelationshipBelongsTo = internalmodel.RelationshipBelongsTo
	RelationshipHasOne    = internalmodel.RelationshipHasOne
	RelationshipHasMany   = internalmodel.RelationshipHasMany
)

// Relationship exposes typed relationship metadata alongside the existing
// dotted keys for backward compatibility. See docs/adr/RELATIONSHIP_STRUCT_ADR.md.
type Relationship = internalmodel.Relationship

// Validation rule identifiers mirror OpenAPI keyword semantics and are emitted
// by the form model builder when schemas define matching constraints.
const (
	ValidationRuleMin       = internalmodel.ValidationRuleMin
	ValidationRuleMax       = internalmodel.ValidationRuleMax
	ValidationRuleMinLength = internalmodel.ValidationRuleMinLength
	ValidationRuleMaxLength = internalmodel.ValidationRuleMaxLength
	ValidationRulePattern   = internalmodel.ValidationRulePattern
)

// ValidationRule represents an OpenAPI-derived constraint. Threshold-based rules
// encode their limit in Params["value"], pattern rules preserve the original
// expression in Params["pattern"], and boolean qualifiers such as exclusivity
// remain string typed to keep JSON snapshots deterministic.
type ValidationRule = internalmodel.ValidationRule

// Field mirrors internal model fields for renderer consumption.
type Field = internalmodel.Field
type FormModel = internalmodel.FormModel
