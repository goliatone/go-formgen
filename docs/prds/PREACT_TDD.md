# go-formgen – PREACT_TDD (Parity with Vanilla Renderer)

This document captures the scope, gaps, and requirements to bring the **preact** renderer to feature parity with the **vanilla** renderer. It is meant to be the shared source of truth for planning, implementation, and test coverage.

## Background

`go-formgen` renders `FormModel` output through multiple renderers:
- **vanilla**: server-rendered HTML via Go templates and embedded Tailwind CSS.
- **preact**: server-rendered shell + client runtime hydration to interactive components.

The vanilla renderer currently defines the “gold standard” for:
- layout & section structure
- UI schema interpretation
- styling and theme assets
- relationship behaviors (typeahead/chips)
- validation + error feedback

The preact renderer must reach parity with vanilla, while retaining its client-driven interactivity and runtime relationship hooks.

## Goals

1) Match vanilla’s rendered structure, layout rules, and UI schema semantics.
2) Support the same widgets/components and field behaviors as vanilla.
3) Match relationship features (typeahead, chips, create actions) and current value seeding.
4) Provide validation + error/feedback UX comparable to vanilla.
5) Ensure theming, assets, and UI metadata flow through both renderers consistently.
6) Keep contract tests and goldens aligned across renderers.

## Non-Goals

- Redesigning the preact renderer UI/UX beyond parity with vanilla.
- Introducing new widgets or behaviors that vanilla does not support.
- Rewriting the form model pipeline or UI schema loader.

## Current State (Summary)

### Vanilla Renderer
- Go templates in `pkg/renderers/vanilla/templates` and component renderers in `pkg/renderers/vanilla/components`.
- Default styles embedded in `pkg/renderers/vanilla/assets/formgen-vanilla.css`.
- Uses UI schema metadata + theme config to render layout, sections, actions, and chrome.
- Emits relationship `data-*` attributes and includes runtime bundles when needed.

### Preact Renderer
- Shell template in `pkg/renderers/preact/templates/page.tmpl`.
- Hydrates runtime bundle in `pkg/renderers/preact/assets/formgen-preact.min.js`.
- Relationship runtime hooks exposed in `client/src/frameworks/preact.ts`.
- Theme assets configurable via `preact` renderer options and theme asset keys.

## Parity Gaps (Known)

> Source of truth for open gaps: `TODO.md`.

1) **JSON Editor**
   - Vanilla has a JSON editor; preact does not yet support the same component.
   - See `TODO.md` and widget identifiers in `pkg/widgets`.

2) **Relationship Chips / Typeahead**
   - Preact needs chip selector for multi-select and has-one lookahead.
   - Runtime behaviors must match vanilla’s relationship features.

3) **Validation + Error Feedback**
   - Preact needs validation messages, field-level errors, and feedback chrome.
   - Should match vanilla’s error markup + semantics.

4) **UI Schema Layout Semantics**
   - Sections, layout hints, field ordering, and grid should align with vanilla.
   - Ensure preact respects metadata like `layout.*`, actions, and field ordering.

5) **Visual/Styling Parity**
   - Preact styles should align with vanilla’s defaults (or explicit mapping).
   - Theme tokens and CSS vars must apply consistently.

## Requirements (Parity Matrix)

### Layout + Structure
- Render form container, header, sections, fieldsets consistent with vanilla structure.
- Honor layout hints: grid columns, gutter, field spans, row/column start.
- Respect section definitions and field ordering metadata.

### Actions
- Render primary/secondary actions per UI schema config.
- Match default behavior for missing/partial action configs.

### Widgets + Components
- Same widget registry identifiers in `pkg/widgets`.
- Cover vanilla component behavior (input, select, textarea, switches, file upload, JSON editor, etc.).
- Component chrome (label, description, help text, required indicator) should align.

### Relationships
- Emit and consume the same `data-*` attributes as vanilla.
- Typeahead (single-select) and chips (multi-select) must match behavior.
- Allow “create action” per `docs/prds/HAS_TDD.md`.
- Support seeded current values (`data-relationship-current` and label seeding).

### Validation + Feedback
- Display field-level errors and global form feedback similarly to vanilla.
- Ensure validation metadata and runtime events are wired to UI.

### Theming
- Apply theme config (tokens, CSS vars, partials, asset URLs) consistently.
- Preact must honor `preact.vendor`, `preact.app`, `preact.stylesheet` asset keys.

## Implementation Notes

### Renderers
- Vanilla renderer: `pkg/renderers/vanilla/renderer.go` + templates.
- Preact renderer: `pkg/renderers/preact/preact.go` + `pkg/renderers/preact/templates/page.tmpl`.

### Runtime + Client
- Relationship runtime: `client/src/index.ts`, `client/src/frameworks/preact.ts`.
- Preact runtime bundle: `client/scripts/build.ts` output to `pkg/renderers/preact/assets`.

### UI Schema + Metadata
- UI schema reference: `docs/GUIDE_CUSTOMIZATION.md` and `docs/README_SCHEMA.md`.
- Layout/field order metadata stored under `FormModel.Metadata`.

## Tests + Goldens

- Preact renderer contract tests: `pkg/renderers/preact/renderer_contract_test.go`.
- Vanilla renderer contract tests: `pkg/renderers/vanilla/renderer_contract_test.go`.
- Orchestrator integration tests for multiple renderers: `pkg/orchestrator/orchestrator_integration_test.go`.
- Client runtime tests: `client/tests`.
- Golden updates: `client/scripts/update_goldens.sh` (see `docs/RELEASE_NOTES.md`).

## Acceptance Criteria

1) All parity gaps listed here are implemented for preact.
2) Preact renderer contract tests pass with refreshed goldens.
3) Vanilla renderer tests remain unchanged (parity must not regress vanilla).
4) Relationship behaviors and UI schema features operate the same between renderers.
5) Theme assets and tokens apply consistently across renderers.

## References

- `TODO.md`
- `docs/GUIDE_CUSTOMIZATION.md`
- `docs/GUIDE_RELATIONSHIPS.md`
- `docs/GUIDE_STYLING.md`
- `docs/README_SCHEMA.md`
- `docs/prds/HAS_TDD.md`
- `pkg/renderers/vanilla/`
- `pkg/renderers/preact/`
- `client/src/frameworks/preact.ts`
