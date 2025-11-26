package visibility

// Evaluator determines whether a field should be visible based on a rule
// string and optional context such as current values or scope metadata.
type Evaluator interface {
	Eval(fieldPath, rule string, ctx Context) (bool, error)
}

// Context provides inputs to a VisibilityEvaluator. Values typically comes from
// render options (prefilled values) while Extras allows callers to inject
// arbitrary context such as user roles or feature flags.
type Context struct {
	Values map[string]any
	Extras map[string]any
}

// EvaluatorFunc adapts a function into a VisibilityEvaluator.
type EvaluatorFunc func(fieldPath, rule string, ctx Context) (bool, error)

// Eval delegates to the underlying function.
func (fn EvaluatorFunc) Eval(fieldPath, rule string, ctx Context) (bool, error) {
	return fn(fieldPath, rule, ctx)
}
