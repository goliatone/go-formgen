package model_test

import (
	"path/filepath"
	"testing"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestBuilder_CreatePet(t *testing.T) {
	t.Skip("pending implementation")

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
}
