# formgen

A Go library that turns OpenAPI 3.x operations into ready-to-embed forms. It loads and parses an OpenAPI document, builds a typed form model, then renders HTML (or interactive CLI prompts) through pluggable renderers.

## Documentation

- [Architecture & Guides](go-form-gen.md)
- [Styling & Customization Guide](docs/GUIDE_STYLING.md)
- [Form Customization Guide](docs/GUIDE_CUSTOMIZATION.md) — Action buttons, sections, widgets, behaviors
- [API Reference](https://pkg.go.dev/github.com/goliatone/go-formgen)
- [Task & Roadmap Notes](TODO.md)
- [JSON Schema Adapter Guide](docs/README_JSON_SCHEMA.md)

## Features

- OpenAPI 3.x → typed `FormModel` (fields, validations, relationships, metadata/UI hints)
- JSON Schema Draft 2020-12 adapter with deterministic `$ref` resolution and form discovery
- Loaders for file, `fs.FS`, or HTTP; parser wraps kin-openapi output in stable domain types and merges `allOf`
- Pluggable renderers: vanilla (Go templates), Preact (hydrated, embedded assets), and TUI/CLI
- Orchestrator wiring with renderer registry, widget registry, endpoint overrides, and visibility evaluators
- UI schema overlays (JSON/YAML) for sections, layout, icons, actions, and component overrides without touching templates
- Optional i18n: UI schema `*Key` fields + render-time localization and template helpers
- Render options: subsets (groups/tags/sections), prefill + provenance/readonly/disabled, hidden fields, server errors, visibility context
- Block-union support (`oneOf` + `_type`) for modular content blocks via JSON Schema (`x-formgen.widget=block`)
- Theme integration via `go-theme` selectors/providers + partial fallbacks; reuse or override embedded templates/assets
- Transformers (JSON presets or JS runner seam) to mutate form models before decoration
- Contract/golden tests cover parser, builder, renderers, and CLI paths

## Installation

```bash
go get github.com/goliatone/go-formgen
```

Requires Go 1.23+.

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/goliatone/go-formgen"
	"github.com/goliatone/go-formgen/pkg/openapi"
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

- `vanilla`: Server-rendered HTML using Go templates. Accepts `WithTemplatesFS`/`WithTemplatesDir` and `WithTemplateFuncs` for custom bundles/helpers.
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
		{
			OperationID: "createPet",
			FieldPath:   "owner_id",
			Endpoint: formgen.EndpointConfig{
				URL:        "https://api.example.com/owners",
				Method:     "GET",
				LabelField: "name",
				ValueField: "id",
			},
		},
	}),
	orchestrator.WithThemeProvider(myThemeProvider, "default", "light"),
	orchestrator.WithThemeFallbacks(nil),
	orchestrator.WithVisibilityEvaluator(myVisibilityEvaluator),
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

Add a transformer when you need to rename fields or inject metadata without changing the OpenAPI source:

```go
jsonPreset, _ := orchestrator.NewJSONPresetTransformerFromFS(os.DirFS("./presets"), "article.json")
gen := formgen.NewOrchestrator(orchestrator.WithSchemaTransformer(jsonPreset))
```

## Examples & CLI

- `go run ./examples/basic` – minimal end-to-end HTML generation
- `go run ./examples/multi-renderer` – emit outputs for each registered renderer (copies Preact assets)
- `go run ./examples/http` – tiny HTTP server serving rendered forms and assets; supports subsets (`?groups=notifications`), renderer switches, prefill/errors, and theme overrides
- `go run ./cmd/formgen-cli --renderer tui --operation createPet --source examples/fixtures/petstore.json --tui-format json`

When serving HTML, remember to register and set a default renderer, and ensure the matching assets/partials are reachable (vanilla embeds templates; Preact assets live in `preact.AssetsFS()`).

## Templates & Assets

- Reuse `formgen.EmbeddedTemplates()` for vanilla or supply your own via `WithTemplatesFS/Dir`.
- Preact ships embedded assets (`preact.AssetsFS()`); copy them to your static host or set `WithAssetURLPrefix` to point at a CDN/handler.
- Serve the browser runtime bundles (relationships + runtime components like `file_uploader`) from `formgen.RuntimeAssetsFS()` and mount them at `/runtime/` so `<script src="/runtime/formgen-relationships.min.js">` works.
- Component overrides and UI schema metadata (`placeholder`, `helpText`, `layout.*`, icons, actions, behaviors) flow through to renderers for fine grained control.
- Theme selection is resolved via `WithThemeProvider/WithThemeSelector`, providing partials/tokens/assets to renderers; set `WithThemeFallbacks` to ensure template keys always resolve.

Example mount:

```go
mux.Handle("/runtime/",
  http.StripPrefix("/runtime/",
    http.FileServerFS(formgen.RuntimeAssetsFS()),
  ),
)
```

Runtime usage (vanilla renderer or custom pages):

```html
<script src="/runtime/formgen-relationships.min.js" defer></script>
<script>
  window.addEventListener("DOMContentLoaded", () => {
    window.FormgenRelationships?.initRelationships?.();
  });
</script>
```

## Localization (i18n)

Formgen supports localization without depending on a specific i18n package: supply any implementation of `render.Translator` (compatible with `github.com/goliatone/go-i18n`).

### How To

1) Add explicit translation keys in your UI schema while keeping human readable fallbacks:

```json
{
  "operations": {
    "createPet": {
      "form": { "title": "Create Pet", "titleKey": "forms.createPet.title" },
      "sections": [{ "id": "main", "title": "Main", "titleKey": "forms.createPet.sections.main.title" }],
      "fields": { "name": { "label": "Name", "labelKey": "fields.pet.name" } },
      "actions": [{ "label": "Save", "labelKey": "actions.save", "type": "submit" }]
    }
  }
}
```

2) Provide locale + translator at render time (missing translations go through `OnMissing`):

```go
output, err := gen.Generate(ctx, orchestrator.Request{
  Source:      openapi.SourceFromFile("openapi.json"),
  OperationID: "createPet",
  RenderOptions: render.RenderOptions{
    Locale:     "es-MX",
    Translator: myTranslator, // implements: Translate(locale, key string, args ...any) (string, error)
    OnMissing: func(locale, key string, args []any, err error) string {
      return key // or inspect args[0].(map[string]any)["default"]
    },
  },
})
```

3) (Optional) Use template level translation helpers in custom templates:

```go
funcs := render.TemplateI18nFuncs(myTranslator, render.TemplateI18nConfig{
  FuncName:  "translate",
  LocaleKey: "locale",
})

renderer, _ := vanilla.New(
  vanilla.WithTemplatesFS(myTemplates),
  vanilla.WithTemplateFuncs(funcs),
)
```

In templates you can call `{{ translate(locale, "forms.createPet.title") }}` (formgen also passes `render_options.locale`).

## Styling & Customization

Formgen provides multiple layers of styling customization, from default Tailwind styles to fully custom themes.

### Quick Styling Options

**Use default styles:**
```go
renderer, _ := vanilla.New(
    vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()),
    vanilla.WithDefaultStyles(),  // Bundled Tailwind CSS
)
```

**Inject custom CSS:**
```go
renderer, _ := vanilla.New(
    vanilla.WithInlineStyles(`
        form[data-formgen-auto-init] { width: 100%; max-width: none; }
    `),
)
```

**Add external stylesheet:**
```go
renderer, _ := vanilla.New(
    vanilla.WithStylesheet("/static/custom-forms.css"),
)
```

**Disable all styles:**
```go
renderer, _ := vanilla.New(vanilla.WithoutStyles())
```

### Theme Integration with `go-theme`

Formgen integrates with [`go-theme`](https://github.com/goliatone/go-theme) to provide theme management:

**Key Features:**
- Multi theme support with runtime switching
- Theme variants (light/dark, compact/spacious, etc.)
- Design tokens auto converted to CSS variables
- Template overrides per theme
- Asset management with URL resolution

**Basic Example:**

```go
import theme "github.com/goliatone/go-theme"

manifest := &theme.Manifest{
    Name:    "acme",
    Version: "1.0.0",

    // Design tokens (auto converted to CSS vars)
    Tokens: map[string]string{
        "primary-color":       "#3b82f6",
        "container-max-width": "100%",
        "border-radius":       "0.5rem",
    },

    // Theme variants
    Variants: map[string]theme.Variant{
        "dark": {
            Tokens: map[string]string{
                "primary-color": "#60a5fa",
                "bg-primary":    "#1f2937",
            },
        },
        "two-column": {
            Tokens: map[string]string{
                "grid-columns-desktop": "2",
            },
        },
    },

    // Template overrides
    Templates: map[string]string{
        "forms.input": "themes/acme/input.tmpl",
    },

    // Asset bundle
    Assets: theme.Assets{
        Prefix: "/static/themes/acme",
        Files: map[string]string{
            "logo":       "logo.svg",
            "stylesheet": "acme.css",
        },
    },
}

provider := theme.NewRegistry()
provider.Register(manifest)

gen := formgen.NewOrchestrator(
    orchestrator.WithThemeProvider(provider, "acme", "dark"),
    orchestrator.WithThemeFallbacks(map[string]string{
        "forms.select": "templates/components/select.tmpl",
    }),
)

// Use default theme
output, _ := gen.Generate(ctx, orchestrator.Request{
    OperationID: "createPet",
})

// Override variant per request
output, _ := gen.Generate(ctx, orchestrator.Request{
    OperationID:  "createPet",
    ThemeVariant: "two-column",
})
```

**What gets rendered:**

```html
<!-- CSS variables from theme tokens -->
<style data-formgen-theme-vars>
:root {
  --bg-primary: #1f2937;
  --border-radius: 0.5rem;
  --container-max-width: 100%;
  --primary-color: #60a5fa;
}
</style>

<!-- Theme metadata for JavaScript -->
<script id="formgen-theme" type="application/json">
{"name":"acme","variant":"dark","tokens":{...},"cssVars":{...}}
</script>

<form data-formgen-theme="acme" data-formgen-theme-variant="dark">
  <!-- form fields -->
</form>
```

**See the [Styling Guide](docs/GUIDE_STYLING.md#4-theme-integration-go-theme) for:**
- Complete theme manifest structure
- Token merging and precedence
- Template lookup order
- Asset URL resolution
- Custom theme selectors

### Responsive Layouts

Control grid layout via UI schema metadata:

```json
{
  "uiHints": {
    "layout.gridColumns": "12"
  },
  "fields": [
    {
      "name": "title",
      "uiHints": { "layout.span": "12" }  // full width
    },
    {
      "name": "email",
      "uiHints": { "layout.span": "6" }   // half width
    }
  ]
}
```

Or use responsive CSS:

```css
@media (min-width: 1024px) {
  form[data-formgen-auto-init] .grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
```

**See the complete [Styling & Customization Guide](docs/GUIDE_STYLING.md) for:**
- Custom template bundles
- Fluid vs. fixed width containers
- Responsive two column layouts
- Theme variants and CSS variables
- Component level customization
- Complete working examples

## Form Customization

Beyond styling, formgen supports extensive form behavior customization through **UI Schemas** - JSON/YAML files that configure forms without modifying OpenAPI specs or templates.

### Custom Action Buttons

Add submit, reset, cancel, or custom buttons via UI schema:

```json
{
  "operations": {
    "createArticle": {
      "form": {
        "actions": [
          {
            "kind": "secondary",
            "label": "Clear Form",
            "type": "reset"
          },
          {
            "kind": "secondary",
            "label": "Save Draft",
            "type": "button"
          },
          {
            "kind": "primary",
            "label": "Publish",
            "type": "submit"
          }
        ]
      }
    }
  }
}
```

### Sections and Layout

Organize fields into logical sections with custom grid layouts:

```json
{
  "operations": {
    "createPet": {
      "form": {
        "title": "Add New Pet",
        "layout": {
          "gridColumns": 12,
          "gutter": "md"
        }
      },
      "sections": [
        {
          "id": "basic-info",
          "title": "Basic Information",
          "fieldset": true,
          "order": 0
        },
        {
          "id": "health",
          "title": "Health Records",
          "order": 1
        }
      ],
      "fields": {
        "name": {
          "section": "basic-info",
          "grid": { "span": 8 },
          "helpText": "Your pet's name"
        },
        "age": {
          "section": "basic-info",
          "grid": { "span": 4 }
        }
      }
    }
  }
}
```

### Widgets and Components

Use built-in widgets or register custom components:

```json
{
  "fields": {
    "description": {
      "component": "wysiwyg",
      "componentOptions": {
        "toolbar": ["bold", "italic", "link"]
      }
    },
    "category": {
      "component": "custom-select",
      "componentOptions": {
        "endpoint": "/api/categories"
      }
    }
  }
}
```

### Behaviors

Add client-side behaviors like auto slug:

```json
{
  "fields": {
    "title": {
      "label": "Article Title"
    },
    "slug": {
      "helpText": "Auto-generated from title",
      "behaviors": {
        "autoSlug": {
          "source": "title"
        }
      }
    }
  }
}
```

### Loading UI Schemas

```go
//go:embed ui-schemas
var uiSchemas embed.FS

gen := formgen.NewOrchestrator(
    orchestrator.WithUISchemaFS(uiSchemas),
)

// OperationID selects which entry under "operations" is applied.
output, _ := gen.Generate(ctx, orchestrator.Request{
    OperationID: "createArticle",
})
```

**See the complete [Form Customization Guide](docs/GUIDE_CUSTOMIZATION.md) for:**
- Action button configuration (submit, reset, cancel, custom)
- Section and fieldset organization
- Field-level customization (labels, help text, grid positioning)
- Widgets and components (vanilla components + Preact widget hints)
- Custom component registration
- Icons and visual enhancements
- Behaviors (auto slug + custom behavior hooks)
- Field ordering and presets
- Three complete working examples (blog, registration, e-commerce)

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
- Relationship endpoints missing from the OpenAPI? Provide `WithEndpointOverrides` with `FieldPath`/`OperationID` or embed `x-endpoint` metadata.
- Visibility rules present but ignored? Pass a `visibility.Evaluator` via `WithVisibilityEvaluator` and feed `RenderOptions.VisibilityContext` / `Values`.

## License

MIT © Goliatone
