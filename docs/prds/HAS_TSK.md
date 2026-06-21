# HAS_TSK – Implementation Plan (Relationship Create Action)

Roadmap aligned with `HAS_TDD.md`, following the phased structure used in other modules.

## Phase 0. Planning & Scaffolding
**Why**: Align the create-action feature scope with existing inline-create behavior and runtime contracts.

**Tasks**
- [x] Task 0.1 – Cross-reference `HAS_TDD.md` with existing relationship runtime (`chips`, `typeahead`, `relationship-events`) and document integration points.
- [x] Task 0.2 – Confirm decisions: create action + inline create coexistence, hook precedence, keyboard semantics, and non-search availability.
- [x] Task 0.3 – Add TODO markers in `HAS_TDD.md` for any deferred open questions (if any remain).
- [x] Task 0.4 – Coordinate with `UXI_TSK.md` for footer structure and hybrid keyboard model to prevent divergence.

**Acceptance Criteria**
- Decisions recorded; scope is clear; no unresolved conflicts with inline-create flow.

## Phase 1. Config & Dataset Plumbing
**Why**: Surface create-action metadata in the runtime model and dataset parsing.

**Tasks**
- [x] Task 1.1 – Extend `FieldConfig` with `createAction`, `createActionLabel`, `createActionId`, and `createActionSelect`.
- [x] Task 1.2 – Parse new `data-endpoint-create-action*` attributes in `client/src/index.ts`.
- [x] Task 1.3 – Add `onCreateAction` to `GlobalConfig` and wire into `ResolvedGlobalConfig`.
- [x] Task 1.4 – Update `allowCreate` docstring to clarify inline-create vs create-action.

**Acceptance Criteria**
- New dataset attributes map into `FieldConfig`; config types updated without breaking existing builds.

## Phase 2. Event Contract
**Why**: Provide a standardized DOM event payload when the hook is not supplied.

**Tasks**
- [x] Task 2.1 – Define `formgen:relationship:create-action` event name and payload type.
- [x] Task 2.2 – Add a small helper for dispatching the event (in `relationship-events.ts` or renderer-local).
- [x] Task 2.3 – Ensure the event is emitted only when `onCreateAction` is not provided.

**Acceptance Criteria**
- Event contract defined and used consistently; hook precedence is enforced.

## Phase 3. Typeahead Create Action UI
**Why**: Add the create action row to has-one typeahead dropdowns.

**Tasks**
- [x] Task 3.1 – Render a create action row in `typeahead` dropdown when enabled, even when options exist.
- [x] Task 3.2 – Ensure the action row uses `role="button"` outside the listbox and can be reached via the hybrid focus model (ArrowDown from last option).
- [x] Task 3.3 – Wire activation to `onCreateAction` or event dispatch; close dropdown on activation.
- [x] Task 3.4 – Restrict returned values to a single `Option` (ignore arrays or convert to first item).

**Acceptance Criteria**
- Create action appears alongside matches and is keyboard-accessible without breaking selection flow.

## Phase 4. Chips Create Action UI
**Why**: Add the create action footer to multi-select chips dropdowns.

**Tasks**
- [x] Task 4.1 – Add a footer container to chips dropdown and render create action there when enabled.
- [x] Task 4.2 – Ensure the action row uses `role="button"` outside the listbox and is reachable via the hybrid focus model.
- [x] Task 4.3 – Wire activation to `onCreateAction` or event dispatch.
- [x] Task 4.4 – Implement append/replace selection behavior for returned `Option | Option[]`.
- [x] Task 4.5 – Apply the default "Model 1" menu behavior (close + clear query).

**Acceptance Criteria**
- Create action appears in dropdown footer, supports multiple created items, and respects append/replace.

## Phase 5. Selection Application & State Updates
**Why**: Ensure created items integrate cleanly with selection mirrors and UI updates.

**Tasks**
- [x] Task 5.1 – Implement option injection + selection updates for returned create-action results.
- [x] Task 5.2 – Emit `formgen:relationship:update` (selection) and native `change` in renderer flows.
- [x] Task 5.3 – Ensure created options are added to `store.options` for consistent filtering.

**Acceptance Criteria**
- New selections are reflected in chips/typeahead UI and in hidden/json mirrors.

## Phase 6. Tests
**Why**: Lock down the contract and prevent regressions.

**Tasks**
- [x] Task 6.1 – Add runtime tests for typeahead create action (rendering, event payload, hook precedence).
- [x] Task 6.2 – Add runtime tests for chips create action (footer placement, append/replace behavior).
- [x] Task 6.3 – Add keyboard navigation tests for the hybrid focus transitions (last option → action, action → input).
- [x] Task 6.4 – Add non-search-mode tests for both renderers.

**Acceptance Criteria**
- Tests cover create-action rendering, selection behavior, and keyboard navigation.

## Phase 7. Docs & Examples
**Why**: Ensure the new contract is usable by consumers.

**Tasks**
- [x] Task 7.1 – Update `client/README.md` with new attributes, hook signature, and event payload.
- [x] Task 7.2 – Add a vanilla example (or `client/dev/vanilla.ts`) that listens for the create-action event and injects a created option.
- [x] Task 7.3 – Add migration guidance: inline create vs create action, coexistence, hook precedence.

**Acceptance Criteria**
- Docs and examples are updated and align with `HAS_TDD.md`.

## Phase 8. QA & Release Checks
**Why**: Validate behavior across modes and ensure compatibility.

**Tasks**
- [ ] Task 8.1 – Manual verification of typeahead and chips create action in search and default modes.
- [ ] Task 8.2 – Verify selection mirrors (`hidden`/`json`) update correctly after create action.
- [ ] Task 8.3 – Update changelog/release notes if required.

**Acceptance Criteria**
- QA confirms behavior in both renderers; tests green; docs current.
