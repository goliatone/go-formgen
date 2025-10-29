package vanilla_test

import (
	"path/filepath"
	"testing"

	"github.com/goliatone/formgen/pkg/renderers/vanilla"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestRenderer_RenderContract(t *testing.T) {
	t.Skip("pending implementation")

	form := testsupport.MustLoadFormModel(t, filepath.Join("testdata", "form_model.json"))

	renderer, err := vanilla.New()
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

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
