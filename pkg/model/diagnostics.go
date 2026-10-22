package model

import (
	internalmodel "github.com/goliatone/go-formgen/internal/model"
	"github.com/goliatone/go-formgen/pkg/schema"
)

// DiagnoseExtensions optionally diagnoses package metadata without restricting
// arbitrary host namespaces or changing legacy extraction behavior.
func DiagnoseExtensions(extensions map[string]any, path string) []DefinitionDiagnostic {
	return internalmodel.DiagnoseExtensions(extensions, path)
}

// DiagnoseSchema returns normalized definition locations. It does not perform
// renderer component registration checks or change metadata precedence.
func DiagnoseSchema(definition schema.Schema) []DefinitionDiagnostic {
	return internalmodel.DiagnoseSchema(definition)
}
