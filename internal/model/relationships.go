package model

import (
	"strings"
)

// ensureRelationship hydrates the typed relationship struct from metadata and
// mirrors the canonical dotted keys back into the metadata map. When required
// keys are absent the relationship pointer remains nil (TODO(REL_STRUCT): add
// logging once a structured logger is available).
func ensureRelationship(field *Field) {
	if field == nil {
		return
	}

	rel, ok := relationshipFromMetadata(field.Metadata)
	if !ok {
		field.Relationship = nil
		return
	}

	field.Relationship = rel
	field.Metadata = syncRelationshipMetadata(field.Metadata, rel)
}

func relationshipFromMetadata(metadata map[string]string) (*Relationship, bool) {
	if len(metadata) == 0 {
		return nil, false
	}

	rawType, ok := metadata[relationshipTypeKey]
	if !ok {
		return nil, false
	}

	kind, ok := normalizeRelationshipKind(rawType)
	if !ok {
		return nil, false
	}

	target := strings.TrimSpace(metadata[relationshipTargetKey])
	if target == "" {
		// TODO(REL_STRUCT): log missing relationship.target when structured logging lands.
		return nil, false
	}

	cardinality := strings.TrimSpace(metadata[relationshipCardKey])
	if cardinality == "" {
		cardinality = deriveCardinality(kind)
		if cardinality == "" {
			// TODO(REL_STRUCT): log missing cardinality for future diagnostics.
			return nil, false
		}
	}

	relationship := &Relationship{
		Kind:        kind,
		Target:      target,
		Cardinality: strings.ToLower(cardinality),
	}

	if foreignKey := strings.TrimSpace(metadata[relationshipForeignKeyKey]); foreignKey != "" {
		relationship.ForeignKey = foreignKey
	}
	if inverse := strings.TrimSpace(metadata[relationshipInverseKey]); inverse != "" {
		relationship.Inverse = inverse
	}
	if sourceField := strings.TrimSpace(metadata[relationshipSourceKey]); sourceField != "" {
		relationship.SourceField = sourceField
	}

	return relationship, true
}

func normalizeRelationshipKind(raw string) (RelationshipKind, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "belongsto":
		return RelationshipBelongsTo, true
	case "hasone":
		return RelationshipHasOne, true
	case "hasmany":
		return RelationshipHasMany, true
	default:
		return "", false
	}
}

func deriveCardinality(kind RelationshipKind) string {
	switch kind {
	case RelationshipHasMany:
		return "many"
	case RelationshipBelongsTo, RelationshipHasOne:
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

func syncRelationshipMetadata(metadata map[string]string, rel *Relationship) map[string]string {
	if rel == nil {
		return metadata
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata[relationshipTypeKey] = string(rel.Kind)
	metadata[relationshipTargetKey] = rel.Target
	metadata[relationshipCardKey] = rel.Cardinality

	if rel.ForeignKey != "" {
		metadata[relationshipForeignKeyKey] = rel.ForeignKey
	} else {
		delete(metadata, relationshipForeignKeyKey)
	}

	if rel.Inverse != "" {
		metadata[relationshipInverseKey] = rel.Inverse
	} else {
		delete(metadata, relationshipInverseKey)
	}

	if rel.SourceField != "" {
		metadata[relationshipSourceKey] = rel.SourceField
	} else {
		delete(metadata, relationshipSourceKey)
	}

	return metadata
}
