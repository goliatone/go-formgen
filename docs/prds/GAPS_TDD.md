# GAPS_TDD — Remaining Code Gaps (Post-Docs Audit)

This document captures the remaining gaps that require **code changes** (not just documentation edits) identified while validating the project’s guides (for example `docs/GUIDE_CUSTOMIZATION.md`, `docs/GUIDE_STYLING.md`) against the current repo implementation.

The intent is to let a new session pick up with clear, testable targets.

## Background

The UI schema system (`pkg/uischema`) reliably decorates the form model with layout hints, actions, section metadata, per-field overrides, and behavior configuration. Renderers then decide how much of that metadata is actually honored.

During the audit, several “looks like it should work” knobs were found to be:
- accepted and stored on the model but **not used by default templates/runtime**, or
- documented as “built-in” but actually **requires custom runtime code**, or
- implemented in one renderer but not the others.

## Scope

### In scope
- Implement missing behavior so documented knobs work end-to-end (or explicitly choose to keep them opt-in and adjust docs accordingly).
- Add/adjust tests and snapshots to lock behavior in.
- Keep changes offline-friendly (no mandatory network fetches at runtime or build time).

### Out of scope
- Large UI redesigns.
- Adding large icon libraries as embedded assets unless there’s a clear size/maintenance plan.
- Full expression language or external evaluator dependencies for visibility rules.

## Gap Report Format (Multi-Guide)

Gaps can be sourced from **multiple guides**. New gaps should include a short **Sources** block near the top so it’s obvious which guide(s) the work is intended to make true.

Recommended metadata for each gap:
- `Status: Open|Closed`
- `Sources:` list of guide references (and optionally the relevant code pointers)
- When closed: `Closed on: YYYY-MM-DD`, plus `PR checklist:` + `Changelog:`

## Gaps To Close

### Gap 1 — `layout.gutter` is stored but not applied (vanilla renderer)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/renderers/vanilla/templates/form.tmpl`, `pkg/renderers/vanilla/renderer_contract_test.go`, `pkg/renderers/vanilla/testdata/form_output_gutter_sm.golden.html`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT go test ./pkg/renderers/vanilla -count=1`

Changelog:
- Vanilla renderer now maps `layout.gutter` (`sm`/`md`/`lg`) to Tailwind gap classes in the main grid containers.
- Added a golden contract test fixture for `layout.gutter: "sm"` asserting `gap-4`.

**Current state**
- UI schema decorator writes `form.UIHints["layout.gutter"]` (`pkg/uischema/decorator.go`).
- Vanilla template reads `layout.gutter` into `grid_gutter` but does not use it (`pkg/renderers/vanilla/templates/form.tmpl`).
- Styling docs currently imply `layout.gutter` affects spacing (`docs/GUIDE_STYLING.md`).

**Desired state**
- `layout.gutter` changes the grid gap in the vanilla renderer (at least for sectioned + unsectioned grids).

**Implementation sketch**
- Update `pkg/renderers/vanilla/templates/form.tmpl` to map `grid_gutter` to a class list:
  - `sm` → `gap-4`
  - `md` → `gap-6` (current default)
  - `lg` → `gap-8`
- Keep a safe fallback when the value is unknown.
- Consider applying gutter to both sectioned and unsectioned grids.

**Acceptance criteria**
- A UI schema with `form.layout.gutter: "sm"` produces `gap-4` in the relevant grid container(s).
- Existing templates still render when `layout.gutter` is missing.

**Test plan**
- Update/add a renderer contract test or golden HTML fixture that sets `layout.gutter` and asserts the expected class.

---

### Gap 2 — `autoResize` behavior is referenced but not implemented in the shipped runtime

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `client/src/behaviors/auto-resize.ts`, `client/src/behaviors/index.ts`, `client/tests/behaviors.test.ts`, `pkg/runtime/assets/formgen-behaviors.min.js`, `pkg/runtime/assets/formgen-behaviors.min.js.map`, `GAPS_TDD.md`
- Tests run: `npm test -- tests/behaviors.test.ts` (in `client/`)

Changelog:
- Added built-in `autoResize` behavior for textareas with optional `minRows`/`maxRows` bounds.
- Registered `autoResize` in the shipped behaviors runtime and committed updated bundles under `pkg/runtime/assets`.
- Added a DOM-based runtime test covering resize + clamping behavior.

**Current state**
- UI schema behaviors are serialized into:
  - `data-behavior="…"` and
  - `data-behavior-config="…"`
  via `pkg/uischema/decorator.go` + vanilla’s `buildDataAttributes` (`pkg/renderers/vanilla/renderer.go`).
- Shipped behaviors runtime (`pkg/runtime/assets/formgen-behaviors.min.js`) registers only `autoSlug`.
- Docs now explicitly mark `autoResize` as “not implemented yet” (`docs/GUIDE_CUSTOMIZATION.md`).

**Desired state**
- Add a built-in `autoResize` behavior to `formgen-behaviors.min.js` for textareas.

**Behavior contract (proposed)**
- Target: textarea or wrapper containing a textarea.
- Config:
  - `minRows?: number`
  - `maxRows?: number`
- Behavior:
  - On init and input, adjust height/rows to fit content within bounds.

**Implementation sketch**
- Implement `autoResize` in the JS runtime source under `client` (then rebuild/commit bundles under `pkg/runtime/assets`).
- Register it in the runtime registry alongside `autoSlug`.
- Keep it robust: handle missing/invalid config gracefully.

**Acceptance criteria**
- A textarea with `data-behavior="autoResize"` resizes as content grows/shrinks.
- `data-behavior-config` supports both a single-behavior config object and a multi-behavior map (must match current `selectBehaviorConfig` logic in the runtime).

**Test plan**
- Add runtime tests under `client` (preferred) and/or a minimal browserless DOM test harness.
- Keep Go-side tests unchanged unless markup changes.

---

### Gap 3 — `icon` + `iconSource` don’t render real icons without `iconRaw`

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `client/src/icons/registry.ts`, `client/src/icons/index.ts`, `client/src/behaviors/index.ts`, `client/tests/icons.test.ts`, `client/dist/browser/formgen-behaviors.min.js`, `client/dist/browser/formgen-behaviors.min.js.map`, `pkg/runtime/assets/formgen-behaviors.min.js`, `pkg/runtime/assets/formgen-behaviors.min.js.map`, `pkg/renderers/vanilla/assets/formgen-behaviors.min.js`, `GAPS_TDD.md`
- Tests run: `npm test -- tests/icons.test.ts` (in `client/`), `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test . ./pkg/renderers/vanilla -count=1`

Changelog:
- Added an icon provider registry + `initIcons()` runtime that replaces vanilla’s placeholder glyph with provider-supplied inline SVG when `data-icon`/`data-icon-source` are present.
- Exposed `registerIconProvider()` via the behaviors bundle and run `initIcons()` automatically during `initBehaviors()`.
- Synced the rebuilt behaviors bundle into `pkg/runtime/assets` and `pkg/renderers/vanilla/assets`.

**Current state**
- UI schema decorator copies:
  - `icon` → `field.UIHints["icon"]` and `field.Metadata["icon"]`
  - `iconSource` → `field.UIHints["iconSource"]` and `field.Metadata["icon.source"]`
  - `iconRaw` → inline sanitized SVG in `field.UIHints["iconRaw"]` and `field.Metadata["icon.raw"]`
  (`pkg/uischema/decorator.go`).
- Vanilla templates render a placeholder glyph unless `iconRaw` exists (`pkg/renderers/vanilla/templates/components/input.tmpl`, `textarea.tmpl`).
- The metadata is already emitted as data attributes (`data-icon`, `data-icon-source`, `data-icon-raw`) via `buildDataAttributes` (`pkg/renderers/vanilla/renderer.go`).

**Desired state (choose one)**
- **Option A (recommended): runtime extensibility**
  - Provide a small runtime hook that lets apps register icon providers:
    - `registerIconProvider(source, (name) => svgString | null)`
  - On init, resolve `data-icon`/`data-icon-source` into inline SVG in the DOM.
  - Ship with no heavy built-in icon sets, but make it easy to plug in.
- **Option B: Go-side resolution**
  - Embed a curated icon subset and map `{iconSource, icon}` → SVG at render time.

**Acceptance criteria**
- With an icon provider registered, fields with `icon`+`iconSource` render a real SVG without requiring `iconRaw`.
- Without a provider, behavior remains unchanged (placeholder glyph is fine).

**Test plan**
- Runtime unit tests for provider registration and DOM replacement behavior.
- Optional: golden HTML update only if markup changes (prefer runtime-only DOM enhancement so goldens stay stable).

---

### Gap 4 — Widget semantics are inconsistent across renderers (vanilla vs Preact)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/renderers/vanilla/renderer.go`, `pkg/renderers/vanilla/widget_contract_test.go`, `pkg/renderers/preact/assets/formgen-preact.min.js`, `client/tests/preact-widgets.test.ts`, `GAPS_TDD.md`
- Tests run: `npm test -- tests/preact-widgets.test.ts` (in `client/`), `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test ./pkg/renderers/vanilla ./pkg/renderers/preact -count=1`

Changelog:
- Vanilla renderer now maps common `widget` hints (including metadata-only hints) to concrete components (`textarea`, `select`, `boolean`, `json_editor`) and aliases some widget values to built-in vanilla components (`wysiwyg`, `file_uploader`, `datetime-range`).
- Preact runtime now renders `json-editor` and `code-editor` widgets as `<textarea>` controls and defaults relationship fields to `<select>` controls.
- Added tests covering vanilla widget→component mapping and Preact runtime widget rendering.

**Current state**
- Widget names exist and are used by the widget registry (`pkg/widgets/registry.go`).
- Vanilla renderer only special-cases a small subset of widget hints (notably `textarea`, `json-editor`) and otherwise resolves components based on type/enum/relationship (`pkg/renderers/vanilla/renderer.go`).
- Preact renderer preserves widget hints and renders client-side UI based on them (see `pkg/renderers/preact` + its runtime bundles).
- TODOs exist for renderer parity (e.g., JSON editor in Preact, chips/typeahead in Preact) (`TODO.md`).

**Desired state**
- Define a small, explicit “widget contract” for the project:
  - which widget names are supported,
  - which renderer(s) support them,
  - what runtime is required (if any),
  - and a mapping strategy for vanilla (widget → component/template).

**Implementation sketch**
- Vanilla: extend component resolution so common widget names map to concrete components:
  - `toggle` → a switch/toggle component (may require runtime; decide)
  - `code-editor` → code editor component (runtime)
  - `key-value` → key/value editor component (runtime)
  - `chips`/`select` → prefer existing select/relationship patterns where applicable
- Preact: close the TODO parity items (at least `json-editor`).

**Acceptance criteria**
- For each “supported widget”, vanilla and preact produce a sensible UI without the caller having to also set `component`.
- Unsupported widget values are preserved but do not break rendering.

**Test plan**
- Add/extend renderer contract tests for widget hints.
- Update goldens where renderer output changes.

---

### Gap 5 — Visibility rules have no built-in evaluator (only an interface)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/visibility/expr/evaluator.go`, `pkg/visibility/expr/evaluator_test.go`, `pkg/orchestrator/visibility_decorator_test.go`, `pkg/orchestrator/testdata/create_widget_vanilla.golden.html`, `pkg/orchestrator/testdata/create_widget_vanilla_partial_taxonomy.golden.html`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test ./pkg/visibility/... ./pkg/orchestrator -count=1`

Changelog:
- Added a dependency-free visibility evaluator (`pkg/visibility/expr`) supporting `==`/`!=`, truthy checks, `!`, and basic `&&`/`||` composition against `RenderOptions.Values`/`VisibilityContext`.
- Added unit tests for the evaluator and an orchestrator integration test exercising real `visibilityRule` filtering using the built-in evaluator.

**Current state**
- Visibility is applied only when an evaluator is provided (`pkg/orchestrator/visibility_decorator.go`).
- `visibilityRule` is treated as an opaque string sourced from metadata/UI hints.

**Desired state (optional)**
- Provide a small built-in evaluator implementation for common cases, while keeping the evaluator pluggable:
  - simple comparisons (`==`, `!=`),
  - boolean checks,
  - maybe `&&`/`||` with parentheses (optional).

**Implementation sketch**
- Add `pkg/visibility/expr` (or similar) implementing `visibility.Evaluator`.
- Evaluate against `visibility.Context.Values` (fed from `RenderOptions.Values`).
- Keep it dependency-free and deterministic.

**Acceptance criteria**
- A rule like `enabled == true` can be evaluated without a custom evaluator.
- Existing behavior (no evaluator → no filtering) remains unchanged.

**Test plan**
- Unit tests for the built-in evaluator.
- Integration tests proving orchestrator filtering works when using the built-in evaluator.

---

### Gap 6 — Runtime auto-injection is incomplete for runtime-backed components (WYSIWYG)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/renderers/vanilla/components/defaults.go`, `pkg/renderers/vanilla/renderer_contract_test.go`, `pkg/renderers/vanilla/testdata/form_output_wysiwyg_only.golden.html`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test ./pkg/renderers/vanilla -count=1`, `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test ./pkg/renderers/vanilla/components -count=1`

Changelog:
- Vanilla `wysiwyg` component now auto-injects `/runtime/formgen-relationships.min.js` plus the init snippet (deduped with other runtime-backed components).
- Added a golden contract test proving scripts are emitted when a form uses only `component: "wysiwyg"`.

**Current state**
- `file_uploader` auto-injects `/runtime/formgen-relationships.min.js` and an init call via its component descriptor (`pkg/renderers/vanilla/components/defaults.go`).
- `wysiwyg` relies on the same runtime bundle but does not auto-inject scripts.

**Desired state**
- Using `component: "wysiwyg"` in vanilla should also result in the runtime script/init being emitted (deduped with other components).

**Implementation sketch**
- Add the same script deps to the `wysiwyg` component descriptor in `pkg/renderers/vanilla/components/defaults.go`.
- Keep deduping behavior (registry already dedupes by src/inline signature).

**Acceptance criteria**
- Vanilla output includes runtime script/init when a form uses only `wysiwyg`.
- Script is still included only once when both `file_uploader` and `wysiwyg` are present.

**Test plan**
- Add/update a vanilla golden that includes a wysiwyg field and assert scripts are present once.

## Next Steps / Suggested Order

1. **`layout.gutter`** (small, Go-only, easy to validate via goldens).
2. **WYSIWYG runtime auto-injection** (small Go change + goldens).
3. **`autoResize` runtime behavior** (JS runtime + committed bundles + tests).
4. **Icon provider hook** (runtime-only; keeps Go templates stable).
5. **Widget contract + renderer parity** (bigger scope; tie into `TODO.md` items).
6. **Optional built-in visibility evaluator** (new package + tests; keep pluggable).

## References

- Docs
  - `docs/GUIDE_CUSTOMIZATION.md`
  - `docs/GUIDE_STYLING.md`
- UI schema + rendering
  - `pkg/uischema/decorator.go`
  - `pkg/uischema/types.go`
  - `pkg/renderers/vanilla/templates/form.tmpl`
  - `pkg/renderers/vanilla/renderer.go`
  - `pkg/renderers/vanilla/components/defaults.go`
- Runtime bundles
  - `pkg/runtime/assets/formgen-behaviors.min.js`
  - `pkg/runtime/assets/formgen-relationships.min.js`
  - `runtime_assets.go`
- TODO tracking
  - `TODO.md`

---

### Gap 7 — `autoResize` docs and vanilla embedded bundle are stale

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `docs/GUIDE_CUSTOMIZATION.md`, `runtime_assets_test.go`, `pkg/renderers/vanilla/assets_test.go`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test . ./pkg/renderers/vanilla -count=1`

Changelog:
- Docs now list `autoResize` as a shipped built-in behavior and document its `minRows`/`maxRows` config.
- Added Go-side checks that both `formgen.RuntimeAssetsFS()` and `vanilla.AssetsFS()` expose a behaviors bundle containing `autoResize`.

**Current state**
- Docs previously said `autoResize` was “not implemented yet”.
- The behaviors bundle is duplicated between `pkg/runtime/assets` and `pkg/renderers/vanilla/assets`, so it can drift without checks.

**Desired state**
- Docs reflect that `autoResize` is available in the shipped runtime.
- Vanilla embedded assets are either kept in sync with `pkg/runtime/assets` or the duplicate bundle is removed to avoid drift.

**Acceptance criteria**
- `autoResize` documentation no longer claims it’s unimplemented.
- Serving `vanilla.AssetsFS()` provides a behaviors runtime that includes `autoResize` (or the project documents that behaviors must be served from `RuntimeAssetsFS()` instead).

---

### Gap 8 — Document the icon provider hook (`registerIconProvider`)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `docs/GUIDE_CUSTOMIZATION.md`, `GAPS_TDD.md`
- Tests run: `npm test -- tests/icons.test.ts` (in `client/`)

Changelog:
- Documented `FormgenBehaviors.registerIconProvider()` + `FormgenBehaviors.initBehaviors()` usage for resolving `icon`/`iconSource` into inline SVG at runtime.

**Current state**
- Docs state `icon`/`iconSource` are emitted but not resolved by default, and imply `iconRaw` is the only built-in way to get real icons without custom runtime/templates.

**Desired state**
- Docs include a minimal example showing how to register an icon provider and initialize the behaviors runtime so `icon`/`iconSource` can resolve to inline SVG.

**Acceptance criteria**
- `docs/GUIDE_CUSTOMIZATION.md` (and/or README) includes a short example using `FormgenBehaviors.registerIconProvider(...)` and `FormgenBehaviors.initBehaviors()`.

---

### Gap 9 — Preact “advanced widgets” UI parity (chips/typeahead/toggle)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/renderers/preact/assets/formgen-preact.min.js`, `client/tests/preact-widgets.test.ts`, `GAPS_TDD.md`
- Tests run: `npm test -- tests/preact-widgets.test.ts` (in `client/`), `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test ./pkg/renderers/preact -count=1`

Changelog:
- Preact runtime now supports `widget: "chips"` and `widget: "typeahead"` by emitting `data-endpoint-renderer` hints so `FormgenRelationships.initRelationships()` upgrades relationship `<select>` controls to the richer UIs.
- Preact runtime now supports `widget: "toggle"` by tagging checkboxes for switch enhancement and auto-initializing switches via `FormgenRelationships.renderSwitch()` when available.

**Current state**
- The Preact runtime falls back to basic HTML controls for several widget values:
  - `chips` → `<select multiple>` (no chip UI)
  - `toggle` → `<input type="checkbox">` (no switch UI)
  - `select` → `<select>` (works, but no richer UX)
- The relationships runtime already contains chips/typeahead renderers for vanilla selects, but the Preact renderer’s UI could be improved to match the documented widget names.

**Desired state**
- Preact provides richer UI for these widget hints, aligned with the project’s widget registry names.

**Acceptance criteria**
- `widget: "chips"` renders a chip selector UI (has-many/multi) in Preact.
- `widget: "toggle"` renders a switch/toggle UI in Preact.
- Relationship-backed `has-one` can use a typeahead UI in Preact when configured (parity with vanilla runtime behavior).

---

### Gap 10 — Expose a theme asset resolver helper to templates (`assetURL(...)`)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/render/template/gotemplate/adapter.go`, `pkg/renderers/vanilla/renderer.go`, `pkg/renderers/preact/preact.go`, `pkg/renderers/vanilla/renderer_contract_test.go`, `pkg/renderers/vanilla/testdata/form_output_with_styles.golden.html`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT GOCACHE="$PWD/.cache/go-build" go test ./pkg/render/template/... ./pkg/renderers/vanilla ./pkg/renderers/preact -count=1`

Changelog:
- Templates now receive `theme.assetURL(key)` (vanilla + preact), backed by `RenderOptions.Theme.AssetURL`.
- Updated the default template engine adapter to preserve callable values in render context, enabling per-render helpers like `theme.assetURL`.
- Added vanilla contract tests for the helper and refreshed the `WithDefaultStyles` golden.

Sources:
- `docs/GUIDE_STYLING.md:397` (notes the helper is not exposed)

**Current state**
- The vanilla renderer resolves stylesheet/script paths using `Theme.AssetURL` on the Go side, but templates have no helper to resolve theme assets inside markup (e.g. `<img src="...">`).
- `templates/form.tmpl` gets a `theme` object (tokens/css vars/partials/json), but it does not include an asset resolver function.

**Desired state**
- Custom templates can resolve theme assets inside template markup, e.g.:
  - `{{ theme.assetURL("logo") }}`.
- Behavior is safe when no theme is configured (returns empty string or original key).

**Implementation sketch**
- Vanilla renderer:
  - Add an asset resolver function to the theme context passed into templates (e.g. `theme.assetURL(key) -> string`), backed by `renderOptions.Theme.AssetURL`.
  - Keep it compatible with custom template renderers: use data/context rather than relying on engine-specific function registration.
- Optional parity:
  - Expose the same helper in the preact page template context (`pkg/renderers/preact/templates/page.tmpl`) for consistency.

**Acceptance criteria**
- A template can emit `<img src="{{ theme.assetURL("logo") }}">` and it resolves to the configured theme asset URL.
- When theme is unset, rendering does not error and the helper degrades gracefully.

**Test plan**
- Add a renderer contract test that:
  - supplies a theme with assets + `AssetURL` resolver,
  - uses a custom `templates/form.tmpl` fixture referencing `theme.assetURL`,
  - asserts the resolved URL appears in the output.

---

### Gap 11 — First-class responsive grid (breakpoint spans/columns) in UI schema

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/uischema/types.go`, `pkg/uischema/decorator.go`, `pkg/uischema/decorator_test.go`, `pkg/uischema/testdata/responsive_grid/schema.json`, `pkg/renderers/vanilla/renderer.go`, `pkg/renderers/vanilla/templates/form.tmpl`, `pkg/renderers/vanilla/renderer_contract_test.go`, `pkg/renderers/vanilla/testdata/form_output_responsive_grid.golden.html`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT GOCACHE="$PWD/.cache/go-build-uischema" go test ./pkg/uischema -count=1`, `env -u GOROOT GOCACHE="$PWD/.cache/go-build-vanilla" go test ./pkg/renderers/vanilla -count=1`

Changelog:
- Added `grid.breakpoints` to UI schema field grid config, emitting `layout.span.<breakpoint>` / `layout.start.<breakpoint>` / `layout.row.<breakpoint>` UI hints for `sm`/`md`/`lg`/`xl`/`2xl`.
- Vanilla renderer now emits responsive grid CSS vars + `fg-grid-responsive` wrapper class when breakpoint hints are present, and injects a small `<style data-formgen-responsive-grid>` block so responsive spans work without custom CSS/templates.
- Added a UI schema decorator unit test fixture and a vanilla renderer golden contract test covering a `layout.span.lg` override.

Sources:
- `docs/GUIDE_STYLING.md:622` (no first-class breakpoint grid support)
- `docs/GUIDE_STYLING.md:681` (breakpoint-scoped UI schema overrides not implemented)

**Current state**
- UI schema supports only numeric grid hints (`grid.span/start/row`) and global form layout (`layout.gridColumns/gutter`).
- Vanilla renderer outputs inline per-field `grid-column` styles, which can’t express breakpoint-specific spans without custom templates/CSS.

**Desired state**
- UI schema can express breakpoint-specific layout, e.g. “span 12 on mobile, span 6 on `lg`”.
- Vanilla renderer can honor this in a stable way without requiring ad-hoc template rewrites.

**Implementation sketch (choose one)**
- **Option A: CSS variable + class strategy (no per-field <style> tags)**
  - Extend UI schema types to allow breakpoint values (e.g. `grid.breakpoints.lg.span`).
  - Renderer emits data attributes and/or inline CSS vars (e.g. `--fg-span: 12; --fg-span-lg: 6`) and a class on each field wrapper.
  - Default stylesheet (or a small embedded `<style>`) defines breakpoint rules mapping vars to `grid-column`.
- **Option B: Template-driven**
  - Extend render context so templates can compute class names and emit breakpoint classes/styles themselves.
  - Document vanilla default as “non-responsive” unless you override `templates/form.tmpl`.

**Acceptance criteria**
- A UI schema can set a breakpoint-specific span and vanilla output reflects it at the right breakpoint with no custom template.
- Existing non-responsive layouts remain unchanged when no breakpoint config is present.

**Test plan**
- Unit tests for schema parsing/normalization of breakpoint configs.
- Golden HTML contract test demonstrating a breakpoint span emits the expected attributes/classes/CSS (depending on chosen strategy).

---

### Gap 12 — Component templates don’t receive full render context (notably `theme`)

Status: Closed
Closed on: 2025-12-17

PR checklist:
- Files changed: `pkg/renderers/vanilla/components/registry.go`, `pkg/renderers/vanilla/components/defaults.go`, `pkg/renderers/vanilla/components/json_editor.go`, `pkg/renderers/vanilla/field_renderer.go`, `pkg/renderers/vanilla/component_renderer_test.go`, `GAPS_TDD.md`
- Tests run: `env -u GOROOT GOCACHE="$PWD/.cache/go-build-vanilla" go test ./pkg/renderers/vanilla -count=1`, `env -u GOROOT GOCACHE="$PWD/.cache/go-build-vanilla-components" go test ./pkg/renderers/vanilla/components -count=1`

Changelog:
- Component templates now receive `theme` (tokens/css vars/partials + `assetURL`) in the render payload, enabling token-driven component overrides without custom Go renderers.
- Added a vanilla component renderer test asserting `theme.tokens` is accessible inside a component template.

Sources:
- `docs/GUIDE_STYLING.md:858` (component templates only receive `field` + `config`)

**Current state**
- Vanilla component templates (e.g. `templates/components/input.tmpl`) are rendered with `{field, config}` only.
- They cannot reference theme tokens/CSS vars/partials (beyond whatever is already baked into classes), which blocks token-driven component markup without writing custom component renderers.

**Desired state**
- Component templates can access the same high-level context that the form wrapper gets where it makes sense (at minimum: `theme`, and optionally `render_options.locale` / helpers).

**Implementation sketch**
- Plumb theme context into component rendering:
  - Extend `pkg/renderers/vanilla/components.ComponentData` to carry the full theme context (not only `ThemePartials`).
  - Update the default template renderer payload to include `theme` (and possibly `render_options`).
- Keep chrome partials compatible:
  - Optionally pass `theme` into chrome templates too.

**Acceptance criteria**
- A theme token can be referenced from a component override template (e.g. `{{ theme.tokens.brand }}`) without custom Go renderers.
- Existing templates continue to render unchanged when theme is nil.

**Test plan**
- Extend `pkg/renderers/vanilla/component_renderer_test.go` to render a component template that references `theme.tokens` and assert the rendered output includes the expected token.


---
## Prompt

Use the following prompt to work in the gaps:

```
Read GAPS_TDD.md end-to-end. Find the first gap section whose header starts with ### Gap and is not marked Status: Closed. Implement that gap end-to-end (code + tests/goldens as described), keeping changes tightly scoped. If the gap depends on runtime bundles under pkg/runtime/assets, update them in-repo as needed. Run the most relevant local tests for the touched areas.

When finished:

Update that gap section in GAPS_TDD.md by adding/updating:
Status: Closed
Closed on: YYYY-MM-DD
PR checklist: with bullets for files changed + tests run
Add a short Changelog: bullet list under the gap describing what changed.
If you discover follow-up work, add it as a new ### Gap N — ... section at the end (leave it Status: Open).
Then stop.
```
