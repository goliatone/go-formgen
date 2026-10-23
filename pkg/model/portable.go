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
	check.walk(root, "#")
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

func (c *portableDefinitionCheck) walk(value any, path string) {
	switch v := value.(type) {
	case json.Number:
		c.checkNumber(string(v), path)
	case []any:
		for i, item := range v {
			c.walk(item, fmt.Sprintf("%s/%d", path, i))
		}
	case map[string]any:
		c.checkDefinition(v, path)
		// Sort keys so diagnostics are reproducible, independent of Go map iteration.
		for _, key := range sortedPortableKeys(v) {
			c.walkProperty(key, v[key], path+"/"+portablePointer(key))
		}
	}
}

func (c *portableDefinitionCheck) checkDefinition(v map[string]any, path string) {
	// Supported inputs are raw JSON Schema/OpenAPI or serialized FormModel.
	if pattern, ok := v["pattern"].(string); ok && !IsPortablePattern(pattern) {
		c.add("unsupportedPattern", path+"/pattern", "Pattern is outside the portable ASCII grammar")
	}
	if kind, _ := v["kind"].(string); kind == "min" || kind == "max" {
		params, _ := v["params"].(map[string]any)
		if bound, ok := params["value"].(string); ok {
			c.checkNumber(bound, path+"/params/value")
		}
	}
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

func (c *portableDefinitionCheck) walkProperty(key string, value any, path string) {
	if key == "x-formgen" {
		if extension, ok := value.(map[string]any); ok {
			c.numbers(extension["conditions"], path+"/conditions")
		}
		return
	}
	if strings.HasPrefix(key, "x-") || key == "metadata" || key == "uiHints" {
		return
	}
	switch key {
	// These objects are dictionaries of names, not definition nodes.
	// A question/component may itself be named metadata, default or x-*.
	case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas", "schemas", "paths", "webhooks", "responses", "requestBodies", "parameters", "headers", "content", "callbacks":
		if named, ok := value.(map[string]any); ok {
			for _, name := range sortedPortableKeys(named) {
				c.walk(named[name], path+"/"+portablePointer(name))
			}
		} else {
			c.walk(value, path)
		}
	case "default", "enum", "const", "options", "conditions":
		c.numbers(value, path)
	default:
		c.walk(value, path)
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
