package orchestrator_test

import (
	"path/filepath"
	"testing"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestOrchestrator_Generate_CreatePet(t *testing.T) {
	t.Skip("pending implementation")

	ctx := testsupport.Context()
	source := pkgopenapi.SourceFromFile(filepath.Join("testdata", "petstore.yaml"))

	gen := orchestrator.New()
	output, err := gen.Generate(ctx, orchestrator.Request{
		Source:      source,
		OperationID: "createPet",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	goldenPath := filepath.Join("testdata", "create_pet.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("output mismatch (-want +got):\n%s", diff)
	}
}
