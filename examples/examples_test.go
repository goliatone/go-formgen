//go:build example

package examples_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

var examplePackages = []string{
	"./examples/basic",
	"./examples/cli",
	"./examples/http",
	"./examples/multi-renderer",
}

func TestExamplesCompile(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	for _, pkg := range examplePackages {
		pkg := pkg
		t.Run(pkg, func(t *testing.T) {
			cmd := exec.Command("/Users/goliatone/.g/go/bin/go", "build", pkg)
			cmd.Dir = repoRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go build %s: %v\n%s", pkg, err, output)
			}
		})
	}
}
