package jsonschema

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/goliatone/go-formgen/pkg/schema"
)

var supportedSchemaKeys = map[string]struct{}{
	"$schema":          {},
	"$id":              {},
	"$defs":            {},
	"$ref":             {},
	"$anchor":          {},
	"type":             {},
	"properties":       {},
	"required":         {},
	"items":            {},
	"oneOf":            {},
	"anyOf":            {},
	"enum":             {},
	"const":            {},
	"title":            {},
	"description":      {},
	"default":          {},
	"readOnly":         {},
	"read_only":        {},
	"minimum":          {},
	"maximum":          {},
	"exclusiveMinimum": {},
	"exclusiveMaximum": {},
	"minLength":        {},
	"maxLength":        {},
	"minItems":         {},
	"maxItems":         {},
	"pattern":          {},
	"format":           {},
}

// schemaFromJSONSchema converts a JSON Schema payload into the canonical schema tree.
func schemaFromJSONSchema(node any, path string) (schema.Schema, error) {
	return schemaFromJSONSchemaWithContext(node, path, normalizeContext{})
}

type normalizeContext struct {
	allowOneOf           bool
	requireDiscriminator bool
}

func (ctx normalizeContext) forItems() normalizeContext {
	return normalizeContext{allowOneOf: true}
}

func (ctx normalizeContext) forOneOfVariant() normalizeContext {
	return normalizeContext{requireDiscriminator: true}
}

func (ctx normalizeContext) forChild() normalizeContext {
	return normalizeContext{}
}

func schemaFromJSONSchemaWithContext(node any, path string, ctx normalizeContext) (schema.Schema, error) {
	if node == nil {
		return schema.Schema{}, fmt.Errorf("jsonschema: schema is nil at %s", path)
	}
	payload, ok := node.(map[string]any)
	if !ok {
		return schema.Schema{}, fmt.Errorf("jsonschema: schema must be an object at %s", path)
	}

	if ref := strings.TrimSpace(readString(payload, "$ref")); ref != "" {
		return schema.Schema{}, fmt.Errorf("jsonschema: unresolved $ref %q at %s", ref, path)
	}

	extensions := extractExtensions(payload)
	if err := validateKeywords(payload, path); err != nil {
		return schema.Schema{}, err
	}

	schemaType, err := inferSchemaType(payload, path)
	if err != nil {
		return schema.Schema{}, err
	}
	readOnly, err := readOnlyAnnotation(payload, path)
	if err != nil {
		return schema.Schema{}, err
	}

	out := schema.Schema{
		Type:        schemaType,
		Title:       strings.TrimSpace(readString(payload, "title")),
		Description: strings.TrimSpace(readString(payload, "description")),
		Default:     payload["default"],
		ReadOnly:    readOnly,
		Const:       payload["const"],
		Format:      strings.TrimSpace(readString(payload, "format")),
		Extensions:  extensions,
	}

	if err := applyScalarSchemaKeywords(&out, payload, path); err != nil {
		return schema.Schema{}, err
	}

	childCtx := ctx.forChild()

	if err := validateDefs(payload, path, childCtx); err != nil {
		return schema.Schema{}, err
	}
	if err := applyProperties(&out, payload, path, childCtx); err != nil {
		return schema.Schema{}, err
	}
	if err := applyItems(&out, payload, path, ctx.forItems()); err != nil {
		return schema.Schema{}, err
	}
	if err := applyOneOf(&out, payload, path, ctx); err != nil {
		return schema.Schema{}, err
	}
	if err := applyAnyOf(&out, payload, path, ctx); err != nil {
		return schema.Schema{}, err
	}

	if err := applyDiscriminatorRules(&out, path, ctx.requireDiscriminator); err != nil {
		return schema.Schema{}, err
	}

	if err := enforceBlockWidget(out, path); err != nil {
		return schema.Schema{}, err
	}

	return out, nil
}

func inferSchemaType(payload map[string]any, path string) (string, error) {
	schemaType, _, err := parseType(payload, path)
	if err != nil {
		return "", err
	}
	if schemaType != "" {
		return schemaType, nil
	}
	if _, ok := payload["items"]; ok {
		return "array", nil
	}
	if _, ok := payload["properties"]; ok {
		return "object", nil
	}
	return "", nil
}

func applyScalarSchemaKeywords(out *schema.Schema, payload map[string]any, path string) error {
	if err := applyEnum(out, payload, path); err != nil {
		return err
	}
	if err := applyRequired(out, payload, path); err != nil {
		return err
	}
	if err := applyNumericKeywords(out, payload, path); err != nil {
		return err
	}
	if err := applyStringKeywords(out, payload, path); err != nil {
		return err
	}
	return applyArrayKeywords(out, payload, path)
}

func applyEnum(out *schema.Schema, payload map[string]any, path string) error {
	enumRaw, ok := payload["enum"]
	if !ok {
		return nil
	}
	enumList, ok := enumRaw.([]any)
	if !ok {
		return fmt.Errorf("jsonschema: enum must be an array at %s", path)
	}
	out.Enum = append([]any(nil), enumList...)
	return nil
}

func applyRequired(out *schema.Schema, payload map[string]any, path string) error {
	requiredRaw, ok := payload["required"]
	if !ok {
		return nil
	}
	list, ok := requiredRaw.([]any)
	if !ok {
		return fmt.Errorf("jsonschema: required must be an array at %s", path)
	}
	required := make([]string, 0, len(list))
	for idx, item := range list {
		str, ok := item.(string)
		if !ok || strings.TrimSpace(str) == "" {
			return fmt.Errorf("jsonschema: required[%d] must be a string at %s", idx, path)
		}
		required = append(required, str)
	}
	out.Required = required
	return nil
}

func applyNumericKeywords(out *schema.Schema, payload map[string]any, path string) error {
	if err := applyNumberBound(&out.Minimum, payload, "minimum", path); err != nil {
		return err
	}
	if err := applyNumberBound(&out.Maximum, payload, "maximum", path); err != nil {
		return err
	}
	if err := applyExclusiveNumberBound(out, payload, "exclusiveMinimum", path); err != nil {
		return err
	}
	return applyExclusiveNumberBound(out, payload, "exclusiveMaximum", path)
}

func applyNumberBound(target **float64, payload map[string]any, key, path string) error {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	value, ok := toFloat(raw)
	if !ok {
		return fmt.Errorf("jsonschema: %s must be a number at %s", key, path)
	}
	*target = &value
	return nil
}

func applyExclusiveNumberBound(out *schema.Schema, payload map[string]any, key, path string) error {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	if value, isBool := raw.(bool); isBool {
		setExclusiveFlag(out, key, value)
		return nil
	}
	number, ok := toFloat(raw)
	if !ok {
		return fmt.Errorf("jsonschema: %s must be a number at %s", key, path)
	}
	return setExclusiveNumber(out, key, number, path)
}

func setExclusiveFlag(out *schema.Schema, key string, value bool) {
	if key == "exclusiveMinimum" {
		out.ExclusiveMinimum = value
		return
	}
	out.ExclusiveMaximum = value
}

func setExclusiveNumber(out *schema.Schema, key string, number float64, path string) error {
	if key == "exclusiveMinimum" {
		if out.Minimum != nil {
			return fmt.Errorf("jsonschema: minimum conflicts with exclusiveMinimum at %s", path)
		}
		out.Minimum = &number
		out.ExclusiveMinimum = true
		return nil
	}
	if out.Maximum != nil {
		return fmt.Errorf("jsonschema: maximum conflicts with exclusiveMaximum at %s", path)
	}
	out.Maximum = &number
	out.ExclusiveMaximum = true
	return nil
}

func applyStringKeywords(out *schema.Schema, payload map[string]any, path string) error {
	if err := applyLengthBound(&out.MinLength, payload, "minLength", path); err != nil {
		return err
	}
	if err := applyLengthBound(&out.MaxLength, payload, "maxLength", path); err != nil {
		return err
	}
	patternRaw, ok := payload["pattern"]
	if !ok {
		return nil
	}
	pattern, ok := patternRaw.(string)
	if !ok {
		return fmt.Errorf("jsonschema: pattern must be a string at %s", path)
	}
	out.Pattern = pattern
	return nil
}

func applyLengthBound(target **int, payload map[string]any, key, path string) error {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	value, ok := toInt(raw)
	if !ok {
		return fmt.Errorf("jsonschema: %s must be an integer at %s", key, path)
	}
	*target = &value
	return nil
}

func applyArrayKeywords(out *schema.Schema, payload map[string]any, path string) error {
	if _, hasMin := payload["minItems"]; hasMin && out.Type != "array" {
		return fmt.Errorf("jsonschema: minItems is only supported on arrays at %s", path)
	}
	if _, hasMax := payload["maxItems"]; hasMax && out.Type != "array" {
		return fmt.Errorf("jsonschema: maxItems is only supported on arrays at %s", path)
	}
	if err := applyItemBound(&out.MinItems, payload, "minItems", path); err != nil {
		return err
	}
	if err := applyItemBound(&out.MaxItems, payload, "maxItems", path); err != nil {
		return err
	}
	if out.MinItems != nil && out.MaxItems != nil && *out.MinItems > *out.MaxItems {
		return fmt.Errorf("jsonschema: minItems exceeds maxItems at %s", path)
	}
	return nil
}

func applyItemBound(target **int, payload map[string]any, key, path string) error {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	value, ok := toInt(raw)
	if !ok {
		return fmt.Errorf("jsonschema: %s must be an integer at %s", key, path)
	}
	if value < 0 {
		return fmt.Errorf("jsonschema: %s must be non-negative at %s", key, path)
	}
	*target = &value
	return nil
}

func validateDefs(payload map[string]any, path string, ctx normalizeContext) error {
	defsRaw, ok := payload["$defs"]
	if !ok {
		return nil
	}
	defs, ok := defsRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("jsonschema: $defs must be an object at %s", path)
	}
	for _, key := range sortedKeys(defs) {
		childPath := joinPath(path, "$defs", key)
		if _, err := schemaFromJSONSchemaWithContext(defs[key], childPath, ctx); err != nil {
			return err
		}
	}
	return nil
}

func applyProperties(out *schema.Schema, payload map[string]any, path string, ctx normalizeContext) error {
	propertiesRaw, ok := payload["properties"]
	if !ok {
		return nil
	}
	props, ok := propertiesRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("jsonschema: properties must be an object at %s", path)
	}
	out.Properties = make(map[string]schema.Schema, len(props))
	nullableProps := make(map[string]struct{})
	for _, key := range sortedKeys(props) {
		if hasNullableSchema(props[key]) {
			nullableProps[key] = struct{}{}
		}
		childPath := joinPath(path, "properties", key)
		converted, err := schemaFromJSONSchemaWithContext(props[key], childPath, ctx)
		if err != nil {
			return err
		}
		out.Properties[key] = converted
	}
	removeNullableRequired(out, nullableProps)
	return nil
}

func removeNullableRequired(out *schema.Schema, nullableProps map[string]struct{}) {
	if len(nullableProps) == 0 || len(out.Required) == 0 {
		return
	}
	filtered := out.Required[:0]
	for _, entry := range out.Required {
		if _, ok := nullableProps[entry]; !ok {
			filtered = append(filtered, entry)
		}
	}
	out.Required = filtered
	if len(out.Required) == 0 {
		out.Required = nil
	}
}

func applyItems(out *schema.Schema, payload map[string]any, path string, ctx normalizeContext) error {
	itemsRaw, ok := payload["items"]
	if !ok {
		return nil
	}
	typed, ok := itemsRaw.(map[string]any)
	if !ok {
		if _, isTuple := itemsRaw.([]any); isTuple {
			return fmt.Errorf("jsonschema: tuple items are not supported at %s", path)
		}
		return fmt.Errorf("jsonschema: items must be an object at %s", path)
	}
	converted, err := schemaFromJSONSchemaWithContext(typed, joinPath(path, "items"), ctx)
	if err != nil {
		return err
	}
	out.Items = &converted
	return nil
}

func applyOneOf(out *schema.Schema, payload map[string]any, path string, ctx normalizeContext) error {
	oneOfRaw, ok := payload["oneOf"]
	if !ok {
		return nil
	}
	if !ctx.allowOneOf {
		return fmt.Errorf("jsonschema: oneOf is only supported for array items at %s", path)
	}
	list, ok := oneOfRaw.([]any)
	if !ok {
		return fmt.Errorf("jsonschema: oneOf must be an array at %s", path)
	}
	if len(list) == 0 {
		return fmt.Errorf("jsonschema: oneOf must include at least one schema at %s", path)
	}
	out.OneOf = make([]schema.Schema, 0, len(list))
	for idx, entry := range list {
		childPath := joinPath(path, "oneOf", fmt.Sprintf("%d", idx))
		converted, err := schemaFromJSONSchemaWithContext(entry, childPath, ctx.forOneOfVariant())
		if err != nil {
			return err
		}
		out.OneOf = append(out.OneOf, converted)
	}
	return nil
}

func applyAnyOf(out *schema.Schema, payload map[string]any, path string, ctx normalizeContext) error {
	anyOfRaw, ok := payload["anyOf"]
	if !ok {
		return nil
	}
	list, ok := anyOfRaw.([]any)
	if !ok {
		return fmt.Errorf("jsonschema: anyOf must be an array at %s", path)
	}
	if len(list) == 0 {
		return fmt.Errorf("jsonschema: anyOf must include at least one schema at %s", path)
	}

	branches := make([]schema.Schema, 0, len(list))
	branchPaths := make([]string, 0, len(list))
	for idx, entry := range list {
		childPath := joinPath(path, "anyOf", fmt.Sprintf("%d", idx))
		branchPaths = append(branchPaths, childPath)
		if nullSchema, isNullSchema, err := explicitNullSchema(entry, childPath); isNullSchema || err != nil {
			if err != nil {
				return err
			}
			branches = append(branches, nullSchema)
			continue
		}
		converted, err := schemaFromJSONSchemaWithContext(entry, childPath, ctx.forChild())
		if err != nil {
			return err
		}
		branches = append(branches, converted)
	}

	if allDiscriminatorBranches(branches) {
		if !ctx.allowOneOf {
			return fmt.Errorf("jsonschema: anyOf discriminator unions are only supported for array items at %s", path)
		}
		for idx := range branches {
			if err := applyDiscriminatorRules(&branches[idx], branchPaths[idx], true); err != nil {
				return err
			}
		}
		out.OneOf = branches
		return nil
	}

	merged, ok, err := mergeCompatibleAnyOfBranches(out, branches, path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("jsonschema: unsupported anyOf shape at %s", path)
	}
	*out = merged
	return nil
}

func validateKeywords(payload map[string]any, path string) error {
	keys := sortedKeys(payload)
	for _, key := range keys {
		if isVendorExtension(key) {
			continue
		}
		if _, ok := supportedSchemaKeys[key]; ok {
			continue
		}
		return fmt.Errorf("jsonschema: unsupported keyword %q at %s", key, path)
	}
	return nil
}

func isVendorExtension(key string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "x-")
}

func extractExtensions(payload map[string]any) map[string]any {
	var extensions map[string]any
	keys := sortedKeys(payload)
	for _, key := range keys {
		if !isVendorExtension(key) {
			continue
		}
		if extensions == nil {
			extensions = make(map[string]any)
		}
		extensions[key] = payload[key]
	}
	return extensions
}

func readOnlyAnnotation(payload map[string]any, path string) (bool, error) {
	value, hasReadOnly, err := readBoolKeyword(payload, "readOnly", path)
	if err != nil {
		return false, err
	}
	compat, hasCompat, err := readBoolKeyword(payload, "read_only", path)
	if err != nil {
		return false, err
	}
	if hasReadOnly && hasCompat && value != compat {
		return false, fmt.Errorf("jsonschema: readOnly conflicts with read_only at %s", path)
	}
	if hasCompat {
		return compat, nil
	}
	if hasReadOnly {
		return value, nil
	}
	return false, nil
}

func readBoolKeyword(payload map[string]any, key, path string) (bool, bool, error) {
	raw, ok := payload[key]
	if !ok {
		return false, false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, true, fmt.Errorf("jsonschema: %s must be a boolean at %s", key, path)
	}
	return value, true, nil
}

func explicitNullSchema(node any, path string) (schema.Schema, bool, error) {
	payload, ok := node.(map[string]any)
	if !ok || payload == nil {
		return schema.Schema{}, false, nil
	}
	if err := validateKeywords(payload, path); err != nil {
		return schema.Schema{}, false, err
	}
	raw, ok := payload["type"]
	if !ok {
		return schema.Schema{}, false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return schema.Schema{}, false, nil
	}
	if strings.TrimSpace(value) != "null" {
		return schema.Schema{}, false, nil
	}
	readOnly, err := readOnlyAnnotation(payload, path)
	if err != nil {
		return schema.Schema{}, false, err
	}
	return schema.Schema{
		Title:       strings.TrimSpace(readString(payload, "title")),
		Description: strings.TrimSpace(readString(payload, "description")),
		Default:     payload["default"],
		ReadOnly:    readOnly,
		Enum:        []any{nil},
		Extensions:  extractExtensions(payload),
	}, true, nil
}

func allDiscriminatorBranches(branches []schema.Schema) bool {
	if len(branches) == 0 {
		return false
	}
	for _, branch := range branches {
		prop, ok := branch.Properties["_type"]
		if !ok {
			return false
		}
		if _, ok := discriminatorValue(prop); !ok {
			return false
		}
	}
	return true
}

func mergeCompatibleAnyOfBranches(base *schema.Schema, branches []schema.Schema, path string) (schema.Schema, bool, error) {
	var merged schema.Schema
	seenConcrete := false
	for _, branch := range branches {
		if isNullSchema(branch) {
			continue
		}
		if !seenConcrete {
			merged = branch
			seenConcrete = true
			continue
		}
		return schema.Schema{}, false, nil
	}
	if !seenConcrete {
		return schema.Schema{}, false, fmt.Errorf("jsonschema: anyOf must include a non-null schema at %s", path)
	}
	merged.Title = firstNonEmpty(base.Title, merged.Title)
	merged.Description = firstNonEmpty(base.Description, merged.Description)
	if base.Default != nil {
		merged.Default = base.Default
	}
	if base.ReadOnly {
		merged.ReadOnly = true
	}
	if base.Format != "" {
		merged.Format = base.Format
	}
	if len(base.Extensions) > 0 {
		merged.Extensions = mergeExtensions(merged.Extensions, base.Extensions)
	}
	return merged, true, nil
}

func isNullSchema(input schema.Schema) bool {
	return input.Type == "" && input.Const == nil && len(input.Enum) == 1 && input.Enum[0] == nil
}

func mergeExtensions(left, right map[string]any) map[string]any {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := make(map[string]any, len(left)+len(right))
	maps.Copy(out, left)
	for key, value := range right {
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseType(payload map[string]any, path string) (string, bool, error) {
	if payload == nil {
		return "", false, nil
	}
	raw, ok := payload["type"]
	if !ok {
		return "", false, nil
	}

	switch value := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", false, fmt.Errorf("jsonschema: type must be a string at %s", path)
		}
		if !isAllowedType(trimmed) {
			return "", false, fmt.Errorf("jsonschema: unsupported type %q at %s", trimmed, path)
		}
		return trimmed, false, nil
	case []any:
		var types []string
		nullable := false
		for idx, entry := range value {
			str, ok := entry.(string)
			if !ok {
				return "", false, fmt.Errorf("jsonschema: type[%d] must be a string at %s", idx, path)
			}
			trimmed := strings.TrimSpace(str)
			if trimmed == "" {
				return "", false, fmt.Errorf("jsonschema: type[%d] must be a string at %s", idx, path)
			}
			if trimmed == "null" {
				nullable = true
				continue
			}
			types = append(types, trimmed)
		}
		if len(types) == 0 {
			if nullable {
				return "", true, fmt.Errorf("jsonschema: type must include a non-null value at %s", path)
			}
			return "", false, fmt.Errorf("jsonschema: type must include at least one value at %s", path)
		}
		if len(types) > 1 {
			return "", false, fmt.Errorf("jsonschema: unsupported type union at %s", path)
		}
		primary := types[0]
		if !isAllowedType(primary) {
			return "", false, fmt.Errorf("jsonschema: unsupported type %q at %s", primary, path)
		}
		return primary, nullable, nil
	default:
		return "", false, fmt.Errorf("jsonschema: type must be a string or array at %s", path)
	}
}

func hasNullableSchema(node any) bool {
	return hasNullableType(node) || hasNullableAnyOf(node)
}

func hasNullableType(node any) bool {
	payload, ok := node.(map[string]any)
	if !ok || payload == nil {
		return false
	}
	raw, ok := payload["type"]
	if !ok {
		return false
	}
	list, ok := raw.([]any)
	if !ok {
		return false
	}
	hasNull := false
	hasOther := false
	for _, entry := range list {
		str, ok := entry.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(str)
		if trimmed == "" {
			continue
		}
		if trimmed == "null" {
			hasNull = true
		} else {
			hasOther = true
		}
	}
	return hasNull && hasOther
}

func hasNullableAnyOf(node any) bool {
	payload, ok := node.(map[string]any)
	if !ok || payload == nil {
		return false
	}
	raw, ok := payload["anyOf"]
	if !ok {
		return false
	}
	list, ok := raw.([]any)
	if !ok {
		return false
	}
	hasNull := false
	hasOther := false
	for _, entry := range list {
		if isRawNullSchema(entry) {
			hasNull = true
			continue
		}
		if _, ok := entry.(map[string]any); ok {
			hasOther = true
		}
	}
	return hasNull && hasOther
}

func isRawNullSchema(node any) bool {
	payload, ok := node.(map[string]any)
	if !ok || payload == nil {
		return false
	}
	raw, ok := payload["type"]
	if !ok {
		return false
	}
	value, ok := raw.(string)
	return ok && strings.TrimSpace(value) == "null"
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		parsed, err := v.Float64()
		return parsed, err == nil
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int64ToInt(parsed)
		}
		parsed, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return floatToInt(parsed)
	case float64:
		return floatToInt(v)
	case float32:
		return floatToInt(float64(v))
	case int:
		return v, true
	case int64:
		return int64ToInt(v)
	default:
		return 0, false
	}
}

func floatToInt(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, false
	}
	converted := int(value)
	if float64(converted) != value {
		return 0, false
	}
	return converted, true
}

func int64ToInt(value int64) (int, bool) {
	converted := int(value)
	if int64(converted) != value {
		return 0, false
	}
	return converted, true
}

func isAllowedType(value string) bool {
	switch value {
	case "object", "array", "string", "integer", "number", "boolean":
		return true
	default:
		return false
	}
}

func joinPath(path string, segments ...string) string {
	if path == "" || path == "#" {
		path = "#"
	}
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		path = path + "/" + escapeJSONPointer(segment)
	}
	return path
}

func escapeJSONPointer(value string) string {
	replacer := strings.NewReplacer("~", "~0", "/", "~1")
	return replacer.Replace(value)
}

func sortedKeys(payload map[string]any) []string {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func applyDiscriminatorRules(target *schema.Schema, path string, required bool) error {
	if !required {
		return nil
	}
	if target == nil {
		return fmt.Errorf("jsonschema: invalid discriminator target at %s", path)
	}
	if target.Type != "" && target.Type != "object" {
		return fmt.Errorf("jsonschema: oneOf variant must be an object at %s", path)
	}
	if len(target.Properties) == 0 {
		return fmt.Errorf("jsonschema: oneOf variant missing properties at %s", path)
	}

	prop, ok := target.Properties["_type"]
	if !ok {
		return fmt.Errorf("jsonschema: oneOf variant missing _type discriminator at %s", path)
	}
	value, ok := discriminatorValue(prop)
	if !ok {
		return fmt.Errorf("jsonschema: oneOf variant _type must be a const string at %s", path)
	}
	if prop.Type == "" {
		prop.Type = "string"
	} else if prop.Type != "string" {
		return fmt.Errorf("jsonschema: oneOf variant _type must be a string at %s", path)
	}

	if prop.Const == nil {
		prop.Const = value
	}
	prop.ReadOnly = true
	prop.Extensions = ensureReadonlyExtension(prop.Extensions)
	target.Properties["_type"] = prop

	if !containsString(target.Required, "_type") {
		target.Required = append(target.Required, "_type")
	}
	return nil
}

func enforceBlockWidget(target schema.Schema, path string) error {
	if target.Type != "array" || target.Items == nil || len(target.Items.OneOf) == 0 {
		return nil
	}
	if hasBlockWidget(target.Extensions) {
		return nil
	}
	return fmt.Errorf("jsonschema: oneOf blocks require x-formgen.widget=block at %s", path)
}

func hasBlockWidget(ext map[string]any) bool {
	return hasWidget(ext, "x-formgen") || hasWidget(ext, "x-admin") ||
		hasWidgetValue(ext, "x-formgen-widget") || hasWidgetValue(ext, "x-admin-widget")
}

func hasWidget(ext map[string]any, key string) bool {
	if len(ext) == 0 {
		return false
	}
	raw, ok := ext[key]
	if !ok {
		return false
	}
	mapped, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	widget := strings.TrimSpace(strings.ToLower(readString(mapped, "widget")))
	return widget == "block"
}

func hasWidgetValue(ext map[string]any, key string) bool {
	if len(ext) == 0 {
		return false
	}
	value, ok := ext[key]
	if !ok {
		return false
	}
	widget := strings.TrimSpace(strings.ToLower(toString(value)))
	return widget == "block"
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func discriminatorValue(prop schema.Schema) (string, bool) {
	if value, ok := prop.Const.(string); ok && strings.TrimSpace(value) != "" {
		return value, true
	}
	if len(prop.Enum) == 1 {
		if value, ok := prop.Enum[0].(string); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func ensureReadonlyExtension(ext map[string]any) map[string]any {
	if ext == nil {
		ext = make(map[string]any)
	}
	const key = "x-formgen"
	nested, _ := ext[key].(map[string]any)
	if nested == nil {
		nested = make(map[string]any)
	}
	nested["readonly"] = true
	ext[key] = nested
	return ext
}

func containsString(list []string, value string) bool {
	return slices.Contains(list, value)
}
