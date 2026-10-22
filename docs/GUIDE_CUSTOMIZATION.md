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

------------------------------------------------------------------------

## Table of Contents

1.  [UI Schema Basics](#1-ui-schema-basics)
2.  [Custom Action Buttons](#2-custom-action-buttons)
3.  [Form Layout and Grid](#3-form-layout-and-grid)
4.  [Sections and Fieldsets](#4-sections-and-fieldsets)
5.  [Field Customization](#5-field-customization)
6.  [Widgets and Components](#6-widgets-and-components)
7.  [Icons and Visual Enhancements](#7-icons-and-visual-enhancements)
8.  [Behaviors and Interactions](#8-behaviors-and-interactions)
9.  [Visibility Rules](#9-visibility-rules)
10. [Field Ordering](#10-field-ordering)
11. [Render Options (Prefill and Subsets)](#11-render-options-prefill-and-subsets)
12. [OpenAPI Loader Configuration](#12-openapi-loader-configuration)
13. [Complete Examples](#13-complete-examples)

------------------------------------------------------------------------

## 1. UI Schema Basics

### What is a UI Schema?

A UI Schema is a JSON or YAML file that describes how to render a form without modifying the OpenAPI specification. It maps operation IDs to form configuration.

### Basic Structure

``` json
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

``` go
//go:embed ui-schemas
var uiSchemaFS embed.FS

gen := formgen.NewOrchestrator(
    orchestrator.WithUISchemaFS(uiSchemaFS),
)
```

**Option 2: Load from filesystem**

``` go
gen := formgen.NewOrchestrator(
    orchestrator.WithUISchemaFS(os.DirFS("./config/ui-schemas")),
)
```

**Option 3: Disable UI schemas**

``` go
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

------------------------------------------------------------------------

## Localization (i18n)

UI schema strings can be localized by pairing a `*Key` field with its fallback value.

- **Form**: `form.titleKey` / `form.subtitleKey`
- **Sections**: `sections[].titleKey` / `sections[].descriptionKey`
- **Fields**: `fields.<name>.labelKey`, `descriptionKey`, `placeholderKey`, `helpTextKey`
- **Actions**: `form.actions[].labelKey`

At render time, supply a `render.Translator` and `render.RenderOptions.Locale`. Missing translations can be customized with `render.RenderOptions.OnMissing`.

For template-level i18n (custom templates), inject a translate helper into template funcs:

``` go
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

``` html
<button type="submit">Submit</button>
```

### ActionConfig Structure

From [pkg/uischema/types.go](../pkg/uischema/types.go#L45):

``` go
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

| Field  | Value         | Result                           |
|--------|---------------|----------------------------------|
| `href` | URL string    | Renders `<a>` styled as button   |
| `type` | `"submit"`    | Submit button (form submission)  |
| `type` | `"reset"`     | Reset button (clears form)       |
| `type` | `"button"`    | Generic button (for JS handlers) |
| `kind` | `"primary"`   | Blue background, emphasized      |
| `kind` | `"secondary"` | White background, subtle         |

Notes for the built-in vanilla renderer:

- Button actions default to `type: "submit"` when omitted.
- For `<button>` actions, `kind` defaults to primary when omitted.
- For `<a href="…">` actions, `kind` defaults to secondary when omitted.

### Example: Submit + Cancel

``` json
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

``` html
<div class="formgen-actions">
  <a class="... bg-white text-gray-800 ..." href="/pets">Cancel</a>
  <button type="submit" class="... bg-blue-600 text-white ...">Create Pet</button>
</div>
```

### Example: Submit + Reset

``` json
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

``` json
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

``` html
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

------------------------------------------------------------------------

## 3. Form Layout and Grid

### Grid Configuration

Forms use CSS Grid with configurable columns ([form.tmpl](../pkg/renderers/vanilla/templates/form.tmpl)):

``` json
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

The built-in vanilla template honors `layout.gutter` and maps it to `--formgen-grid-gap`:

- `"sm"` → `1rem`
- `"md"` (default) → `1.5rem`
- `"lg"` → `2rem`

The renderer emits the resolved values as `data-formgen-columns` and `data-formgen-gutter` on the grid container and `data-formgen-span`, `data-formgen-start`, `data-formgen-row` (plus `-sm`…`-2xl` breakpoint variants) on each field wrapper. The bundled stylesheet turns those attributes into grid placement, so rendered forms carry no inline `style` attributes and work under a `style-src 'self'` Content-Security-Policy. The full contract is in the [Styling Guide, §5](GUIDE_STYLING.md#5-responsive-layouts).

### Form Title and Subtitle

``` json
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

``` html
<header class="formgen-header">
  <h1 class="text-2xl font-bold">Add New Pet</h1>
  <p class="text-sm text-gray-600">Fill in the details below to register a new pet</p>
</header>
```

------------------------------------------------------------------------

## 4. Sections and Fieldsets

### Section Configuration

From [pkg/uischema/types.go](../pkg/uischema/types.go#L55):

``` go
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

``` json
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

``` html
<section class="formgen-fieldset">
  <header class="space-y-1">
    <h2 class="text-lg font-semibold">Basic Information</h2>
    <p class="text-sm text-gray-600">Article title and metadata</p>
  </header>
  <fieldset>
    <div data-formgen-layout="grid" data-formgen-columns="12" data-formgen-gutter="md" class="formgen-grid">
      <div data-formgen-span="8"><!-- title --></div>
      <div data-formgen-span="4"><!-- slug --></div>
    </div>
  </fieldset>
</section>

<section class="formgen-section">
  <header class="space-y-1">
    <h2 class="text-lg font-semibold">Content</h2>
    <p class="text-sm text-gray-600">Article body and images</p>
  </header>
  <div data-formgen-layout="grid" data-formgen-columns="12" data-formgen-gutter="md" class="formgen-grid">
    <div data-formgen-span="12"><!-- body --></div>
  </div>
</section>
```

### Fieldset vs Plain Section

- `"fieldset": true` — Wraps fields in `<fieldset>`, adds border and padding
- `"fieldset": false` — Plain `<div>` wrapper, no visual grouping

### Chrome class overrides (per request)

If you want to reuse the same UI schema but change the chrome styling (form wrapper, section box, grid, errors, actions), you can do so via `RenderOptions.ChromeClasses`:

``` go
html, err := renderer.Render(ctx, form, render.RenderOptions{
  ChromeClasses: &render.ChromeClasses{
    Form:    vanilla.DefaultFormClass + " formgen-form--embed",
    Section: vanilla.DefaultSectionClass,
    Grid:    vanilla.DefaultGridClass,
    Actions: vanilla.DefaultActionsClass,
  },
})
```

------------------------------------------------------------------------

## 5. Field Customization

### FieldConfig Structure

From [pkg/uischema/types.go](../pkg/uischema/types.go#L69):

``` go
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

``` json
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

- `span` — Number of columns to span (1 to `gridColumns`; defaults to the full width)
- `start` — Starting column (1 to `gridColumns`)
- `row` — Row number (for explicit positioning)
- `breakpoints` — Per-breakpoint overrides for `span`/`start`/`row` (`sm`/`md`/`lg`/`xl`/`2xl`); overrides cascade upward (mobile-first)

The values become `data-formgen-span`, `data-formgen-start`, `data-formgen-row`, and `data-formgen-<property>-<breakpoint>` attributes on the field wrapper. The bundled stylesheet covers values 1–24; larger values still emit the attribute and need a host rule.

### Labels and Help Text

``` json
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

``` html
<div>
  <label class="block text-sm font-medium">Email Address</label>
  <p class="text-xs text-gray-500 mb-1">Primary contact email for notifications</p>
  <input type="email" placeholder="you@example.com" ... />
  <p class="text-xs text-gray-500 mt-1">We'll never share your email with anyone else.</p>
</div>
```

------------------------------------------------------------------------

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

### JSON Editor Component

The `json_editor` component renders the JSON editor (raw textarea, GUI editor, or hybrid toggle).

**Active view (hybrid only):**

- Valid values: `raw`, `gui`.
- Default: `raw` (backwards compatible).
- If `mode` is `gui` (GUI-only), the active view is always `gui` regardless of hints.

**Configuration precedence (highest → lowest):**

1.  `uiHints.editorActiveView` / `uiHints.editorMode`
2.  `metadata["editor.activeView"]` / `metadata["editor.mode"]`
3.  `componentOptions.activeView` / `componentOptions.mode`

Other supported hints:

- `schemaHint` (`uiHints.schemaHint`, `metadata["schema.hint"]`, `componentOptions.schemaHint`)
- `jsonExample` (`uiHints.jsonExample`, `metadata["schema.example"]`, `componentOptions.example`)
- `collapsed` (`uiHints.collapsed`, `metadata["json.collapsed"]`, `componentOptions.collapsed`)

**Example (UI hints):**

``` json
{
  "fields": {
    "settings": {
      "widget": "json-editor",
      "uiHints": {
        "editorMode": "hybrid",
        "editorActiveView": "gui",
        "schemaHint": "Key/value configuration for the app",
        "jsonExample": "{ \"enabled\": true }"
      }
    }
  }
}
```

**Example (componentOptions):**

``` json
{
  "fields": {
    "settings": {
      "component": "json_editor",
      "componentOptions": {
        "mode": "hybrid",
        "activeView": "gui"
      }
    }
  }
}
```

### Example: WYSIWYG Component

``` json
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

Runtime note: the WYSIWYG editor and runtime-backed form widgets are implemented in the browser runtime bundle `formgen-relationships.min.js` (`/runtime/formgen-relationships.min.js`). The `wysiwyg`, `file_uploader`, and `media_picker` components inject this runtime automatically; if you render relationship fields without one of those components, include the runtime yourself and call `FormgenRelationships.initRelationships()`.

For embedded forms, the same bundle exposes `window.Formgen.attach(root)`. The returned controller supports `getValues`, `setValues`, `reset`, `setErrors`, `clearErrors`, `onChange`, `focus`, and `destroy`, and works with full form roots or fields-only roots. Runtime-backed option fields should use the root-scoped lifecycle so resolver state is also torn down:

``` js
const registry = await FormgenRelationships.initFormgenRoot(root, resolverConfig);
const controller = FormgenRelationships.Formgen.attach(root, { registry });
// controller.reset() restores defaults and synchronizes enhanced widgets.
// controller.destroy() destroys its registry; registry.destroy(root) is also
// available when the host owns lifecycle directly.

// Embedded hydration contract (opt-in through the options object):
controller.setValues(snapshot, { mode: "replace", notify: false });
controller.onChange((values, event, meta) => {
  // meta.origin: "ui" | "program" | "hydrate" | "reset"
  // meta.paths: canonical changed paths such as "rows[1].name"
});
```

Hydration notifications: `setValues(values)` without options keeps the legacy behavior of one callback per native `input`/`change` event, including duplicate notifications for radio and checkbox groups. With an options object the mutation settles once with `origin: "hydrate"` and emits no callback when nothing changed; `notify: false` suppresses that controller's subscribers only. Enhanced widgets, submission mirrors, dependent option refreshes, components, and native listeners on changed controls still run. `mode: "replace"` clears omitted enabled controls, treats `null` as an explicit clear, replaces checkbox groups and multi-selects, hydrates rendered repeated rows from arrays of objects, and throws before any control changes when a payload row has no rendered row. Disabled controls are cleared only through explicit values. `reset(options)` settles with `origin: "reset"`; `reset()` keeps the legacy per-event notifications. Native events report `origin: "ui"` when trusted and `"program"` when dispatched by scripts; hydration does not rewrite existing option labels.

Model-aware validation: pass the complete form model (the descriptor's `form`) as `Formgen.attach(root, { model })` to unlock `controller.normalize()` and `controller.validate(scope)`. Normalization mirrors the Go `submission` package: typed enum control values decode to their JSON types, `false` and `0` are preserved, string `"false"` stays a string for string fields, optional empty non-string scalars become `null`, row objects keyed by index become arrays, and non-integral or unsafe integers report `type` issues instead of rounding. Validation reuses `required`, `type`, `enum`, `min`/`max`, `minLength`/`maxLength` (code points), `pattern` (portable subset), `minItems`/ `maxItems`, and `uniqueItems`. `validate({ paths: ["colors"] })` checks only those canonical subtrees so unrelated required fields do not block a step; `validate()` checks the whole form. Issues carry precise paths, legacy `rendererPath` grouping, and replaceable messages without answer values. The raw collection rules still apply: disabled controls are excluded, hidden inputs are included unless `includeHidden: false`, readonly and CSS-hidden controls are included. Applicability, inactive values, and asynchronous validators are outside this layer.

Reactive conditions: when the compiled model includes root `x-formgen.conditions` (`formgen.conditions/v1`), the descriptor carries them and `Formgen.attach(root, { model })` evaluates them in the browser with the same semantics as `submission.ValidateConditional` on the server. Inapplicable controls are hidden through their component wrapper and marked with `data-formgen-applicable="false"`, requiredness follows `requiredWhen`, values are cleared or retained per `onInapplicable`, obsolete issues are removed, and `meta.applicable` accompanies each change notification. Use `controller.effective().values` as the submission payload and `conditions: { hideInapplicable: false }` to keep layout control. Invalid definitions compile to `condition` issues without hiding anything.

Native choice components: the default registry includes three optional components that are selected only through component metadata and never replace the default boolean checkbox, select, or repeater:

- `choice` renders a single-choice radio group (`fieldset`/`legend`). It uses `options` or `enum`; a boolean field without either renders Yes/No, with `component.config` `trueLabel`/`falseLabel` overrides.
- `choice-multiple` renders a checkbox group for an array field whose items (or the field itself) declare `enum`/`options`; controls are named `path[]`.
- `scale` renders discrete numeric radios for an integer or number field, from `options`/`enum` or from inclusive `min`/`max` rules (optional `step`, `lowLabel`, `highLabel` in `component.config`); zero is a valid bound.

Option values use the typed enum codec, nothing is selected without a default, and IDs follow `control.id` (`fg-<path>` by default, reindexed in repeater rows). `choice` and `scale` fieldsets have `role="radiogroup"` and carry `aria-required`/`aria-invalid`; their radios also carry `required` but never `aria-required`, which ARIA supports on the group, not on `radio`. `choice-multiple` fieldsets are plain groups, so requiredness and invalidity are conveyed by `data-formgen-required`/`data-validation-state` and a visually hidden legend hint (`component.config.requiredHint`, default "(required)"); checkbox members never carry `required` or `aria-required`. Readonly and disabled render disabled native controls with `aria-readonly`/`aria-disabled`. Group descriptions and errors live inside the fieldset (`[data-relationship-error]`), so `setIssues` marks the group. Distinct multi-choice answers are enforced by the `uniqueItems` model rule, not inferred from markup. Style with the `formgen-choice`, `formgen-choice-option`, `formgen-choice-input`, `formgen-choice-text`, `formgen-choice-hint`, and `data-formgen-choice="single|multiple|scale"` hooks; `--formgen-choice-target` (default 2.75rem) sets the minimum target size. Inline metadata spelling: `"x-formgen": { "component.name": "scale", "component.config": { "lowLabel": "Low" } }`.

Typed snapshots: `controller.snapshot()` (model required) captures effective typed values plus explicit presence per path (`present`, `null`, `absent`), retained inactive values, locked-control values, and issues for unsafe integers, unknown controls, and detached rows; `controller.restore(snapshot)` puts a snapshot back with presence intact and reports unknown or unsafe paths. An unanswered checkbox stays absent instead of becoming `false`, a cleared integer is `null`, a cleared string is `""`, and an emptied checkbox group is `[]`.

Issue presentation: `controller.setIssues(issues)` displays browser or server issues without changing values or submitting. Precise paths target the exact rendered control (`rows[1].name` marks only that row), form-level issues and issues without a rendered control render into the summary container (`[data-formgen-form-errors]`, which the vanilla form summary now carries), and repeater items or fieldsets containing invalid controls receive `data-formgen-group-invalid="true"` for group chrome. `focusIssue(issue)` focuses the first enabled, visible control and otherwise focuses the summary; `clearIssues()` clears inline and summary display. Field messages stay replaceable through `registerErrorRenderer`; summaries through `registerFormErrorRenderer` plus `data-formgen-form-errors-renderer`. The runnable `examples/embedded` program wires the whole Phase 1 flow.

Root initialization is isolated: `resolverConfig` and the returned registry do not replace global resolver configuration or `globalThis.formgenRelationships`. The attached controller uses that exact registry for value/error hydration. Controller or registry teardown also disposes refresh/search listeners, pending delayed work, enhanced relationship widgets, components, array repeaters, and binding markers. A still-mounted root can therefore be initialized again with a new registry or configuration without stale closures or duplicate handlers. Call `initRelationships` only when a page-wide registry is intended.

Enhanced multi-selects keep their generated hidden inputs for native browser submission, but the controller excludes those transport mirrors from `getValues()`. It reads the underlying select as the authoritative field, so a one-item multi-select remains an array and JSON submission mirrors do not turn controller values into serialized strings. Host-authored hidden inputs are still included by default.

`resolverConfig.beforeFetch` can rewrite or sign a generic option request while preserving its `AbortSignal`. This is the intended host boundary for protected option endpoints; loading, empty/error state, dependent refresh, caching, stale-response rejection, and select/chips DOM updates remain runtime-owned. Absolute URI references, including host-owned synthetic schemes such as `command-options://`, are preserved until this hook runs. Relative endpoint paths continue to use `baseUrl`. A synthetic scheme must always be rewritten by the hook; it is never an application route and should not be dispatched as-is.

### Custom Repeatable Array Renderers

The relationship runtime also powers generated repeatable-array add/remove controls. If a custom renderer emits compatible controls, keep the row metadata contract intact:

- The array item container must have `data-formgen-array-item="true"`.
- Rows loaded from stored values must also have `data-formgen-array-existing="true"`.
- Prototype rows and newly added rows must have `data-formgen-array-existing="false"`.
- Remove buttons use `data-formgen-array-action="remove"`.
- Persisted rows that should be soft-deleted need a hidden `_delete` control in the same array item, for example `name="links[0]._delete" value="false"`.

On remove, the runtime sets the current item-level `_delete` control to `true`, disables the rest of that row, and hides the row. Newly cloned rows are removed from the DOM instead, so unsaved rows do not submit delete sentinels. Without `data-formgen-array-existing`, the runtime treats a row as new and removes it physically.

### Example: File Uploader Component

``` json
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

### Example: Media Picker Component

``` json
{
  "fields": {
    "heroImage": {
      "component": "media_picker",
      "label": "Hero Image",
      "componentOptions": {
        "libraryPath": "/admin/api/media/library",
        "itemEndpoint": "/admin/api/media/library/:id",
        "resolveEndpoint": "/admin/api/media/resolve",
        "capabilitiesEndpoint": "/admin/api/media/capabilities",
        "uploadEndpoint": "/admin/api/media/upload",
        "presignEndpoint": "/admin/api/media/presign",
        "confirmEndpoint": "/admin/api/media/confirm",
        "valueMode": "url",
        "acceptedKinds": ["image"]
      }
    }
  }
}
```

`media_picker` expects the host to normalize any schema-specific metadata into `component` plus `componentOptions` before rendering. The runtime reads only the serialized `component.config` payload on the rendered field wrapper.

- `valueMode` defaults to `url`; enable `id` only when the host also exposes `itemEndpoint` or `resolveEndpoint` for ID hydration.
- `capabilitiesEndpoint` should expose request-scoped operations, upload limits, and supported picker value modes. The runtime uses that payload to decide whether it should upload with multipart or presign+confirm.
- `multiple: true` mirrors `file_uploader` behavior and serializes repeated hidden inputs using the field’s logical name plus `[]`.
- `libraryPath` is used for browsing only. Current value hydration goes through `itemEndpoint` or `resolveEndpoint` so the picker never scans the whole library just to render one selected item.
- The future cleanup path is to retire the legacy `go-admin` `formgen/file_uploader` asset once supported hosts have moved to the shared `media_picker` contract.

### Metadata sources, precedence, registration timing, and preflight

Three authoring surfaces configure components and copy, and they are merged before rendering:

- **Inline schema metadata** (`x-formgen` inside a JSON Schema or OpenAPI schema): `component.name`, `component.config`, `label`, `placeholder`, `helpText`. The aliases `component`, `componentName`, `component-name`, `componentOptions`, and `title` are ignored; `model.DiagnoseExtensions`, `model.DiagnoseSchema`, and `jsonschema.DiagnoseDefinition` report them with their JSON Pointer locations.
- **JSON Schema overlays** (`jsonschema.ParseOverlay`/`ApplyOverlay`) inject extensions by JSON Pointer before normalization. Run `jsonschema.DiagnoseDefinition` on the applied overlay to inspect the effective definition; the canonical IR keeps normalized paths, not original file lines.
- **Operation UI schemas** (`fields.<path>.component`, `componentOptions`, `label`) are applied by the orchestrator decorator; `uischema.DiagnoseDefinition` checks the raw file before ignored spellings are discarded.

Precedence: an explicit UI schema `component` (stored as `UIHints["component"]`) wins over inline `component.name`, which wins over widget aliases and the field-type default. UI schema `label` overrides the inline label; the field-name labeler is the fallback; a primitive schema `title` never labels a field. `component.config` must be a JSON object (inline string or object).

Registration timing: register Go components on a `components.Registry` before calling `vanilla.New(vanilla.WithComponentRegistry(registry))`; the renderer clones the registry, so later registrations are invisible to `Render` and `Preflight`. In the browser, call `registerComponent` before `initRelationships`/`initFormgenRoot` so `initComponents` can enhance the wrappers; re-initialize a root after late registrations.

Two registries, one headless compiler: the Go registry decides which server-side markup a `component.name` produces, and the browser factory registry decides which `[data-component]` wrappers receive runtime behavior. Neither is consulted while compiling the form model, so headless builds and the JSON descriptor stay renderer-free. Server-only components (inputs, selects, the native choice groups) legitimately report `enhanced: false` in the browser preflight.

Preflight: `renderer.Preflight(form)` resolves every field's component the way `Render` does and returns `DefinitionDiagnostic` entries with dotted field paths (`rows[].answer` for array items): `componentUnknown` for unregistered names (`Render` still fails for them exactly as before), `componentConfig` for malformed config, and `componentRender` when a registered component rejects the field (for example `scale` without integer bounds). Messages contain component names and causes only, never values. In the browser, `preflightComponents(root)` lists wrappers with their `enhanced` flag and any `data-component-config` parse error; `registeredComponentNames()` lists the factories.

Adapter limits: OpenAPI and JSON Schema both carry inline `x-formgen`, but overlays exist only for the JSON Schema adapter, and neither adapter preserves original file positions after `$ref` resolution.

### Example: Custom Component

``` json
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

``` go
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

------------------------------------------------------------------------

## 7. Icons and Visual Enhancements

### Field Icons

``` json
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

``` html
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

``` json
{
  "fields": {
    "notes": {
      "cssClass": "field--notes custom-textarea"
    }
  }
}
```

**Custom CSS:**

``` css
.field--notes textarea {
  min-height: 200px;
  font-family: monospace;
}
```

------------------------------------------------------------------------

## 8. Behaviors and Interactions

### Auto-Slug Behavior

``` json
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

``` go
// Serve runtime bundles like /runtime/formgen-behaviors.min.js
mux.Handle("/runtime/",
  http.StripPrefix("/runtime/",
    http.FileServerFS(formgen.RuntimeAssetsFS()),
  ),
)
```

``` html
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

``` json
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

``` json
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

``` js
const el = document.querySelector('#fg-amount'); // the rendered <input>/<textarea>/<select>
const names = (el.dataset.behavior || "").split(/\s+/).filter(Boolean);
const config = el.dataset.behaviorConfig ? JSON.parse(el.dataset.behaviorConfig) : undefined;
// If there is one behavior, config is that behavior's config (e.g. { locale: "en-US" }).
// If there are multiple behaviors, config is an object keyed by behavior name.
```

------------------------------------------------------------------------

## 9. Visibility Rules

`go-formgen` supports conditional display via a `visibilityRule` string stored on fields. Rules are **no-op by default**: you must configure a `visibility.Evaluator` for them to be applied.

### Setting a Rule

UI schemas don’t have a dedicated `visibilityRule` field, but you can set it via `uiHints` or `metadata`:

``` json
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

``` go
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

``` go
import visibilityexpr "github.com/goliatone/go-formgen/pkg/visibility/expr"

gen := formgen.NewOrchestrator(
  orchestrator.WithVisibilityEvaluator(visibilityexpr.New()),
)
```

------------------------------------------------------------------------

## 10. Field Ordering

### Inline Order

``` json
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

``` json
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

``` json
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

- Field paths in ordering patterns are normalized the same way as field keys (see `pkg/uischema.NormalizeFieldPath`), so `tags[]`, `tags[0]`, and `tags.items` refer to the same field.
- Block union variants can be targeted via `blocks.items.oneOf.<type>.<field>` (for example, `blocks.items.oneOf.hero.headline`).
- Use `"*"` inside an order preset to include “all remaining fields” at that position.
- For schema-owned layout, sibling properties can declare `x-formgen.order` or `x-admin.order` directly on the schema. This is honored recursively, including object fields inside array items such as `columns[].title` before `columns[].entries`.
- A single ordered sibling is enough to affect order. Ordered siblings render first by numeric order, then unordered siblings render by property name; order every sibling in the set when you need complete placement control.
- JSON Schema overlays can use nested maps (`"x-formgen": {"order": 1}`), dashed keys (`"x-formgen-order": 1`), or dotted keys (`"x-formgen.order": 1`).

------------------------------------------------------------------------

## 11. Render Options (Prefill and Subsets)

UI schema handles layout and metadata, but some customization is per-render:

- **Prefill values** (including relationship defaults)
- **Provenance badges** and **Readonly/Disabled** flags
- **Subset rendering** by groups/tags/sections
- **Server errors** and hidden fields

### Example: Prefill + Readonly/Disabled

``` go
output, err := gen.Generate(ctx, orchestrator.Request{
  OperationID: "createPet",
  RenderOptions: render.RenderOptions{
    Values: map[string]any{
      "name": render.ValueWithProvenance{
        Value:      "Fido",
        Provenance: "tenant default",
        Readonly:   true,
      },
      "status": render.ValueWithProvenance{
        Value:    "active",
        Disabled: true,
      },
    },
  },
})
```

### Example: Subset Rendering + Server Errors

``` go
output, err := gen.Generate(ctx, orchestrator.Request{
  OperationID: "createPet",
  RenderOptions: render.RenderOptions{
    Subset: render.FieldSubset{Groups: []string{"notifications"}},
    Errors: map[string][]string{
      "email": {"Email already used"},
    },
  },
})
```

------------------------------------------------------------------------

## 12. OpenAPI Loader Configuration

The loader resolves OpenAPI sources (filesystem, `fs.FS`, or HTTP). By default, `go-formgen` is **offline-first**; HTTP loading is disabled unless explicitly enabled.

### Example: Custom Filesystem + HTTP Fallback

``` go
import (
  "net/http"
  "os"
  "time"

  "github.com/goliatone/go-formgen"
  "github.com/goliatone/go-formgen/pkg/openapi"
  "github.com/goliatone/go-formgen/pkg/orchestrator"
)

loader := formgen.NewLoader(
  openapi.WithFileSystem(os.DirFS("./specs")),
  openapi.WithHTTPClient(http.DefaultClient),
  openapi.WithHTTPFallback(5*time.Second),
)

gen := formgen.NewOrchestrator(
  orchestrator.WithLoader(loader),
)
```

### Example: Allow Default HTTP Sources

``` go
loader := formgen.NewLoader(openapi.WithDefaultSources())
gen := formgen.NewOrchestrator(orchestrator.WithLoader(loader))
```

------------------------------------------------------------------------

## 13. Complete Examples

### Example 1: Blog Article Form

**`ui-schemas/create-article.json`:**

``` json
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

``` go
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

------------------------------------------------------------------------

### Example 2: User Registration Form

**`ui-schemas/register-user.yaml`:**

``` yaml
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

------------------------------------------------------------------------

### Example 3: E-commerce Product Form

**`ui-schemas/create-product.json`:**

``` json
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

------------------------------------------------------------------------

## Reference Tables

### Action Configuration

| Field | Type | Description |
|----|----|----|
| `kind` | `string` | `"primary"` or `"secondary"` |
| `label` | `string` | Button text |
| `labelKey` | `string` | Optional i18n key for `label` |
| `href` | `string` | Link URL (renders `<a>`) |
| `type` | `string` | `"submit"`, `"reset"`, `"button"` (defaults to `"submit"`) |
| `icon` | `string` | Icon identifier (renderer-dependent) |

### Layout Configuration

| Field | Type | Description |
|----|----|----|
| `gridColumns` | `int` | Number of grid columns (1-12) |
| `gutter` | `string` | Grid spacing (`"sm"` → `1rem`, `"md"` → `1.5rem`, `"lg"` → `2rem`) |

### Section Configuration

| Field            | Type           | Description                         |
|------------------|----------------|-------------------------------------|
| `id`             | `string`       | Unique section identifier           |
| `title`          | `string`       | Section heading                     |
| `titleKey`       | `string`       | Optional i18n key for `title`       |
| `description`    | `string`       | Section description                 |
| `descriptionKey` | `string`       | Optional i18n key for `description` |
| `order`          | `int`          | Display order                       |
| `fieldset`       | `bool`         | Wrap in `<fieldset>`                |
| `orderPreset`    | `string/array` | Field ordering pattern              |

### Field Configuration

| Field | Type | Description |
|----|----|----|
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
| `component` | `string` | Vanilla component name (e.g. `wysiwyg`, `file_uploader`, `media_picker`) |
| `componentOptions` | `object` | Component configuration (serialized into `component.config`) |
| `icon` | `string` | Icon identifier (emitted as `data-icon`) |
| `iconSource` | `string` | Free-form source hint (emitted as `data-icon-source`) |
| `iconRaw` | `string` | Inline SVG markup (sanitized) |
| `behaviors` | `object` | Behavior configuration (emitted as `data-behavior*`) |
| `cssClass` | `string` | Additional wrapper CSS classes (sanitized in vanilla) |
| `uiHints` | `object` | Extra per-field UI hints (string map) |
| `metadata` | `object` | Extra per-field metadata (string map) |

------------------------------------------------------------------------

## Best Practices

1.  **Use sections** for logical grouping of related fields
2.  **Set explicit order** for fields within sections to avoid layout shifts
3.  **Provide help text** for complex or non-obvious fields
4.  **Use grid spanning** to create visual hierarchy (wide for important fields)
5.  **Choose appropriate widgets** based on field data type and validation
6.  **Add icons** to improve scanability and visual appeal
7.  **Declare behavior metadata** in UI schema; enable/implement handlers via the runtime JS
8.  **Use fieldsets** for accessibility and semantic grouping
9.  **Test action buttons** across different form states (valid, invalid, submitting)
10. **Document custom components** and their configuration options

------------------------------------------------------------------------

## See Also

- [Styling & Customization Guide](GUIDE_STYLING.md) — Theme integration, CSS customization
- [Architecture & Design](../go-form-gen.md) — Package structure and design principles
- [API Reference](https://pkg.go.dev/github.com/goliatone/go-formgen) — Complete API documentation
