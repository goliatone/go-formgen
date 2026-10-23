package submission

import (
	"sort"
	"strings"

	"github.com/goliatone/go-formgen/internal/fieldpath"
	"github.com/goliatone/go-formgen/pkg/conditions"
	"github.com/goliatone/go-formgen/pkg/model"
)

// InactiveInputPolicy governs supplied inactive inputs in the opt-in pipeline.
type InactiveInputPolicy int

const (
	InactiveReject InactiveInputPolicy = iota
	InactiveNormalize
)

// ConditionalResult contains effective values and independent retained inputs.
// Issues includes the original parse issues as well as condition and validation issues.
type ConditionalResult struct {
	Result
	Retained   map[string]any
	Applicable map[string]bool
	Required   map[string]bool
}

// ValidateConditional composes any parser Result with the authoritative model's
// conditions and full effective-value validation. It never mutates its arguments.
func ValidateConditional(form model.FormModel, parsed Result, policy InactiveInputPolicy, options ...Option) ConditionalResult {
	out := ConditionalResult{Result: Result{Values: Values{}, Issues: append([]Issue(nil), parsed.Issues...)}}
	program, diagnostics := conditions.Compile(form)
	if policy != InactiveReject && policy != InactiveNormalize {
		out.Issues = append(out.Issues, issue(CodeCondition, "", "invalid inactive-input policy", nil))
		return out
	}
	if len(diagnostics) > 0 {
		for _, diagnostic := range diagnostics {
			out.Issues = append(out.Issues, issue(CodeCondition, diagnostic.Path, diagnostic.Message, nil))
		}
		out.Issues = append(out.Issues, Validate(form, parsed.Values, options...)...)
		return out
	}
	cfg := applyOptions(options)
	projection := program.Evaluate(map[string]any(parsed.Values), cfg.inactive...)
	out.Values = Values(projection.Values)
	out.Retained = projection.Retained
	out.Applicable = projection.Applicable
	out.Required = projection.Required
	var inactive []string
	for path, active := range projection.Applicable {
		if !active {
			inactive = append(inactive, path)
		}
	}
	sort.Strings(inactive)
	for _, path := range inactive {
		if policy == InactiveReject && suppliedInactive(map[string]any(parsed.Values), path) {
			out.Issues = append(out.Issues, issue(CodeInactiveInput, path, "input is not applicable", nil))
		}
	}
	options = append(options, func(opts *Options) { opts.required = projection.Required; opts.inactive = inactive })
	out.Issues = append(out.Issues, Validate(form, out.Values, options...)...)
	out.Issues = append(out.Issues, missingConditionalIssues(form, out, options)...)
	return out
}

func suppliedInactive(values map[string]any, path string) bool {
	value, supplied := fieldpath.Get(values, path)
	// Indexed nulls preserve fixed coordinates after clear; they carry no answer.
	return supplied && (value != nil || !strings.HasSuffix(path, "]"))
}

// An optional absent container must not hide a conditionally required child.
func missingConditionalIssues(form model.FormModel, out ConditionalResult, options []Option) []Issue {
	var issues []Issue
	paths := make([]string, 0, len(out.Required))
	for path, required := range out.Required {
		if required {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, exists := fieldpath.Get(map[string]any(out.Values), path); exists {
			continue
		}
		already := false
		for _, item := range out.Issues {
			if item.Path == path && item.Code == CodeRequired {
				already = true
				break
			}
		}
		if already {
			continue
		}
		issues = append(issues, ValidatePaths(form, out.Values, []string{path}, options...)...)
	}
	return issues
}
