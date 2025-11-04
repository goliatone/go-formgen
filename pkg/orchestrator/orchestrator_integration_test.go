package orchestrator_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	pkgmodel "github.com/goliatone/formgen/pkg/model"
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
	defaultSource := pkgopenapi.SourceFromFile(filepath.Join("testdata", "petstore.yaml"))

	registry := render.NewRegistry()
	registry.MustRegister(mustVanilla(t))
	registry.MustRegister(mustPreact(t))
	names := registry.List()
	if len(names) != 2 || names[0] != "preact" || names[1] != "vanilla" {
		t.Fatalf("unexpected registry names: %v", names)
	}

	orch := orchestrator.New(
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer("vanilla"),
		orchestrator.WithUISchemaFS(nil),
	)

	type goldenCase struct {
		name        string
		renderer    string
		sourcePath  string
		operationID string
		golden      string
		formGolden  string
	}

	cases := []goldenCase{
		{
			name:        "DefaultRenderer",
			renderer:    "",
			operationID: "createPet",
			golden:      "create_pet_vanilla.golden.html",
			formGolden:  "create_pet_formmodel.golden.json",
		},
		{
			name:        "ExplicitVanilla",
			renderer:    "vanilla",
			operationID: "createPet",
			golden:      "create_pet_vanilla.golden.html",
			formGolden:  "create_pet_formmodel.golden.json",
		},
		{
			name:        "PreactRenderer",
			renderer:    "preact",
			operationID: "createPet",
			golden:      "create_pet_preact.golden.html",
			formGolden:  "create_pet_formmodel.golden.json",
		},
		{
			name:        "WidgetVanilla",
			renderer:    "vanilla",
			sourcePath:  "extensions.yaml",
			operationID: "createWidget",
			golden:      "create_widget_vanilla.golden.html",
			formGolden:  "create_widget_formmodel.golden.json",
		},
		{
			name:        "WidgetPreact",
			renderer:    "preact",
			sourcePath:  "extensions.yaml",
			operationID: "createWidget",
			golden:      "create_widget_preact.golden.html",
			formGolden:  "create_widget_formmodel.golden.json",
		},
	}

	collected := make(map[string][]byte)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			src := defaultSource
			if tc.sourcePath != "" {
				src = pkgopenapi.SourceFromFile(filepath.Join("testdata", tc.sourcePath))
			}

			output, err := orch.Generate(ctx, orchestrator.Request{
				Source:      src,
				OperationID: tc.operationID,
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

			if tc.renderer == "preact" {
				formGolden := tc.formGolden
				if formGolden == "" {
					formGolden = "create_pet_formmodel.golden.json"
				}
				assertPreactFormModel(t, output, formGolden)
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

func assertPreactFormModel(t *testing.T, html []byte, golden string) {
	t.Helper()

	const scriptPrefix = `<script id="formgen-preact-data" type="application/json">`

	start := bytes.Index(html, []byte(scriptPrefix))
	if start < 0 {
		t.Fatalf("preact html missing data script tag")
	}
	start += len(scriptPrefix)
	end := bytes.Index(html[start:], []byte("</script>"))
	if end < 0 {
		t.Fatalf("preact html missing closing script tag")
	}

	payload := html[start : start+end]
	payload = bytes.TrimSpace(payload)

	var form pkgmodel.FormModel
	if err := json.Unmarshal(payload, &form); err != nil {
		t.Fatalf("unmarshal preact payload: %v", err)
	}

	goldenPath := filepath.Join("testdata", golden)
	testsupport.WriteFormModel(t, goldenPath, form)
	want := testsupport.MustLoadFormModel(t, goldenPath)

	if diff := testsupport.CompareGolden(want, form); diff != "" {
		t.Fatalf("preact form model mismatch (-want +got):\n%s", diff)
	}
}

func mustPreact(t *testing.T) render.Renderer {
	t.Helper()

	r, err := preact.New()
	if err != nil {
		t.Fatalf("preact renderer: %v", err)
	}
	return r
}
