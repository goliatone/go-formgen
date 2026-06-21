# Form Style Overrides (Vanilla Renderer)

## Summary

go-formgen's vanilla renderer hard-codes the root `<form>` class list in
`pkg/renderers/vanilla/templates/form.tmpl`. When go-admin embeds the generated
HTML in a preview container that already has border/background/padding, the
result is a "box in a box" effect. There is no per-request way to override the
`<form>` class list; the only workaround is template replacement at renderer
construction time, which is global.

This document proposes a phased plan:

1. Per-request chrome class overrides (RenderOptions.ChromeClasses).
2. go-theme driven root template overrides.
3. Semantic class + CSS refactor.

## Context

- go-admin's Content Type Builder calls `orchestrator.Generate()` using the
  vanilla renderer to produce a preview.
- The preview container already provides border/background/padding.
- The vanilla renderer emits:
  `<form class="max-w-4xl mx-auto space-y-6 p-6 bg-white rounded-xl border ...">`
- `RenderOptions` has no class override field.
- go-theme is already wired for component partials and tokens, but not for the
  root form template.

## Goals

- Allow per-request override of root `<form>` classes without template swaps.
- Provide an ergonomic path for overriding multiple "chrome" classes, not just
  the root form.
- Make it possible for go-theme to supply a custom root form template.
- Move toward semantic classes and CSS variables for long-term theming.

## Non-goals

- Redesign the component rendering pipeline.
- Break existing HTML output or test snapshots by default.
- Replace Tailwind utilities immediately.

## Phase 1: RenderOptions.ChromeClasses (per-request) — Complete

### Overview

Add a structured chrome-class override to `RenderOptions`. This unlocks
per-request styling changes (e.g., preview mode) without global template swaps.

### API additions (RenderOptions)

```go
// ChromeClasses defines class overrides for high-level form chrome elements.
type ChromeClasses struct {
    // Form overrides the root <form> class list.
    // When non-empty, the value replaces the default classes entirely.
    Form string

    // Header overrides the <header> that wraps title/subtitle.
    Header string

    // Section overrides the <section> wrapper for grouped fields.
    Section string

    // Fieldset overrides the class list on <section> when section.fieldset is true.
    Fieldset string

    // Actions overrides the class list on the footer action row.
    Actions string

    // Errors overrides the class list on the form errors container.
    Errors string

    // Grid overrides the class list on grid containers that wrap fields.
    Grid string
}

type RenderOptions struct {
    // existing fields ...
    ChromeClasses *ChromeClasses
}
```

### Template behavior

- For each chrome slot, if the corresponding class override is non-empty,
  replace the default class list entirely.
- If an override is empty or nil, use the existing default class list.

Example for the `<form>` tag:

```jinja
<form class="{% if render_options.chrome_classes.form %}{{ render_options.chrome_classes.form }}{% else %}{{ default_form_class }}{% endif %}" ...>
```

### Renderer changes

- Add a default class constant in `pkg/renderers/vanilla` for reuse:
  `DefaultFormClass`.
- Pass `ChromeClasses` into the template context under
  `render_options.chrome_classes`.
- Optionally pass `default_form_class` as a template variable to avoid repeating
  literals in the template.

### Backward compatibility

- Default behavior remains identical when ChromeClasses is nil or empty.

### Tests

- Add tests to the vanilla renderer:
  - When `ChromeClasses.Form` is set, ensure `<form class="...">` uses it.
  - When `ChromeClasses` is nil or empty, ensure defaults are preserved.

### Implementation notes

- `render.ChromeClasses` and `RenderOptions.ChromeClasses` added.
- `vanilla.DefaultFormClass` constant introduced.
- Vanilla template updated to honor `chrome_classes.*` overrides.
- Tests added under `pkg/renderers/vanilla/renderer_chrome_classes_test.go`.

## Phase 2: go-theme driven root template override — Complete

### Overview

Allow go-theme to override the root form template, not just components. This
enables a theme to replace the overall form chrome structure.

### Implementation plan

- Add a new theme partial key, e.g. `forms.form` (or `layout.form`).
- Extend `defaultThemeFallbacks()` to include:
  `"forms.form": "templates/form.tmpl"`
- In the vanilla renderer, resolve the template name before rendering:
  - If `renderOptions.Theme.Partials["forms.form"]` is set, use that template.
  - Otherwise use the default `templates/form.tmpl`.

### Backward compatibility

- Without a theme override, output is unchanged.
- Component partials continue to work as today.

### Tests

- Add a renderer test that provides a theme partial for `forms.form` and asserts
  the renderer uses it (can be a small custom template in an in-memory FS).

### Implementation notes

- Added `forms.form` to `defaultThemeFallbacks()`.
- Vanilla renderer resolves the root form template via `forms.form`.
- Test added to assert the override is honored.

## Phase 3: Semantic classes + CSS refactor — Complete

### Overview

Replace embedded Tailwind utility lists with stable semantic classes and move
styling into CSS. This improves customizability and makes go-theme tokens more
impactful without template overrides.

Decision: ship semantic-only. No ClassMode toggle and no multi-release
migration plan.

Status: complete (2026-02-03).

### Changes to implement

- Replace all Tailwind utility class lists in chrome elements with semantic
  classes:
  - `.formgen-form`, `.formgen-header`, `.formgen-section`,
    `.formgen-fieldset`, `.formgen-actions`, `.formgen-errors`, `.formgen-grid`
- Ship a default stylesheet that applies the current visual appearance via
  those semantic classes. Styling moves from inline utility classes to CSS
  rules targeting `.formgen-*` selectors.
- Use CSS variables from go-theme tokens (`--primary-color`, `--bg-primary`,
  etc.) in the `.formgen-*` rules where applicable.
- `ChromeClasses` continues to work as-is: non-empty string overrides replace
  the semantic class, with no interaction to manage.
- Drop ClassMode entirely. One code path, no compatibility switch.

### Implementation notes

- Use typed constants for the semantic class names (e.g.,
  `const ClassForm = "formgen-form"`). Templates should reference the constants
  rather than string literals.
- Update `DefaultFormClass` and equivalents to use semantic class names instead
  of Tailwind lists.
- The default CSS must produce output visually identical to the current
  Tailwind styling. This is the acceptance criterion.

### Tests

- Update goldens. Every golden that asserts class attributes will change.
  Run `UPDATE_GOLDENS=1` after the refactor and review diffs.
- No browser-level CSS regression tests required beyond golden comparisons.

## Open questions

- Is `ChromeClasses` best modeled as `*ChromeClasses` or a value with empty
  fields? (Pointer is convenient to signal "no overrides".)
- Should we support an "append" strategy (default + extra) in Phase 1, or keep
  replacement-only for simplicity?
- Which template key should go-theme use for root form override:
  `forms.form` vs `layout.form`?

## Success criteria

- Per-request override can remove the "box in a box" in go-admin preview.
- Theme authors can swap the root template when needed.
- Long-term, semantic classes and CSS tokens become the primary styling tool.
