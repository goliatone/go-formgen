package model

import internalmodel "github.com/goliatone/formgen/internal/model"

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

// Validation rule identifiers mirror OpenAPI keyword semantics and are emitted
// by the form model builder when schemas define matching constraints.
const (
	ValidationRuleMin       = internalmodel.ValidationRuleMin
	ValidationRuleMax       = internalmodel.ValidationRuleMax
	ValidationRuleMinLength = internalmodel.ValidationRuleMinLength
	ValidationRuleMaxLength = internalmodel.ValidationRuleMaxLength
	ValidationRulePattern   = internalmodel.ValidationRulePattern
)

type ValidationRule = internalmodel.ValidationRule
// ValidationRule describes a single constraint with canonical identifiers and
// string parameters (documented in go-form-gen.md) that renderers can translate
// into attributes or runtime checks.
type Field = internalmodel.Field
type FormModel = internalmodel.FormModel
