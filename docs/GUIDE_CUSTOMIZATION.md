# Form Customization Guide

## Overview

This guide covers non-styling customizations for `go-formgen` forms, including:

- **Custom action buttons** (submit, reset, cancel, custom handlers)
- **Form layout and sections** (grid, fieldsets, ordering)
- **Field-level customization** (widgets, components, behaviors)
- **UI hints and metadata** (icons, help text, placeholders)
- **Visibility rules** (conditional field display)

All customizations are managed through the **UI Schema** system, which allows you to configure form behavior without modifying templates or code.

UI schemas are best for **layout/labels/metadata**. Interactive features (relationships, WYSIWYG, file uploaders, custom widgets) may also require **runtime JavaScript** and/or registering **custom components**.

---

## Table of Contents

1. [UI Schema Basics](#1-ui-schema-basics)
2. [Custom Action Buttons](#2-custom-action-buttons)
3. [Form Layout and Grid](#3-form-layout-and-grid)
4. [Sections and Fieldsets](#4-sections-and-fieldsets)
5. [Field Customization](#5-field-customization)
6. [Widgets and Components](#6-widgets-and-components)
7. [Icons and Visual Enhancements](#7-icons-and-visual-enhancements)
8. [Behaviors and Interactions](#8-behaviors-and-interactions)
9. [Visibility Rules](#9-visibility-rules)
10. [Field Ordering](#10-field-ordering)
11. [Complete Examples](#11-complete-examples)

---

## 1. UI Schema Basics

### What is a UI Schema?

A UI Schema is a JSON or YAML file that describes how to render a form without modifying the OpenAPI specification. It maps operation IDs to form configuration.

### Basic Structure

```json
{
  "fieldOrderPresets": { ... },
  "operations": {
    "operationId": {
      "form": {
        "title": "Form Title",
        "titleKey": "forms.operationId.title",
        "subtitle": "Form subtitle",
        "subtitleKey": "forms.operationId.subtitle",
        "layout": { ... },
        "actions": [ ... ],
        "metadata": { ... },
        "uiHints": { ... }
      },
      "sections": [ ... ],
      "fields": {
        "fieldName": { ... }
      }
    }
  }
}
```

### Loading UI Schemas

**Option 1: Embed from directory**

```go
//go:embed ui-schemas
var uiSchemaFS embed.FS

gen := formgen.NewOrchestrator(
    orchestrator.WithUISchemaFS(uiSchemaFS),
)
```

**Option 2: Load from filesystem**

```go
gen := formgen.NewOrchestrator(
    orchestrator.WithUISchemaFS(os.DirFS("./config/ui-schemas")),
)
```

**Option 3: Disable UI schemas**

```go
gen := formgen.NewOrchestrator(
    orchestrator.WithUISchemaFS(nil),  // Disables all UI schemas (including embedded defaults)
)
```

If you omit `WithUISchemaFS` entirely, `go-formgen` loads its embedded UI schema bundle (`pkg/uischema/ui/schema`) when present.

### File Organization

`go-formgen` loads every `*.json`, `*.yaml`, and `*.yml` file in the configured `fs.FS` (recursively) and registers the operations found under `operations`.

- File names are not significant; operation IDs come from the keys in `operations`.
- You can use one file per operation (recommended for readability) or group multiple operations in one file.
- Duplicate operation IDs across files are an error.

---

## Localization (i18n)

UI schema strings can be localized by pairing a `*Key` field with its fallback value.

- **Form**: `form.titleKey` / `form.subtitleKey`
- **Sections**: `sections[].titleKey` / `sections[].descriptionKey`
- **Fields**: `fields.<name>.labelKey`, `descriptionKey`, `placeholderKey`, `helpTextKey`
- **Actions**: `form.actions[].labelKey`

At render time, supply a `render.Translator` and `render.RenderOptions.Locale`. Missing translations can be customized with `render.RenderOptions.OnMissing`.

For template-level i18n (custom templates), inject a translate helper into template funcs:

```go
funcs := render.TemplateI18nFuncs(translator, render.TemplateI18nConfig{
  FuncName:  "translate",
  LocaleKey: "locale",
})

renderer, _ := vanilla.New(vanilla.WithTemplateFuncs(funcs))
// preact.New(preact.WithTemplateFuncs(funcs))
```

## 2. Custom Action Buttons

### Default Behavior

Without configuration, forms render a single "Submit" button ([form.tmpl:116](../pkg/renderers/vanilla/templates/form.tmpl#L116)):

```html
<button type="submit">Submit</button>
```

### ActionConfig Structure

From [pkg/uischema/types.go](../pkg/uischema/types.go#L45):

```go
type ActionConfig struct {
    Kind     string // "primary" or "secondary" (styling hint)
    Label    string // Button text
    LabelKey string // Optional i18n key for Label
    Href     string // When set, renders an <a> styled like a button
    Type     string // "submit", "reset", or "button" (defaults to "submit")
    Icon     string // Icon identifier (optional; renderer-dependent)
}
```

### Action Types

| Field | Value | Result |
|-------|-------|--------|
| `href` | URL string | Renders `<a>` styled as button |
| `type` | `"submit"` | Submit button (form submission) |
| `type` | `"reset"` | Reset button (clears form) |
| `type` | `"button"` | Generic button (for JS handlers) |
| `kind` | `"primary"` | Blue background, emphasized |
| `kind` | `"secondary"` | White background, subtle |

Notes for the built-in vanilla renderer:
- Button actions default to `type: "submit"` when omitted.
- For `<button>` actions, `kind` defaults to primary when omitted.
- For `<a href="…">` actions, `kind` defaults to secondary when omitted.

### Example: Submit + Cancel

```json
{
  "operations": {
    "createPet": {
      "form": {
        "actions": [
          {
            "kind": "secondary",
            "label": "Cancel",
            "href": "/pets"
          },
          {
            "kind": "primary",
            "label": "Create Pet",
            "type": "submit"
          }
        ]
      }
    }
  }
}
```

**Rendered output:**

```html
<div class="flex gap-x-2 pt-4 border-t border-gray-200">
  <a class="... bg-white text-gray-800 ..." href="/pets">Cancel</a>
  <button type="submit" class="... bg-blue-600 text-white ...">Create Pet</button>
</div>
```

### Example: Submit + Reset

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
            "kind": "primary",
            "label": "Publish Article",
            "type": "submit"
          }
        ]
      }
    }
  }
}
```

### Example: Multiple Actions with Custom Handler

```json
{
  "operations": {
    "editSession": {
      "form": {
        "actions": [
          {
            "kind": "secondary",
            "label": "Back to List",
            "href": "/sessions"
          },
          {
            "kind": "secondary",
            "label": "Save Draft",
            "type": "button"
          },
          {
            "kind": "secondary",
            "label": "Preview",
            "type": "button"
          },
          {
            "kind": "primary",
            "label": "Update Session",
            "type": "submit"
          }
        ]
      }
    }
  }
}
```

**Add JavaScript handler for "Save Draft":**

```html
<script>
document.addEventListener('DOMContentLoaded', () => {
  const form = document.querySelector('form[data-formgen-auto-init]');
  if (!form) return;
  const buttons = form.querySelectorAll('button[type="button"]');

  buttons.forEach(button => {
    if (button.textContent.includes('Save Draft')) {
      button.addEventListener('click', async () => {
        const formData = new FormData(form);
        const response = await fetch('/api/sessions/draft', {
          method: 'POST',
          body: formData
        });
        if (response.ok) {
          alert('Draft saved successfully!');
        }
      });
    }
  });
});
</script>
```

### Action Button Styling

**Primary `<button>` actions** (`kind: "primary"` or omitted):
- Background: `bg-blue-600`
- Text: `text-white`
- Hover: `hover:bg-blue-700`
- Focus ring: `focus:ring-blue-600`

**Secondary `<button>` actions** (`kind: "secondary"`):
- Background: `bg-white` / `dark:bg-slate-900`
- Border: `border-gray-200`
- Text: `text-gray-800` / `dark:text-white`
- Hover: `hover:bg-gray-50`
- Focus ring: `focus:ring-gray-400`

**`<a href="…">` actions** default to secondary styles unless `kind: "primary"` is set.

---

## 3. Form Layout and Grid

### Grid Configuration

Forms use CSS Grid with configurable columns ([form.tmpl:61](../pkg/renderers/vanilla/templates/form.tmpl#L61)):

```json
{
  "operations": {
    "createPet": {
      "form": {
        "layout": {
          "gridColumns": 12,
          "gutter": "md"
        }
      }
    }
  }
}
```

The built-in vanilla template honors `layout.gutter` and maps it to Tailwind gap classes:
- `"sm"` → `gap-4`
- `"md"` (default) → `gap-6`
- `"lg"` → `gap-8`

### Form Title and Subtitle

```json
{
  "operations": {
    "createPet": {
      "form": {
        "title": "Add New Pet",
        "subtitle": "Fill in the details below to register a new pet"
      }
    }
  }
}
```

**Rendered output:**

```html
<header class="space-y-2 pb-4 border-b">
  <h1 class="text-2xl font-bold">Add New Pet</h1>
  <p class="text-sm text-gray-600">Fill in the details below to register a new pet</p>
</header>
```

---

## 4. Sections and Fieldsets

### Section Configuration

From [pkg/uischema/types.go](../pkg/uischema/types.go#L55):

```go
type SectionConfig struct {
    ID             string
    Title          string
    TitleKey       string
    Description    string
    DescriptionKey string
    Order          *int
    Fieldset       *bool
    OrderPreset    OrderPreset // string preset name, or []string inline pattern
    Metadata       map[string]string
    UIHints        map[string]string
}
```

### Example: Two Sections

```json
{
  "operations": {
    "createArticle": {
      "form": {
        "layout": {
          "gridColumns": 12
        }
      },
      "sections": [
        {
          "id": "basic-info",
          "title": "Basic Information",
          "description": "Article title and metadata",
          "order": 0,
          "fieldset": true
        },
        {
          "id": "content",
          "title": "Content",
          "description": "Article body and images",
          "order": 1,
          "fieldset": false
        }
      ],
      "fields": {
        "title": {
          "section": "basic-info"
        },
        "slug": {
          "section": "basic-info"
        },
        "body": {
          "section": "content"
        }
      }
    }
  }
}
```

**Rendered output:**

```html
<section class="space-y-4 p-4 border border-gray-200 rounded-lg">
  <header class="space-y-1">
    <h2 class="text-lg font-semibold">Basic Information</h2>
    <p class="text-sm text-gray-600">Article title and metadata</p>
  </header>
  <fieldset>
    <div class="grid gap-6" style="grid-template-columns: repeat(12, minmax(0, 1fr))">
      <!-- title and slug fields -->
    </div>
  </fieldset>
</section>

<section class="space-y-4">
  <header class="space-y-1">
    <h2 class="text-lg font-semibold">Content</h2>
    <p class="text-sm text-gray-600">Article body and images</p>
  </header>
  <div class="grid gap-6" style="grid-template-columns: repeat(12, minmax(0, 1fr))">
    <!-- body field -->
  </div>
</section>
```

### Fieldset vs Plain Section

- `"fieldset": true` — Wraps fields in `<fieldset>`, adds border and padding
- `"fieldset": false` — Plain `<div>` wrapper, no visual grouping

---

## 5. Field Customization

### FieldConfig Structure

From [pkg/uischema/types.go](../pkg/uischema/types.go#L69):

```go
type FieldConfig struct {
    Section          string
    Order            *int
    Grid             *GridConfig
    Label            string
    LabelKey         string
    Description      string
    DescriptionKey   string
    HelpText         string
    HelpTextKey      string
    Placeholder      string
    PlaceholderKey   string
    Widget           string
    Component        string
    ComponentOptions map[string]any
    Icon             string
    IconSource       string
    IconRaw          string
    Behaviors        map[string]any
    CSSClass         string
    UIHints          map[string]string
    Metadata         map[string]string
}
```

### Grid Positioning

```json
{
  "fields": {
    "title": {
      "grid": {
        "span": 12,
        "start": 1,
        "row": 1
      }
    },
    "firstName": {
      "grid": {
        "span": 6
      }
    },
    "lastName": {
      "grid": {
        "span": 6
      }
    }
  }
}
```

**GridConfig:**
- `span` — Number of columns to span (1-12)
- `start` — Starting column (1-12)
- `row` — Row number (for explicit positioning)
- `breakpoints` — Per-breakpoint overrides for `span`/`start`/`row` (`sm`/`md`/`lg`/`xl`/`2xl`)

### Labels and Help Text

```json
{
  "fields": {
    "email": {
      "label": "Email Address",
      "placeholder": "you@example.com",
      "helpText": "We'll never share your email with anyone else.",
      "description": "Primary contact email for notifications"
    }
  }
}
```

**Rendered output:**

```html
<div>
  <label class="block text-sm font-medium">Email Address</label>
  <p class="text-xs text-gray-500 mb-1">Primary contact email for notifications</p>
  <input type="email" placeholder="you@example.com" ... />
  <p class="text-xs text-gray-500 mt-1">We'll never share your email with anyone else.</p>
</div>
```

---

## 6. Widgets and Components

UI schemas give you two levers that affect field rendering:

- `fields.<name>.widget` — a string hint stored on the form model (`field.UIHints["widget"]`).
- `fields.<name>.component` — a concrete component name (used by the vanilla renderer’s component registry).

### Widgets (hint)

Built-in widget identifiers (used by the widget registry and the Preact renderer) are defined in `pkg/widgets`:

- `toggle`
- `select`
- `chips`
- `code-editor`
- `json-editor`
- `key-value`

Vanilla renderer notes:
- `widget: "textarea"` switches a field to the `textarea` component.
- `widget: "json-editor"` switches a field to the `json_editor` component.
- Other widget values are preserved on the model but do not change vanilla HTML unless your templates/runtime interpret them.

### Components (vanilla renderer)

The vanilla renderer ships a component registry (`pkg/renderers/vanilla/components`). Built-in component names include:

- `input`, `textarea`, `select`, `boolean`
- `object`, `array`, `datetime-range`
- `wysiwyg`, `json_editor`, `file_uploader`

Use `componentOptions` to configure components; it is serialized into `field.metadata["component.config"]` and exposed to the component renderer as `components.ComponentData.Config`.

### Example: WYSIWYG Component

```json
{
  "fields": {
    "body": {
      "component": "wysiwyg",
      "label": "Article Content",
      "componentOptions": {
        "toolbar": ["bold", "italic", "link", "heading"],
        "placeholder": "Start writing..."
      }
    }
  }
}
```

Runtime note: the WYSIWYG editor is implemented in the browser runtime bundle `formgen-relationships.min.js` (`/runtime/formgen-relationships.min.js`). The `file_uploader` component injects this runtime automatically; if you only use `wysiwyg`, include the runtime yourself and call `FormgenRelationships.initRelationships()`.

### Example: File Uploader Component

```json
{
  "fields": {
    "featuredImage": {
      "component": "file_uploader",
      "label": "Featured Image",
      "componentOptions": {
        "variant": "image",
        "uploadEndpoint": "/api/uploads/hero",
        "allowedTypes": "image/*",
        "maxSize": 5242880,
        "preview": true,
        "multiple": false
      }
    }
  }
}
```

`file_uploader` options are defined by the runtime’s `FileUploaderConfig` (see `pkg/runtime/assets/formgen-relationships.min.js.map`).

### Example: Custom Component

```json
{
  "fields": {
    "event_id": {
      "component": "event-select",
      "componentOptions": {
        "placeholder": "Search events",
        "endpoint": "/api/events",
        "labelField": "name",
        "valueField": "id"
      }
    }
  }
}
```

**Register custom component:**

```go
// In your code:
// import "bytes"
// import "github.com/goliatone/go-formgen/pkg/model"
// import "github.com/goliatone/go-formgen/pkg/renderers/vanilla"
// import "github.com/goliatone/go-formgen/pkg/renderers/vanilla/components"
registry := components.NewDefaultRegistry()
registry.MustRegister("event-select", components.Descriptor{
    Renderer: func(buf *bytes.Buffer, field model.Field, data components.ComponentData) error {
        // Custom rendering logic
        return nil
    },
})

renderer, _ := vanilla.New(
    vanilla.WithComponentRegistry(registry),
)
```

---

## 7. Icons and Visual Enhancements

### Field Icons

```json
{
  "fields": {
    "search": {
      "icon": "search",
      "iconSource": "iconoir",
      "iconRaw": "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.5\"><circle cx=\"11\" cy=\"11\" r=\"6\"/><path d=\"M16 16L21 21\"/></svg>"
    }
  }
}
```

Notes for the built-in vanilla renderer:
- `iconRaw` is rendered inline (sanitized). When present it takes precedence over runtime icon resolution.
- `icon` and `iconSource` are emitted as data attributes (`data-icon`, `data-icon-source`). By default, vanilla templates render a placeholder glyph when only `icon` is present.

### Runtime Icon Providers

To render real icons without using `iconRaw`, register an icon provider in JavaScript. Providers map `{ iconSource, icon }` to an SVG string, which the runtime sanitizes and injects into the icon slot.

```html
<script src="/runtime/formgen-behaviors.min.js" defer></script>
<script>
  document.addEventListener("DOMContentLoaded", () => {
    const iconoir = {
      search: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="11" cy="11" r="6"/><path d="M16 16L21 21"/></svg>`,
    };

    window.FormgenBehaviors?.registerIconProvider?.("iconoir", (name) => iconoir[name] ?? null);

    // initBehaviors() also runs icon resolution (and any field behaviors).
    window.FormgenBehaviors?.initBehaviors?.(document);
  });
</script>
```

### CSS Classes

```json
{
  "fields": {
    "notes": {
      "cssClass": "field--notes custom-textarea"
    }
  }
}
```

**Custom CSS:**

```css
.field--notes textarea {
  min-height: 200px;
  font-family: monospace;
}
```

---

## 8. Behaviors and Interactions

### Auto-Slug Behavior

```json
{
  "fields": {
    "title": {
      "section": "basic-info",
      "placeholder": "My Article Title"
    },
    "slug": {
      "section": "basic-info",
      "placeholder": "my-article-title",
      "helpText": "Auto-generated from title but editable",
      "behaviors": {
        "autoSlug": {
          "source": "title"
        }
      }
    }
  }
}
```

### Runtime Setup

`go-formgen` emits behavior data attributes but does not execute them. To enable built-in behaviors, serve and include the runtime bundle:

```go
// Serve runtime bundles like /runtime/formgen-behaviors.min.js
mux.Handle("/runtime/",
  http.StripPrefix("/runtime/",
    http.FileServerFS(formgen.RuntimeAssetsFS()),
  ),
)
```

```html
<script src="/runtime/formgen-behaviors.min.js" defer></script>
<script>
  document.addEventListener("DOMContentLoaded", () => {
    window.FormgenBehaviors?.initBehaviors?.(document);
  });
</script>
```

Current built-in behaviors in `formgen-behaviors.min.js`:
- `autoSlug`
- `autoResize`

### Auto-Resize Textarea

`autoResize` automatically adjusts a textarea height to fit its content.

Config:
- `minRows?: number` - minimum row count (clamps the minimum height)
- `maxRows?: number` - maximum row count (clamps the maximum height)

```json
{
  "fields": {
    "notes": {
      "widget": "textarea",
      "uiHints": {
        "rows": 2
      },
      "behaviors": {
        "autoResize": {
          "minRows": 2,
          "maxRows": 6
        }
      }
    }
  }
}
```

### Custom Behavior Metadata

Behaviors are serialized into field metadata and exposed as:

- `data-behavior="name1 name2"` (space-separated)
- `data-behavior-config="…"` (JSON)

```json
{
  "fields": {
    "amount": {
      "behaviors": {
        "currencyFormat": {
          "locale": "en-US",
          "currency": "USD"
        }
      }
    }
  }
}
```

**Access from JavaScript:**

```js
const el = document.querySelector('#fg-amount'); // the rendered <input>/<textarea>/<select>
const names = (el.dataset.behavior || "").split(/\s+/).filter(Boolean);
const config = el.dataset.behaviorConfig ? JSON.parse(el.dataset.behaviorConfig) : undefined;
// If there is one behavior, config is that behavior's config (e.g. { locale: "en-US" }).
// If there are multiple behaviors, config is an object keyed by behavior name.
```

---

## 9. Visibility Rules

`go-formgen` supports conditional display via a `visibilityRule` string stored on fields. Rules are **no-op by default**: you must configure a `visibility.Evaluator` for them to be applied.

### Setting a Rule

UI schemas don’t have a dedicated `visibilityRule` field, but you can set it via `uiHints` or `metadata`:

```json
{
  "fields": {
    "discountCode": {
      "uiHints": {
        "visibilityRule": "hasDiscount == true"
      }
    }
  }
}
```

The rule language is evaluator-defined; `go-formgen` treats it as an opaque string.

### Enabling Evaluation

```go
gen := formgen.NewOrchestrator(
  orchestrator.WithVisibilityEvaluator(myEvaluator),
)

html, err := gen.Generate(ctx, orchestrator.Request{
  OperationID: "checkout",
  RenderOptions: render.RenderOptions{
    Values: map[string]any{
      "hasDiscount": true,
    },
  },
})
```

Notes:
- The evaluator receives `RenderOptions.VisibilityContext` (and `RenderOptions.Values` as a fallback for `ctx.Values`).
- Fields that evaluate to false are removed from the form model and won’t render.

### Built-in evaluator (optional)

`go-formgen` ships a small, dependency-free evaluator you can use for common cases:

```go
import visibilityexpr "github.com/goliatone/go-formgen/pkg/visibility/expr"

gen := formgen.NewOrchestrator(
  orchestrator.WithVisibilityEvaluator(visibilityexpr.New()),
)
```

---

## 10. Field Ordering

### Inline Order

```json
{
  "fields": {
    "title": { "order": 0 },
    "slug": { "order": 1 },
    "body": { "order": 2 },
    "published": { "order": 3 }
  }
}
```

### Order Presets

Define reusable ordering patterns at the document level, then reference them from sections:

```json
{
  "fieldOrderPresets": {
    "default": ["title", "slug", "category", "body", "published"],
    "minimal": ["title", "body"]
  },
  "operations": {
    "createArticle": {
      "sections": [
        {
          "id": "main",
          "orderPreset": "default"
        }
      ]
    }
  }
}
```

### Inline Array Order

```json
{
  "sections": [
    {
      "id": "main",
      "orderPreset": ["title", "author", "body", "tags"]
    }
  ]
}
```

Ordering notes:
- Field paths in ordering patterns are normalized the same way as field keys (see `pkg/uischema.NormalizeFieldPath`), so `tags[]` and `tags.items` refer to the same field.
- Use `"*"` inside an order preset to include “all remaining fields” at that position.

---

## 11. Complete Examples

### Example 1: Blog Article Form

**`ui-schemas/create-article.json`:**

```json
{
  "operations": {
    "createArticle": {
      "form": {
        "title": "Create New Article",
        "subtitle": "Fill in the article details below",
        "layout": {
          "gridColumns": 12,
          "gutter": "md"
        },
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
            "label": "Publish Article",
            "type": "submit"
          }
        ]
      },
      "sections": [
        {
          "id": "basic-info",
          "title": "Basic Information",
          "description": "Article title and metadata",
          "order": 0,
          "fieldset": true
        },
        {
          "id": "content",
          "title": "Content",
          "description": "Article body and media",
          "order": 1,
          "fieldset": false
        },
        {
          "id": "publishing",
          "title": "Publishing Options",
          "order": 2,
          "fieldset": true
        }
      ],
      "fields": {
        "title": {
          "section": "basic-info",
          "order": 0,
          "grid": { "span": 8 },
          "label": "Article Title",
          "placeholder": "Enter a compelling title",
          "helpText": "This will be displayed as the main heading"
        },
        "slug": {
          "section": "basic-info",
          "order": 1,
          "grid": { "span": 4 },
          "label": "URL Slug",
          "placeholder": "article-title",
          "helpText": "Auto-generated but editable",
          "behaviors": {
            "autoSlug": {
              "source": "title"
            }
          }
        },
        "category": {
          "section": "basic-info",
          "order": 2,
          "grid": { "span": 6 },
          "label": "Category",
          "uiHints": { "input": "select" }
        },
        "tags": {
          "section": "basic-info",
          "order": 3,
          "grid": { "span": 6 },
          "label": "Tags",
          "placeholder": "Add tags...",
          "component": "tag-input"
        },
        "excerpt": {
          "section": "content",
          "order": 0,
          "grid": { "span": 12 },
          "label": "Excerpt",
          "widget": "textarea",
          "placeholder": "Brief summary for listings",
          "helpText": "Maximum 200 characters"
        },
        "body": {
          "section": "content",
          "order": 1,
          "grid": { "span": 12 },
          "label": "Article Content",
          "component": "wysiwyg",
          "componentOptions": {
            "toolbar": ["bold", "italic", "link", "heading", "bulletList", "orderedList"],
            "placeholder": "Start writing your article..."
          }
        },
        "featuredImage": {
          "section": "content",
          "order": 2,
          "grid": { "span": 12 },
          "label": "Featured Image",
          "component": "file_uploader",
          "componentOptions": {
            "variant": "image",
            "uploadEndpoint": "/api/uploads/hero",
            "allowedTypes": "image/*",
            "maxSize": 5242880,
            "preview": true
          }
        },
        "published": {
          "section": "publishing",
          "order": 0,
          "grid": { "span": 6 },
          "label": "Publish Immediately"
        },
        "publishDate": {
          "section": "publishing",
          "order": 1,
          "grid": { "span": 6 },
          "label": "Scheduled Publish Date",
          "uiHints": { "inputType": "datetime-local" }
        }
      }
    }
  }
}
```

**Usage:**

```go
package main

import (
    "context"
    "embed"
    "fmt"
    "log"

    "github.com/goliatone/go-formgen"
    "github.com/goliatone/go-formgen/pkg/orchestrator"
    "github.com/goliatone/go-formgen/pkg/openapi"
)

//go:embed ui-schemas
var uiSchemas embed.FS

func main() {
    gen := formgen.NewOrchestrator(
        orchestrator.WithUISchemaFS(uiSchemas),
    )

    html, err := gen.Generate(context.Background(), orchestrator.Request{
        Source:      openapi.SourceFromFile("blog-api.json"),
        OperationID: "createArticle",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(string(html))
}
```

---

### Example 2: User Registration Form

**`ui-schemas/register-user.yaml`:**

```yaml
operations:
  registerUser:
    form:
      title: "Create Your Account"
      subtitle: "Join our community today"
      layout:
        gridColumns: 12
        gutter: md
      actions:
        - kind: secondary
          label: Already have an account?
          href: /login
        - kind: primary
          label: Create Account
          type: submit

    sections:
      - id: personal
        title: Personal Information
        fieldset: true
        order: 0
      - id: account
        title: Account Setup
        fieldset: true
        order: 1
      - id: preferences
        title: Preferences
        fieldset: false
        order: 2

    fields:
      firstName:
        section: personal
        order: 0
        grid:
          span: 6
        label: First Name
        placeholder: John

      lastName:
        section: personal
        order: 1
        grid:
          span: 6
        label: Last Name
        placeholder: Doe

      email:
        section: account
        order: 0
        grid:
          span: 12
        label: Email Address
        placeholder: you@example.com
        icon: mail
        iconSource: iconoir
        helpText: "We'll send a verification email"

      password:
        section: account
        order: 1
        grid:
          span: 6
        label: Password
        helpText: Minimum 8 characters

      passwordConfirm:
        section: account
        order: 2
        grid:
          span: 6
        label: Confirm Password

      newsletter:
        section: preferences
        order: 0
        grid:
          span: 12
        label: Subscribe to newsletter
        widget: toggle

      timezone:
        section: preferences
        order: 1
        grid:
          span: 6
        label: Timezone
        widget: select
```

---

### Example 3: E-commerce Product Form

**`ui-schemas/create-product.json`:**

```json
{
  "operations": {
    "createProduct": {
      "form": {
        "title": "Add New Product",
        "layout": {
          "gridColumns": 12
        },
        "actions": [
          {
            "kind": "secondary",
            "label": "Cancel",
            "href": "/products"
          },
          {
            "kind": "secondary",
            "label": "Save as Draft",
            "type": "button"
          },
          {
            "kind": "primary",
            "label": "Publish Product",
            "type": "submit"
          }
        ]
      },
      "sections": [
        {
          "id": "general",
          "title": "General Information",
          "order": 0
        },
        {
          "id": "pricing",
          "title": "Pricing & Inventory",
          "order": 1
        },
        {
          "id": "media",
          "title": "Product Media",
          "order": 2
        }
      ],
      "fields": {
        "name": {
          "section": "general",
          "grid": { "span": 8 },
          "label": "Product Name"
        },
        "sku": {
          "section": "general",
          "grid": { "span": 4 },
          "label": "SKU",
          "placeholder": "PROD-001"
        },
        "description": {
          "section": "general",
          "grid": { "span": 12 },
          "component": "wysiwyg",
          "label": "Description",
          "componentOptions": {
            "placeholder": "Write a detailed description..."
          }
        },
        "price": {
          "section": "pricing",
          "grid": { "span": 4 },
          "label": "Price",
          "placeholder": "0.00",
          "behaviors": {
            "currencyFormat": {
              "currency": "USD"
            }
          }
        },
        "compareAtPrice": {
          "section": "pricing",
          "grid": { "span": 4 },
          "label": "Compare at Price",
          "helpText": "Original price for sale display"
        },
        "stock": {
          "section": "pricing",
          "grid": { "span": 4 },
          "label": "Stock Quantity"
        },
        "images": {
          "section": "media",
          "grid": { "span": 12 },
          "component": "file_uploader",
          "label": "Product Images",
          "componentOptions": {
            "multiple": true,
            "allowedTypes": "image/*",
            "uploadEndpoint": "/api/uploads/products",
            "preview": true
          }
        }
      }
    }
  }
}
```

---

## Reference Tables

### Action Configuration

| Field | Type | Description |
|-------|------|-------------|
| `kind` | `string` | `"primary"` or `"secondary"` |
| `label` | `string` | Button text |
| `labelKey` | `string` | Optional i18n key for `label` |
| `href` | `string` | Link URL (renders `<a>`) |
| `type` | `string` | `"submit"`, `"reset"`, `"button"` (defaults to `"submit"`) |
| `icon` | `string` | Icon identifier (renderer-dependent) |

### Layout Configuration

| Field | Type | Description |
|-------|------|-------------|
| `gridColumns` | `int` | Number of grid columns (1-12) |
| `gutter` | `string` | Grid spacing (`"sm"` → `gap-4`, `"md"` → `gap-6`, `"lg"` → `gap-8`) |

### Section Configuration

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique section identifier |
| `title` | `string` | Section heading |
| `titleKey` | `string` | Optional i18n key for `title` |
| `description` | `string` | Section description |
| `descriptionKey` | `string` | Optional i18n key for `description` |
| `order` | `int` | Display order |
| `fieldset` | `bool` | Wrap in `<fieldset>` |
| `orderPreset` | `string/array` | Field ordering pattern |

### Field Configuration

| Field | Type | Description |
|-------|------|-------------|
| `section` | `string` | Section ID |
| `order` | `int` | Display order within section |
| `grid.span` | `int` | Column span (1-12) |
| `grid.start` | `int` | Starting column |
| `grid.row` | `int` | Row number |
| `label` | `string` | Field label |
| `labelKey` | `string` | Optional i18n key for `label` |
| `description` | `string` | Field description |
| `descriptionKey` | `string` | Optional i18n key for `description` |
| `helpText` | `string` | Help text below input |
| `helpTextKey` | `string` | Optional i18n key for `helpText` |
| `placeholder` | `string` | Input placeholder |
| `placeholderKey` | `string` | Optional i18n key for `placeholder` |
| `widget` | `string` | Widget hint (renderer/runtime-dependent) |
| `component` | `string` | Vanilla component name (e.g. `wysiwyg`, `file_uploader`) |
| `componentOptions` | `object` | Component configuration (serialized into `component.config`) |
| `icon` | `string` | Icon identifier (emitted as `data-icon`) |
| `iconSource` | `string` | Free-form source hint (emitted as `data-icon-source`) |
| `iconRaw` | `string` | Inline SVG markup (sanitized) |
| `behaviors` | `object` | Behavior configuration (emitted as `data-behavior*`) |
| `cssClass` | `string` | Additional wrapper CSS classes (sanitized in vanilla) |
| `uiHints` | `object` | Extra per-field UI hints (string map) |
| `metadata` | `object` | Extra per-field metadata (string map) |

---

## Best Practices

1. **Use sections** for logical grouping of related fields
2. **Set explicit order** for fields within sections to avoid layout shifts
3. **Provide help text** for complex or non-obvious fields
4. **Use grid spanning** to create visual hierarchy (wide for important fields)
5. **Choose appropriate widgets** based on field data type and validation
6. **Add icons** to improve scanability and visual appeal
7. **Declare behavior metadata** in UI schema; enable/implement handlers via the runtime JS
8. **Use fieldsets** for accessibility and semantic grouping
9. **Test action buttons** across different form states (valid, invalid, submitting)
10. **Document custom components** and their configuration options

---

## See Also

- [Styling & Customization Guide](GUIDE_STYLING.md) — Theme integration, CSS customization
- [Architecture & Design](../go-form-gen.md) — Package structure and design principles
- [API Reference](https://pkg.go.dev/github.com/goliatone/go-formgen) — Complete API documentation
