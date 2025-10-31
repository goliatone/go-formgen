package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
		inspectFlag   = flag.Bool("inspect", false, "Print form metadata/UI hints as JSON (stderr)")
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

	parser := formgen.NewParser()
	builder := model.NewBuilder()

	generator := formgen.NewOrchestrator(
		orchestrator.WithLoader(loader),
		orchestrator.WithParser(parser),
		orchestrator.WithModelBuilder(builder),
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

	document, err := loader.Load(ctx, source)
	if err != nil {
		log.Fatalf("load document: %v", err)
	}

	request := orchestrator.Request{
		Document:    &document,
		OperationID: *operationFlag,
		Renderer:    *rendererFlag,
	}

	html, err := generator.Generate(ctx, request)
	if err != nil {
		log.Fatalf("generate: %v", err)
	}

	if *inspectFlag {
		form, inspectErr := buildFormModel(ctx, parser, builder, document, *operationFlag)
		if inspectErr != nil {
			log.Printf("inspect form: %v", inspectErr)
		} else if err := writeInspection(os.Stderr, form); err != nil {
			log.Printf("write inspection: %v", err)
		}
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

func buildFormModel(ctx context.Context, parser pkgopenapi.Parser, builder model.Builder, document pkgopenapi.Document, operationID string) (model.FormModel, error) {
	operations, err := parser.Operations(ctx, document)
	if err != nil {
		return model.FormModel{}, fmt.Errorf("parse operations: %w", err)
	}
	op, ok := operations[operationID]
	if !ok {
		return model.FormModel{}, fmt.Errorf("operation %q not found", operationID)
	}
	form, err := builder.Build(op)
	if err != nil {
		return model.FormModel{}, fmt.Errorf("build form model: %w", err)
	}
	return form, nil
}

func writeInspection(out io.Writer, form model.FormModel) error {
	type fieldSummary struct {
		Name     string            `json:"name"`
		Type     model.FieldType   `json:"type"`
		Metadata map[string]string `json:"metadata,omitempty"`
		UIHints  map[string]string `json:"uiHints,omitempty"`
	}

	summary := struct {
		OperationID string            `json:"operationId"`
		Endpoint    string            `json:"endpoint"`
		Method      string            `json:"method"`
		Metadata    map[string]string `json:"metadata,omitempty"`
		UIHints     map[string]string `json:"uiHints,omitempty"`
		Fields      []fieldSummary    `json:"fields,omitempty"`
	}{
		OperationID: form.OperationID,
		Endpoint:    form.Endpoint,
		Method:      form.Method,
		Metadata:    form.Metadata,
		UIHints:     form.UIHints,
	}

	for _, field := range form.Fields {
		summary.Fields = append(summary.Fields, fieldSummary{
			Name:     field.Name,
			Type:     field.Type,
			Metadata: field.Metadata,
			UIHints:  field.UIHints,
		})
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
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
