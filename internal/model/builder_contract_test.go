package model_test

import (
	"path/filepath"
	"testing"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/testsupport"
)

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
}
