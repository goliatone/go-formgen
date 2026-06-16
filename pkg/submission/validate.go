package submission

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	if missingValue(field, value, exists) {
		if field.Required {
			return []Issue{issue(CodeRequired, path, makeMessage(field, path, "is required"), nil)}
		}
		return nil
	}
	if isEmpty(value) {
		if field.Required {
			return []Issue{issue(CodeRequired, path, makeMessage(field, path, "is required"), value)}
		}
		return nil
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
	return !exists && !isEmpty(value)
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
	issues := validateArrayRules(field, items, path)
	issues = append(issues, validateArrayEnum(field, items, path)...)
	issues = append(issues, validateArrayItems(field, items, path, opts)...)
	return issues
}

func validateArrayEnum(field model.Field, items []any, path string) []Issue {
	if len(field.Enum) == 0 {
		return nil
	}
	var issues []Issue
	for i, item := range items {
		issues = append(issues, validateEnum(field, item, fmt.Sprintf("%s[%d]", path, i))...)
	}
	return issues
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
			if limit, ok := ruleInt(rule); ok && len(value) < limit {
				issues = append(issues, issue(CodeMinLength, path, makeMessage(field, path, fmt.Sprintf("must be at least %d characters", limit)), value))
			}
		case model.ValidationRuleMaxLength:
			if limit, ok := ruleInt(rule); ok && len(value) > limit {
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
		i, err := v.Int64()
		return i, err == nil
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
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
