package uischema

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/model"
	"gopkg.in/yaml.v3"
)

// DiagnoseDefinition optionally checks raw UI schema metadata before typed loading
// discards ignored aliases. It leaves host keys and existing loader behavior intact.
func DiagnoseDefinition(raw []byte) []model.DefinitionDiagnostic {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return []model.DefinitionDiagnostic{{Code: "metadataShape", Path: "#", Message: "invalid UI schema document"}}
	}
	var out []model.DefinitionDiagnostic
	operations, _ := doc["operations"].(map[string]any)
	for _, id := range diagnosticKeys(operations) {
		op, _ := operations[id].(map[string]any)
		out = append(out, diagnoseOperation(op, "#/operations/"+diagnosticPointer(id))...)
	}
	return out
}
func diagnoseOperation(op map[string]any, path string) []model.DefinitionDiagnostic {
	var out []model.DefinitionDiagnostic
	if form, ok := op["form"].(map[string]any); ok {
		out = append(out, model.DiagnoseExtensions(form, path+"/form")...)
	}
	fields, _ := op["fields"].(map[string]any)
	for _, name := range diagnosticKeys(fields) {
		if field, ok := fields[name].(map[string]any); ok {
			location := path + "/fields/" + diagnosticPointer(name)
			out = append(out, model.DiagnoseExtensions(field, location)...)
			out = append(out, diagnoseFieldCopy(field, location)...)
		}
	}
	sections, _ := op["sections"].([]any)
	for i, section := range sections {
		if node, ok := section.(map[string]any); ok {
			out = append(out, model.DiagnoseExtensions(node, path+"/sections/"+strconv.Itoa(i))...)
		}
	}
	return out
}
func diagnoseFieldCopy(node map[string]any, path string) []model.DefinitionDiagnostic {
	var out []model.DefinitionDiagnostic
	add := func(key, code, message string) {
		out = append(out, model.DefinitionDiagnostic{Code: code, Path: path + "/" + diagnosticPointer(key), Message: message})
	}
	for _, alias := range []string{"component.name", "componentName", "component-name", "title"} {
		if _, exists := node[alias]; exists {
			add(alias, "metadataAlias", "UI schema fields use component and label copy keys")
		}
	}
	for _, key := range []string{"component", "label"} {
		if value, exists := node[key]; exists {
			if _, ok := value.(string); !ok {
				add(key, "metadataShape", "UI schema copy key must be a string")
			}
		}
	}
	if value, exists := node["componentOptions"]; exists {
		if _, ok := value.(map[string]any); !ok {
			add("componentOptions", "metadataShape", "componentOptions must be an object")
		}
	}
	return append(out, diagnoseCopyConflicts(node, path)...)
}
func diagnoseCopyConflicts(node map[string]any, path string) []model.DefinitionDiagnostic {
	var out []model.DefinitionDiagnostic
	ext, _ := node["x-formgen"].(map[string]any)
	for _, pair := range [][2]string{{"component", "component.name"}, {"label", "label"}} {
		a, aok := node[pair[0]]
		b, bok := ext[pair[1]]
		if !aok || !bok || a == nil || b == nil {
			continue
		}
		left, _ := json.Marshal(a)
		right, _ := json.Marshal(b)
		if string(left) != string(right) {
			out = append(out, model.DefinitionDiagnostic{Code: "metadataConflict", Path: path + "/" + pair[0], Message: "UI schema copy and extension hint compete; inspect the decorated model precedence"})
		}
	}
	return out
}
func diagnosticKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func diagnosticPointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}
