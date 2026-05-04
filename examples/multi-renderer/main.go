package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/goliatone/go-formgen"
	"github.com/goliatone/go-formgen/examples/internal/exampleutil"
	"github.com/goliatone/go-formgen/internal/safefile"
	pkgopenapi "github.com/goliatone/go-formgen/pkg/openapi"
	"github.com/goliatone/go-formgen/pkg/orchestrator"
	"github.com/goliatone/go-formgen/pkg/render"
	"github.com/goliatone/go-formgen/pkg/renderers/preact"
	"github.com/goliatone/go-formgen/pkg/renderers/vanilla"
)

const preactRenderer = "preact"

func main() {
	ctx := context.Background()
	fixture := exampleutil.FixturePath("petstore.json")
	outputDir := flag.String("output", defaultOutputDir(), "Directory to write renderer outputs")
	flag.Parse()

	if err := safefile.MkdirAll(*outputDir); err != nil {
		log.Fatalf("mkdir output: %v", err)
	}

	registry := render.NewRegistry()
	registry.MustRegister(mustVanilla())
	registry.MustRegister(mustPreact())

	generator := formgen.NewOrchestrator(
		orchestrator.WithRegistry(registry),
	)

	doc, err := formgen.NewLoader(pkgopenapi.WithDefaultSources()).
		Load(ctx, pkgopenapi.SourceFromFile(fixture))
	if err != nil {
		log.Fatal(err)
	}

	assetsCopied := false

	for _, name := range registry.List() {
		output, err := generator.Generate(ctx, orchestrator.Request{
			Document:    &doc,
			OperationID: "createPet",
			Renderer:    name,
		})
		if err != nil {
			log.Printf("renderer %s failed: %v", name, err)
			continue
		}

		path, err := writeOutput(*outputDir, name, output)
		if err != nil {
			log.Printf("write %s output: %v", name, err)
			continue
		}

		if name == preactRenderer && !assetsCopied {
			if err := copyAssets(preact.AssetsFS(), filepath.Join(*outputDir, "assets")); err != nil {
				log.Printf("copy assets: %v", err)
			} else {
				assetsCopied = true
			}
		}

		log.Printf("%s output written to %s (%d bytes)", name, path, len(output))
	}
}

func mustVanilla() render.Renderer {
	r, err := vanilla.New(vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()))
	if err != nil {
		log.Fatal(err)
	}
	return r
}

func mustPreact() render.Renderer {
	r, err := preact.New()
	if err != nil {
		log.Fatal(err)
	}
	return r
}

func writeOutput(dir, name string, data []byte) (string, error) {
	file := filepath.Join(dir, fmt.Sprintf("%s.html", name))
	if err := safefile.WriteFile(file, data); err != nil {
		return "", err
	}
	return file, nil
}

func copyAssets(store fs.FS, dest string) error {
	return fs.WalkDir(store, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "assets/")
		if rel == path {
			rel = strings.TrimPrefix(path, "assets")
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return safefile.MkdirAll(target)
		}
		data, readErr := fs.ReadFile(store, path)
		if readErr != nil {
			return readErr
		}
		if err := safefile.MkdirAll(filepath.Dir(target)); err != nil {
			return err
		}
		return safefile.WriteFile(target, data)
	})
}

func defaultOutputDir() string {
	fixturesDir := filepath.Dir(exampleutil.FixturePath("petstore.json"))
	examplesDir := filepath.Dir(fixturesDir)
	return filepath.Join(examplesDir, "multi-renderer", "out")
}
