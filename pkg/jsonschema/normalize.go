package jsonschema

import (
	"fmt"
	"math"
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
	"enum":             {},
	"const":            {},
	"title":            {},
	"description":      {},
	"default":          {},
	"minimum":          {},
	"maximum":          {},
	"exclusiveMinimum": {},
	"exclusiveMaximum": {},
	"minLength":        {},
	"maxLength":        {},
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

	out := schema.Schema{
		Type:        strings.TrimSpace(readString(payload, "type")),
		Title:       strings.TrimSpace(readString(payload, "title")),
		Description: strings.TrimSpace(readString(payload, "description")),
		Default:     payload["default"],
		Const:       payload["const"],
		Format:      strings.TrimSpace(readString(payload, "format")),
		Extensions:  extensions,
	}

	if out.Type != "" && !isAllowedType(out.Type) {
		return schema.Schema{}, fmt.Errorf("jsonschema: unsupported type %q at %s", out.Type, path)
	}

	if enumRaw, ok := payload["enum"]; ok {
		enumList, ok := enumRaw.([]any)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: enum must be an array at %s", path)
		}
		out.Enum = append([]any(nil), enumList...)
	}

	if requiredRaw, ok := payload["required"]; ok {
		list, ok := requiredRaw.([]any)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: required must be an array at %s", path)
		}
		required := make([]string, 0, len(list))
		for idx, item := range list {
			str, ok := item.(string)
			if !ok || strings.TrimSpace(str) == "" {
				return schema.Schema{}, fmt.Errorf("jsonschema: required[%d] must be a string at %s", idx, path)
			}
			required = append(required, str)
		}
		out.Required = required
	}

	if minRaw, ok := payload["minimum"]; ok {
		value, ok := toFloat(minRaw)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: minimum must be a number at %s", path)
		}
		out.Minimum = &value
	}

	if maxRaw, ok := payload["maximum"]; ok {
		value, ok := toFloat(maxRaw)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: maximum must be a number at %s", path)
		}
		out.Maximum = &value
	}

	if exclusiveMinRaw, ok := payload["exclusiveMinimum"]; ok {
		switch value := exclusiveMinRaw.(type) {
		case bool:
			out.ExclusiveMinimum = value
		default:
			number, ok := toFloat(exclusiveMinRaw)
			if !ok {
				return schema.Schema{}, fmt.Errorf("jsonschema: exclusiveMinimum must be a number at %s", path)
			}
			if out.Minimum != nil {
				return schema.Schema{}, fmt.Errorf("jsonschema: minimum conflicts with exclusiveMinimum at %s", path)
			}
			out.Minimum = &number
			out.ExclusiveMinimum = true
		}
	}

	if exclusiveMaxRaw, ok := payload["exclusiveMaximum"]; ok {
		switch value := exclusiveMaxRaw.(type) {
		case bool:
			out.ExclusiveMaximum = value
		default:
			number, ok := toFloat(exclusiveMaxRaw)
			if !ok {
				return schema.Schema{}, fmt.Errorf("jsonschema: exclusiveMaximum must be a number at %s", path)
			}
			if out.Maximum != nil {
				return schema.Schema{}, fmt.Errorf("jsonschema: maximum conflicts with exclusiveMaximum at %s", path)
			}
			out.Maximum = &number
			out.ExclusiveMaximum = true
		}
	}

	if minLenRaw, ok := payload["minLength"]; ok {
		value, ok := toInt(minLenRaw)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: minLength must be an integer at %s", path)
		}
		out.MinLength = &value
	}

	if maxLenRaw, ok := payload["maxLength"]; ok {
		value, ok := toInt(maxLenRaw)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: maxLength must be an integer at %s", path)
		}
		out.MaxLength = &value
	}

	if patternRaw, ok := payload["pattern"]; ok {
		pattern, ok := patternRaw.(string)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: pattern must be a string at %s", path)
		}
		out.Pattern = pattern
	}

	childCtx := ctx.forChild()

	if defsRaw, ok := payload["$defs"]; ok {
		defs, ok := defsRaw.(map[string]any)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: $defs must be an object at %s", path)
		}
		keys := sortedKeys(defs)
		for _, key := range keys {
			child := defs[key]
			childPath := joinPath(path, "$defs", key)
			if _, err := schemaFromJSONSchemaWithContext(child, childPath, childCtx); err != nil {
				return schema.Schema{}, err
			}
		}
	}

	if propertiesRaw, ok := payload["properties"]; ok {
		props, ok := propertiesRaw.(map[string]any)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: properties must be an object at %s", path)
		}
		out.Properties = make(map[string]schema.Schema, len(props))
		keys := sortedKeys(props)
		for _, key := range keys {
			child := props[key]
			childPath := joinPath(path, "properties", key)
			converted, err := schemaFromJSONSchemaWithContext(child, childPath, childCtx)
			if err != nil {
				return schema.Schema{}, err
			}
			out.Properties[key] = converted
		}
	}

	if itemsRaw, ok := payload["items"]; ok {
		switch typed := itemsRaw.(type) {
		case map[string]any:
			childPath := joinPath(path, "items")
			converted, err := schemaFromJSONSchemaWithContext(typed, childPath, ctx.forItems())
			if err != nil {
				return schema.Schema{}, err
			}
			out.Items = &converted
		case []any:
			return schema.Schema{}, fmt.Errorf("jsonschema: tuple items are not supported at %s", path)
		default:
			return schema.Schema{}, fmt.Errorf("jsonschema: items must be an object at %s", path)
		}
	}

	if oneOfRaw, ok := payload["oneOf"]; ok {
		if !ctx.allowOneOf {
			return schema.Schema{}, fmt.Errorf("jsonschema: oneOf is only supported for array items at %s", path)
		}
		list, ok := oneOfRaw.([]any)
		if !ok {
			return schema.Schema{}, fmt.Errorf("jsonschema: oneOf must be an array at %s", path)
		}
		if len(list) == 0 {
			return schema.Schema{}, fmt.Errorf("jsonschema: oneOf must include at least one schema at %s", path)
		}
		out.OneOf = make([]schema.Schema, 0, len(list))
		for idx, entry := range list {
			childPath := joinPath(path, "oneOf", fmt.Sprintf("%d", idx))
			converted, err := schemaFromJSONSchemaWithContext(entry, childPath, ctx.forOneOfVariant())
			if err != nil {
				return schema.Schema{}, err
			}
			out.OneOf = append(out.OneOf, converted)
		}
	}

	if err := applyDiscriminatorRules(&out, path, ctx.requireDiscriminator); err != nil {
		return schema.Schema{}, err
	}

	return out, nil
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

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
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
	case float64:
		if v == math.Trunc(v) {
			return int(v), true
		}
		return 0, false
	case float32:
		if v == float32(math.Trunc(float64(v))) {
			return int(v), true
		}
		return 0, false
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
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
	prop.Extensions = ensureReadonlyExtension(prop.Extensions)
	target.Properties["_type"] = prop

	if !containsString(target.Required, "_type") {
		target.Required = append(target.Required, "_type")
	}
	return nil
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
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
