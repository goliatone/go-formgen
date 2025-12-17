package vanilla

import (
	"io/fs"
	"strings"
	"testing"
)

func TestAssetsFSBehaviorsBundleIncludesAutoResize(t *testing.T) {
	data, err := fs.ReadFile(AssetsFS(), "formgen-behaviors.min.js")
	if err != nil {
		t.Fatalf("expected behaviors bundle to be readable: %v", err)
	}
	if !strings.Contains(string(data), "autoResize") {
		t.Fatalf("expected behaviors bundle to include autoResize")
	}
}

