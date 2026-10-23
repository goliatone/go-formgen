package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// IsPortablePattern accepts the conservative ASCII grammar shared with the browser.
// It deliberately excludes shorthand classes, groups, dot, negation and flags.
func IsPortablePattern(pattern string) bool {
	if strings.TrimSpace(pattern) != pattern || len(pattern) > 1024 {
		return false
	}
	for _, c := range pattern {
		if c < 32 || c > 126 {
			return false
		}
	}
	body := strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")
	atom := regexp.MustCompile(`^(?:\\[\\^$.*+?()\[\]{}|\-]|\[[A-Za-z0-9 _.-]+\]|[A-Za-z0-9 _:@,/=\-])(?:[?*+]|\{(?:0|[1-9]\d{0,2})(?:,(?:0|[1-9]\d{0,2}))?\})?`)
	for len(body) > 0 {
		token := atom.FindString(body)
		if token == "" {
			return false
		}
		body = body[len(token):]
	}
	_, err := regexp.Compile(pattern)
	return err == nil
}

// DiagnosePortableDefinitionJSON checks original JSON numbers before any float64
// decoding. Pass original schema/model bytes before adapters/browser transport.
// This opt-in capability leaves legacy headless schemas and validation unchanged.
func DiagnosePortableDefinitionJSON(data []byte) ([]DefinitionDiagnostic, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("portable definition: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("portable definition: expected one JSON document")
	}
	var check portableDefinitionCheck
	check.walkRoot(root, "#")
	return check.diagnostics, nil
}

type portableDefinitionCheck struct {
	diagnostics []DefinitionDiagnostic
}

func (c *portableDefinitionCheck) add(code, path, message string) {
	c.diagnostics = append(c.diagnostics, DefinitionDiagnostic{Code: code, Path: path, Message: message})
}

func (c *portableDefinitionCheck) checkNumber(text, path string) {
	if !regexp.MustCompile(`^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$`).MatchString(text) || len(text) > 128 {
		c.add("unsafeDefinitionNumber", path, "Numeric definition exceeds portable precision")
		return
	}
	if !portableExponent(text) {
		c.add("unsafeDefinitionNumber", path, "Numeric exponent exceeds portable range")
		return
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok || new(big.Rat).Abs(number).Cmp(big.NewRat(9007199254740991, 1)) > 0 {
		c.add("unsafeDefinitionNumber", path, "Browser definition number exceeds the portable range")
	}
}

func portableExponent(text string) bool {
	at := strings.IndexAny(text, "eE")
	if at < 0 {
		return true
	}
	exponent, err := strconv.Atoi(text[at+1:])
	return err == nil && exponent <= 308 && exponent >= -308
}

func portablePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func (c *portableDefinitionCheck) numbers(value any, path string) {
	switch v := value.(type) {
	case json.Number:
		c.checkNumber(string(v), path)
	case []any:
		for i, item := range v {
			c.numbers(item, fmt.Sprintf("%s/%d", path, i))
		}
	case map[string]any:
		for _, key := range sortedPortableKeys(v) {
			c.numbers(v[key], path+"/"+portablePointer(key))
		}
	}
}

// Traversal follows definition positions, never arbitrary object members. In
// particular, JSON Schema annotations and OpenAPI examples are literal data.
func (c *portableDefinitionCheck) walkRoot(value any, path string) {
	v, ok := value.(map[string]any)
	if !ok {
		return
	}
	if v["openapi"] != nil || v["swagger"] != nil {
		c.walkAPI(v, path, "document")
		return
	}
	if fields, ok := v["fields"].([]any); ok {
		c.walkDefinitions(fields, path+"/fields", true)
		c.numbers(v["conditions"], path+"/conditions")
		return
	}
	c.walkDefinition(v, path, false)
}

func (c *portableDefinitionCheck) walkDefinitions(value any, path string, model bool) {
	if values, ok := value.([]any); ok {
		for i, item := range values {
			c.walkDefinition(item, fmt.Sprintf("%s/%d", path, i), model)
		}
	}
}

func (c *portableDefinitionCheck) walkNamedDefinitions(value any, path string) {
	if named, ok := value.(map[string]any); ok {
		for _, name := range sortedPortableKeys(named) {
			c.walkDefinition(named[name], path+"/"+portablePointer(name), false)
		}
	}
}

func (c *portableDefinitionCheck) walkDefinition(value any, path string, model bool) {
	v, ok := value.(map[string]any)
	if !ok {
		return // Boolean schemas have no constraints to check.
	}
	c.checkDefinition(v, path)
	if model {
		c.walkModelChildren(v, path)
	}
	for _, key := range sortedPortableKeys(v) {
		child, at := v[key], path+"/"+portablePointer(key)
		switch key {
		case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
			c.walkNamedDefinitions(child, at)
		case "items":
			if _, tuple := child.([]any); tuple {
				c.walkDefinitions(child, at, model)
			} else {
				c.walkDefinition(child, at, model)
			}
		case "additionalProperties", "additionalItems", "unevaluatedProperties", "unevaluatedItems", "contains", "propertyNames", "not", "if", "then", "else", "contentSchema":
			c.walkDefinition(child, at, false)
		case "allOf", "anyOf", "oneOf", "prefixItems":
			c.walkDefinitions(child, at, model)
		case "dependencies":
			// Draft-07 permits either schemas or arrays of property names.
			c.walkNamedDefinitions(child, at)
		case "default", "enum", "const", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties", "minContains", "maxContains":
			c.numbers(child, at)
		case "x-formgen":
			if extension, ok := child.(map[string]any); ok {
				c.numbers(extension["conditions"], at+"/conditions")
			}
		}
	}
}

func (c *portableDefinitionCheck) walkModelChildren(v map[string]any, path string) {
	c.walkDefinitions(v["nested"], path+"/nested", true)
	c.walkOptions(v["options"], path+"/options")
	c.walkValidations(v["validations"], path+"/validations")
}

func (c *portableDefinitionCheck) walkOptions(value any, path string) {
	options, _ := value.([]any)
	for i, option := range options {
		if object, ok := option.(map[string]any); ok {
			c.numbers(object["value"], fmt.Sprintf("%s/%d/value", path, i))
		}
	}
}

func (c *portableDefinitionCheck) walkValidations(value any, path string) {
	rules, _ := value.([]any)
	for i, rule := range rules {
		v, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		at := fmt.Sprintf("%s/%d/params", path, i)
		params, _ := v["params"].(map[string]any)
		switch v["kind"] {
		case "min", "max":
			if bound, ok := params["value"].(string); ok {
				c.checkNumber(bound, at+"/value")
			} else {
				c.numbers(params["value"], at+"/value")
			}
		case "pattern":
			c.checkPattern(params, at)
		}
	}
}

// OpenAPI object kinds have different child positions. Only these positions
// contain definitions; examples, links and annotations remain opaque.
type portableAPIPosition struct {
	kind string
	many string // empty = one object, "map" = named objects, "list" = array
}

var portableAPIPositions = map[string]map[string]portableAPIPosition{
	"document": {
		"components": {kind: "components"},
		"paths":      {kind: "path", many: "map"}, "webhooks": {kind: "path", many: "map"},
		"definitions": {kind: "schema", many: "map"},
		"parameters":  {kind: "parameter", many: "map"}, "responses": {kind: "response", many: "map"},
	},
	"components": {
		"schemas": {kind: "schema", many: "map"}, "pathItems": {kind: "path", many: "map"},
		"parameters": {kind: "parameter", many: "map"}, "responses": {kind: "response", many: "map"},
		"requestBodies": {kind: "body", many: "map"}, "headers": {kind: "header", many: "map"},
		"callbacks": {kind: "callback", many: "map"},
	},
	"path": {
		"get": {kind: "operation"}, "put": {kind: "operation"}, "post": {kind: "operation"},
		"delete": {kind: "operation"}, "options": {kind: "operation"}, "head": {kind: "operation"},
		"patch": {kind: "operation"}, "trace": {kind: "operation"},
		"parameters": {kind: "parameter", many: "list"},
	},
	"operation": {
		"parameters": {kind: "parameter", many: "list"}, "requestBody": {kind: "body"},
		"responses": {kind: "response", many: "map"}, "callbacks": {kind: "callback", many: "map"},
	},
	"body": {"content": {kind: "media", many: "map"}},
	"response": {
		"content": {kind: "media", many: "map"}, "headers": {kind: "header", many: "map"}, "schema": {kind: "schema"},
	},
	"parameter": {"schema": {kind: "schema"}, "content": {kind: "media", many: "map"}},
	"header":    {"schema": {kind: "schema"}, "content": {kind: "media", many: "map"}},
	"media":     {"schema": {kind: "schema"}, "encoding": {kind: "encoding", many: "map"}},
	"encoding":  {"headers": {kind: "header", many: "map"}},
}

func (c *portableDefinitionCheck) walkAPI(value any, path, kind string) {
	if kind == "schema" {
		c.walkDefinition(value, path, false)
		return
	}
	v, ok := value.(map[string]any)
	if !ok {
		return
	}
	if kind == "callback" {
		c.walkAPINamed(v, path, "path")
		return
	}
	for _, key := range sortedPortableKeys(v) {
		position, known := portableAPIPositions[kind][key]
		if !known {
			continue
		}
		child, at := v[key], path+"/"+portablePointer(key)
		switch position.many {
		case "map":
			c.walkAPINamed(child, at, position.kind)
		case "list":
			items, _ := child.([]any)
			for i, item := range items {
				c.walkAPI(item, fmt.Sprintf("%s/%d", at, i), position.kind)
			}
		default:
			c.walkAPI(child, at, position.kind)
		}
	}
	if kind == "parameter" || kind == "header" {
		// OpenAPI 2 inlines primitive constraints on these objects.
		c.walkDefinition(v, path, false)
	}
}

func (c *portableDefinitionCheck) walkAPINamed(value any, path, kind string) {
	if named, ok := value.(map[string]any); ok {
		for _, name := range sortedPortableKeys(named) {
			c.walkAPI(named[name], path+"/"+portablePointer(name), kind)
		}
	}
}

func (c *portableDefinitionCheck) checkPattern(v map[string]any, path string) {
	if pattern, ok := v["pattern"].(string); ok && !IsPortablePattern(pattern) {
		c.add("unsupportedPattern", path+"/pattern", "Pattern is outside the portable ASCII grammar")
	}
}

func (c *portableDefinitionCheck) checkDefinition(v map[string]any, path string) {
	// Supported inputs are raw JSON Schema/OpenAPI or serialized FormModel.
	c.checkPattern(v, path)
	items, _ := v["items"].(map[string]any)
	if v["type"] == "integer" || v["type"] == "array" && items["type"] == "integer" {
		c.checkIntegerDefinition(v, path)
	}
}

func (c *portableDefinitionCheck) checkIntegerDefinition(v map[string]any, path string) {
	c.checkInteger(v["default"], path+"/default")
	c.checkInteger(v["enum"], path+"/enum")
	options, _ := v["options"].([]any)
	for i, option := range options {
		if object, ok := option.(map[string]any); ok {
			c.checkInteger(object["value"], fmt.Sprintf("%s/options/%d/value", path, i))
		}
	}
}

func (c *portableDefinitionCheck) checkInteger(candidate any, path string) {
	switch v := candidate.(type) {
	case []any:
		for i, item := range v {
			c.checkInteger(item, fmt.Sprintf("%s/%d", path, i))
		}
	case json.Number:
		// The numeric walk reports overlong numbers and out-of-range exponents.
		if len(v) > 128 || !portableExponent(string(v)) {
			return
		}
		r, valid := new(big.Rat).SetString(string(v))
		if !valid || !r.IsInt() {
			c.add("invalidDefinitionInteger", path, "Integer definitions require integral values")
		}
	default:
		if candidate != nil {
			c.add("invalidDefinitionInteger", path, "Integer definitions require integral numeric values")
		}
	}
}

// DiagnosePortableModel checks losslessly represented Go values before JSON transport.
// Original source preflight is still necessary if an adapter already rounded a number.
func DiagnosePortableModel(form FormModel) []DefinitionDiagnostic {
	data, err := json.Marshal(form)
	if err != nil {
		return []DefinitionDiagnostic{{Code: "invalidDefinition", Path: "#", Message: "Definition must be JSON serializable"}}
	}
	out, err := DiagnosePortableDefinitionJSON(data)
	if err != nil {
		return []DefinitionDiagnostic{{Code: "invalidDefinition", Path: "#", Message: err.Error()}}
	}
	return out
}

func sortedPortableKeys(v map[string]any) []string {
	keys := make([]string, 0, len(v))
	for key := range v {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
