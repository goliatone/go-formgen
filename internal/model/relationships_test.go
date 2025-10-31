package model

import "testing"

func TestRelationshipFromExtensionsComplete(t *testing.T) {
	ext := map[string]any{
		relationshipExtensionKey: map[string]string{
			relationshipTypeAttr:       "belongsTo",
			relationshipTargetAttr:     "#/components/schemas/Author",
			relationshipForeignKeyAttr: "author_id",
			relationshipCardAttr:       "one",
			relationshipInverseAttr:    "articles",
			relationshipSourceAttr:     "author_id",
		},
	}

	rel := relationshipFromExtensions(ext)
	if rel == nil {
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

func TestRelationshipFromExtensionsDerivesCardinality(t *testing.T) {
	ext := map[string]any{
		relationshipExtensionKey: map[string]string{
			relationshipTypeAttr:   "HaSmAnY",
			relationshipTargetAttr: "#/components/schemas/Tag",
		},
	}

	rel := relationshipFromExtensions(ext)
	if rel == nil {
		t.Fatalf("expected relationship to be detected")
	}
	if rel.Kind != RelationshipHasMany {
		t.Fatalf("kind mismatch: got %q", rel.Kind)
	}
	if rel.Cardinality != "many" {
		t.Fatalf("expected cardinality \"many\", got %q", rel.Cardinality)
	}
}

func TestRelationshipFromExtensionsMissingTarget(t *testing.T) {
	ext := map[string]any{
		relationshipExtensionKey: map[string]string{
			relationshipTypeAttr: "belongsTo",
		},
	}

	if rel := relationshipFromExtensions(ext); rel != nil {
		t.Fatalf("expected missing target to yield no relationship, got %#v", rel)
	}
}

func TestPropagateRelationshipToItemsClonesStruct(t *testing.T) {
	field := Field{
		Type: FieldTypeArray,
		Relationship: &Relationship{
			Kind:        RelationshipHasMany,
			Target:      "#/components/schemas/Tag",
			Cardinality: "many",
		},
		Items: &Field{Name: "tagsItem"},
	}

	propagateRelationshipToItems(&field)

	if field.Items.Relationship == nil {
		t.Fatalf("expected relationship propagated to items")
	}
	if field.Items.Relationship == field.Relationship {
		t.Fatalf("expected cloned relationship, pointers match")
	}
	if field.Items.Relationship.Cardinality != "many" {
		t.Fatalf("expected cardinality many, got %q", field.Items.Relationship.Cardinality)
	}
	if field.Items.Relationship.SourceField != "" {
		t.Fatalf("expected empty source field on array items, got %q", field.Items.Relationship.SourceField)
	}
}

func TestDecorateRelationshipSiblingsClonesHost(t *testing.T) {
	fields := []Field{
		{
			Name: "author_id",
			Relationship: &Relationship{
				Kind:        RelationshipBelongsTo,
				Target:      "#/components/schemas/Author",
				ForeignKey:  "author_id",
				Cardinality: "one",
			},
		},
		{
			Name: "author",
			Relationship: &Relationship{
				SourceField: "author_id",
			},
		},
	}

	decorateRelationshipSiblings(fields)

	rel := fields[1].Relationship
	if rel == nil {
		t.Fatalf("expected cloned relationship on sibling")
	}
	if rel == fields[0].Relationship {
		t.Fatalf("expected a cloned relationship, pointers match")
	}
	if rel.Target != "#/components/schemas/Author" {
		t.Fatalf("target mismatch: got %q", rel.Target)
	}
	if rel.SourceField != "author_id" {
		t.Fatalf("source field mismatch: got %q", rel.SourceField)
	}
	if rel.Kind != RelationshipBelongsTo {
		t.Fatalf("kind mismatch: got %q", rel.Kind)
	}
}
