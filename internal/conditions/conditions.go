// Package conditions implements the renderer-free embedded condition contract.
package conditions

import (
	"encoding/json"
	"math/big"
	"reflect"
	"sort"

	"github.com/goliatone/go-formgen/internal/fieldpath"
	"github.com/goliatone/go-formgen/internal/model"
)

type compiledRule struct {
	rule     model.ConditionRule
	required bool
}

// Program is an immutable, validated evaluation plan. It is safe for concurrent use.
type Program struct{ rules []compiledRule }

// Projection owns independent values. Retained is keyed by precise canonical path.
type Projection struct {
	Values     map[string]any  `json:"values"`
	Retained   map[string]any  `json:"retained"`
	Applicable map[string]bool `json:"applicable"`
	Required   map[string]bool `json:"required"`
}

// Compile validates references, operand shapes, versions and dependency cycles.
// The input remains authoritative and is never filtered or mutated.
func Compile(form model.FormModel) (*Program, []model.DefinitionDiagnostic) {
	if form.Conditions == nil {
		return &Program{}, nil
	}
	c := compiler{form: form, rules: map[string]model.ConditionRule{}, refs: map[string][]string{}}
	if form.Conditions.Version != model.ConditionsVersion {
		c.add("conditionVersion", "conditions.version", "unsupported condition contract version")
		return nil, c.diagnostics
	}
	for _, rule := range form.Conditions.Rules {
		c.collectRule(rule)
	}
	if len(c.diagnostics) > 0 {
		return nil, c.diagnostics
	}
	rules := c.orderedRules()
	if len(c.diagnostics) > 0 {
		return nil, c.diagnostics
	}
	return &Program{rules: rules}, nil
}

type compiler struct {
	form        model.FormModel
	rules       map[string]model.ConditionRule
	refs        map[string][]string
	diagnostics []model.DefinitionDiagnostic
}

func (c *compiler) add(code, path, message string) {
	c.diagnostics = append(c.diagnostics, model.DefinitionDiagnostic{Code: code, Path: path, Message: message})
}
func (c *compiler) collectRule(rule model.ConditionRule) {
	path := rule.Path
	if _, ok := fieldpath.Lookup(c.form, path); !ok {
		c.add("conditionPath", path, "condition target must be a known canonical field path")
	}
	if _, exists := c.rules[path]; exists {
		c.add("conditionDuplicate", path, "only one condition rule is allowed per path")
	}
	if rule.OnInapplicable != "" && rule.OnInapplicable != "clear" && rule.OnInapplicable != "retain" {
		c.add("conditionPolicy", path, "onInapplicable must be clear or retain")
	}
	if rule.ApplicableWhen == nil && rule.RequiredWhen == nil {
		c.add("conditionExpression", path, "condition rule requires an expression")
	}
	for _, expr := range []*model.Expression{rule.ApplicableWhen, rule.RequiredWhen} {
		if expr != nil {
			c.checkExpression(*expr, path, 0)
		}
	}
	// Invalid trees are never cloned or evaluated.
	if len(c.diagnostics) == 0 {
		c.rules[path] = cloneRule(rule)
	} else {
		c.rules[path] = rule
	}
}
func (c *compiler) dependencies(paths []string) map[string]map[string]bool {
	deps := map[string]map[string]bool{}
	for _, path := range paths {
		deps[path] = map[string]bool{}
		for _, gate := range paths {
			if gate != path && fieldpath.Within(path, gate) {
				deps[path][gate] = true
			}
			for _, ref := range c.refs[path] {
				if fieldpath.Within(ref, gate) || fieldpath.Within(gate, ref) {
					deps[path][gate] = true
				}
			}
		}
	}
	return deps
}
func dependenciesResolved(deps, visited map[string]bool) bool {
	for dep := range deps {
		if !visited[dep] {
			return false
		}
	}
	return true
}
func (c *compiler) orderedRules() []compiledRule {
	paths := make([]string, 0, len(c.rules))
	for path := range c.rules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	deps := c.dependencies(paths)
	visited := map[string]bool{}
	var out []compiledRule
	for len(visited) < len(paths) {
		progressed := false
		for _, path := range paths {
			if visited[path] || !dependenciesResolved(deps[path], visited) {
				continue
			}
			field, _ := fieldpath.Lookup(c.form, path)
			out = append(out, compiledRule{rule: c.rules[path], required: field.Required})
			visited[path] = true
			progressed = true
		}
		if !progressed {
			c.reportCycles(paths, visited)
			break
		}
	}
	return out
}
func (c *compiler) reportCycles(paths []string, visited map[string]bool) {
	for _, path := range paths {
		if !visited[path] {
			c.add("conditionCycle", path, "cyclic condition or parent dependency")
		}
	}
}
func (c *compiler) checkExpression(e model.Expression, target string, depth int) {
	if depth > 64 {
		c.add("conditionExpression", target, "condition nesting exceeds 64 levels")
		return
	}
	switch e.Op {
	case "present", "eq":
		c.checkLeaf(e, target)
	case "and", "or":
		if len(e.Args) == 0 || e.Path != "" || e.Value != nil {
			c.add("conditionExpression", target, "and/or require nonempty args and no path or value")
		}
		for _, arg := range e.Args {
			c.checkExpression(arg, target, depth+1)
		}
	default:
		c.add("conditionOperator", target, "unsupported condition operator")
	}
}
func (c *compiler) checkLeaf(e model.Expression, target string) {
	field, ok := fieldpath.Lookup(c.form, e.Path)
	if !ok {
		c.add("conditionReference", target, "condition reference must be a known canonical field path")
	}
	c.refs[target] = append(c.refs[target], e.Path)
	if len(e.Args) > 0 {
		c.add("conditionExpression", target, "leaf expressions cannot contain args")
	}
	if e.Op == "present" && e.Value != nil {
		c.add("conditionExpression", target, "present cannot contain a value")
	}
	if e.Op != "eq" {
		return
	}
	if !scalar(e.Value) {
		c.add("conditionExpression", target, "eq requires a non-null scalar literal")
	}
	if ok && (field.Type == model.FieldTypeArray || field.Type == model.FieldTypeObject) {
		c.add("conditionExpression", target, "eq references must be scalar fields")
	}
}
func scalar(value any) bool {
	switch value.(type) {
	case string, bool:
		return true
	}
	_, ok := number(value)
	return ok
}
func cloneRule(rule model.ConditionRule) model.ConditionRule {
	rule.ApplicableWhen = cloneExpression(rule.ApplicableWhen)
	rule.RequiredWhen = cloneExpression(rule.RequiredWhen)
	return rule
}
func cloneExpression(expr *model.Expression) *model.Expression {
	if expr == nil {
		return nil
	}
	out := *expr
	out.Value = fieldpath.Clone(expr.Value)
	if expr.Args != nil {
		out.Args = make([]model.Expression, len(expr.Args))
		for i := range expr.Args {
			out.Args[i] = *cloneExpression(&expr.Args[i])
		}
	}
	return &out
}

// Evaluate projects already normalized typed JSON values. Inactive gates are
// removed before downstream rules read values; retained inputs never become effective.
// inactivePaths supplies trusted structural exclusions such as deleted row
// coordinates. They remain null holes, gate descendants and never enter retained
// values. Hosts derive these from validated row intents, not authorization data.
func (p *Program) Evaluate(values map[string]any, inactivePaths ...string) Projection {
	out := Projection{Values: map[string]any{}, Retained: map[string]any{}, Applicable: map[string]bool{}, Required: map[string]bool{}}
	if values != nil {
		out.Values = fieldpath.Clone(values).(map[string]any)
	}
	applyInactivePaths(&out, inactivePaths)
	retentionSource := values
	if len(inactivePaths) > 0 {
		// Retained ancestors cannot reintroduce excluded descendants.
		retentionSource = fieldpath.Clone(out.Values).(map[string]any)
	}
	if p == nil {
		return out
	}
	for _, entry := range p.rules {
		rule := entry.rule
		active := ruleApplicable(rule, out)
		out.Applicable[rule.Path] = active
		out.Required[rule.Path] = active && (entry.required || (rule.RequiredWhen != nil && evaluate(*rule.RequiredWhen, out.Values)))
		if !active {
			if rule.OnInapplicable == "retain" && !withinAnyPath(rule.Path, inactivePaths) {
				if value, ok := fieldpath.Get(retentionSource, rule.Path); ok {
					out.Retained[rule.Path] = fieldpath.Clone(value)
				}
			}
			fieldpath.Remove(out.Values, rule.Path)
		}
	}
	return out
}

func applyInactivePaths(out *Projection, paths []string) {
	for _, path := range paths {
		if _, err := fieldpath.Parse(path); err != nil {
			continue
		}
		out.Applicable[path] = false
		fieldpath.Remove(out.Values, path)
	}
}

func withinAnyPath(path string, parents []string) bool {
	for _, parent := range parents {
		if fieldpath.Within(path, parent) {
			return true
		}
	}
	return false
}

func ruleApplicable(rule model.ConditionRule, out Projection) bool {
	if !fieldpath.IndexesExist(out.Values, rule.Path) {
		return false
	}
	for parent, active := range out.Applicable {
		if !active && fieldpath.Within(rule.Path, parent) {
			return false
		}
	}
	return rule.ApplicableWhen == nil || evaluate(*rule.ApplicableWhen, out.Values)
}
func evaluate(e model.Expression, values map[string]any) bool {
	switch e.Op {
	case "present":
		value, ok := fieldpath.Get(values, e.Path)
		return ok && value != nil
	case "eq":
		value, ok := fieldpath.Get(values, e.Path)
		return ok && value != nil && equal(value, e.Value)
	case "and":
		for _, arg := range e.Args {
			if !evaluate(arg, values) {
				return false
			}
		}
		return true
	case "or":
		for _, arg := range e.Args {
			if evaluate(arg, values) {
				return true
			}
		}
		return false
	}
	return false
}
func equal(a, b any) bool {
	if ar, ok := number(a); ok {
		br, ok := number(b)
		return ok && ar.Cmp(br) == 0
	}
	return reflect.DeepEqual(a, b)
}
func number(value any) (*big.Rat, bool) {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		body, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		return new(big.Rat).SetString(string(body))
	}
	return nil, false
}
