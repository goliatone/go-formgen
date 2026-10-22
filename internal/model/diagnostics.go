package model

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/goliatone/go-formgen/pkg/schema"
)

// DiagnoseExtensions checks only package-owned metadata spellings. Unknown host
// keys remain extensible. Locations are definition paths, never answer values.
func DiagnoseExtensions(extensions map[string]any, path string) []DefinitionDiagnostic {
	var out []DefinitionDiagnostic
	add := func(code, key, message string) {
		out = append(out, DefinitionDiagnostic{Code: code, Path: path + "/" + pointer(key), Message: message})
	}
	namespace := map[string]any{}
	if raw, exists := extensions[extensionNamespace]; exists {
		var ok bool
		namespace, ok = raw.(map[string]any)
		if !ok {
			add("metadataShape", extensionNamespace, "x-formgen must be an object")
			namespace = map[string]any{}
		}
	}
	keys := make([]string, 0, len(namespace))
	for key := range namespace {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := namespace[key]
		if key == "conditions" && path != "#" {
			out = append(out, DefinitionDiagnostic{Code: "conditionLocation", Path: path + "/x-formgen/conditions", Message: "conditions are compiled only from the schema/form root extension"})
		}
		diagnoseMetadataValue(key, value, func(code, message string) {
			out = append(out, DefinitionDiagnostic{Code: code, Path: path + "/x-formgen/" + pointer(key), Message: message})
		})
		if flat, exists := extensions[extensionNamespace+"-"+key]; exists && !reflect.DeepEqual(flat, value) {
			out = append(out, DefinitionDiagnostic{Code: "metadataConflict", Path: path + "/x-formgen/" + pointer(key), Message: "flat x-formgen extension overrides the nested key"})
		}
	}
	keys = keys[:0]
	for key := range extensions {
		if strings.HasPrefix(key, extensionNamespace+"-") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "x-formgen-conditions" {
			add("metadataAlias", key, "conditions require the nested root x-formgen.conditions key")
		}
		diagnoseMetadataValue(strings.TrimPrefix(key, extensionNamespace+"-"), extensions[key], func(code, message string) { add(code, key, message) })
	}
	return out
}
func diagnoseMetadataValue(key string, value any, add func(string, string)) {
	switch key {
	case "component", "componentName", "component-name":
		add("metadataAlias", "inline component aliases are ignored; use flat component.name inside x-formgen")
	case "componentOptions", "component.options":
		add("metadataAlias", "inline component options use component.config")
	case "title":
		add("metadataUnused", "inline title does not set a field label; use label or the UI schema copy field")
	case "component.name", "label", "placeholder", "helpText":
		if text, ok := value.(string); !ok || strings.TrimSpace(text) == "" {
			add("metadataShape", "package text hint must be a nonempty string")
		}
	case "component.config":
		if !configObject(value) {
			add("metadataShape", "component.config must be an object or JSON object string")
		}
	case "conditions":
		if _, err := DecodeConditions(value); err != nil {
			add("conditionShape", "conditions must match the versioned condition definition shape")
		}
	}
}
func configObject(value any) bool {
	if text, ok := value.(string); ok {
		var obj map[string]any
		return json.Unmarshal([]byte(text), &obj) == nil && obj != nil
	}
	switch value.(type) {
	case map[string]any, map[string]string:
		return true
	}
	return false
}
func pointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

// DiagnoseSchema checks normalized schema metadata. Original $ref/overlay source
// locations are not retained by canonical schema IR; use adapter diagnostics first
// when original JSON Pointer locations are needed.
func DiagnoseSchema(definition schema.Schema) []DefinitionDiagnostic {
	var out []DefinitionDiagnostic
	var walk func(schema.Schema, string)
	walk = func(def schema.Schema, path string) {
		out = append(out, DiagnoseExtensions(def.Extensions, path)...)
		if def.Title != "" && def.Type != "object" && def.Type != "array" {
			_, hints := ParseUIExtensions(def.Extensions)
			if hints["label"] == "" {
				out = append(out, DefinitionDiagnostic{Code: "metadataUnused", Path: path + "/title", Message: "primitive schema title is not a field label; use x-formgen.label or a UI schema label"})
			}
		}
		names := make([]string, 0, len(def.Properties))
		for name := range def.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			walk(def.Properties[name], path+"/properties/"+pointer(name))
		}
		if def.Items != nil {
			walk(*def.Items, path+"/items")
		}
		for _, branch := range []struct {
			name   string
			fields []schema.Schema
		}{{"oneOf", def.OneOf}, {"anyOf", def.AnyOf}, {"allOf", def.AllOf}} {
			for i, child := range branch.fields {
				walk(child, path+"/"+branch.name+"/"+strconv.Itoa(i))
			}
		}
	}
	walk(definition, "#")
	return out
}
