//go:build example

package examples_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var examplePackages = []struct {
	name string
	dir  string
	pkg  string
}{
	{name: "basic", pkg: "./examples/basic"},
	{name: "cli", pkg: "./examples/cli"},
	{name: "http", dir: "examples/http", pkg: "."},
	{name: "multi-renderer", pkg: "./examples/multi-renderer"},
}

func TestExamplesCompile(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	for _, example := range examplePackages {
		example := example
		t.Run(example.name, func(t *testing.T) {
			cmd := exec.Command(goBin(), "build", example.pkg)
			cmd.Dir = repoRoot
			if example.dir != "" {
				cmd.Dir = filepath.Join(repoRoot, example.dir)
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go build %s: %v\n%s", example.pkg, err, output)
			}
		})
	}
}

func goBin() string {
	if goBin := os.Getenv("GO_BIN"); goBin != "" {
		return goBin
	}
	return "go"
}
