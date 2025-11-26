# formgen

A Go library that turns OpenAPI 3.x operations into ready-to-embed forms. It loads and parses an OpenAPI document, builds a typed form model, then renders HTML (or interactive CLI prompts) through pluggable renderers.

## Documentation

- [Architecture & Guides](go-form-gen.md)
- [API Reference](https://pkg.go.dev/github.com/goliatone/formgen)
- [Task & Roadmap Notes](TODO.md)

## Features

- OpenAPI 3.x → typed `FormModel` (`Field`, `ValidationRule`, metadata, relationships)
- Offline-first loaders (file, `fs.FS`, optional HTTP) and wrapper types instead of raw `kin-openapi`
- Pluggable renderers: vanilla HTML, Preact (with embedded assets), and a TUI/CLI renderer
- Renderer registry + orchestrator helpers for one-shot HTML generation or custom wiring
- UI schema + extension metadata passed through to renderers for layout, chrome, behaviours, and theming overrides
- Render options for subsets, prefill/provenance, disabled/readonly flags, and server errors (go-errors compatible)
- Theme provider/partials + widget registry so adapters (settings/media/export) can inject templates and evaluators at runtime
- Contract-tested goldens for parser, builder, and renderer outputs

## Installation

```bash
go get github.com/goliatone/formgen
```

Requires Go 1.21+.

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/goliatone/formgen"
	"github.com/goliatone/formgen/pkg/openapi"
)

func main() {
	ctx := context.Background()

	html, err := formgen.GenerateHTML(
		ctx,
		openapi.SourceFromFile("examples/fixtures/petstore.json"),
		"createPet",
		"vanilla", // or "preact", "tui"
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(html))
}
```

## How It Works

1) Loader resolves an OpenAPI document (file, `fs.FS`, or HTTP).  
2) Parser wraps operations into `formgen.Document`/`Operation` types.  
3) Builder emits a typed `FormModel` (`Fields`, validation rules, metadata, relationships).  
4) Renderer turns the model into HTML (vanilla/Preact) or CLI prompts (TUI).  
5) Optional UI schema decorators and endpoint overrides enrich metadata and layout hints.

Use the orchestrator when you want all stages wired for you, or inject your own loader/parser/renderer implementations via options.

## Renderers

- `vanilla`: Server-rendered HTML using Go templates. Accepts `WithTemplatesFS`/`WithTemplatesDir` for custom bundles.
- `preact`: Hydrate-able markup plus embedded JS/CSS (`preact.AssetsFS()`); `WithAssetURLPrefix` rewrites asset URLs for HTTP servers or CDNs.
- `tui`: Interactive terminal prompts (JSON/form-url-encoded/pretty output). Run with `--renderer tui` in the CLI example or register it in the renderer registry.

```go
registry := render.NewRegistry()
registry.MustRegister(vanilla.Must(vanilla.WithTemplatesFS(formgen.EmbeddedTemplates())))
registry.MustRegister(preact.New())

gen := formgen.NewOrchestrator(
	orchestrator.WithRegistry(registry),
	orchestrator.WithDefaultRenderer("vanilla"),
	orchestrator.WithWidgetRegistry(widgets.NewRegistry()), // adapters can RegisterWidget later
)
```

## Programmatic Usage

Compose your own pipeline when you need custom sources, decorators, or render options:

```go
ctx := context.Background()

loader := formgen.NewLoader(openapi.WithDefaultSources())
registry := render.NewRegistry()
registry.MustRegister(vanilla.Must(vanilla.WithTemplatesFS(formgen.EmbeddedTemplates())))

gen := formgen.NewOrchestrator(
	orchestrator.WithLoader(loader),
	orchestrator.WithParser(formgen.NewParser()),
	orchestrator.WithModelBuilder(model.NewBuilder()),
	orchestrator.WithRegistry(registry),
	orchestrator.WithEndpointOverrides([]formgen.EndpointOverride{
		{Field: "owner_id", Endpoint: "https://api.example.com/owners"},
	}),
)

output, err := gen.Generate(ctx, orchestrator.Request{
	Source:      openapi.SourceFromFile("openapi.json"),
	OperationID: "createPet",
	RenderOptions: render.RenderOptions{
		Method: "PATCH",
		Subset: render.FieldSubset{Groups: []string{"notifications"}}, // render a tab/section subset
		Values: map[string]any{
			"name": render.ValueWithProvenance{
				Value:      "Fido",
				Provenance: "tenant default",
				Disabled:   true,
			},
		},
		Errors: map[string][]string{"slug": {"Taken"}}, // surface server errors (go-errors compatible)
	},
})
```

UI schema files can also be injected (`orchestrator.WithUISchemaFS`) to control layout, sections, and action bars without editing templates.

## Examples & CLI

- `go run ./examples/basic` – minimal end-to-end HTML generation
- `go run ./examples/multi-renderer` – emit outputs for each registered renderer (copies Preact assets)
- `go run ./examples/http` – tiny HTTP server serving rendered forms and assets; supports subsets (`?groups=notifications`), renderer switches, prefill/errors, and theme overrides
- `go run ./cmd/formgen-cli --renderer tui --operation createPet --source examples/fixtures/petstore.json --tui-format json`

## Templates & Assets

- Reuse `formgen.EmbeddedTemplates()` for vanilla or supply your own via `WithTemplatesFS/Dir`.
- Preact ships embedded assets (`preact.AssetsFS()`); copy them to your static host or set `WithAssetURLPrefix` to point at a CDN/handler.
- Component overrides and UI schema metadata (`placeholder`, `helpText`, `layout.*`, etc.) flow through to renderers for fine-grained control.

## Testing & Tooling

```
./taskfile dev:test            # go test ./... with cached modules
./taskfile dev:test:contracts  # contract + integration suites (renderer coverage)
./taskfile dev:test:examples   # compile example binaries with -tags example (vanilla + Preact)
./taskfile dev:ci              # vet + optional golangci-lint (includes example build)
./taskfile dev:goldens         # regenerate vanilla/Preact snapshots via scripts/update_goldens.sh
./scripts/update_goldens.sh    # refresh vanilla/Preact snapshots and rerun example builds
```

## Troubleshooting

- Stay offline? Omit HTTP loader options and load from files/embedded assets.
- Template validation failures? Reuse `formgen.EmbeddedTemplates()` (vanilla) or `preact.TemplatesFS()`.
- Renderer not found? Ensure it is registered in the `render.Registry` and set as the default when using the orchestrator helpers.

## License

MIT © Goliat One
