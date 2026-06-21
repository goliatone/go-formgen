# go-formgen Implementation Plan

Roadmap aligned with `FORMGEN_TDD.md`, following the phased structure used in other modules.

## Phase 0. Planning & Scaffolding
**Why**: Confirm scope, dependencies, and ownership across enhancements.

**Tasks**
- [x] Task 0.1 – Cross-reference `FORMGEN_TDD.md` with related TDDs (GO_SETTINGS_TDD, GO_MEDIA_TDD, EXPORT_TDD, THEMING_TDD) and document ownership split in `docs/prds/FORMGEN_OVERVIEW.md`.
- [x] Task 0.2 – Create umbrella issues for extensions mapping, widget registry, visibility evaluator, provenance/prefill metadata, error integration, theming/templates, JSON/object editor, submission helpers, partial generation.
- [x] Task 0.3 – Add CI/test placeholders for new features/snapshots.

**Acceptance Criteria**
- Ownership and scope documented; tracking issues opened.
- CI placeholders ready.

## Phase 1. Extensions & Model Updates
**Why**: Surface admin extensions and metadata in the FormModel.

**Tasks**
- [x] Task 1.1 – Add vendor extension parsing for `x-admin-*` (group, tags, widget, visibility rule, help, placeholder, readonly, order).
- [x] Task 1.2 – Plumb extension metadata into `FormModel` fields/sections.
- [x] Task 1.3 – Tests/goldens for extension propagation.

**Acceptance Criteria**
- Extensions parsed and exposed in model; tests updated; loader/render paths remain offline-friendly (no mandatory HTTP fetches).

## Phase 2. Widget/Renderer Registry
**Why**: Runtime-customizable widgets.

**Tasks**
- [x] Task 2.1 – Implement runtime widget/field renderer registry with priorities and `x-admin-widget` mapping.
- [x] Task 2.2 – Register built-in widgets (toggle, select+chips, code editor, JSON/object editor, key/value editor).
- [x] Task 2.3 – Tests for registry behavior and built-ins.

**Acceptance Criteria**
- Registry available; default widgets registered and covered by tests.

## Phase 3. Conditional Visibility
**Why**: Show/hide fields/sections based on rules.

**Tasks**
- [x] Task 3.1 – Define `VisibilityEvaluator` interface and context payload.
- [x] Task 3.2 – Wire evaluator into model/renderers; default no-op.
- [x] Task 3.3 – Tests for visibility evaluation when evaluator provided.

**Acceptance Criteria**
- Visibility rules evaluated when configured; no-op by default; tests cover.

## Phase 4. Provenance & Prefill Metadata
**Why**: Show scope/source of values.

**Tasks**
- [x] Task 4.1 – Extend render options to carry value provenance/labels.
- [x] Task 4.2 – Update renderers to display provenance badges/readonly state when provided.
- [x] Task 4.3 – Tests/snapshots for provenance rendering.

**Acceptance Criteria**
- Provenance surfaced in renderers; tests updated.

## Phase 5. Error Integration & Submission Helpers
**Why**: Align with go-errors and operational needs.

**Tasks**
- [x] Task 5.1 – Map error payloads (path -> messages) compatible with go-errors to field IDs; support form-level errors.
- [x] Task 5.2 – Add submission helpers for CSRF/auth/version fields and method overrides.
- [x] Task 5.3 – Tests for error mapping and hidden field helpers.

**Acceptance Criteria**
- Errors map cleanly; helpers available; tests pass.

## Phase 6. Theming & Templates
**Why**: Consistent theming with go-cms/go-settings.

**Tasks**
- [x] Task 6.1 – Add ThemeProvider option to orchestrator/renderers for template roots/partials and token injection (per THEMING_TDD).
- [x] Task 6.2 – Ensure Preact/vanilla renderers honor theme tokens/assets/partials.
- [x] Task 6.3 – Tests/snapshots for theme override behavior.

**Acceptance Criteria**
- Theme provider works; renderers theme-aware; snapshots updated.

## Phase 7. JSON/Object Editor
**Why**: Improve rendering of arbitrary objects/maps.

**Tasks**
- [x] Task 7.1 – Add richer JSON/object editor widget with schema hints, collapse/expand, pretty-print.
- [x] Task 7.2 – Register widget in registry; integrate with `x-admin-widget` mapping.
- [x] Task 7.3 – Tests/snapshots for JSON editor.

**Acceptance Criteria**
- JSON/object editor available and tested.

## Phase 8. Partial Form Generation
**Why**: Render subsets (tabs/sections) per tags/groups.

**Tasks**
- [x] Task 8.1 – Add support to generate forms by tag/group/section subset.
- [x] Task 8.2 – Tests for partial generation in vanilla/Preact.

**Acceptance Criteria**
- Partial generation works; tests pass.

## Phase 9. Submission & Data Sources
**Why**: Improve input handling.

**Tasks**
- [x] Task 9.1 – Support prefill values and disabled/readonly flags per field via render options.
- [x] Task 9.2 – Ensure registry allows runtime injection of custom widgets from adapters (settings/media/export).
- [x] Task 9.3 – Document adapter pattern for go-settings/go-media/go-export to register widgets/visibility evaluators.

**Acceptance Criteria**
- Prefill/readonly handled; adapters can register widgets/evaluators; docs updated.

## Phase 10. Docs, Examples, QA & Release
**Why**: Ship with confidence and guidance.

**Tasks**
- [x] Task 10.1 – Update README/docs with new options (extensions, widgets, visibility, provenance, theming, errors, partials).
- [x] Task 10.2 – Refresh examples/snapshots (vanilla/Preact) covering new widgets and visibility/provenance.
- [x] Task 10.3 – CI/tests green; goldens updated.
- [x] Task 10.4 – Release notes and migration guidance for consumers (opt-in features, defaults).

**Acceptance Criteria**
- Docs/examples current; tests green; release notes published.

## Phase 11. Runtime File Uploader Hydration (Edit Forms)
**Why**: Make the runtime `file_uploader` usable for create + edit flows without app-specific glue.

**Tasks**
- [x] Task 11.1 – Implement automatic hydration for existing values in `client/src/components/file-uploader/index.ts` (single + multiple) per `FORMGEN_TDD.md` (“Runtime: File Uploader Hydration + Go-Consumable Assets”).
- [x] Task 11.2 – Ensure serialization remains runtime-owned and stable (single keeps one hidden input; multiple emits repeated `name="field[]"` inputs and consumes any pre-rendered extras).
- [x] Task 11.3 – Extend `client/tests/file-uploader.test.ts` to cover hydration + remove behavior in single and multiple modes.

**Acceptance Criteria**
- The acceptance criteria in `FORMGEN_TDD.md` (“Runtime: File Uploader Hydration + Go-Consumable Assets”) passes, including “Remove” clearing serialized values and preview.

## Phase 12. Runtime Assets as Go-Embeddable FS (No npm required)
**Why**: Allow Go apps to serve runtime bundles directly from the go-formgen module.

**Tasks**
- [x] Task 12.1 – Commit and version browser bundles under `pkg/runtime/assets/...` (generated from `client` build output), and ensure releases include them.
- [x] Task 12.2 – Add `formgen.RuntimeAssetsFS() fs.FS` exposing the committed runtime bundles (see `FORMGEN_TDD.md` section “Go-Consumable Runtime Assets”).
- [x] Task 12.3 – Document mounting + `<script>` usage for Go servers (README or `docs/`), including the required init call and `data-formgen-auto-init` usage.

**Acceptance Criteria**
- A Go consumer can mount `formgen.RuntimeAssetsFS()` via `http.FileServerFS` and use the runtime (including `file_uploader`) without running npm.

## Phase 13. Optional: Auto-inject Runtime Scripts
**Why**: Remove per-form manual `<script>` tags when runtime components are present.

**Tasks**
- [x] Task 13.1 – Teach orchestrator/renderer to populate `component_scripts` when runtime components appear (at minimum `file_uploader`), deduping entries (see `FORMGEN_TDD.md` “Optional: Auto-inject runtime scripts when used”).
- [x] Task 13.2 – Update render output/goldens to include auto-injected scripts where appropriate.

**Acceptance Criteria**
- Forms that include `file_uploader` automatically include the runtime script once, without app-specific templates.
