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

	if len(form.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(form.Fields))
	}

	expectations := map[string][]pkgmodel.ValidationRule{
		"age": {
			{Kind: pkgmodel.ValidationRuleMin, Params: map[string]string{"value": "0"}},
			{Kind: pkgmodel.ValidationRuleMax, Params: map[string]string{"value": "25"}},
		},
		"name": {
			{Kind: pkgmodel.ValidationRuleMinLength, Params: map[string]string{"value": "3"}},
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "64"}},
			{Kind: pkgmodel.ValidationRulePattern, Params: map[string]string{"expr": "^[A-Za-z ]+$"}},
		},
		"tag": {
			{Kind: pkgmodel.ValidationRuleMaxLength, Params: map[string]string{"value": "32"}},
		},
	}

	for _, field := range form.Fields {
		wantRules, ok := expectations[field.Name]
		if !ok {
			continue
		}
		if diff := testsupport.CompareGolden(wantRules, field.Validations); diff != "" {
			t.Fatalf("field %q validations mismatch (-want +got):\n%s", field.Name, diff)
		}
	}
}
