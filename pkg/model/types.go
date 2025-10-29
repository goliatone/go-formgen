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

type ValidationRule = internalmodel.ValidationRule
type Field = internalmodel.Field
type FormModel = internalmodel.FormModel
