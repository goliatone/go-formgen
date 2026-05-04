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
	goBin := exampleGoBin()

	examples := map[string]struct {
		dir string
		pkg string
	}{
		"cli":            {pkg: "./examples/cli"},
		"http":           {dir: "examples/http", pkg: "."},
		"multi-renderer": {pkg: "./examples/multi-renderer"},
	}

	for name, example := range examples {
		name, example := name, example
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			outputPath := filepath.Join(tempDir, name)

			cmd := exec.Command(goBin, "build", "-o", outputPath, example.pkg)
			cmd.Dir = root
			if example.dir != "" {
				cmd.Dir = filepath.Join(root, example.dir)
			}
			cmd.Env = os.Environ()

			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build %s: %v\n%s", example.pkg, err, string(out))
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

func exampleGoBin() string {
	if goBin := os.Getenv("GO_BIN"); goBin != "" {
		return goBin
	}
	return "go"
}
