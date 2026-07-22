package formgen

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
)

func TestRuntimeAssetsFSContainsRuntimeBundle(t *testing.T) {
	fsys := RuntimeAssetsFS()
	data, err := fs.ReadFile(fsys, "formgen-relationships.min.js")
	if err != nil {
		t.Fatalf("expected runtime bundle to be readable: %v", err)
	}
	assertBundleExposesFormgenController(t, data)
	assertBundlePreservesAbsoluteURISchemes(t, data)
}

func TestVanillaAssetsFSContainsCurrentRuntimeBundle(t *testing.T) {
	data, err := fs.ReadFile(vanilla.AssetsFS(), "formgen-relationships.min.js")
	if err != nil {
		t.Fatalf("expected vanilla runtime bundle to be readable: %v", err)
	}
	assertBundleExposesFormgenController(t, data)
	assertBundlePreservesAbsoluteURISchemes(t, data)
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

func assertBundleExposesFormgenController(t *testing.T, data []byte) {
	t.Helper()
	bundle := string(data)
	if !strings.Contains(bundle, "Formgen") || !strings.Contains(bundle, "attach") {
		t.Fatalf("expected runtime bundle to expose window.Formgen.attach")
	}
}

func assertBundlePreservesAbsoluteURISchemes(t *testing.T, data []byte) {
	t.Helper()
	if !strings.Contains(string(data), `^[a-z][a-z0-9+.-]*:`) {
		t.Fatalf("expected runtime bundle to preserve absolute URI schemes for beforeFetch hooks")
	}
}
