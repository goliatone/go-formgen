package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing/fstest"

	formgen "github.com/goliatone/formgen"
	"github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/uischema"
)

const snapshotRendererName = "form-model-snapshot"

type snapshotRenderer struct {
	path string
}

func (r *snapshotRenderer) Name() string {
	return snapshotRendererName
}

func (r *snapshotRenderer) ContentType() string {
	return "application/json"
}

func (r *snapshotRenderer) Render(_ context.Context, form model.FormModel, _ render.RenderOptions) ([]byte, error) {
	payload, err := json.MarshalIndent(form, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(r.path, payload, 0o644); err != nil {
		return nil, err
	}
	return payload, nil
}

func main() {
	var (
		schemaPath   = flag.String("schema", "client/data/schema.json", "OpenAPI schema path")
		uiSchemaPath = flag.String("uischema", "pkg/uischema/ui/schema/createArticle.json", "UI schema file")
		operationID  = flag.String("operation", "createArticle", "operation ID to snapshot")
		outputPath   = flag.String("output", "pkg/renderers/vanilla/testdata/form_model.json", "output path for the serialized form model")
	)
	flag.Parse()

	ctx := context.Background()

	registry := render.NewRegistry()
	registry.MustRegister(&snapshotRenderer{path: *outputPath})

	decorator, err := loadUIDecorator(*uiSchemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load UI schema: %v\n", err)
		os.Exit(1)
	}

	orch := orchestrator.New(
		orchestrator.WithLoader(formgen.NewLoader(pkgopenapi.WithDefaultSources())),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(snapshotRendererName),
		orchestrator.WithUIDecorators(decorator),
	)

	_, err = orch.Generate(ctx, orchestrator.Request{
		Source:      pkgopenapi.SourceFromFile(*schemaPath),
		OperationID: *operationID,
		Renderer:    snapshotRendererName,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to snapshot form model: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Wrote form model snapshot to %s\n", *outputPath)
}

func loadUIDecorator(path string) (model.Decorator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ui schema: %w", err)
	}
	fs := fstest.MapFS{
		filepath.Base(path): {
			Data: data,
		},
	}
	store, err := uischema.LoadFS(fs)
	if err != nil {
		return nil, fmt.Errorf("load ui schema: %w", err)
	}
	return uischema.NewDecorator(store), nil
}
