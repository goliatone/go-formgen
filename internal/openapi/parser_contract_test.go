package openapi_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-formgen"
	"github.com/goliatone/go-formgen/pkg/testsupport"
)

func TestParser_Operations_Petstore(t *testing.T) {
	t.Skip("pending implementation")

	ctx := context.Background()
	doc := testsupport.LoadDocument(t, filepath.Join("testdata", "petstore.yaml"))
	parser := formgen.NewParser()

	got, err := parser.Operations(ctx, doc)
	if err != nil {
		t.Fatalf("operations: %v", err)
	}

	goldenPath := filepath.Join("testdata", "petstore_operations.golden.json")
	testsupport.WriteGolden(t, goldenPath, got)
	want := testsupport.MustLoadOperations(t, goldenPath)

	if diff := testsupport.CompareGolden(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestParser_Operations_Extensions(t *testing.T) {
	ctx := context.Background()
	doc := testsupport.LoadDocument(t, filepath.Join("testdata", "extensions.yaml"))
	parser := formgen.NewParser()

	got, err := parser.Operations(ctx, doc)
	if err != nil {
		t.Fatalf("operations: %v", err)
	}

	goldenPath := filepath.Join("testdata", "extensions_operations.golden.json")
	testsupport.WriteGolden(t, goldenPath, got)
	want := testsupport.MustLoadOperations(t, goldenPath)

	if diff := testsupport.CompareGolden(want, got); diff != "" {
		t.Fatalf("extensions mismatch (-want +got):\n%s", diff)
	}
}

func TestParser_Operations_Relationships(t *testing.T) {
	ctx := context.Background()
	doc := testsupport.LoadDocument(t, filepath.Join("testdata", "relationships.yaml"))
	parser := formgen.NewParser()

	got, err := parser.Operations(ctx, doc)
	if err != nil {
		t.Fatalf("operations: %v", err)
	}

	goldenPath := filepath.Join("testdata", "relationships_operations.golden.json")
	testsupport.WriteGolden(t, goldenPath, got)
	want := testsupport.MustLoadOperations(t, goldenPath)

	if diff := testsupport.CompareGolden(want, got); diff != "" {
		t.Fatalf("relationships mismatch (-want +got):\n%s", diff)
	}
}
