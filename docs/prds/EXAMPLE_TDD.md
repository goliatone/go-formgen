# EXAMPLE_TDD - Advanced HTTP Example (Create Action Modals)

This document specifies an advanced example for `examples/http` that mirrors the
Formgen Runtime Sandbox but uses real, go-formgen-generated forms inside modal
dialogs to create related entities.

## Background

The current HTTP example (`examples/http`) demonstrates relationship resolution,
search mode, chips/typeahead rendering, and runtime bootstrapping. The runtime
sandbox (`client/dev/vanilla.ts`) showcases the "Create ..." action, but uses a
prompt instead of a full form. We want an advanced example that:

- Uses the same OpenAPI metadata and UI schema as the main form.
- Renders create forms for related entities using go-formgen.
- Shows those forms in a modal (Tailwind + vanilla TS).
- Demonstrates a complete create-action flow (open modal, submit, update selection).

## Goals

1) Provide a "Create ..." modal flow for relationship fields (typeahead + chips).
2) Generate create forms for related entities using go-formgen (no manual HTML).
3) Render only the minimum required fields for each related entity.
4) Use Pongo2 templates (go-router style) to assemble the page and modals.
5) Keep the example self-contained (no DB, in-memory dataset only).

## Non-goals

- Production-grade auth, persistence, or validation.
- Framework-specific UI (no React/Preact).
- Full CRUD coverage for every model (create-only is sufficient).

## Requirements

### Functional

- Add an advanced view under `examples/http` (e.g. `/form?view=advanced` or `/advanced`)
  that renders:
  - A primary form (e.g. `post-book:create`).
  - Hidden modals that contain go-formgen-generated create forms for related entities.
- Enable create action metadata for relationship fields in the primary form:
  - `author_id` (typeahead, replace selection)
  - `publisher_id` (typeahead, replace selection)
  - `tags` (chips, append selection)
  - Optional: `chapters` (chips, append selection)
- Provide metadata for the create action:
  - `relationship.endpoint.createAction = "true"`
  - `relationship.endpoint.createActionId = "<id>"`
  - `relationship.endpoint.createActionLabel = "Create <Label>"`
  - `relationship.endpoint.createActionSelect = "append|replace"`
- Generate minimal create forms using go-formgen for:
  - Author (`post-author:create`)
  - Publishing House (`post-publishing-house:create`)
  - Tag (`post-tag:create`)
  - Chapter (`post-chapter:create`) if chapters are enabled
  - Optional: Author Profile / Headquarters for extra coverage
- Implement POST handlers for create endpoints that append to the in-memory dataset
  and return the created record as JSON.
- Implement a runtime `onCreateAction` hook that:
  - Opens the corresponding modal.
  - Prefills a field with the current search query when applicable.
  - Submits the modal form via `fetch`.
  - Returns `{ value, label }` (or multiple options for chips).
  - Closes the modal and updates the relationship selection.

### Non-functional

- The advanced example must run with `go run ./examples/http` and no extra services.
- Modal styling uses Tailwind classes (consistent with existing example output).
- The generated forms should remain valid if JS is disabled (progressive enhancement).

## UX / Interaction Model

- Create action button opens a modal overlay (dialog-like).
- Modal uses an inner form rendered by go-formgen.
- Submit closes the modal and updates the underlying relationship field.
- Escape or "Cancel" closes the modal without changing selection.

## Technical Design

### 1) UI Schema Changes

Extend `examples/http/ui/schema.json` (or add a new UI schema file) to:

- Add create-action metadata to relationship fields via `metadata`:
  - Example (author):
    - `relationship.endpoint.createAction = "true"`
    - `relationship.endpoint.createActionId = "author"`
    - `relationship.endpoint.createActionLabel = "Create Author"`
- Tag minimal fields for modal usage, using tags or sections:
  - Add a `tags` array to field configs (e.g. `"tags": ["modal-min"]`).
  - Use `render.RenderOptions.Subset.Tags = ["modal-min"]` for modal forms.

### 2) Template Layout (Pongo2)

Add a Pongo2 layout template for the advanced view:

- `examples/http/templates/advanced.tmpl`
  - Accepts:
    - `main_form_html`
    - `modals` (map of actionId -> form HTML + metadata)
  - Renders the main form and hidden modal containers.

Modal markup uses Tailwind classes and a consistent structure:

```
<div class="fixed inset-0 hidden ..." data-fg-modal="author">
  <div class="...">
    <form id="form-author-create">...</form>
  </div>
</div>
```

### 3) Server Rendering (examples/http/main.go)

Add a new handler for the advanced view:

- Build the main form using `orchestrator.Generate`.
- Build modal forms per actionId:
  - Use `RenderOptions.Subset` to restrict to required fields.
  - Use `form.actions` in UI schema to add "Cancel" + "Create" buttons.
- Render the Pongo2 layout with the generated HTML strings.

### 4) Runtime Hook (vanillaRuntimeBootstrap)

Extend the injected script to register `onCreateAction`:

- Maintain a map of actionId -> modal config:
  - `modalId`, `formId`, `endpoint`, `valueField`, `labelField`, `prefillField`.
- Open the modal and prefill the input when `detail.query` is present.
- Submit via `fetch`, parse JSON, return `Option` to the runtime.
- Support `append` for chips and `replace` for typeahead.

### 5) Create Endpoints (examples/http/main.go)

Add POST handlers for the create endpoints:

- `/api/authors` (create minimal author)
- `/api/publishing-houses` (create minimal publisher)
- `/api/tags` (create minimal tag)
- `/api/chapters` (optional)
- `/api/author-profiles` / `/api/headquarters` (optional)

Each handler should:

- Parse JSON payload.
- Validate required fields (minimal).
- Generate an ID (UUID or timestamp).
- Append to in-memory dataset.
- Return the created record as JSON.

## Create Action Registry (Initial Mapping)

| actionId | Operation ID | Minimal fields | Select behavior |
| --- | --- | --- | --- |
| author | `post-author:create` | `full_name`, `email` | `replace` |
| publisher | `post-publishing-house:create` | `name`, `imprint_prefix` | `replace` |
| tag | `post-tag:create` | `name` | `append` |
| chapter (optional) | `post-chapter:create` | `title`, `word_count` | `append` |

## Acceptance Criteria

- Advanced view renders a primary form plus modal forms generated by go-formgen.
- Create action button opens the correct modal (author/publisher/tag).
- Submitting a modal creates a record, closes the modal, and updates selection.
- Tag create action appends chips; author/publisher replace selection.
- No external dependencies are required beyond `go run ./examples/http`.

## Implementation Plan

1) UI schema updates for create-action metadata + modal subset tagging.
2) Pongo2 template for advanced view layout + modal containers.
3) Server handler to render main form + modal forms.
4) Runtime hook for modal open/submit/selection update.
5) POST endpoints for create actions (in-memory dataset).
6) Update `examples/http/README.md` with usage instructions.
