package parser

import (
	"strings"
	"unicode"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

const (
	relationshipExtensionKey = "x-relationships"

	relationshipTypeAttr       = "type"
	relationshipTargetAttr     = "target"
	relationshipForeignKeyAttr = "foreignKey"
	relationshipThroughAttr    = "through"
	relationshipInverseAttr    = "inverse"
	relationshipCardAttr       = "cardinality"
	relationshipSourceAttr     = "sourceField"
)

var relationshipKeyLookup = map[string]string{
	"type":        relationshipTypeAttr,
	"kind":        relationshipTypeAttr,
	"target":      relationshipTargetAttr,
	"foreignkey":  relationshipForeignKeyAttr,
	"foreign_id":  relationshipForeignKeyAttr,
	"foreign-id":  relationshipForeignKeyAttr,
	"through":     relationshipThroughAttr,
	"pivot":       relationshipThroughAttr,
	"inverse":     relationshipInverseAttr,
	"cardinality": relationshipCardAttr,
	"sourcefield": relationshipSourceAttr,
}

func normaliseRelationshipExtension(value any) map[string]any {
	raw, ok := value.(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}

	normalised := make(map[string]any)
	for key, val := range raw {
		canonical, ok := canonicalRelationshipKey(key)
		if !ok {
			continue
		}
		if strVal, ok := toString(val); ok {
			normalised[canonical] = strVal
		}
	}

	if len(normalised) == 0 {
		return nil
	}

	if relType, ok := normalised[relationshipTypeAttr].(string); ok && relType != "" {
		if _, exists := normalised[relationshipCardAttr]; !exists {
			if card := deriveCardinality(relType); card != "" {
				normalised[relationshipCardAttr] = card
			}
		}
	}

	return normalised
}

func canonicalRelationshipKey(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	sanitised := normaliseKey(raw)
	if sanitised == "" {
		return "", false
	}
	if canonical, ok := relationshipKeyLookup[sanitised]; ok {
		return canonical, true
	}
	// Allow unknown keys to flow through using the sanitised form.
	return sanitised, true
}

func normaliseKey(raw string) string {
	var builder strings.Builder
	builder.Grow(len(raw))
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			builder.WriteRune(unicode.ToLower(r))
		default:
			// drop separators such as '-', '_', ' '
		}
	}
	return builder.String()
}

func toString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, v != ""
	default:
		return "", false
	}
}

func deriveCardinality(relType string) string {
	switch strings.ToLower(relType) {
	case "belongsto", "hasone":
		return "one"
	case "hasmany":
		return "many"
	default:
		return ""
	}
}

// propagateRelationshipMetadata copies canonical relationship metadata to the
// foreign key field when the extension references a sibling. The original node
// retains a breadcrumb pointing to the canonical host.
func propagateRelationshipMetadata(props map[string]pkgopenapi.Schema) map[string]pkgopenapi.Schema {
	if len(props) == 0 {
		return props
	}

	updated := make(map[string]pkgopenapi.Schema, len(props))
	for name, schema := range props {
		updated[name] = schema
	}

	for name, schema := range props {
		hostName, ok := hostField(schema, name, updated)
		if !ok {
			continue
		}

		host := updated[hostName]
		host.Extensions = mergeRelationshipExtensions(host.Extensions, schema.Extensions)
		updated[hostName] = host

		original := updated[name]
		original.Extensions = breadcrumbExtensions(original.Extensions, hostName)
		updated[name] = original
	}

	return updated
}

func hostField(schema pkgopenapi.Schema, name string, props map[string]pkgopenapi.Schema) (string, bool) {
	rel := relationshipFromExtensions(schema.Extensions)
	if len(rel) == 0 {
		return "", false
	}
	fk := rel[relationshipForeignKeyAttr]
	if fk == "" || fk == name {
		return "", false
	}
	if _, exists := props[fk]; !exists {
		return "", false
	}
	return fk, true
}

func mergeRelationshipExtensions(host map[string]any, source map[string]any) map[string]any {
	hostRel := relationshipFromExtensions(host)
	sourceRel := relationshipFromExtensions(source)
	if len(sourceRel) == 0 {
		if label := labelFieldFromExtensions(source); label != "" {
			return setLabelFieldExtension(host, label)
		}
		return host
	}

	if len(hostRel) == 0 {
		hostRel = make(map[string]string, len(sourceRel))
	}
	for key, value := range sourceRel {
		if key == relationshipSourceAttr {
			continue
		}
		hostRel[key] = value
	}
	host = setRelationshipExtension(host, hostRel)
	if label := labelFieldFromExtensions(source); label != "" {
		host = setLabelFieldExtension(host, label)
	}
	return host
}

func breadcrumbExtensions(ext map[string]any, host string) map[string]any {
	rel := relationshipFromExtensions(ext)
	if len(rel) == 0 {
		rel = make(map[string]string, 1)
	}
	rel[relationshipSourceAttr] = host
	return setRelationshipExtension(ext, rel)
}

func relationshipFromExtensions(ext map[string]any) map[string]string {
	if len(ext) == 0 {
		return nil
	}
	raw, ok := ext[relationshipExtensionKey]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case map[string]string:
		return cloneRelationshipMap(value)
	case map[string]any:
		result := make(map[string]string, len(value))
		for key, v := range value {
			if str, ok := toString(v); ok {
				result[key] = str
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

func setRelationshipExtension(ext map[string]any, rel map[string]string) map[string]any {
	if ext == nil {
		ext = make(map[string]any)
	}
	// Remove any legacy dotted keys that might still exist.
	for key := range ext {
		if strings.HasPrefix(key, "relationship.") {
			delete(ext, key)
		}
	}
	if len(rel) == 0 {
		delete(ext, relationshipExtensionKey)
		return ext
	}
	converted := make(map[string]any, len(rel))
	for key, value := range rel {
		if value == "" {
			continue
		}
		converted[key] = value
	}
	if len(converted) == 0 {
		delete(ext, relationshipExtensionKey)
		return ext
	}
	ext[relationshipExtensionKey] = converted
	return ext
}

func labelFieldFromExtensions(ext map[string]any) string {
	if len(ext) == 0 {
		return ""
	}
	if value, ok := ext["x-formgen-label-field"]; ok {
		if str, ok := value.(string); ok && str != "" {
			return str
		}
	}
	if nested, ok := ext["x-formgen"].(map[string]any); ok {
		if str, ok := nested["label-field"].(string); ok && str != "" {
			return str
		}
	}
	return ""
}

func setLabelFieldExtension(ext map[string]any, label string) map[string]any {
	if label == "" {
		return ext
	}
	if ext == nil {
		ext = make(map[string]any)
	}
	ext["x-formgen-label-field"] = label
	return ext
}

func cloneRelationshipMap(rel map[string]string) map[string]string {
	if len(rel) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(rel))
	for key, value := range rel {
		if value == "" {
			continue
		}
		cloned[key] = value
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}
