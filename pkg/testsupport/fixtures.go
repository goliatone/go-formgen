package testsupport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

// LoadDocument reads a fixture and builds an openapi.Document using a file
// source. Testing helpers panic on failure to keep contract tests concise.
func LoadDocument(t *testing.T, path string) pkgopenapi.Document {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	doc, err := pkgopenapi.NewDocument(pkgopenapi.SourceFromFile(path), data)
	if err != nil {
		t.Fatalf("new document: %v", err)
	}
	return doc
}

// MustLoadOperations loads a JSON golden file into the provided map pointer.
func MustLoadOperations(t *testing.T, path string) map[string]pkgopenapi.Operation {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("load golden: %v", err)
	}
	var out map[string]pkgopenapi.Operation
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	return out
}

// MustLoadFormModel loads a JSON golden file into a FormModel structure.
func MustLoadFormModel(t *testing.T, path string) pkgmodel.FormModel {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("load golden: %v", err)
	}
	var out pkgmodel.FormModel
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	return out
}

// WriteFormModel writes a form model golden when UPDATE_GOLDENS is enabled.
func WriteFormModel(t *testing.T, path string, value pkgmodel.FormModel) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDENS") == "" {
		return
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal form model: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
}

// WriteGolden writes arbitrary data to a golden file when UPDATE_GOLDENS is set.
func WriteGolden(t *testing.T, path string, value any) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDENS") == "" {
		return
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
}

// CompareGolden returns a diff string if the values differ.
func CompareGolden(want, got any) string {
	return cmp.Diff(want, got)
}

// MustReadGolden reads a golden file and returns its raw bytes.
func MustReadGolden(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return data
}

// WriteMaybeGolden updates a golden file when UPDATE_GOLDENS is set. Returns
// true if the golden was written (test should exit early).
func WriteMaybeGolden(t *testing.T, path string, data []byte) bool {
	t.Helper()
	if os.Getenv("UPDATE_GOLDENS") == "" {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	return true
}

// Context returns a background context for tests.
func Context() context.Context {
	return context.Background()
}
