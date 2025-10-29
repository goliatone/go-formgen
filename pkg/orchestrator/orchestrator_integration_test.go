package orchestrator_test

import (
	"path/filepath"
	"testing"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/preact"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
	"github.com/goliatone/formgen/pkg/testsupport"
)

func TestOrchestrator_Integration_MultiRenderer(t *testing.T) {
	t.Parallel()

	ctx := testsupport.Context()
	source := pkgopenapi.SourceFromFile(filepath.Join("testdata", "petstore.yaml"))

	registry := render.NewRegistry()
	registry.MustRegister(mustVanilla(t))
	registry.MustRegister(mustPreact(t))

	orch := orchestrator.New(
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer("vanilla"),
	)

	type goldenCase struct {
		name     string
		renderer string
		golden   string
	}

	cases := []goldenCase{
		{name: "DefaultRenderer", renderer: "", golden: "create_pet_vanilla.golden.html"},
		{name: "ExplicitVanilla", renderer: "vanilla", golden: "create_pet_vanilla.golden.html"},
		{name: "PreactRenderer", renderer: "preact", golden: "create_pet_preact.golden.html"},
	}

	collected := make(map[string][]byte)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			output, err := orch.Generate(ctx, orchestrator.Request{
				Source:      source,
				OperationID: "createPet",
				Renderer:    tc.renderer,
			})
			if err != nil {
				t.Fatalf("generate (%s): %v", tc.name, err)
			}

			if prior, ok := collected[tc.golden]; ok {
				if diff := testsupport.CompareGolden(string(prior), string(output)); diff != "" {
					t.Fatalf("renderer output mismatch (-want +got):\n%s", diff)
				}
			} else {
				copied := append([]byte(nil), output...)
				collected[tc.golden] = copied
			}

			goldenPath := filepath.Join("testdata", tc.golden)
			if testsupport.WriteMaybeGolden(t, goldenPath, output) {
				return
			}

			want := testsupport.MustReadGolden(t, goldenPath)
			if diff := testsupport.CompareGolden(string(want), string(output)); diff != "" {
				t.Fatalf("golden mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func mustVanilla(t *testing.T) render.Renderer {
	t.Helper()

	r, err := vanilla.New()
	if err != nil {
		t.Fatalf("vanilla renderer: %v", err)
	}
	return r
}

func mustPreact(t *testing.T) render.Renderer {
	t.Helper()

	r, err := preact.New()
	if err != nil {
		t.Fatalf("preact renderer: %v", err)
	}
	return r
}
