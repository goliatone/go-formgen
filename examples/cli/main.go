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
	"strings"
	"time"

	"github.com/goliatone/go-formgen"
	"github.com/goliatone/go-formgen/examples/internal/exampleutil"
	"github.com/goliatone/go-formgen/internal/safefile"
	"github.com/goliatone/go-formgen/pkg/model"
	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/orchestrator"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/preact"
	"github.com/goliatone/go-formgen/pkg/renderers/tui"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
)

type cliConfig struct {
	source     string
	operation  string
	renderer   string
	output     string
	timeout    time.Duration
	inspect    bool
	tuiFormat  string
	tuiNoFetch bool
}

func main() {
	cfg := parseFlags()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	registry := render.NewRegistry()
	registry.MustRegister(mustVanilla())
	registry.MustRegister(mustPreact())

	tuiOpts := []tui.Option{tui.WithOutputFormat(parseTUIFormat(cfg.tuiFormat))}
	if !cfg.tuiNoFetch {
		tuiOpts = append(tuiOpts, tui.WithHTTPClient(http.DefaultClient))
	}
	if tuiRenderer, err := tui.New(tuiOpts...); err == nil {
		registry.MustRegister(tuiRenderer)
	}

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
		orchestrator.WithDefaultRenderer(cfg.renderer),
	)

	source, _, err := exampleutil.ResolveSource(cfg.source)
	if err != nil {
		log.Fatalf("resolve source: %v", err)
	}

	if !registry.Has(cfg.renderer) {
		log.Fatalf("renderer %q not registered (available: %v)", cfg.renderer, registry.List())
	}

	document, err := loader.Load(ctx, source)
	if err != nil {
		log.Fatalf("load document: %v", err)
	}

	request := orchestrator.Request{
		Document:    &document,
		OperationID: cfg.operation,
		Renderer:    cfg.renderer,
	}

	html, err := generator.Generate(ctx, request)
	if err != nil {
		log.Fatalf("generate: %v", err)
	}

	if cfg.inspect {
		form, inspectErr := buildFormModel(ctx, parser, builder, document, cfg.operation)
		if inspectErr != nil {
			log.Printf("inspect form: %v", inspectErr)
		} else if err := writeInspection(os.Stderr, form); err != nil {
			log.Printf("write inspection: %v", err)
		}
	}

	if cfg.output == "" {
		fmt.Println(string(html))
		return
	}

	if err := writeFile(cfg.output, html); err != nil {
		log.Fatalf("write output: %v", err)
	}
	log.Printf("wrote %d bytes to %s", len(html), cfg.output)
}

func parseFlags() cliConfig {
	defaultSource := exampleutil.FixturePath("petstore.json")

	sourceFlag := flag.String("source", defaultSource, "Path or URL to an OpenAPI document")
	operationFlag := flag.String("operation", "createPet", "Operation ID to render")
	rendererFlag := flag.String("renderer", "vanilla", "Renderer to use (vanilla, preact, tui)")
	outputFlag := flag.String("output", "", "Optional file path for the generated markup (stdout when empty)")
	timeoutFlag := flag.Duration("timeout", 15*time.Second, "Generation timeout")
	inspectFlag := flag.Bool("inspect", false, "Print form metadata/UI hints as JSON (stderr)")
	tuiFormatFlag := flag.String("tui-format", "json", "TUI output format (json, form, pretty)")
	tuiNoFetch := flag.Bool("tui-no-fetch", false, "Disable relationship HTTP fetches for TUI")
	flag.Parse()

	return cliConfig{
		source:     *sourceFlag,
		operation:  *operationFlag,
		renderer:   *rendererFlag,
		output:     *outputFlag,
		timeout:    *timeoutFlag,
		inspect:    *inspectFlag,
		tuiFormat:  *tuiFormatFlag,
		tuiNoFetch: *tuiNoFetch,
	}
}

func parseTUIFormat(raw string) tui.OutputFormat {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "form", "url", "urlencoded":
		return tui.OutputFormatFormURLEncoded
	case "pretty", "text":
		return tui.OutputFormatPrettyText
	default:
		return tui.OutputFormatJSON
	}
}

func writeFile(path string, data []byte) error {
	if err := safefile.MkdirAll(filepath.Dir(path)); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := safefile.WriteFile(path, data); err != nil {
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
	form, err := builder.Build(pkgopenapi.FormFromOperation(op))
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
