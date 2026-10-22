package jsonschema

import (
	"sort"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
)

// DiagnoseDefinition checks package metadata on a raw or resolved/overlaid JSON
// Schema. It walks schema keywords only, never defaults, examples or answer maps.
// Run it on the resolved overlay result to diagnose the effective definition.
func DiagnoseDefinition(payload map[string]any) []model.DefinitionDiagnostic {
	var out []model.DefinitionDiagnostic
	var walk func(map[string]any, string)
	walk = func(node map[string]any, path string) {
		out = append(out, model.DiagnoseExtensions(node, path)...)
		if ignoredPrimitiveTitle(node) {
			out = append(out, model.DefinitionDiagnostic{Code: "metadataUnused", Path: path + "/title", Message: "primitive schema title is not a field label; use x-formgen.label or a UI schema label"})
		}
		walkSchemaMaps(node, path, walk)
		walkSchemaNodes(node, path, walk)
		walkSchemaLists(node, path, walk)
	}
	walk(payload, "#")
	return out
}
func ignoredPrimitiveTitle(node map[string]any) bool {
	title, _ := node["title"].(string)
	kind, _ := node["type"].(string)
	_, hints := model.ParseUIExtensions(node)
	return title != "" && kind != "" && kind != "object" && kind != "array" && hints["label"] == ""
}
func walkSchemaMaps(node map[string]any, path string, walk func(map[string]any, string)) {
	for _, key := range []string{"properties", "$defs", "definitions", "patternProperties", "dependentSchemas"} {
		children, _ := node[key].(map[string]any)
		names := make([]string, 0, len(children))
		for name := range children {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if child, ok := children[name].(map[string]any); ok {
				walk(child, path+"/"+key+"/"+diagnosticPointer(name))
			}
		}
	}
}
func walkSchemaNodes(node map[string]any, path string, walk func(map[string]any, string)) {
	for _, key := range []string{"items", "additionalProperties", "contains", "not", "if", "then", "else"} {
		if child, ok := node[key].(map[string]any); ok {
			walk(child, path+"/"+key)
		}
	}
}
func walkSchemaLists(node map[string]any, path string, walk func(map[string]any, string)) {
	for _, key := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		children, _ := node[key].([]any)
		for i, raw := range children {
			if child, ok := raw.(map[string]any); ok {
				walk(child, path+"/"+key+"/"+strconv.Itoa(i))
			}
		}
	}
}
func diagnosticPointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}
