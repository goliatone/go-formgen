# Form Styling Customization Guide

## Overview

`go-formgen` provides multiple layers of styling customization, from using the default Tailwind-based theme to building completely custom themes via the `go-theme` integration.

---

## 1. Default Styling

### Using Built-In Styles

The vanilla renderer includes embedded Tailwind CSS styles. Enable them with:

```go
renderer, _ := vanilla.New(
    vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()),
    vanilla.WithDefaultStyles(),  // Injects bundled Tailwind CSS
)
```

**Default form container classes** ([form.tmpl:31](../pkg/renderers/vanilla/templates/form.tmpl#L31)):
```html
<form class="max-w-4xl mx-auto space-y-6 p-6 bg-white rounded-xl border border-gray-200 dark:bg-slate-900 dark:border-gray-700">
```

This gives you:
- **Fixed max width** (`max-w-4xl`) — form won't exceed 56rem
- **Centered** (`mx-auto`)
- **Padding and rounded corners**
- **Dark mode support**

---

## 2. Custom Inline Styles

### Inject Your Own CSS

Replace or supplement the default styles:

```go
customCSS := `
  .formgen-form { width: 100%; max-width: none; }
  @media (min-width: 768px) {
    .formgen-grid { grid-template-columns: repeat(2, 1fr); }
  }
`

renderer, _ := vanilla.New(
    vanilla.WithInlineStyles(customCSS),  // Replaces default
)
```

Or add external stylesheets:

```go
renderer, _ := vanilla.New(
    vanilla.WithStylesheet("/static/custom-forms.css"),
)
```

Disable all bundled styles:

```go
renderer, _ := vanilla.New(
    vanilla.WithoutStyles(),  // No inline CSS or default links
)
```

---

## 3. Custom Templates

### Override the Form Wrapper

The form container is defined in [templates/form.tmpl](../pkg/renderers/vanilla/templates/form.tmpl). To customize it:

**Option A: Replace the entire template bundle**

```go
customFS := os.DirFS("./my-templates")
renderer, _ := vanilla.New(
    vanilla.WithTemplatesFS(customFS),
)
```

Your `my-templates/templates/form.tmpl` could start with:

```html
<div class="w-full">  <!-- fluid wrapper -->
  <form class="space-y-6" method="{{ .render_options.method_attr }}" action="{{ .form.endpoint }}">
    <!-- rest of form -->
  </form>
</div>
```

**Option B: Use theme partials (see §4)**

---

## 4. Theme Integration (`go-theme`)

### Overview

`go-formgen` integrates with the [`go-theme`](https://github.com/goliatone/go-theme) package to provide a powerful theme management system. This enables:

- **Multi-theme support** — Register multiple themes and switch between them
- **Theme variants** — Define light/dark, compact/spacious, or custom variants
- **Design tokens** — Centralize design decisions (colors, spacing, typography)
- **CSS variables** — Tokens auto-convert to CSS custom properties
- **Template overrides** — Replace component templates per theme
- **Asset management** — Organized theme-specific assets with URL resolution

### How the Integration Works

The theme system follows this flow:

1. **Define** a `theme.Manifest` with tokens, templates, and assets
2. **Register** the manifest with a `theme.Registry` (the provider)
3. **Configure** the orchestrator with `WithThemeProvider(provider, defaultTheme, defaultVariant)`
4. **Request** forms with optional theme/variant overrides
5. **Renderer receives** a `theme.RendererConfig` with resolved partials, tokens, CSS vars, and asset resolver

The orchestrator handles theme resolution ([orchestrator.go:344-356](../pkg/orchestrator/orchestrator.go#L344-356)):

```go
func (o *Orchestrator) resolveTheme(themeName, variant string) (*theme.RendererConfig, error) {
    selection, err := o.themeSelector.Select(themeName, variant)
    cfg := selection.RendererTheme(o.themeFallbacks)  // Merges with fallbacks
    return &cfg, nil
}
```

### Basic Theme Setup

```go
import theme "github.com/goliatone/go-theme"

manifest := &theme.Manifest{
    Name:    "acme",
    Version: "1.0.0",

    // Design tokens (converted to CSS variables)
    Tokens: map[string]string{
        "primary-color":       "#3b82f6",
        "container-max-width": "64rem",
        "border-radius":       "0.5rem",
        "font-family":         "Inter, system-ui, sans-serif",
    },

    // Template overrides (optional)
    Templates: map[string]string{
        "forms.input":    "themes/acme/input.tmpl",
        "forms.textarea": "themes/acme/textarea.tmpl",
    },

    // Asset configuration
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
    orchestrator.WithThemeProvider(provider, "acme", "default"),
)
```

**What gets passed to the renderer** ([orchestrator_theme_test.go:154-186](../pkg/orchestrator/orchestrator_theme_test.go#L154-186)):

- `Theme` — Theme name (`"acme"`)
- `Variant` — Variant name (`"default"`)
- `Partials` — Template path map (`{"forms.input": "themes/acme/input.tmpl"}`)
- `Tokens` — Design token map (`{"primary-color": "#3b82f6"}`)
- `CSSVars` — Auto-generated CSS variables (`{"--primary-color": "#3b82f6"}`)
- `AssetURL(key)` — Function resolving `"logo"` → `"/static/themes/acme/logo.svg"`

### Theme Variants

Define multiple variants for different contexts (light/dark, fluid/boxed, etc.):

```go
manifest := &theme.Manifest{
    Name: "acme",

    // Base tokens
    Tokens: map[string]string{
        "primary-color":       "#3b82f6",
        "container-max-width": "64rem",
        "bg-primary":          "#ffffff",
        "text-primary":        "#1f2937",
    },

    Variants: map[string]theme.Variant{
        "dark": {
            Tokens: map[string]string{
                "primary-color": "#60a5fa",  // Lighter blue
                "bg-primary":    "#1f2937",
                "text-primary":  "#f9fafb",
            },
            Templates: map[string]string{
                "forms.checkbox": "themes/acme/dark/checkbox.tmpl",
            },
            Assets: theme.Assets{
                Files: map[string]string{
                    "stylesheet": "acme-dark.css",
                },
            },
        },
        "fluid": {
            Tokens: map[string]string{
                "container-max-width": "100%",
            },
        },
        "compact": {
            Tokens: map[string]string{
                "container-max-width": "48rem",
                "spacing":             "0.5rem",
            },
        },
    },
}
```

**Token merging**: Base tokens are merged with variant tokens, with **variant values taking precedence**.

### Requesting Themes and Variants

**Option 1: Set defaults at orchestrator creation**

```go
gen := formgen.NewOrchestrator(
    orchestrator.WithThemeProvider(provider, "acme", "dark"),
)

// All requests use "acme" theme with "dark" variant by default
output, _ := gen.Generate(ctx, orchestrator.Request{
    OperationID: "createPet",
})
```

**Option 2: Override per request**

```go
// Use different variant for this request
output, _ := gen.Generate(ctx, orchestrator.Request{
    OperationID:  "createPet",
    ThemeName:    "acme",
    ThemeVariant: "fluid",
})
```

**Option 3: Switch themes entirely**

```go
output, _ := gen.Generate(ctx, orchestrator.Request{
    OperationID:  "createPet",
    ThemeName:    "corporate",  // Different theme
    ThemeVariant: "light",
})
```

### CSS Variables in Templates

Theme tokens are automatically converted to CSS variables and injected into the page ([form.tmpl:9-11](../pkg/renderers/vanilla/templates/form.tmpl#L9-11)):

**Generated HTML:**

```html
<style data-formgen-theme-vars>
:root {
  --bg-primary: #1f2937;
  --border-radius: 0.5rem;
  --container-max-width: 64rem;
  --font-family: Inter, system-ui, sans-serif;
  --primary-color: #60a5fa;
  --text-primary: #f9fafb;
}
</style>
```

**Using CSS variables in custom styles:**

```css
form[data-formgen-auto-init] {
  max-width: var(--container-max-width, 56rem);
  border-color: var(--primary-color, #3b82f6);
  font-family: var(--font-family, system-ui);
  background: var(--bg-primary, white);
  color: var(--text-primary, black);
}

input, textarea, select {
  border-radius: var(--border-radius, 0.375rem);
}
```

### Template Overrides

Themes can override specific component templates:

```go
manifest := &theme.Manifest{
    Name: "acme",
    Templates: map[string]string{
        "forms.input":         "themes/acme/input.tmpl",
        "forms.select":        "themes/acme/select.tmpl",
        "forms.textarea":      "themes/acme/textarea.tmpl",
        "forms.checkbox":      "themes/acme/checkbox.tmpl",
        "forms.wysiwyg":       "themes/acme/wysiwyg.tmpl",
        "forms.file-uploader": "themes/acme/file-uploader.tmpl",
    },
}
```

**Template lookup order:**

1. Variant-specific template (if defined in variant)
2. Theme base template (if defined in manifest)
3. Fallback template (from `WithThemeFallbacks`)
4. Default embedded template

**Setting fallbacks:**

```go
gen := formgen.NewOrchestrator(
    orchestrator.WithThemeProvider(provider, "acme", "dark"),
    orchestrator.WithThemeFallbacks(map[string]string{
        "forms.input":    "templates/components/input.tmpl",
        "forms.select":   "templates/components/select.tmpl",
        "forms.textarea": "templates/components/textarea.tmpl",
    }),
)
```

### Asset Management

Themes can bundle assets (CSS, JS, images, fonts):

```go
manifest := &theme.Manifest{
    Name: "acme",
    Assets: theme.Assets{
        Prefix: "/static/themes/acme",
        Files: map[string]string{
            "logo":            "logo.svg",
            "icon":            "favicon.ico",
            "stylesheet":      "acme.css",
            "script":          "acme.js",
            "font-regular":    "fonts/inter-regular.woff2",
            "font-bold":       "fonts/inter-bold.woff2",
        },
    },
}
```

**Resolving asset URLs** (automatically handled by the renderer):

```go
// In your code or templates
assetURL := renderOptions.Theme.AssetURL("logo")
// Returns: "/static/themes/acme/logo.svg"
```

**In templates:**

```html
<img src="{{ theme.assetURL "logo" }}" alt="Logo">
<link rel="stylesheet" href="{{ theme.assetURL "stylesheet" }}">
```

### Complete Working Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goliatone/go-formgen"
    "github.com/goliatone/go-formgen/pkg/openapi"
    "github.com/goliatone/go-formgen/pkg/orchestrator"
    "github.com/goliatone/go-formgen/pkg/render"
    "github.com/goliatone/go-formgen/pkg/renderers/vanilla"
    theme "github.com/goliatone/go-theme"
)

func main() {
    // 1. Define theme manifest
    manifest := &theme.Manifest{
        Name:    "corporate",
        Version: "1.0.0",

        Tokens: map[string]string{
            "brand":               "#2563eb",
            "container-max-width": "64rem",
            "border-radius":       "0.5rem",
            "spacing":             "1rem",
        },

        Templates: map[string]string{
            "forms.input": "themes/corporate/input.tmpl",
        },

        Assets: theme.Assets{
            Prefix: "/static/themes/corporate",
            Files: map[string]string{
                "logo":       "logo.svg",
                "stylesheet": "corporate.css",
            },
        },

        Variants: map[string]theme.Variant{
            "dark": {
                Tokens: map[string]string{
                    "brand":       "#60a5fa",
                    "bg-primary":  "#1f2937",
                    "text-primary": "#f9fafb",
                },
                Assets: theme.Assets{
                    Files: map[string]string{
                        "stylesheet": "corporate-dark.css",
                    },
                },
            },
            "compact": {
                Tokens: map[string]string{
                    "container-max-width": "48rem",
                    "spacing":             "0.5rem",
                },
            },
        },
    }

    // 2. Register theme
    provider := theme.NewRegistry()
    if err := provider.Register(manifest); err != nil {
        log.Fatalf("register theme: %v", err)
    }

    // 3. Configure renderer and orchestrator
    renderer, _ := vanilla.New(
        vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()),
        vanilla.WithDefaultStyles(),
    )

    registry := render.NewRegistry()
    registry.MustRegister(renderer)

    gen := formgen.NewOrchestrator(
        orchestrator.WithRegistry(registry),
        orchestrator.WithThemeProvider(provider, "corporate", "dark"),
        orchestrator.WithThemeFallbacks(map[string]string{
            "forms.select":   "templates/components/select.tmpl",
            "forms.checkbox": "templates/components/boolean.tmpl",
        }),
    )

    // 4. Generate form with default theme
    output, err := gen.Generate(context.Background(), orchestrator.Request{
        Source:      openapi.SourceFromFile("openapi.json"),
        OperationID: "createPet",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(string(output))

    // 5. Generate with different variant
    compactOutput, _ := gen.Generate(context.Background(), orchestrator.Request{
        Source:       openapi.SourceFromFile("openapi.json"),
        OperationID:  "createPet",
        ThemeVariant: "compact",  // Override variant
    })

    fmt.Println(string(compactOutput))
}
```

### Theme Data in Rendered Output

The renderer includes theme metadata in the HTML output:

```html
<!-- CSS variables from theme tokens -->
<style data-formgen-theme-vars>
:root {
  --bg-primary: #1f2937;
  --border-radius: 0.5rem;
  --brand: #60a5fa;
  --container-max-width: 64rem;
  --spacing: 1rem;
  --text-primary: #f9fafb;
}
</style>

<!-- Theme JSON for client-side JavaScript -->
<script id="formgen-theme" type="application/json">
{
  "name": "corporate",
  "variant": "dark",
  "tokens": {
    "brand": "#60a5fa",
    "container-max-width": "64rem",
    ...
  },
  "cssVars": {
    "--brand": "#60a5fa",
    "--container-max-width": "64rem",
    ...
  }
}
</script>

<!-- Form element with theme attributes -->
<form data-formgen-theme="corporate" data-formgen-theme-variant="dark">
  <!-- form content -->
</form>
```

### Advanced: Custom Theme Selector

For advanced use cases, implement a custom theme selector:

```go
type CustomSelector struct {
    registry theme.ThemeProvider
}

func (s *CustomSelector) Select(name, variant string) (*theme.Selection, error) {
    // Custom logic: tenant-based, user preferences, A/B testing, etc.
    if name == "" {
        name = getTenantTheme()  // Your logic
    }
    if variant == "" {
        variant = getUserPreference()  // Your logic
    }
    return s.registry.Select(name, variant)
}

gen := formgen.NewOrchestrator(
    orchestrator.WithThemeSelector(&CustomSelector{registry: provider}),
)
```

---

## 5. Responsive Layouts

### Grid Columns & Breakpoints

Forms use CSS Grid with configurable columns ([form.tmpl:61](../pkg/renderers/vanilla/templates/form.tmpl#L61)):

```html
<div class="grid gap-6" style="grid-template-columns: repeat(12, minmax(0, 1fr))">
```

**Control grid columns** via UI schema metadata:

```json
{
  "uiHints": {
    "layout.gridColumns": "12"
  }
}
```

**Field-level spanning** ([renderer.go:1154-1179](../pkg/renderers/vanilla/renderer.go#L1154-1179)):

```json
{
  "fields": [
    {
      "name": "title",
      "uiHints": {
        "layout.span": "12"  // full width
      }
    },
    {
      "name": "email",
      "uiHints": {
        "layout.span": "6"  // half width on 12-column grid
      }
    }
  ]
}
```

### Two-Column Layout on Wide Screens

Use custom CSS with media queries:

```css
.formgen-grid {
  display: grid;
  gap: 1.5rem;
  grid-template-columns: 1fr;  /* single column mobile */
}

@media (min-width: 1024px) {
  .formgen-grid {
    grid-template-columns: repeat(2, 1fr);  /* two columns desktop */
  }
}
```

Apply via theme tokens or inline styles.

---

## 6. Common Customization Scenarios

### Scenario 1: Fluid-Width Forms

**Problem:** Default form container is `max-w-4xl` (56rem max width).

**Solution A: Custom template**

Create `templates/form.tmpl` with dynamic class:

```html
{% set container_class = theme.tokens['container-class'] | default('max-w-4xl mx-auto') %}
<form class="{{ container_class }} space-y-6 p-6 bg-white rounded-xl border">
```

Then pass via theme:

```go
Tokens: map[string]string{
    "container-class": "w-full",  // fluid
}
```

**Solution B: CSS override**

```css
form[data-formgen-auto-init] {
  max-width: none !important;  /* fluid */
  width: 100%;
}
```

**Solution C: Wrapper element**

Wrap the form with a custom div in your template partial override:

```html
<div class="container-fluid">
  {{ original form HTML }}
</div>
```

### Scenario 2: Responsive Two-Column Layout

**Approach 1: Media queries in theme CSS**

```css
@media (min-width: 1024px) {
  form[data-formgen-auto-init] .grid {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}
```

**Approach 2: Tailwind responsive classes**

Override [form.tmpl:61](../pkg/renderers/vanilla/templates/form.tmpl#L61):

```html
<div class="grid gap-6 grid-cols-1 lg:grid-cols-2">
```

**Approach 3: Dynamic grid columns via UI schema**

Set `layout.gridColumns` and use field spans:

```json
{
  "uiHints": {
    "layout.gridColumns": "12"
  },
  "fields": [
    {
      "name": "title",
      "uiHints": {
        "layout.span": "12",
        "@media (min-width: 1024px)": {
          "layout.span": "6"
        }
      }
    }
  ]
}
```

**Approach 4: JavaScript-based (not recommended)**

```js
const updateGridColumns = () => {
  const form = document.querySelector('[data-formgen-auto-init]');
  const grid = form.querySelector('.grid');
  if (window.innerWidth >= 1024) {
    grid.style.gridTemplateColumns = 'repeat(2, 1fr)';
  } else {
    grid.style.gridTemplateColumns = '1fr';
  }
};
window.addEventListener('resize', updateGridColumns);
updateGridColumns();
```

### Scenario 3: Custom Component Styles

Override individual component templates:

```go
manifest := &theme.Manifest{
    Templates: map[string]string{
        "forms.input":    "themes/acme/input.tmpl",
        "forms.textarea": "themes/acme/textarea.tmpl",
    },
}
```

Or register custom components:

```go
registry := components.NewDefaultRegistry()
registry.MustRegister("custom-input", components.Descriptor{
    Renderer: func(buf *bytes.Buffer, field model.Field, data components.ComponentData) error {
        buf.WriteString(`<input class="my-custom-input" ... />`)
        return nil
    },
})

renderer, _ := vanilla.New(
    vanilla.WithComponentRegistry(registry),
)
```

---

## 7. Complete Example: Custom Fluid Two-Column Theme

```go
package main

import (
    "context"
    "github.com/goliatone/go-formgen"
    "github.com/goliatone/go-formgen/pkg/orchestrator"
    "github.com/goliatone/go-formgen/pkg/render"
    "github.com/goliatone/go-formgen/pkg/renderers/vanilla"
    theme "github.com/goliatone/go-theme"
)

func main() {
    // Define custom theme with fluid container and responsive grid
    manifest := &theme.Manifest{
        Name:    "responsive",
        Version: "1.0.0",
        Tokens: map[string]string{
            "container-max-width": "100%",
            "grid-columns-mobile": "1",
            "grid-columns-desktop": "2",
        },
    }

    provider := theme.NewRegistry()
    provider.Register(manifest)

    // Custom CSS for responsive layout
    customCSS := `
        form[data-formgen-auto-init] {
            max-width: var(--container-max-width, 100%) !important;
            width: 100%;
        }

        .grid {
            grid-template-columns: repeat(var(--grid-columns-mobile, 1), 1fr);
        }

        @media (min-width: 1024px) {
            .grid {
                grid-template-columns: repeat(var(--grid-columns-desktop, 2), 1fr);
            }
        }
    `

    renderer, _ := vanilla.New(
        vanilla.WithTemplatesFS(formgen.EmbeddedTemplates()),
        vanilla.WithInlineStyles(customCSS),
    )

    registry := render.NewRegistry()
    registry.MustRegister(renderer)

    gen := formgen.NewOrchestrator(
        orchestrator.WithThemeProvider(provider, "responsive", "default"),
        orchestrator.WithRegistry(registry),
    )

    output, _ := gen.Generate(context.Background(), orchestrator.Request{
        Source:      openapi.SourceFromFile("openapi.json"),
        OperationID: "createPet",
    })

    // output now has fluid width + two-column layout on desktop
}
```

---

## 8. Advanced: Custom Template Bundle

### Directory Structure

```
my-templates/
├── templates/
│   ├── form.tmpl                    # Main form wrapper
│   └── components/
│       ├── input.tmpl               # Text inputs
│       ├── select.tmpl              # Dropdowns
│       ├── boolean.tmpl             # Checkboxes
│       ├── textarea.tmpl            # Text areas
│       ├── array.tmpl               # Array fields
│       ├── object.tmpl              # Nested objects
│       ├── wysiwyg.tmpl             # Rich text editors
│       ├── file_uploader.tmpl       # File uploads
│       └── json_editor.tmpl         # JSON editors
```

### Loading Custom Templates

```go
customFS := os.DirFS("./my-templates")
renderer, _ := vanilla.New(
    vanilla.WithTemplatesFS(customFS),
    vanilla.WithInlineStyles(myCSS),
)
```

### Template Variables

All templates receive a context with:

- `form` — The `FormModel` with fields, metadata, UI hints
- `layout` — Grid columns, sections, unsectioned fields
- `actions` — Action buttons parsed from metadata
- `theme` — Theme tokens, CSS vars, partials
- `render_options` — Method, hidden fields, form errors
- `stylesheets` — External stylesheet URLs
- `inline_styles` — Inline CSS block
- `component_scripts` — JavaScript dependencies

Component templates receive:

- `field` — The `Field` being rendered
- `config` — Component-specific configuration
- `theme` — Theme context
- `chrome` — Whether to render label/description wrapper

---

## 9. Reference Tables

### Vanilla Renderer Options

| Option | Purpose |
|--------|---------|
| `WithTemplatesFS(fs.FS)` | Supply custom template bundle |
| `WithTemplatesDir(string)` | Load templates from directory |
| `WithTemplateRenderer(renderer)` | Inject custom template engine |
| `WithDefaultStyles()` | Include bundled Tailwind CSS |
| `WithInlineStyles(css)` | Inject custom inline CSS |
| `WithStylesheet(url)` | Add external stylesheet link |
| `WithoutStyles()` | Disable all default styles |
| `WithComponentRegistry(reg)` | Use custom component registry |
| `WithComponentOverrides(map)` | Override components for specific fields |

### Orchestrator Theme Options

| Option | Purpose |
|--------|---------|
| `WithThemeProvider(provider, theme, variant)` | Use `go-theme` provider |
| `WithThemeSelector(selector)` | Custom theme selector logic |
| `WithThemeFallbacks(map)` | Fallback template paths |

### UI Schema Layout Hints

| Hint Key | Purpose | Example |
|----------|---------|---------|
| `layout.gridColumns` | Set grid column count | `"12"` |
| `layout.gutter` | Grid gap size | `"md"` |
| `layout.span` | Field column span | `"6"` (half width) |
| `layout.start` | Grid column start | `"1"` |
| `layout.row` | Grid row placement | `"2"` |
| `layout.sections` | Define form sections | JSON array |
| `layout.section` | Assign field to section | `"personal"` |

### Theme Token Conventions

| Token | Purpose | Example |
|-------|---------|---------|
| `container-max-width` | Form container max width | `"100%"` |
| `container-class` | Form container classes | `"w-full"` |
| `grid-columns-mobile` | Mobile grid columns | `"1"` |
| `grid-columns-desktop` | Desktop grid columns | `"2"` |
| `primary-color` | Primary brand color | `"#3b82f6"` |
| `border-radius` | Input border radius | `"0.375rem"` |

---

## Summary

| Goal | Solution |
|------|----------|
| **Use default styles** | `vanilla.WithDefaultStyles()` |
| **Custom CSS** | `vanilla.WithInlineStyles(css)` or `WithStylesheet(url)` |
| **No bundled styles** | `vanilla.WithoutStyles()` |
| **Override templates** | `vanilla.WithTemplatesFS(customFS)` |
| **Theme system** | `orchestrator.WithThemeProvider(provider, theme, variant)` |
| **Fluid container** | Override form template or CSS `max-width: none` |
| **Responsive grid** | Media queries or Tailwind responsive classes |
| **Per-field layout** | UI schema `layout.span`, `layout.start`, `layout.row` |
| **Component overrides** | `WithComponentRegistry()` or theme partials |

All styling customization happens **before rendering** via renderer options or **at runtime** via theme selection in the orchestrator request.
