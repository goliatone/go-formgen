//go:build example
// +build example

package examples_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExampleBinariesBuild(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	goBin := "/Users/goliatone/.g/go/bin/go"

	examples := map[string]string{
		"cli":            "./examples/cli",
		"http":           "./examples/http",
		"multi-renderer": "./examples/multi-renderer",
	}

	for name, pkg := range examples {
		name, pkg := name, pkg
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, name)

			cmd := exec.Command(goBin, "build", "-o", outputPath, pkg)
			cmd.Dir = root
			cmd.Env = os.Environ()

			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build %s: %v\n%s", pkg, err, string(out))
			}
			if _, err := os.Stat(outputPath); err != nil {
				t.Fatalf("compiled binary not found at %s: %v", outputPath, err)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("examples: unable to resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}
