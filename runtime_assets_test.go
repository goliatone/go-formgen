package formgen

import (
	"io/fs"
	"testing"
)

func TestRuntimeAssetsFSContainsRuntimeBundle(t *testing.T) {
	fsys := RuntimeAssetsFS()
	_, err := fs.ReadFile(fsys, "formgen-relationships.min.js")
	if err != nil {
		t.Fatalf("expected runtime bundle to be readable: %v", err)
	}
}

