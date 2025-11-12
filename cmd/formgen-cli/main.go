package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
	"github.com/goliatone/formgen/pkg/orchestrator"
)

func main() {
	opID := flag.String("operation", "createArticle", "operation ID to render")
	renderer := flag.String("renderer", "vanilla", "renderer to use")
	output := flag.String("output", "", "output file (stdout if empty)")
	source := flag.String("source", "client/data/schema.json", "OpenAPI document path or URL")
	flag.Parse()

	ctx := context.Background()

	src := parseSource(*source)
	if src == nil {
		log.Fatalf("invalid source: %q", *source)
	}

	gen := orchestrator.New()

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
