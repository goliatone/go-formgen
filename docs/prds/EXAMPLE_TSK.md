# EXAMPLE_TSK – Advanced HTTP Example (Create Action Modals)

Roadmap aligned with `EXAMPLE_TDD.md`, following the phased structure used in other modules.

## Phase 0. Planning & Scaffolding
**Why**: Lock scope, entry points, and identify dependencies between backend and frontend work.

**Tasks**
- [x] Task 0.1 – Review `EXAMPLE_TDD.md` and confirm target operations + create-action mapping (author, publisher, tag, optional chapter/headquarters).
- [x] Task 0.2 – Inventory existing UI schema + dataset coverage in `examples/http` and document any missing fields needed for modal forms.
- [x] Task 0.3 – Decide advanced view entry point (`/advanced` vs `/form?view=advanced`) and document routing choice in this plan. (Chosen: `/advanced`)

**Acceptance Criteria**
- Scope and routing finalized; dependencies understood.
**Status**: Complete.

## Phase 1. Backend: UI Schema + Metadata (Create Action + Modal Subsets)
**Why**: Emit create-action metadata to the runtime and tag minimal fields for modal rendering.

**Tasks**
- [x] Task 1.1 – Add create-action metadata to relationship fields in `examples/http/ui/schema.json` (author, publisher, tags, optional chapters).  
  Depends on: Task 0.1, Task 0.2
- [x] Task 1.2 – Tag minimal modal fields per related entity (e.g., `tags: ["modal-min"]`) in UI schema.  
  Depends on: Task 0.2
- [x] Task 1.3 – Add modal form actions (Cancel + Create) in UI schema for the create operations.  
  Depends on: Task 1.2

**Acceptance Criteria**
- Primary form emits `data-endpoint-create-action*` attributes.
- Modal subset tags and actions exist in UI schema.

## Phase 2. Backend: Advanced Example Rendering
**Why**: Generate main form + modal forms with go-formgen.

**Tasks**
- [x] Task 2.1 – Add advanced view handler in `examples/http/main.go` to render:
  - main form (e.g., `post-book:create`)
  - modal forms per related entity (subset by tag)  
  Depends on: Task 1.1, Task 1.2, Task 1.3
- [x] Task 2.2 – Add Pongo2 templates for advanced page + modal layout (Tailwind styling).  
  Depends on: Task 2.1
- [x] Task 2.3 – Wire template rendering into the handler (pass main form + modal HTML).  
  Depends on: Task 2.2

**Acceptance Criteria**
- Advanced view renders main form + hidden modal forms.

## Phase 3. Backend: Create Endpoints (In-Memory)
**Why**: Support actual creation flows in the example.

**Tasks**
- [x] Task 3.1 – Add POST handlers for `/api/authors`, `/api/publishing-houses`, `/api/tags`.  
  Depends on: Task 0.2
- [ ] Task 3.2 – Optional: add POST handlers for `/api/chapters`, `/api/author-profiles`, `/api/headquarters`.  
  Depends on: Task 0.2
- [x] Task 3.3 – Ensure responses return created entity payloads used by the runtime selection.  
  Depends on: Task 3.1

**Acceptance Criteria**
- Create endpoints append to in-memory dataset and return JSON.

## Phase 4. Frontend: Modal Runtime + Create Action Wiring
**Why**: Open modal, submit, and return options to runtime.

**Tasks**
- [x] Task 4.1 – Extend `vanillaRuntimeBootstrap` in `examples/http/main.go` with `onCreateAction` hook and modal registry.  
  Depends on: Task 2.3
- [x] Task 4.2 – Implement modal open/close/prefill and form submission via fetch.  
  Depends on: Task 4.1, Task 3.1
- [x] Task 4.3 – Map create responses to `Option` or `Option[]` and return to runtime.  
  Depends on: Task 4.2

**Acceptance Criteria**
- Create action opens modal and updates selection on submit.

## Phase 5. Frontend: Progressive Enhancement + UI Polishing
**Why**: Keep the example usable without JS and improve UX consistency.

**Tasks**
- [x] Task 5.1 – Ensure modal forms are valid HTML and can submit without JS (fallback).  
  Depends on: Task 2.2, Task 3.1
- [x] Task 5.2 – Add Tailwind modal polish (overlay, focus ring, close button, escape handling).  
  Depends on: Task 4.2
- [x] Task 5.3 – Add prefill behavior for search query to the modal form inputs.  
  Depends on: Task 4.2

**Acceptance Criteria**
- Modal UX aligns with runtime sandbox styling; fallback is usable.

## Phase 6. QA & Docs
**Why**: Ensure the example is discoverable and works end-to-end.

**Tasks**
- [ ] Task 6.1 – Manual QA: create author/publisher/tag via modal updates selection (typeahead + chips).  
  Depends on: Task 4.3
- [x] Task 6.2 – Update `examples/http/README.md` with advanced view usage + how to test create actions.  
  Depends on: Task 2.3, Task 4.3
- [x] Task 6.3 – Add mention in `docs/GUIDE_RELATIONSHIPS.md` pointing to the advanced example.  
  Depends on: Task 6.2

**Acceptance Criteria**
- Advanced example is documented; manual QA complete.
