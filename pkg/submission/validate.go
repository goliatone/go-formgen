package submission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goliatone/go-formgen/internal/fieldpath"

	"github.com/goliatone/go-formgen/pkg/model"
)

// Validate checks parsed values against FormModel field constraints.
func Validate(form model.FormModel, values Values, options ...Option) []Issue {
	opts := applyOptions(options)
	var issues []Issue
	for _, field := range form.Fields {
		value, exists := values[field.Name]
		issues = append(issues, validateField(field, value, exists, field.Name, opts)...)
	}
	return issues
}

func validateField(field model.Field, value any, exists bool, path string, opts Options) []Issue {
	if inactivePath(path, opts) {
		return nil
	}
	if opts.required[path] {
		field.Required = true
	}
	if missingValue(field, value, exists) {
		if field.Required {
			return []Issue{issue(CodeRequired, path, makeMessage(field, path, "is required"), nil)}
		}
		return nil
	}
	if isEmpty(value) {
		if field.Type != model.FieldTypeArray {
			if field.Required {
				return []Issue{issue(CodeRequired, path, makeMessage(field, path, "is required"), value)}
			}
			return nil
		}
	}

	switch field.Type {
	case model.FieldTypeString:
		return validateStringField(field, value, path)
	case model.FieldTypeInteger:
		return validateIntegerField(field, value, path)
	case model.FieldTypeNumber:
		return validateNumberField(field, value, path)
	case model.FieldTypeBoolean:
		return validateBooleanField(field, value, path)
	case model.FieldTypeArray:
		return validateArrayField(field, value, path, opts)
	case model.FieldTypeObject:
		return validateObjectField(field, value, path, opts)
	}
	return nil
}

func missingValue(field model.Field, value any, exists bool) bool {
	return !exists || value == nil
}

func validateStringField(field model.Field, value any, path string) []Issue {
	text, ok := value.(string)
	if !ok {
		return []Issue{issue(CodeType, path, makeMessage(field, path, "must be a string"), value)}
	}
	var issues []Issue
	issues = append(issues, validateEnum(field, text, path)...)
	issues = append(issues, validateStringRules(field, text, path)...)
	return issues
}

func validateIntegerField(field model.Field, value any, path string) []Issue {
	num, ok := integerValue(value)
	if !ok {
		return []Issue{issue(CodeType, path, makeMessage(field, path, "must be an integer"), value)}
	}
	var issues []Issue
	issues = append(issues, validateEnum(field, num, path)...)
	issues = append(issues, validateNumberRules(field, float64(num), path)...)
	return issues
}

func validateNumberField(field model.Field, value any, path string) []Issue {
	num, ok := numberValue(value)
	if !ok {
		return []Issue{issue(CodeType, path, makeMessage(field, path, "must be a number"), value)}
	}
	var issues []Issue
	issues = append(issues, validateEnum(field, num, path)...)
	issues = append(issues, validateNumberRules(field, num, path)...)
	return issues
}

func validateBooleanField(field model.Field, value any, path string) []Issue {
	boolean, ok := value.(bool)
	if !ok {
		return []Issue{issue(CodeType, path, makeMessage(field, path, "must be a boolean"), value)}
	}
	return validateEnum(field, boolean, path)
}

func validateArrayField(field model.Field, value any, path string, opts Options) []Issue {
	items, ok := value.([]any)
	if !ok {
		return []Issue{issue(CodeType, path, makeMessage(field, path, "must be an array"), value)}
	}
	effective := effectiveArrayItems(items, path, opts)
	issues := validateArrayRules(field, effective, path)
	if len(effective) == 0 && field.Required && len(issues) == 0 {
		issues = append(issues, issue(CodeRequired, path, makeMessage(field, path, "is required"), value))
	}
	issues = append(issues, validateArrayEnum(field, items, path, opts)...)
	issues = append(issues, validateArrayItems(field, items, path, opts)...)
	return issues
}

func validateArrayEnum(field model.Field, items []any, path string, opts Options) []Issue {
	if len(field.Enum) == 0 {
		return nil
	}
	var issues []Issue
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if !inactivePath(itemPath, opts) {
			issues = append(issues, validateEnum(field, item, itemPath)...)
		}
	}
	return issues
}

func inactivePath(path string, opts Options) bool {
	for _, inactive := range opts.inactive {
		if fieldpath.Within(path, inactive) {
			return true
		}
	}
	return false
}

// Effective constraints count applicable members, while item issues and condition
// references keep the original fixed indexes. Structural null holes are not answers.
func effectiveArrayItems(items []any, path string, opts Options) []any {
	if len(opts.inactive) == 0 {
		return items
	}
	effective := make([]any, 0, len(items))
	for i, item := range items {
		if !inactivePath(fmt.Sprintf("%s[%d]", path, i), opts) {
			effective = append(effective, item)
		}
	}
	return effective
}

func validateArrayItems(field model.Field, items []any, path string, opts Options) []Issue {
	if field.Items == nil {
		return nil
	}
	var issues []Issue
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		issues = append(issues, validateField(*field.Items, item, true, itemPath, opts)...)
	}
	return issues
}

func validateObjectField(field model.Field, value any, path string, opts Options) []Issue {
	obj, ok := value.(map[string]any)
	if !ok {
		return []Issue{issue(CodeObject, path, makeMessage(field, path, "must be an object"), value)}
	}
	if IsRawObjectField(field) {
		return nil
	}
	issues := validateNestedFields(field, obj, path, opts)
	if opts.UnknownFields == UnknownIssue {
		issues = append(issues, validateUnknownObjectFields(field, obj, path)...)
	}
	return issues
}

func validateNestedFields(field model.Field, obj map[string]any, path string, opts Options) []Issue {
	var issues []Issue
	for _, child := range field.Nested {
		childValue, childExists := obj[child.Name]
		issues = append(issues, validateField(child, childValue, childExists, joinPath(path, child.Name), opts)...)
	}
	return issues
}

func validateUnknownObjectFields(field model.Field, obj map[string]any, path string) []Issue {
	known := make(map[string]struct{}, len(field.Nested))
	for _, child := range field.Nested {
		known[child.Name] = struct{}{}
	}
	var issues []Issue
	for key, item := range obj {
		if _, ok := known[key]; ok {
			continue
		}
		unknownPath := joinPath(path, key)
		issues = append(issues, issue(CodeUnknownField, unknownPath, fmt.Sprintf("unknown field %q", unknownPath), item))
	}
	return issues
}

func validateEnum(field model.Field, value any, path string) []Issue {
	if len(field.Enum) == 0 || value == nil {
		return nil
	}
	for _, candidate := range field.Enum {
		if enumValueEqual(candidate, value) {
			return nil
		}
	}
	return []Issue{issue(CodeEnum, path, makeMessage(field, path, "must be one of the allowed values"), value)}
}

func validateStringRules(field model.Field, value string, path string) []Issue {
	var issues []Issue
	for _, rule := range field.Validations {
		switch rule.Kind {
		case model.ValidationRuleMinLength:
			if limit, ok := ruleInt(rule); ok && utf8.RuneCountInString(value) < limit {
				issues = append(issues, issue(CodeMinLength, path, makeMessage(field, path, fmt.Sprintf("must be at least %d characters", limit)), value))
			}
		case model.ValidationRuleMaxLength:
			if limit, ok := ruleInt(rule); ok && utf8.RuneCountInString(value) > limit {
				issues = append(issues, issue(CodeMaxLength, path, makeMessage(field, path, fmt.Sprintf("must be at most %d characters", limit)), value))
			}
		case model.ValidationRulePattern:
			pattern := strings.TrimSpace(rule.Params["pattern"])
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err == nil && !re.MatchString(value) {
				issues = append(issues, issue(CodePattern, path, makeMessage(field, path, "does not match the required pattern"), value))
			}
		}
	}
	return issues
}

func validateNumberRules(field model.Field, value float64, path string) []Issue {
	var issues []Issue
	for _, rule := range field.Validations {
		switch rule.Kind {
		case model.ValidationRuleMin:
			if limit, ok := ruleFloat(rule); ok {
				exclusive := strings.EqualFold(rule.Params["exclusive"], "true")
				if (!exclusive && value < limit) || (exclusive && value <= limit) {
					issues = append(issues, issue(CodeMin, path, makeMessage(field, path, fmt.Sprintf("must be at least %v", limit)), value))
				}
			}
		case model.ValidationRuleMax:
			if limit, ok := ruleFloat(rule); ok {
				exclusive := strings.EqualFold(rule.Params["exclusive"], "true")
				if (!exclusive && value > limit) || (exclusive && value >= limit) {
					issues = append(issues, issue(CodeMax, path, makeMessage(field, path, fmt.Sprintf("must be at most %v", limit)), value))
				}
			}
		}
	}
	return issues
}

func validateArrayRules(field model.Field, value []any, path string) []Issue {
	var issues []Issue
	for _, rule := range field.Validations {
		switch rule.Kind {
		case model.ValidationRuleUniqueItems:
			if hasDuplicateItems(value) {
				issues = append(issues, issue(CodeUniqueItems, path, makeMessage(field, path, "must contain distinct items"), nil))
			}
		case model.ValidationRuleMinItems, model.ValidationRuleMinLength:
			if limit, ok := ruleInt(rule); ok && len(value) < limit {
				issues = append(issues, issue(CodeMinItems, path, makeMessage(field, path, fmt.Sprintf("must contain at least %d items", limit)), value))
			}
		case model.ValidationRuleMaxItems, model.ValidationRuleMaxLength:
			if limit, ok := ruleInt(rule); ok && len(value) > limit {
				issues = append(issues, issue(CodeMaxItems, path, makeMessage(field, path, fmt.Sprintf("must contain at most %d items", limit)), value))
			}
		}
	}
	return issues
}

// hasDuplicateItems reports whether two items share one JSON structure. Each
// item is reduced once to a canonical key, so the outcome never depends on the
// Go container a parser or host supplied. Numeric work depends on the encoded
// digits, never on the magnitude of a decimal power. Items encoding/json rejects
// (NaN, invalid number literals, cyclic containers) have no key and compare
// among themselves with reflect.DeepEqual, which tolerates cycles.
func hasDuplicateItems(items []any) bool {
	seen := make(map[string]struct{}, len(items))
	var opaque []any
	for _, item := range items {
		key, ok := canonicalJSONKey(item)
		if !ok {
			for _, earlier := range opaque {
				if reflect.DeepEqual(item, earlier) {
					return true
				}
			}
			opaque = append(opaque, item)
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

// canonicalJSONKey encodes a value the way ParseJSON would decode it: the
// encoding/json output is read back with UseNumber, every number is reduced to
// one lexeme per mathematical value, and the result is re-encoded with sorted
// object keys. Typed slices, arrays, maps, pointers, structs and Marshaler
// values inside accepted raw objects therefore key like their untyped
// counterparts, integer digits and json.Number text stay exact, floats take
// their shortest form, and nothing rounds through float64.
func canonicalJSONKey(value any) (string, bool) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	var decoded any
	if err = strictDecodeJSON(bytes.NewReader(body), &decoded); err != nil {
		return "", false
	}
	key, err := json.Marshal(normalizeJSONNumbers(decoded))
	if err != nil {
		return "", false
	}
	return string(key), true
}

func normalizeJSONNumbers(value any) any {
	switch v := value.(type) {
	case json.Number:
		return canonicalNumber(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeJSONNumbers(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeJSONNumbers(item)
		}
		return out
	default:
		return value
	}
}

// canonicalNumber rewrites a JSON number literal as sign, digits without
// leading or trailing zeros, and a decimal exponent (`-15e-1` for -1.50), so
// equal mathematical values share one lexeme. Only the exponent is adjusted
// with integer arithmetic; the represented decimal power is never expanded.
func canonicalNumber(number json.Number) json.Number {
	text := strings.TrimPrefix(string(number), "-")
	mantissa, exponent, _ := strings.Cut(strings.ToLower(text), "e")
	var scale big.Int
	if exponent != "" {
		if _, ok := scale.SetString(exponent, 10); !ok {
			return number
		}
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := whole + fraction
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return number
	}
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "0"
	}
	trimmed := strings.TrimRight(digits, "0")
	shift := int64(len(digits)-len(trimmed)) - int64(len(fraction))
	scale.Add(&scale, big.NewInt(shift))
	sign := ""
	if strings.HasPrefix(string(number), "-") {
		sign = "-"
	}
	return json.Number(sign + trimmed + "e" + scale.String())
}

// ValidatePaths validates only selected canonical field subtrees. An empty slice
// selects nothing. It never filters or mutates parsed values or parse issues.
func ValidatePaths(form model.FormModel, values Values, paths []string, options ...Option) []Issue {
	var issues []Issue
	valid := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, err := fieldpath.Parse(path); err != nil {
			issues = append(issues, issue(CodeInvalidPath, path, "invalid canonical field path", nil))
			continue
		}
		if _, ok := fieldpath.Lookup(form, path); !ok {
			issues = append(issues, issue(CodeUnknownField, path, "unknown validation field", nil))
			continue
		}
		valid = append(valid, path)
	}
	opts := applyOptions(options)
	for i, path := range valid {
		covered := false
		for j, other := range valid {
			if i != j && fieldpath.Within(path, other) && (path != other || j < i) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		if inactivePath(path, opts) {
			continue
		}
		resolved, _ := fieldpath.Resolve(form, path)
		value, exists := fieldpath.Get(map[string]any(values), path)
		// Full array validation applies both the container's per-element enum
		// and Items constraints. Preserve both when selecting one indexed item.
		if resolved.Array != nil && resolved.Array.Items != nil {
			issues = append(issues, validateEnum(*resolved.Array, value, path)...)
		}
		issues = append(issues, validateField(resolved.Field, value, exists, path, opts)...)
	}
	return issues
}

func ruleFloat(rule model.ValidationRule) (float64, bool) {
	if rule.Params == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(rule.Params["value"], 64)
	return value, err == nil
}

func ruleInt(rule model.ValidationRule) (int, bool) {
	if rule.Params == nil {
		return 0, false
	}
	value, err := strconv.Atoi(rule.Params["value"])
	return value, err == nil
}

func isEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

func integerValue(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		return integralJSONNumber(v)
	default:
		return 0, false
	}
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, finiteNumber(v)
	case json.Number:
		f, err := v.Float64()
		return f, err == nil && finiteNumber(f)
	default:
		return 0, false
	}
}
