package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/goliatone/formgen"
	"github.com/goliatone/formgen/examples/internal/exampleutil"
	"github.com/goliatone/formgen/pkg/model"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/preact"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
)

func main() {
	defaultSource := exampleutil.FixturePath("petstore.json")

	var (
		sourceFlag    = flag.String("source", defaultSource, "Path or URL to an OpenAPI document")
		operationFlag = flag.String("operation", "createPet", "Operation ID to render")
		rendererFlag  = flag.String("renderer", "vanilla", "Renderer to use (vanilla, preact)")
		outputFlag    = flag.String("output", "", "Optional file path for the generated markup (stdout when empty)")
		timeoutFlag   = flag.Duration("timeout", 15*time.Second, "Generation timeout")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	registry := render.NewRegistry()
	registry.MustRegister(mustVanilla())
	registry.MustRegister(mustPreact())

	loader := formgen.NewLoader(
		pkgopenapi.WithDefaultSources(),
		pkgopenapi.WithHTTPClient(http.DefaultClient),
	)

	generator := formgen.NewOrchestrator(
		orchestrator.WithLoader(loader),
		orchestrator.WithParser(formgen.NewParser()),
		orchestrator.WithModelBuilder(model.NewBuilder()),
		orchestrator.WithRegistry(registry),
		orchestrator.WithDefaultRenderer(*rendererFlag),
	)

	source, _, err := exampleutil.ResolveSource(*sourceFlag)
	if err != nil {
		log.Fatalf("resolve source: %v", err)
	}

	if !registry.Has(*rendererFlag) {
		log.Fatalf("renderer %q not registered (available: %v)", *rendererFlag, registry.List())
	}

	request := orchestrator.Request{
		Source:      source,
		OperationID: *operationFlag,
		Renderer:    *rendererFlag,
	}

	html, err := generator.Generate(ctx, request)
	if err != nil {
		log.Fatalf("generate: %v", err)
	}

	if *outputFlag == "" {
		fmt.Println(string(html))
		return
	}

	if err := writeFile(*outputFlag, html); err != nil {
		log.Fatalf("write output: %v", err)
	}
	log.Printf("wrote %d bytes to %s", len(html), *outputFlag)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func mustVanilla() render.Renderer {
	r, err := vanilla.New(vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()))
	if err != nil {
		log.Fatalf("vanilla renderer: %v", err)
	}
	return r
}

func mustPreact() render.Renderer {
	r, err := preact.New()
	if err != nil {
		log.Fatalf("preact renderer: %v", err)
	}
	return r
}
