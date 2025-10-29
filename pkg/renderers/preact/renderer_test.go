package preact_test

import (
	"path/filepath"
	"testing"

	"github.com/goliatone/formgen/pkg/renderers/preact"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestRenderer_RenderContract(t *testing.T) {
	t.Helper()

	renderer, err := preact.New()
	if err != nil {
		t.Fatalf("preact.New: %v", err)
	}

	if got := renderer.Name(); got != "preact" {
		t.Fatalf("unexpected renderer name: %s", got)
	}
	if got := renderer.ContentType(); got != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", got)
	}

	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))
	output, err := renderer.Render(testsupport.Context(), form)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := filepath.Join("testdata", "form_output.golden.html")
	if testsupport.WriteMaybeGolden(t, goldenPath, output) {
		return
	}

	want := testsupport.MustReadGolden(t, goldenPath)
	if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
		t.Fatalf("output mismatch (-want +got):\n%s", diff)
	}
}
