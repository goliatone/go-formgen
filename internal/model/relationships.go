package model

import (
	"strings"
)

const (
	relationshipExtensionKey   = "x-relationships"
	relationshipTypeAttr       = "type"
	relationshipTargetAttr     = "target"
	relationshipForeignKeyAttr = "foreignKey"
	relationshipCardAttr       = "cardinality"
	relationshipInverseAttr    = "inverse"
	relationshipSourceAttr     = "sourceField"
)

func relationshipFromExtensions(ext map[string]any) *Relationship {
	if len(ext) == 0 {
		return nil
	}
	raw, ok := ext[relationshipExtensionKey]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case map[string]string:
		return relationshipFromMap(value)
	case map[string]any:
		data := make(map[string]string, len(value))
		for key, v := range value {
			switch t := v.(type) {
			case string:
				if t != "" {
					data[key] = t
				}
			default:
				// ignore non-string values; relationship metadata is stringly typed.
			}
		}
		return relationshipFromMap(data)
	default:
		return nil
	}
}

func relationshipFromMap(data map[string]string) *Relationship {
	if len(data) == 0 {
		return nil
	}

	target := strings.TrimSpace(data[relationshipTargetAttr])
	if target == "" {
		return nil
	}

	rel := &Relationship{
		Target: target,
	}

	if kind := strings.TrimSpace(data[relationshipTypeAttr]); kind != "" {
		switch strings.ToLower(kind) {
		case "belongsto":
			rel.Kind = RelationshipBelongsTo
		case "hasone":
			rel.Kind = RelationshipHasOne
		case "hasmany":
			rel.Kind = RelationshipHasMany
		default:
			rel.Kind = RelationshipKind(kind)
		}
	}

	if fk := strings.TrimSpace(data[relationshipForeignKeyAttr]); fk != "" {
		rel.ForeignKey = fk
	}
	if inverse := strings.TrimSpace(data[relationshipInverseAttr]); inverse != "" {
		rel.Inverse = inverse
	}
	if source := strings.TrimSpace(data[relationshipSourceAttr]); source != "" {
		rel.SourceField = source
	}

	cardinality := strings.TrimSpace(data[relationshipCardAttr])
	if cardinality == "" {
		cardinality = deriveCardinality(string(rel.Kind))
	}
	if cardinality != "" {
		rel.Cardinality = strings.ToLower(cardinality)
	}

	return rel
}

func deriveCardinality(relType string) string {
	switch strings.ToLower(relType) {
	case "hasmany":
		return "many"
	case "belongsto", "hasone":
		return "one"
	default:
		return ""
	}
}

func cloneRelationship(rel *Relationship) *Relationship {
	if rel == nil {
		return nil
	}
	cloned := *rel
	return &cloned
}
