package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
	"github.com/goliatone/formgen/pkg/render"
	"github.com/goliatone/formgen/pkg/renderers/preact"
	"github.com/goliatone/formgen/pkg/renderers/tui"
	"github.com/goliatone/formgen/pkg/renderers/vanilla"
)

func main() {
	opID := flag.String("operation", "createArticle", "operation ID to render")
	renderer := flag.String("renderer", "vanilla", "renderer to use (vanilla, preact, tui)")
	output := flag.String("output", "", "output file (stdout if empty)")
	source := flag.String("source", "client/data/schema.json", "OpenAPI document path or URL")
	tuiFormat := flag.String("tui-format", "json", "TUI output format (json, form, pretty)")
	tuiNoFetch := flag.Bool("tui-no-fetch", false, "Disable relationship HTTP fetches for TUI")
	flag.Parse()

	ctx := context.Background()

	src := parseSource(*source)
	if src == nil {
		log.Fatalf("invalid source: %q", *source)
	}

	registry := buildRegistry(*renderer, parseTUIFormat(*tuiFormat), *tuiNoFetch)

	if !registry.Has(*renderer) {
		log.Fatalf("renderer %q not registered (available: %v)", *renderer, registry.List())
	}

	gen := orchestrator.New(orchestrator.WithRegistry(registry))

	req := orchestrator.Request{
		Source:      src,
		OperationID: *opID,
		Renderer:    *renderer,
	}

	outputHTML, err := gen.Generate(ctx, req)
	if err != nil {
		log.Fatalf("Failed to generate form: %v", err)
	}

	if *output != "" {
		if err := os.WriteFile(*output, outputHTML, 0o644); err != nil {
			log.Fatalf("Failed to write output: %v", err)
		}
		fmt.Printf("Form written to %s\n", *output)
	} else {
		fmt.Println(string(outputHTML))
	}
}

func parseSource(raw string) pkgopenapi.Source {
	path := strings.TrimSpace(raw)
	if path == "" {
		return nil
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return pkgopenapi.SourceFromURL(path)
	}
	return pkgopenapi.SourceFromFile(path)
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

func buildRegistry(targetRenderer string, format tui.OutputFormat, noFetch bool) *render.Registry {
	registry := render.NewRegistry()
	if vanillaRenderer, err := vanilla.New(); err == nil {
		registry.MustRegister(vanillaRenderer)
	}
	if preactRenderer, err := preact.New(); err == nil {
		registry.MustRegister(preactRenderer)
	}
	tuiOpts := []tui.Option{tui.WithOutputFormat(format)}
	if !noFetch {
		tuiOpts = append(tuiOpts, tui.WithHTTPClient(http.DefaultClient))
	}
	if tuiRenderer, err := tui.New(tuiOpts...); err == nil {
		registry.MustRegister(tuiRenderer)
	}
	return registry
}
