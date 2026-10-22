# Release Notes & Migration Guide (Phase 10)

## Vanilla Grid Layout Without Inline Styles (next release)

The vanilla renderer no longer writes inline `style` attributes or an inline `<style>` block for UI schema grid layout. Layout intent is emitted as data attributes, and the bundled stylesheet (`formgen-vanilla.css` and `formgen-preact.min.css`, generated from `client/src/theme/grid-layout.css`) places the elements, so forms render identically under a `Content-Security-Policy` of `style-src 'self'`.

- Grid container: `data-formgen-columns="<n>"` (from `layout.gridColumns`, default 12) and `data-formgen-gutter="sm|md|lg"` (from `layout.gutter`, default `md`) replace `style="--formgen-grid-gap: …; grid-template-columns: …"`.
- Field wrapper: `data-formgen-span="<n>"`, optional `data-formgen-start` and `data-formgen-row`, and `-sm`/`-md`/`-lg`/`-xl`/`-2xl` variants replace `style="grid-column: span n / span n; …"`, the `fg-grid-responsive` class, and the `<style data-formgen-responsive-grid>` block.
- Stylesheet rules: `[data-formgen-columns="n"]` sets `grid-template-columns: repeat(n, minmax(0, 1fr))`; `[data-formgen-gutter]` sets `--formgen-grid-gap`; `[data-formgen-span="n"]` sets `grid-column-end: span n`; `[data-formgen-start="n"]` sets `grid-column-start: n`; `[data-formgen-row="n"]` sets `grid-row-start: n`; the breakpoint variants repeat inside `min-width` media queries (640/768/1024/1280/1536 px). Values 1–24 are covered.

Migration:

- Hosts that serve or inline the bundled stylesheet (`WithDefaultStyles`, `WithStylesheet` with `vanilla.AssetsFS()`, or a copied `formgen.css`) only need the rebuilt asset.
- Hosts that style forms with their own CSS (`WithoutStyles`, `WithInlineStyles` replacing the bundle, or a custom theme) must add the rules above (copy `client/src/theme/grid-layout.css`); without them every field falls back to grid auto-placement.
- Custom `form.tmpl` overrides: emit `{{ field.attrs|safe }}` on each field wrapper (`field.style` resolves to the same attribute string) and add `data-formgen-columns`/`data-formgen-gutter` to the grid container (the template context exposes `layout.gutterValue`). The `responsive_grid_styles` template variable is gone; references render empty.
- CSS overrides of `.formgen-grid { grid-template-columns: … }` no longer need `!important`; the bundled selectors have specificity 0,1,0.
- Strict CSP: `examples/embedded --csp` now sends `style-src 'self'`. Theme CSS variables (`<style data-formgen-theme-vars>`), `WithDefaultStyles`, and `WithInlineStyles` still produce inline `<style>` elements; strict hosts serve the stylesheet instead or add a nonce or hash.

Behavior correction: breakpoint overrides now cascade upward like Tailwind's mobile-first breakpoints. Previously an `lg` span applied only between the `lg` and `xl` thresholds, and larger viewports fell back to the base span unless every larger breakpoint was set explicitly.

## Embedded Form Primitives: Compatibility and Migration (next release)

The embedded-form primitives (hydration options, model-aware values and validation, issue presentation, reactive conditions, native choice components, preflight, and typed snapshots) are additive. Definitions, templates, and host code written for v0.33.2 keep working without changes.

Preserved behavior:

- `setValues(values)` and `reset()` without options keep patch semantics and the legacy per-event `onChange` notifications; `getValues()` stays raw DOM strings; `hydrateFormValues`/`Formgen.hydrate` are unchanged.
- Default controls are unchanged: booleans render checkboxes, enums render selects, arrays of objects render repeaters. The choice components are reached only through `component.name`.
- Unregistered explicit components still fail at render time; `Preflight` is an additional, optional check.
- Definitions without `x-formgen.conditions` compile and validate exactly as before; the JSON descriptor gains a `conditions` key only when rules exist.
- Legacy `visibility` rules are untouched and remain separate from applicability.

Additive contracts: `HydrationOptions`, `ChangeMetadata` (third callback argument with `origin`, `paths`, and `applicable`), `EmbeddedModel`/`EmbeddedField` (optional `label`), `EmbeddedIssue`, `ConditionSet`/`Expression` (`formgen.conditions/v1`), `ConditionProjection`, `DefinitionDiagnostic`, `TypedSnapshot`/`PresenceState` (`formgen.snapshot/v1`), the model rule `uniqueItems`, and the shared fixture `testdata/embedded/v1.json`.

Corrections that change observable output (document in host release notes):

- Hydration no longer rewrites existing `<option>` labels with raw values, and hydrating a file input with a non-empty value no longer throws.
- Go `minLength`/`maxLength` count Unicode code points instead of UTF-8 bytes, and a missing required array reports `required` instead of `type`.
- Go `uniqueItems` compares array items by JSON structure and exact numeric value, so raw objects holding typed Go containers (`[]string` versus `[]any`, `map[string]string`) or `float64` versus `json.Number` decimals validate the same through `ParseMap` as the identical JSON through `ParseJSON`; large integers never round through float64. Equivalent numeric lexemes also compare correctly with exponents beyond int32/int64; comparison adjusts only the exponent and never expands the numeric magnitude.
- `ParseMap` unwraps any typed slice or array (and pointers to them) supplied for an array field, as JSON decoding would; previously only `[]any` and `[]string` were unwrapped and other containers became one stringified item. Nil containers parse as null.
- The reactive settle does not set native `required` or `aria-required` on checkbox-group members, and it toggles only native `required` on radio members (never `aria-required`, which ARIA supports on the group, not on `radio`). Radio groups (`choice`, `scale`) render `role="radiogroup"` and carry `aria-required`/`aria-invalid`; checkbox groups carry `data-formgen-required`/`data-validation-state` plus a visually hidden legend hint (`component.config.requiredHint`, default "(required)") because those ARIA states are not valid on a plain fieldset.
- Root `x-formgen.conditions` compile strictly: unknown paths, cycles, and unsupported operators fail model building (`conditionCycle`, `conditionPath`, ...). Definitions that never declared conditions are unaffected.

Scope boundaries: fixed indexed condition paths only (no wildcards or same-row templates); pattern parity is limited to the portable subset (Go RE2 versus JavaScript RegExp); browser integers must be safe JavaScript integers; `uniqueItems` is a model rule and the JSON Schema keyword is not adopted; readonly choice groups render disabled native controls; Preact and TUI get no new controller guarantee; evidence comes from jsdom and Go tests plus a Chromium smoke of the example, not from WKWebView, VoiceOver, or application acceptance.

Release checklist for the maintainer (not performed here): bump the runtime package with `./taskfile client:version:bump patch`, rebuild with `./taskfile client:build`, bump the module with `./taskfile version:bump minor --write`, update `CHANGELOG.md` from the merged commits, and run `./taskfile dev:ci` before `./taskfile release minor`.

## Embedded Hydration Options (next release)

- `FormController.setValues(values, options)` and `reset(options)` accept the embedded/v1 `HydrationOptions` (`mode: "patch" | "replace"`, `notify`). Calls without options keep the legacy patch semantics and per-event notifications, including the pinned six-callback radio fixture.
- With options the mutation settles once, emits no callback when no control changed, and `notify: false` suppresses only that controller's subscribers. Widget, mirror, dependency, and component synchronization still runs.
- `onChange` callbacks receive a third `meta` argument: `origin` (`ui`, `program`, `hydrate`, `reset`) and canonical changed `paths`. Settled notifications pass an undispatched `formgen:controller:change` event.
- Replace mode clears omitted enabled controls, treats `null` as an explicit clear, replaces arrays, hydrates rendered rows from arrays of objects, and throws before mutating when a payload row has no rendered row.
- Defect corrections: hydration no longer rewrites existing `<option>` labels with raw values, and file inputs only accept programmatic clearing instead of throwing. `reset()` and `setValues()` events now carry reset/hydrate origins.

## Model-Aware Browser Values (next release)

- New pure helpers `normalizeValues(model, raw)` and `validateValues(model, values, scope)` mirror the Go `submission` package for the shared subset: typed enum decoding, field-aware coercion, canonical scoped validation, precise paths with legacy `rendererPath`, and `invalidPath`/`unknownField` scope diagnostics.
- `Formgen.attach(root, { model })` adds `controller.normalize()` and `controller.validate(scope)`; `getValues()` remains raw and no fetch, submission, or navigation is started. `Formgen.normalize`/`Formgen.validate` expose the helpers on the global.
- Browser integers must be safe JavaScript integers; larger values report a `type` issue instead of rounding. Unicode lengths count code points. Pattern parity covers the portable subset only. Condition evaluation is not included.

## Typed Snapshots With Explicit Presence (next release)

- `controller.snapshot()` and `controller.restore(snapshot, options)` add an opt-in model-backed presence layer: `present`/`null`/`absent` per leaf path, effective values with retained inactive values apart, locked-control values, independent copies, and explicit issues for unsafe integers, unknown controls, detached rows, and unknown or unsafe snapshot paths.
- `TypedSnapshot` and `PresenceState` join the embedded contract; raw `getValues()` and legacy hydration are unchanged.

## Renderer Preflight and Metadata Precedence (next release)

- `vanilla.Renderer.Preflight(form)` reports `componentUnknown`, `componentConfig`, and `componentRender` diagnostics with dotted field paths (`rows[].answer`) after component registration, without rendering a page or leaking values; `Render` keeps failing for unregistered explicit components.
- Browser `preflightComponents(root)` and `registeredComponentNames()` separate runtime enhancement factories from server-only wrappers and surface malformed `data-component-config`.
- The customization guide documents inline metadata, JSON Schema overlays, UI schema precedence, title/label rules, registration timing, and adapter limits.

## Native Choice Components (next release)

- Optional vanilla components `choice`, `choice-multiple`, and `scale` render native radio and checkbox groups with fieldset/legend chrome, group descriptions and error slots, typed enum option values, unanswered-by-default behavior, `aria-required`/`aria-readonly`/`aria-disabled` states, and `control.id`-based IDs. They are selected only through `component.name` and do not replace the default boolean, select, or repeater controls.
- `scale` derives discrete numeric options from inclusive `min`/`max` rules (`step`, `lowLabel`, `highLabel` via `component.config`); zero is valid.
- Theme CSS adds `formgen-choice*` hooks with a `--formgen-choice-target` minimum size; the runtime routes group issues to the fieldset and no longer sets native `required` on checkbox-group members.

## Reactive Browser Conditions (next release)

- `compileConditions`, `validateConditional`, and `evaluateExpression` mirror the Go `conditions` package for `formgen.conditions/v1`: identical diagnostics, topological gating, clear/retain projection with fixed-index null holes, `inactiveInput` policy, and conditional requiredness.
- `Formgen.attach(root, { model, conditions })` settles applicability on attach, user edits, hydration, reset, replace, and repeater row changes (`formgen:array:change`): `hidden` plus `data-formgen-applicable` markers, `required`/`aria-required`, clear or retain of inapplicable values, obsolete issue clearing, and `meta.applicable` on notifications. New `controller.effective()`, `settle()`, and `conditionDiagnostics()`.
- `validateValues` accepts conditional `required`/`inactive` options; browser `validate()` composes the server-equivalent pipeline over the applicable values only. Inapplicable controls belong to the runtime (cleared, retained, or rendered defaults such as an unchecked box), so `validate()` never reports `inactiveInput`; the server still does for payloads that bypass `effective().values`.
- `examples/embedded` now relies on the shared evaluator; its provisional host applicability adapter is removed.

## Precise Issue Presentation and Embedded Example (next release)

- `controller.setIssues(issues, { append?, summary? })`, `clearIssues()`, and `focusIssue(issue | path)` present browser or server issues on the exact rendered control (indexed paths target one repeated item), route form-level and unmapped issues to a replaceable summary, and fall back to the summary when no visible, enabled control exists. Values are never changed.
- New `registerFormErrorRenderer` hook, `[data-formgen-form-errors]` container (the vanilla form summary now renders `data-formgen-form-errors="true"` and `tabindex="-1"`), and `data-formgen-group-invalid` markers on repeater items and fieldsets with invalid descendants.
- `examples/embedded` demonstrates compile → render → hydrate/edit/cancel → scoped validation → host-owned server issues → typed snapshot → destroy with a registered extension control and a documented host applicability adapter.

## Form Container Width Token (next release)

- Added the canonical `form.container.max-width` semantic token and exported `render.FormContainerMaxWidthToken` / `render.LegacyContainerMaxWidthToken` constants.
- `render.SemanticTokenSpec` now exposes defensive `Aliases` metadata so host profiles can compose the go-formgen registry without duplicating aliases.
- Vanilla and Preact consume the canonical token. The legacy `container-max-width` token and `--container-max-width` CSS variable remain compatible, canonical configuration wins, and omission retains the existing 56rem default.
- Preact document mode now targets its nested hydrated form through the stable `data-formgen-auto-init` hook. Document/form modes report the token consumed; fields mode emits no form-only width rule and reports it unused.

## Semantic Theme Contract (proposed v0.32.0)

- The opt-in `pkg/orchestrator/defaults` theme integration now uses go-theme's constrained projection API with the portable profile extended by go-formgen-owned `form.*` tokens.
- `render.ThemeConfig` carries deterministic safe inline variables, supported semantic tokens, and projection diagnostics without adding go-theme to the headless model or core orchestrator dependency path.
- Vanilla and Preact now share package-token to portable-token fallback semantics for form chrome, controls, focus, validation, disabled, readonly, loading, placeholder, help, and responsive states.
- Preact now renders disabled/readonly attributes and server field/form errors in its client runtime, including accessible invalid-state hooks.
- Preact hydration now preserves document, form, and fields structure. Readonly chips/typeahead controls remain submittable while rejecting all user mutation paths and reflecting runtime loading state.
- `OmitAssets` now suppresses link, style, and script tags consistently in both HTML renderers while preserving theme, variant, and semantic root hooks.
- Empty or unsupported semantic input preserves existing renderer defaults; direct legacy `ThemeConfig`, partials, assets, variants, render modes, vanilla style modes, and class overrides remain compatible.

### Semantic theme migration

- This release requires go-theme v0.5.0 or later. Do not pin the unpublished candidate with a repository `replace`, pseudo-version, or checked-in workspace; update the dependency only after the reviewed go-theme version is published.
- Prefer dotted semantic keys such as `form.control.background`, `color.text.primary`, and `color.focus.ring`. Safe legacy keys continue to project as CSS variables but are diagnosed as unsupported by the semantic profile and are not consumed automatically.
- Invalid token values are intentionally omitted from inline CSS. Inspect the `formgen-theme` JSON diagnostics for `invalid`, `unsupported`, `consumed`, and `unused` outcomes.
- No template or golden refresh is required when semantic tokens are absent.

## API Embedding and Renderer Improvements

- Added a `json` descriptor renderer with a versioned envelope separating form, values, errors, form errors, hidden fields, and metadata.
- Added `RenderModeDocument`, `RenderModeForm`, and `RenderModeFields` for embeddable vanilla and Preact output.
- Added schema-owned nested field ordering: sibling properties that declare `x-formgen.order` or `x-admin.order` are built in that order at top level, inside nested objects, and inside array item objects. A single ordered field is honored; unordered siblings follow in deterministic property-name order.
- Added browser controller hooks at `window.Formgen.attach(root)` for value collection, value/error hydration, default restoration via `reset`, subscriptions, focus, and teardown.
- Added rich renderer-neutral options, generic select/chips option resolvers, and root-scoped `initFormgenRoot` registry lifecycle for embedded forms.
- Embedded-root teardown now disposes refresh/search listeners, pending delayed work, enhanced widgets, components, and array repeaters, allowing safe reinitialization of the same mounted DOM root.
- Added vanilla `StyleModeDefault`, `StyleModeMinimal`, and `StyleModeUnstyled` for host-owned styling.
- Added `Field.Sensitive` detection and default redaction for descriptor and browser renderer output.

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
- Nested field ordering: downstream schema authors can keep order hints on the schema or JSON Schema overlay, including dotted overlay keys such as `x-formgen.order` and `x-admin.order`. Renderer-specific HTML reordering should no longer be needed after updating to this release.
- Repeatable arrays: generated vanilla rows now distinguish persisted rows from newly cloned rows with `data-formgen-array-existing`. Existing rows with a hidden `_delete` sentinel are soft-deleted by the runtime so delete intent is submitted; unsaved cloned rows are removed from the DOM. Custom renderers that use `data-formgen-array-action="remove"` and `_delete` sentinels must emit `data-formgen-array-item="true"` plus `data-formgen-array-existing="true"` for loaded rows and `"false"` for prototype/new rows.
- Templates/themes: when supplying custom templates or go-theme providers, ensure fallback partials map the new component keys (`forms.json-editor`, `forms.file-uploader`).
- Snapshots: refresh vanilla/Preact goldens after updating templates/components using `./taskfile dev:goldens` (requires network access for module downloads).

## Testing Expectations

- `go test ./...` remains the baseline. Renderer suites consume HTML/JSON goldens; set `UPDATE_GOLDENS=1` to regenerate after template changes.
- Example builds (`./taskfile dev:test:examples`) validate CLI/HTTP binaries with pinned toolchain and both renderers in the matrix.

## Known Defaults

- Visibility rules are no-op unless an evaluator is configured.
- Relationship runtime stays offline until an HTTP client is provided.
- Method overrides use `_method` hidden inputs for non-GET/POST verbs in vanilla forms.
