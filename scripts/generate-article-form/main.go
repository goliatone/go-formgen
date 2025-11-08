package main

import (
	"context"
	"fmt"
	"os"

	formgen "github.com/goliatone/formgen"
	pkgopenapi "github.com/goliatone/formgen/pkg/openapi"
)

func main() {
	ctx := context.Background()

	const (
		schemaPath   = "client/data/schema.json"
		operationID  = "createArticle"
		rendererName = "vanilla"
		outputPath   = "client/dev/data/article-form.html"
	)

	source := pkgopenapi.SourceFromFile(schemaPath)
	html, err := formgen.GenerateHTML(ctx, source, operationID, rendererName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate form: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, html, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated article form HTML (%d bytes) → %s\n", len(html), outputPath)
}
