package formgen

import (
	"io/fs"
	"strings"
	"testing"
)

func TestRuntimeAssetsFSContainsRuntimeBundle(t *testing.T) {
	fsys := RuntimeAssetsFS()
	_, err := fs.ReadFile(fsys, "formgen-relationships.min.js")
	if err != nil {
		t.Fatalf("expected runtime bundle to be readable: %v", err)
	}
}

func TestRuntimeAssetsFSBehaviorsBundleIncludesAutoResize(t *testing.T) {
	fsys := RuntimeAssetsFS()
	data, err := fs.ReadFile(fsys, "formgen-behaviors.min.js")
	if err != nil {
		t.Fatalf("expected behaviors bundle to be readable: %v", err)
	}
	if !strings.Contains(string(data), "autoResize") {
		t.Fatalf("expected behaviors bundle to include autoResize")
	}
}
