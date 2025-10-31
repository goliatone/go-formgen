package model

import "testing"

func TestRelationshipFromMetadataComplete(t *testing.T) {
	meta := map[string]string{
		relationshipTypeKey:       "belongsTo",
		relationshipTargetKey:     "#/components/schemas/Author",
		relationshipForeignKeyKey: "author_id",
		relationshipCardKey:       "one",
		relationshipInverseKey:    "articles",
		relationshipSourceKey:     "author_id",
	}

	rel, ok := relationshipFromMetadata(meta)
	if !ok {
		t.Fatalf("expected relationship to be detected")
	}
	if rel.Kind != RelationshipBelongsTo {
		t.Fatalf("kind mismatch: got %q", rel.Kind)
	}
	if rel.Target != "#/components/schemas/Author" {
		t.Fatalf("target mismatch: got %q", rel.Target)
	}
	if rel.ForeignKey != "author_id" {
		t.Fatalf("foreignKey mismatch: got %q", rel.ForeignKey)
	}
	if rel.Cardinality != "one" {
		t.Fatalf("cardinality mismatch: got %q", rel.Cardinality)
	}
	if rel.Inverse != "articles" {
		t.Fatalf("inverse mismatch: got %q", rel.Inverse)
	}
	if rel.SourceField != "author_id" {
		t.Fatalf("sourceField mismatch: got %q", rel.SourceField)
	}
}

func TestRelationshipFromMetadataDerivesCardinality(t *testing.T) {
	meta := map[string]string{
		relationshipTypeKey:   "HaSmAnY",
		relationshipTargetKey: "#/components/schemas/Tag",
	}

	rel, ok := relationshipFromMetadata(meta)
	if !ok {
		t.Fatalf("expected relationship to be detected")
	}
	if rel.Kind != RelationshipHasMany {
		t.Fatalf("kind mismatch: got %q", rel.Kind)
	}
	if rel.Cardinality != "many" {
		t.Fatalf("expected cardinality \"many\", got %q", rel.Cardinality)
	}
}

func TestEnsureRelationshipRemovesEmptyOptionalKeys(t *testing.T) {
	field := Field{
		Metadata: map[string]string{
			relationshipTypeKey:       "hasOne",
			relationshipTargetKey:     "#/components/schemas/Manager",
			relationshipCardKey:       "one",
			relationshipForeignKeyKey: "",
			relationshipInverseKey:    "",
		},
	}

	ensureRelationship(&field)

	if field.Relationship == nil {
		t.Fatalf("expected relationship pointer to be populated")
	}
	if field.Relationship.ForeignKey != "" {
		t.Fatalf("expected empty foreignKey, got %q", field.Relationship.ForeignKey)
	}
	if field.Relationship.Inverse != "" {
		t.Fatalf("expected empty inverse, got %q", field.Relationship.Inverse)
	}
	if _, ok := field.Metadata[relationshipForeignKeyKey]; ok {
		t.Fatalf("expected foreignKey metadata to be removed when empty")
	}
	if _, ok := field.Metadata[relationshipInverseKey]; ok {
		t.Fatalf("expected inverse metadata to be removed when empty")
	}
}

func TestRelationshipFromMetadataMissingTarget(t *testing.T) {
	meta := map[string]string{
		relationshipTypeKey: "belongsTo",
	}

	if rel, ok := relationshipFromMetadata(meta); ok || rel != nil {
		t.Fatalf("expected missing target to yield no relationship, got %#v", rel)
	}
}

func TestPropagateRelationshipToItemsClonesStruct(t *testing.T) {
	field := Field{
		Type: FieldTypeArray,
		Metadata: map[string]string{
			relationshipTypeKey:   "hasMany",
			relationshipTargetKey: "#/components/schemas/Tag",
			relationshipCardKey:   "many",
		},
		Items: &Field{Name: "tagsItem"},
	}

	ensureRelationship(&field)
	propagateRelationshipToItems(&field)

	if field.Items.Relationship == nil {
		t.Fatalf("expected relationship propagated to items")
	}
	if field.Items.Relationship == field.Relationship {
		t.Fatalf("expected cloned relationship, pointers match")
	}
	if field.Items.Relationship.Cardinality != "many" {
		t.Fatalf("expected card many, got %q", field.Items.Relationship.Cardinality)
	}
	if field.Items.Relationship.SourceField != "" {
		t.Fatalf("expected empty source field on array items, got %q", field.Items.Relationship.SourceField)
	}
	if field.Items.Metadata[relationshipTypeKey] != string(RelationshipHasMany) {
		t.Fatalf("expected metadata synced to relationship")
	}
}
