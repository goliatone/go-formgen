package parser

import (
	"strings"
	"unicode"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

const (
	relationshipExtensionKey  = "x-relationships"
	relationshipNamespace     = "relationship."
	relationshipTypeKey       = relationshipNamespace + "type"
	relationshipTargetKey     = relationshipNamespace + "target"
	relationshipForeignKeyKey = relationshipNamespace + "foreignKey"
	relationshipThroughKey    = relationshipNamespace + "through"
	relationshipInverseKey    = relationshipNamespace + "inverse"
	relationshipCardKey       = relationshipNamespace + "cardinality"
	relationshipSourceKey     = relationshipNamespace + "sourceField"
)

var relationshipKeyLookup = map[string]string{
	"type":        relationshipTypeKey,
	"kind":        relationshipTypeKey,
	"target":      relationshipTargetKey,
	"foreignkey":  relationshipForeignKeyKey,
	"foreign_id":  relationshipForeignKeyKey,
	"foreign-id":  relationshipForeignKeyKey,
	"through":     relationshipThroughKey,
	"pivot":       relationshipThroughKey,
	"inverse":     relationshipInverseKey,
	"cardinality": relationshipCardKey,
	"sourcefield": relationshipSourceKey,
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

	relType, _ := normalised[relationshipTypeKey].(string)
	if relType != "" {
		if _, exists := normalised[relationshipCardKey]; !exists {
			if card := deriveCardinality(relType); card != "" {
				normalised[relationshipCardKey] = card
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
	return relationshipNamespace + sanitised, true
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
	if len(schema.Extensions) == 0 {
		return "", false
	}
	value, ok := schema.Extensions[relationshipForeignKeyKey]
	if !ok {
		return "", false
	}
	fk, ok := value.(string)
	if !ok || fk == "" || fk == name {
		return "", false
	}
	if _, exists := props[fk]; !exists {
		return "", false
	}
	return fk, true
}

func mergeRelationshipExtensions(host map[string]any, source map[string]any) map[string]any {
	merged := make(map[string]any, len(host)+len(source))
	for key, value := range host {
		merged[key] = value
	}
	for key, value := range source {
		if !strings.HasPrefix(key, relationshipNamespace) {
			continue
		}
		if key == relationshipSourceKey {
			continue
		}
		merged[key] = value
	}
	return merged
}

func breadcrumbExtensions(ext map[string]any, host string) map[string]any {
	result := make(map[string]any, len(ext)+1)
	for key, value := range ext {
		if strings.HasPrefix(key, relationshipNamespace) && key != relationshipSourceKey {
			continue
		}
		result[key] = value
	}
	result[relationshipSourceKey] = host
	return result
}
