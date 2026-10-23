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
	var out []DefinitionDiagnostic
	add := func(code, path, message string) {
		out = append(out, DefinitionDiagnostic{Code: code, Path: path, Message: message})
	}
	limit := big.NewRat(9007199254740991, 1)
	checkNumber := func(text, path string) {
		if !regexp.MustCompile(`^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$`).MatchString(text) || len(text) > 128 {
			add("unsafeDefinitionNumber", path, "Numeric definition exceeds portable precision")
			return
		}
		if at := strings.IndexAny(text, "eE"); at >= 0 {
			exponent, err := strconv.Atoi(text[at+1:])
			if err != nil || exponent > 308 || exponent < -308 {
				add("unsafeDefinitionNumber", path, "Numeric exponent exceeds portable range")
				return
			}
		}
		number, ok := new(big.Rat).SetString(text)
		if !ok || new(big.Rat).Abs(number).Cmp(limit) > 0 {
			add("unsafeDefinitionNumber", path, "Browser definition number exceeds the portable range")
		}
	}
	pointer := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
	var numbers func(any, string)
	numbers = func(value any, path string) {
		switch v := value.(type) {
		case json.Number:
			checkNumber(string(v), path)
		case []any:
			for i, item := range v {
				numbers(item, fmt.Sprintf("%s/%d", path, i))
			}
		case map[string]any:
			for _, key := range sortedPortableKeys(v) {
				numbers(v[key], path+"/"+pointer(key))
			}
		}
	}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch v := value.(type) {
		case json.Number:
			checkNumber(string(v), path)
		case []any:
			for i, item := range v {
				walk(item, fmt.Sprintf("%s/%d", path, i))
			}
		case map[string]any:
			// Supported inputs are raw JSON Schema/OpenAPI or serialized FormModel.
			if pattern, ok := v["pattern"].(string); ok && !IsPortablePattern(pattern) {
				add("unsupportedPattern", path+"/pattern", "Pattern is outside the portable ASCII grammar")
			}
			if kind, _ := v["kind"].(string); kind == "min" || kind == "max" {
				if params, ok := v["params"].(map[string]any); ok {
					if bound, ok := params["value"].(string); ok {
						checkNumber(bound, path+"/params/value")
					}
				}
			}
			items, _ := v["items"].(map[string]any)
			if v["type"] == "integer" || v["type"] == "array" && items["type"] == "integer" {
				var checkInteger func(any, string)
				checkInteger = func(candidate any, location string) {
					if items, ok := candidate.([]any); ok {
						for i, item := range items {
							checkInteger(item, fmt.Sprintf("%s/%d", location, i))
						}
					} else if n, ok := candidate.(json.Number); ok {
						if len(n) > 128 {
							return
						}
						if at := strings.IndexAny(string(n), "eE"); at >= 0 {
							e, err := strconv.Atoi(string(n)[at+1:])
							if err != nil || e > 308 || e < -308 {
								return
							}
						}
						r, valid := new(big.Rat).SetString(string(n))
						if !valid || !r.IsInt() {
							add("invalidDefinitionInteger", location, "Integer definitions require integral values")
						}
					} else if candidate != nil {
						add("invalidDefinitionInteger", location, "Integer definitions require integral numeric values")
					}
				}
				checkInteger(v["default"], path+"/default")
				checkInteger(v["enum"], path+"/enum")
				if options, ok := v["options"].([]any); ok {
					for i, option := range options {
						if object, ok := option.(map[string]any); ok {
							checkInteger(object["value"], fmt.Sprintf("%s/options/%d/value", path, i))
						}
					}
				}
			}
			// Sort keys so diagnostics are reproducible, independent of Go map iteration.
			for _, key := range sortedPortableKeys(v) {
				if key == "x-formgen" {
					if extension, ok := v[key].(map[string]any); ok {
						numbers(extension["conditions"], path+"/x-formgen/conditions")
					}
					continue
				}
				if strings.HasPrefix(key, "x-") || key == "metadata" || key == "uiHints" {
					continue
				}
				switch key {
				case "default", "enum", "const", "options", "conditions":
					numbers(v[key], path+"/"+pointer(key))
				default:
					walk(v[key], path+"/"+pointer(key))
				}
			}
		}
	}
	walk(root, "#")
	return out, nil
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
