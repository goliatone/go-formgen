package model_test

import (
	"path/filepath"
	"testing"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func stripRelationships(model *pkgmodel.FormModel) {
	for i := range model.Fields {
		stripFieldRelationship(&model.Fields[i])
	}
}

func stripFieldRelationship(field *pkgmodel.Field) {
	field.Relationship = nil
	if field.Items != nil {
		stripFieldRelationship(field.Items)
	}
	for i := range field.Nested {
		stripFieldRelationship(&field.Nested[i])
	}
}

func collectRelationships(fields []pkgmodel.Field) map[string]*pkgmodel.Relationship {
	result := make(map[string]*pkgmodel.Relationship)

	var visit func(prefix string, field pkgmodel.Field)
	visit = func(prefix string, field pkgmodel.Field) {
		key := field.Name
		if prefix != "" {
			key = prefix + "." + key
		}
		if field.Relationship != nil {
			value := *field.Relationship
			result[key] = &value
		} else {
			result[key] = nil
		}

		if field.Items != nil {
			visit(key, *field.Items)
		}
		for _, nested := range field.Nested {
			visit(key, nested)
		}
	}

	for _, field := range fields {
		visit("", field)
	}

	return result
}

func TestBuilder_CreatePet(t *testing.T) {
	operations := testsupport.MustLoadOperations(t, filepath.Join("../openapi", "testdata", "petstore_operations.golden.json"))
	op := operations["createPet"]

	builder := pkgmodel.NewBuilder()
	form, err := builder.Build(op)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	goldenPath := filepath.Join("testdata", "create_pet_formmodel.golden.json")
	testsupport.WriteFormModel(t, goldenPath, form)
	want := testsupport.MustLoadFormModel(t, goldenPath)

	stripRelationships(&form)
	stripRelationships(&want)

	if diff := testsupport.CompareGolden(want, form); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}

	if len(form.Fields) != 7 {
		t.Fatalf("expected 7 fields, got %d", len(form.Fields))
	}

	fieldsByPath := map[string]pkgmodel.Field{}

	var visit func(prefix string, field pkgmodel.Field)
	visit = func(prefix string, field pkgmodel.Field) {
		key := field.Name
		if prefix != "" {
			key = prefix + "." + key
		}
		fieldsByPath[key] = field

		if field.Items != nil {
			visit(key, *field.Items)
		}
		for _, nested := range field.Nested {
			visit(key, nested)
		}
	}

	for _, field := range form.Fields {
		visit("", field)
	}

	expectations := map[string][]pkgmodel.ValidationRule{
		"age": {
			{Kind: pkgmodel.ValidationRuleMin, Params: map[string]string{"value": "1"}},
			{Kind: pkgmodel.ValidationRuleMax, Params: map[string]string{"value": "25"}},
		},
		"favoriteFoods.favoriteFoodsItem": {
			{Kind: pkgmodel.ValidationRuleMinLength, Params: map[string]string{"value": "3"}},
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "24"}},
			{Kind: pkgmodel.ValidationRulePattern, Params: map[string]string{"pattern": "^[a-z]+$"}},
		},
		"favoriteNumbers.favoriteNumbersItem": {
			{Kind: pkgmodel.ValidationRuleMin, Params: map[string]string{"value": "0.1", "exclusive": "true"}},
			{Kind: pkgmodel.ValidationRuleMax, Params: map[string]string{"value": "99.9"}},
		},
		"name": {
			{Kind: pkgmodel.ValidationRuleMinLength, Params: map[string]string{"value": "3"}},
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "64"}},
			{Kind: pkgmodel.ValidationRulePattern, Params: map[string]string{"pattern": "^[A-Za-z ]+$"}},
		},
		"owner.email": {
			{Kind: pkgmodel.ValidationRuleMinLength, Params: map[string]string{"value": "5"}},
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "128"}},
			{Kind: pkgmodel.ValidationRulePattern, Params: map[string]string{"pattern": "^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$"}},
		},
		"owner.phone": {
			{Kind: pkgmodel.ValidationRuleMinLength, Params: map[string]string{"value": "7"}},
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "15"}},
			{Kind: pkgmodel.ValidationRulePattern, Params: map[string]string{"pattern": "^\\+?[0-9\\-]{7,15}$"}},
		},
		"owner.yearsAsCustomer": {
			{Kind: pkgmodel.ValidationRuleMin, Params: map[string]string{"value": "0", "exclusive": "true"}},
			{Kind: pkgmodel.ValidationRuleMax, Params: map[string]string{"value": "30"}},
		},
		"tag": {
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "32"}},
		},
		"weight": {
			{Kind: pkgmodel.ValidationRuleMin, Params: map[string]string{"value": "0.5", "exclusive": "true"}},
			{Kind: pkgmodel.ValidationRuleMax, Params: map[string]string{"value": "60"}},
		},
	}

	for path, wantRules := range expectations {
		field, ok := fieldsByPath[path]
		if !ok {
			t.Fatalf("expected field %q in form model", path)
		}
		if diff := testsupport.CompareGolden(wantRules, field.Validations); diff != "" {
			t.Fatalf("field %q validations mismatch (-want +got):\n%s", path, diff)
		}
	}

	expectRelationships := map[string]*pkgmodel.Relationship{
		"author": {
			Kind:        pkgmodel.RelationshipBelongsTo,
			Target:      "#/components/schemas/Author",
			ForeignKey:  "author_id",
			Cardinality: "one",
			SourceField: "author_id",
		},
		"author_id": {
			Kind:        pkgmodel.RelationshipBelongsTo,
			Target:      "#/components/schemas/Author",
			ForeignKey:  "author_id",
			Cardinality: "one",
		},
		"category_id": {
			Kind:        pkgmodel.RelationshipBelongsTo,
			Target:      "#/components/schemas/Category",
			Cardinality: "one",
		},
		"manager": {
			Kind:        pkgmodel.RelationshipHasOne,
			Target:      "#/components/schemas/Manager",
			ForeignKey:  "manager_id",
			Cardinality: "one",
			SourceField: "manager_id",
		},
		"manager_id": {
			Kind:        pkgmodel.RelationshipHasOne,
			Target:      "#/components/schemas/Manager",
			ForeignKey:  "manager_id",
			Cardinality: "one",
		},
		"tags": {
			Kind:        pkgmodel.RelationshipHasMany,
			Target:      "#/components/schemas/Tag",
			Cardinality: "many",
			Inverse:     "article",
		},
		"tags.tagsItem": {
			Kind:        pkgmodel.RelationshipHasMany,
			Target:      "#/components/schemas/Tag",
			Cardinality: "many",
			Inverse:     "article",
		},
	}

	for path, wantRel := range expectRelationships {
		gotRel, ok := relationshipsByPath[path]
		if !ok {
			t.Fatalf("expected relationship entry for %q", path)
		}
		if gotRel == nil {
			t.Fatalf("expected relationship for %q to be non-nil", path)
		}
		if diff := testsupport.CompareGolden(wantRel, gotRel); diff != "" {
			t.Fatalf("relationship %q mismatch (-want +got):\n%s", path, diff)
		}
	}

	if rel := relationshipsByPath["title"]; rel != nil {
		t.Fatalf("expected title relationship to remain nil, got %#v", rel)
	}
}

func TestBuilder_CreateWidgetExtensions(t *testing.T) {
	operations := testsupport.MustLoadOperations(t, filepath.Join("../openapi", "testdata", "extensions_operations.golden.json"))
	op := operations["createWidget"]

	builder := pkgmodel.NewBuilder()
	form, err := builder.Build(op)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	goldenPath := filepath.Join("testdata", "create_widget_formmodel.golden.json")
	testsupport.WriteFormModel(t, goldenPath, form)
	want := testsupport.MustLoadFormModel(t, goldenPath)

	stripRelationships(&form)
	stripRelationships(&want)

	if diff := testsupport.CompareGolden(want, form); diff != "" {
		t.Fatalf("widget form mismatch (-want +got):\n%s", diff)
	}

	if got := form.Metadata["submitLabel"]; got != "Create widget" {
		t.Fatalf("form metadata submitLabel mismatch: %q", got)
	}
	if got := form.Metadata["priority"]; got != "1" {
		t.Fatalf("form metadata priority mismatch: %q", got)
	}
	if got := form.Metadata["section"]; got != "details" {
		t.Fatalf("form metadata section mismatch: %q", got)
	}

	fields := map[string]pkgmodel.Field{}
	var visit func(prefix string, field pkgmodel.Field)
	visit = func(prefix string, field pkgmodel.Field) {
		key := field.Name
		if prefix != "" {
			key = prefix + "." + key
		}
		fields[key] = field

		if field.Items != nil {
			visit(key, *field.Items)
		}
		for _, nested := range field.Nested {
			visit(key, nested)
		}
	}
	for _, field := range form.Fields {
		visit("", field)
	}

	expectMetadata := map[string]map[string]string{
		"name": {
			"cssClass":    "fg-field--name",
			"helpText":    "Shown to customers",
			"placeholder": "Give it a friendly name",
			"widget":      "textarea",
		},
		"tags": {
			"cssClass":      "fg-array--tags",
			"placeholder":   "Add tag",
			"repeaterLabel": "Tag",
		},
		"tags.tagsItem": {
			"badge":    "info",
			"cssClass": "fg-array__item",
		},
		"settings": {
			"accordion": "true",
			"cssClass":  "fg-fieldset--settings",
		},
		"settings.enabled": {
			"hideLabel": "true",
			"label":     "Enable widget",
		},
		"settings.threshold": {
			"helpText":  "Controls the debounce window",
			"inputType": "range",
			"precision": "2",
			"unit":      "ms",
		},
	}

	for path, wantMeta := range expectMetadata {
		field, ok := fields[path]
		if !ok {
			t.Fatalf("expected field %q in widget form", path)
		}
		if diff := testsupport.CompareGolden(wantMeta, field.Metadata); diff != "" {
			t.Fatalf("field %q metadata mismatch (-want +got):\n%s", path, diff)
		}
	}
}

func TestBuilder_Relationships(t *testing.T) {
	operations := testsupport.MustLoadOperations(t, filepath.Join("../openapi", "testdata", "relationships_operations.golden.json"))
	op := operations["createArticle"]

	builder := pkgmodel.NewBuilder()
	form, err := builder.Build(op)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	goldenPath := filepath.Join("testdata", "create_article_formmodel.golden.json")
	testsupport.WriteFormModel(t, goldenPath, form)
	want := testsupport.MustLoadFormModel(t, goldenPath)

	stripRelationships(&form)
	stripRelationships(&want)

	if diff := testsupport.CompareGolden(want, form); diff != "" {
		t.Fatalf("relationships form mismatch (-want +got):\n%s", diff)
	}

	fields := map[string]pkgmodel.Field{}
	var visit func(prefix string, field pkgmodel.Field)
	visit = func(prefix string, field pkgmodel.Field) {
		key := field.Name
		if prefix != "" {
			key = prefix + "." + key
		}
		fields[key] = field

		if field.Items != nil {
			visit(key, *field.Items)
		}
		for _, nested := range field.Nested {
			visit(key, nested)
		}
	}
	for _, field := range form.Fields {
		visit("", field)
	}

	assertRelationshipField := func(name, wantType, wantInput, wantCardinality string) {
		field, ok := fields[name]
		if !ok {
			t.Fatalf("expected field %q in relationships form", name)
		}
		if got := field.Metadata["relationship.type"]; got != wantType {
			t.Fatalf("%s metadata relationship.type mismatch: want %q, got %q", name, wantType, got)
		}
		if got := field.UIHints["input"]; got != wantInput {
			t.Fatalf("%s ui hint input mismatch: want %q, got %q", name, wantInput, got)
		}
		if got := field.UIHints["cardinality"]; got != wantCardinality {
			t.Fatalf("%s ui hint cardinality mismatch: want %q, got %q", name, wantCardinality, got)
		}
	}

	assertRelationshipField("author_id", "belongsTo", "select", "one")
	assertRelationshipField("author", "belongsTo", "subform", "one")
	assertRelationshipField("manager_id", "hasOne", "select", "one")
	assertRelationshipField("manager", "hasOne", "subform", "one")
	assertRelationshipField("tags", "hasMany", "collection", "many")
	assertRelationshipField("tags.tagsItem", "hasMany", "subform", "many")

	if field := fields["title"]; field.Metadata != nil {
		t.Fatalf("expected title field to remain metadata-free, got %#v", field.Metadata)
	}
}
