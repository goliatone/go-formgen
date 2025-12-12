# Form Customization Guide

## Overview

This guide covers non-styling customizations for `go-formgen` forms, including:

- **Custom action buttons** (submit, reset, cancel, custom handlers)
- **Form layout and sections** (grid, fieldsets, ordering)
- **Field-level customization** (widgets, components, behaviors)
- **UI hints and metadata** (icons, help text, placeholders)
- **Visibility rules** (conditional field display)

All customizations are managed through the **UI Schema** system, which allows you to configure form behavior without modifying templates or code.

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
9. [Field Ordering](#9-field-ordering)
10. [Complete Examples](#10-complete-examples)

---

## 1. UI Schema Basics

### What is a UI Schema?

A UI Schema is a JSON or YAML file that describes how to render a form without modifying the OpenAPI specification. It maps operation IDs to form configuration.

### Basic Structure

```json
{
  "operations": {
    "operationId": {
      "form": {
        "title": "Form Title",
        "subtitle": "Form subtitle",
        "layout": { ... },
        "actions": [ ... ],
        "metadata": { ... },
        "uiHints": { ... }
      },
      "sections": [ ... ],
      "fields": {
        "fieldName": { ... }
      },
      "fieldOrderPresets": { ... }
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
    orchestrator.WithUISchemaFS(nil),  // Disables embedded defaults
)
```

### File Naming Convention

UI schema files are discovered by operation ID or a general schema file:

- `create-pet.json` → Maps to `createPet` operation
- `edit-article.yaml` → Maps to `editArticle` operation
- `schema.json` / `schema.yaml` → Contains multiple operations

---

## 2. Custom Action Buttons

### Default Behavior

Without configuration, forms render a single "Submit" button ([form.tmpl:116](../pkg/renderers/vanilla/templates/form.tmpl#L116)):

```html
<button type="submit">Submit</button>
```

### ActionConfig Structure

From [types.go:43-50](../pkg/uischema/types.go#L43-50):

```go
type ActionConfig struct {
    Kind  string  // "primary" or "secondary" (styling)
    Label string  // Button text
    Href  string  // For link buttons (optional)
    Type  string  // "submit", "reset", "button" (optional)
    Icon  string  // Icon identifier (optional)
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

**Primary buttons** (`kind: "primary"`):
- Background: `bg-blue-600`
- Text: `text-white`
- Hover: `hover:bg-blue-700`
- Focus ring: `focus:ring-blue-600`

**Secondary buttons** (`kind: "secondary"` or omitted):
- Background: `bg-white` / `dark:bg-slate-900`
- Border: `border-gray-200`
- Text: `text-gray-800` / `dark:text-white`
- Hover: `hover:bg-gray-50`
- Focus ring: `focus:ring-gray-400`

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

**Gutter sizes:**
- `"sm"` — Small spacing
- `"md"` — Medium spacing (default)
- `"lg"` — Large spacing

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

From [types.go:52-62](../pkg/uischema/types.go#L52-62):

```go
type SectionConfig struct {
    ID          string
    Title       string
    Description string
    Order       *int
    Fieldset    *bool
    OrderPreset string
    Metadata    map[string]string
    UIHints     map[string]string
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

From [types.go:64-84](../pkg/uischema/types.go#L64-84):

```go
type FieldConfig struct {
    Section          string
    Order            *int
    Grid             *GridConfig
    Label            string
    Description      string
    HelpText         string
    Placeholder      string
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

### Built-in Widgets

Widgets control the field rendering strategy:

- `"text"` — Text input
- `"textarea"` — Multi-line text
- `"select"` — Dropdown
- `"checkbox"` — Single checkbox
- `"radio"` — Radio buttons
- `"datetime"` — Date/time picker
- `"wysiwyg"` — Rich text editor
- `"json-editor"` — JSON editor
- `"file-uploader"` — File upload

### Example: Rich Text Editor

```json
{
  "fields": {
    "body": {
      "widget": "wysiwyg",
      "label": "Article Content",
      "componentOptions": {
        "toolbar": ["bold", "italic", "link", "heading"],
        "placeholder": "Start writing..."
      }
    }
  }
}
```

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
// In your code
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

**Icon sources:**
- `"iconoir"` — Iconoir icon set
- `"heroicons"` — Heroicons
- `"lucide"` — Lucide icons
- Custom SVG via `iconRaw`

### CSS Classes

```json
{
  "fields": {
    "notes": {
      "cssClass": "fg-field--notes custom-textarea"
    }
  }
}
```

**Custom CSS:**

```css
.fg-field--notes textarea {
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

### Auto-Resize Textarea

```json
{
  "fields": {
    "notes": {
      "widget": "textarea",
      "behaviors": {
        "autoResize": {
          "minRows": 3,
          "maxRows": 10
        }
      }
    }
  }
}
```

### Custom Behavior Metadata

Behaviors are stored in field metadata for client-side JavaScript to consume:

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
const field = document.querySelector('[name="amount"]');
const behaviorConfig = JSON.parse(
  field.closest('.field-wrapper').dataset.behaviorConfig
);
// { currencyFormat: { locale: "en-US", currency: "USD" } }
```

---

## 9. Field Ordering

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

Define reusable ordering patterns:

```json
{
  "operations": {
    "createArticle": {
      "fieldOrderPresets": {
        "default": ["title", "slug", "category", "body", "published"],
        "minimal": ["title", "body"]
      },
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

---

## 10. Complete Examples

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
          "widget": "select"
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
          "helpText": "Maximum 200 characters",
          "behaviors": {
            "autoResize": {
              "minRows": 2,
              "maxRows": 4
            }
          }
        },
        "body": {
          "section": "content",
          "order": 1,
          "grid": { "span": 12 },
          "label": "Article Content",
          "widget": "wysiwyg",
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
          "widget": "file-uploader",
          "componentOptions": {
            "accept": "image/*",
            "maxSize": 5242880
          }
        },
        "published": {
          "section": "publishing",
          "order": 0,
          "grid": { "span": 6 },
          "label": "Publish Immediately",
          "widget": "checkbox"
        },
        "publishDate": {
          "section": "publishing",
          "order": 1,
          "grid": { "span": 6 },
          "label": "Scheduled Publish Date",
          "widget": "datetime"
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
        widget: checkbox

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
          "widget": "wysiwyg",
          "label": "Description"
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
          "widget": "file-uploader",
          "label": "Product Images",
          "componentOptions": {
            "multiple": true,
            "accept": "image/*",
            "maxFiles": 10
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
| `href` | `string` | Link URL (renders `<a>`) |
| `type` | `string` | `"submit"`, `"reset"`, `"button"` |
| `icon` | `string` | Icon identifier |

### Layout Configuration

| Field | Type | Description |
|-------|------|-------------|
| `gridColumns` | `int` | Number of grid columns (1-12) |
| `gutter` | `string` | `"sm"`, `"md"`, `"lg"` |

### Section Configuration

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Unique section identifier |
| `title` | `string` | Section heading |
| `description` | `string` | Section description |
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
| `description` | `string` | Field description |
| `helpText` | `string` | Help text below input |
| `placeholder` | `string` | Input placeholder |
| `widget` | `string` | Widget type |
| `component` | `string` | Custom component name |
| `componentOptions` | `object` | Component configuration |
| `icon` | `string` | Icon identifier |
| `iconSource` | `string` | Icon library |
| `behaviors` | `object` | Behavior configuration |
| `cssClass` | `string` | Additional CSS classes |

---

## Best Practices

1. **Use sections** for logical grouping of related fields
2. **Set explicit order** for fields within sections to avoid layout shifts
3. **Provide help text** for complex or non-obvious fields
4. **Use grid spanning** to create visual hierarchy (wide for important fields)
5. **Choose appropriate widgets** based on field data type and validation
6. **Add icons** to improve scanability and visual appeal
7. **Configure behaviors** in UI schema, not in JavaScript
8. **Use fieldsets** for accessibility and semantic grouping
9. **Test action buttons** across different form states (valid, invalid, submitting)
10. **Document custom components** and their configuration options

---

## See Also

- [Styling & Customization Guide](GUIDE_STYLING.md) — Theme integration, CSS customization
- [Architecture & Design](../go-form-gen.md) — Package structure and design principles
- [API Reference](https://pkg.go.dev/github.com/goliatone/go-formgen) — Complete API documentation
