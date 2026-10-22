// Package conditions compiles and evaluates optional typed form applicability.
// It has no renderer, browser, HTTP or legacy visibility dependencies.
package conditions

import (
	internalconditions "github.com/goliatone/go-formgen/internal/conditions"
	"github.com/goliatone/go-formgen/pkg/model"
)

type Program = internalconditions.Program
type Projection = internalconditions.Projection

// Compile returns a reusable immutable program or value-free definition diagnostics.
func Compile(form model.FormModel) (*Program, []model.DefinitionDiagnostic) {
	return internalconditions.Compile(form)
}
