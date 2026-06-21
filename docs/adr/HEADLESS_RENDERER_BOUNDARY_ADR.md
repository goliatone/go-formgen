# ADR: Headless Renderer Dependency Boundary

## Status

Proposed

## Context

go-formgen supports two different use cases:

1. A headless form compiler:
   `schema -> FormModel -> caller-owned renderer`
2. A full form system:
   `schema -> FormModel -> go-formgen renderer -> parse/validate/re-render`

The first ctx integration only needs the compiler path:

```text
workflow manifest -> JSON Schema -> FormModel -> ctx dashboard renderer
```

In that path, ctx does not need go-formgen's HTML renderer, template engine, theme system, sanitizer, browser assets, or TUI renderer. Importing the headless compiler should not pull dependencies such as:

- `pkg/renderers/vanilla`
- `pkg/renderers/preact`
- `pkg/renderers/tui`
- `github.com/goliatone/go-theme`
- `github.com/goliatone/go-template`
- `github.com/flosch/pongo2/v6`
- `github.com/microcosm-cc/bluemonday`
- `github.com/AlecAivazis/survey/v2`

The goal is about dependency boundary, not purity. go-formgen should still support full rendering, themes, templates, sanitization, and TUI flows. Those dependencies should be explicit opt-ins for consumers that render with those packages.

## Decision

Make the headless model-building API renderer-free at the import/dependency level.

The headless compiler path must not import concrete renderers, theme packages, template packages, sanitizers, browser asset packages, or TUI packages.

The conceptual package boundary is:

```text
pkg/orchestrator
  BuildFormModel and schema/model pipeline only
  no concrete renderer imports
  no go-theme imports
  no render.RenderOptions requirement for headless builds

pkg/render
  renderer-facing options and interfaces
  may keep theme integration if needed by renderers

pkg/orchestrator/defaults or root compatibility helpers
  optional renderer registration helpers
  may import vanilla/preact/tui/theme packages

pkg/renderers/vanilla, pkg/renderers/preact, pkg/renderers/tui
  concrete renderer implementations and dependencies
```

`BuildFormModel` should accept a dependency-free build request/options shape. If the existing `orchestrator.Request` continues to carry renderer-only fields such as `render.RenderOptions`, add a separate `BuildRequest` for headless use.

`Generate` may remain a convenience rendering API, but it should be layered on top of `BuildFormModel`:

```text
BuildFormModel(ctx, buildReq)
  -> resolve render options/theme
  -> lookup renderer
  -> render
```

Full rendering can still support `go-theme`; theme-specific config belongs in renderer-enabled APIs or compatibility helpers, not in the headless build path.

## Rationale

- **Smaller API contract:** headless consumers depend on form model generation, not the full web rendering stack.
- **Lower transitive risk:** template engines, sanitizers, browser assets, and theme systems carry CVE/update surface that headless users should not inherit.
- **Cleaner server builds:** server-only compiler use should not grow unrelated renderer/theme/template imports.
- **Better package design:** CLIs, servers, tests, and third-party renderers can build `FormModel` without dragging default HTML rendering.
- **Explicit future adoption:** consumers that later choose vanilla, Preact, TUI, or theme support opt into those dependencies by importing/registering them.

## Consequences

- `pkg/orchestrator` can no longer construct the vanilla renderer directly.
- `pkg/orchestrator` should not import `go-theme` directly.
- `BuildFormModel` should not require `render.RenderOptions`.
- Existing convenience APIs may need compatibility wrappers that import renderer defaults outside the core orchestrator package.
- Integration tests that currently rely on implicit default renderers should explicitly register the renderer they exercise.

## Acceptance Check

Headless import guard:

```bash
go list -deps -f '{{.ImportPath}}' ./pkg/orchestrator | rg 'survey|pongo2|go-theme|bluemonday|renderers/vanilla|renderers/tui|renderers/preact|go-template'
```

This command should print nothing.

Rendering packages may still pull those dependencies when explicitly imported.

## Follow-up Tasks

- Add public `BuildFormModel`.
- Add `BuildRequest` or otherwise make the build API dependency-free.
- Move default renderer registration out of `pkg/orchestrator`.
- Move theme resolution out of the headless build path.
- Add an import-graph guard test or script.
- Update examples/docs to show both headless and full-renderer usage.
