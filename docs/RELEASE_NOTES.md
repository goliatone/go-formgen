# Release Notes & Migration Guide (Phase 10)

## API Embedding and Renderer Improvements

- Added a `json` descriptor renderer with a versioned envelope separating form,
  values, errors, form errors, hidden fields, and metadata.
- Added `RenderModeDocument`, `RenderModeForm`, and `RenderModeFields` for
  embeddable vanilla and Preact output.
- Added schema-owned nested field ordering: sibling properties that declare
  `x-formgen.order` or `x-admin.order` are built in that order at top level,
  inside nested objects, and inside array item objects. A single ordered field is
  honored; unordered siblings follow in deterministic property-name order.
- Added browser controller hooks at `window.Formgen.attach(root)` for value
  collection, value/error hydration, subscriptions, focus, and teardown.
- Added vanilla `StyleModeDefault`, `StyleModeMinimal`, and `StyleModeUnstyled`
  for host-owned styling.
- Added `Field.Sensitive` detection and default redaction for descriptor and
  browser renderer output.

This phase rounds out the admin/settings expansion: widgets and theming are now adapter-friendly, render options cover provenance + disabled/readonly flags, and docs/examples describe how to wire these features end-to-end.

## Highlights
- Headless API: `pkg/orchestrator` now exposes `BuildFormModel`, raw JSON Schema byte helpers, and in-memory document helpers without importing concrete renderers or theme packages.
- Render options: prefill values can carry provenance plus `Readonly`/`Disabled` flags; subsets render groups/tags/sections; go-errors payloads map cleanly into renderer chrome; hidden fields cover CSRF/auth/version helpers.
- Adapter hooks: widget registry and visibility evaluator setters on the orchestrator allow go-settings/go-media/go-export adapters to register widgets/evaluators at runtime.
- Theming: renderer-facing defaults helpers accept go-theme selectors + partial fallbacks; component overrides remain available via UI schema or metadata.
- JSON/object editor: richer widget with pretty-print, collapse/expand, and schema hints; goldens capture provenance/disabled flows.

## Migration Notes
- Renderer defaults: `orchestrator.New()` no longer registers vanilla implicitly. Register renderers explicitly, or use `formgen.NewOrchestrator`, `formgen.GenerateHTML`, or `pkg/orchestrator/defaults.New` for compatibility convenience.
- Prefill payloads: wrap values in `render.ValueWithProvenance` when you need badges or lock inherited defaults; set `Disabled`/`Readonly` on that struct instead of mutating UI schema.
- Widget overrides: register custom widgets via `(*orchestrator.Orchestrator).RegisterWidget` or pass a registry into `WithWidgetRegistry`. Avoid mutating the default registry directly.
- Visibility evaluators: inject with `WithVisibilityEvaluator` at construction time or `SetVisibilityEvaluator` afterwards, and pass context through `RenderOptions.VisibilityContext`.
- Nested field ordering: downstream schema authors can keep order hints on the
  schema or JSON Schema overlay, including dotted overlay keys such as
  `x-formgen.order` and `x-admin.order`. Renderer-specific HTML reordering
  should no longer be needed after updating to this release.
- Repeatable arrays: generated vanilla rows now distinguish persisted rows from
  newly cloned rows with `data-formgen-array-existing`. Existing rows with a
  hidden `_delete` sentinel are soft-deleted by the runtime so delete intent is
  submitted; unsaved cloned rows are removed from the DOM. Custom renderers that
  use `data-formgen-array-action="remove"` and `_delete` sentinels must emit
  `data-formgen-array-item="true"` plus `data-formgen-array-existing="true"`
  for loaded rows and `"false"` for prototype/new rows.
- Templates/themes: when supplying custom templates or go-theme providers, ensure fallback partials map the new component keys (`forms.json-editor`, `forms.file-uploader`).
- Snapshots: refresh vanilla/Preact goldens after updating templates/components using `./taskfile dev:goldens` (requires network access for module downloads).

## Testing Expectations
- `go test ./...` remains the baseline. Renderer suites consume HTML/JSON goldens; set `UPDATE_GOLDENS=1` to regenerate after template changes.
- Example builds (`./taskfile dev:test:examples`) validate CLI/HTTP binaries with pinned toolchain and both renderers in the matrix.

## Known Defaults
- Visibility rules are no-op unless an evaluator is configured.
- Relationship runtime stays offline until an HTTP client is provided.
- Method overrides use `_method` hidden inputs for non-GET/POST verbs in vanilla forms.
