# go-formgen Client – HAS_TDD (Relationship Create Action: Has‑One + Multi‑Select)

This document specifies an optional “Create …” action for relationship fields rendered with:
- **typeahead** (has-one / belongs-to)
- **chips** (has-many / many-to-many)

The action is **not** responsible for creating the record inline; instead it triggers a host-provided action (e.g. open a modal) and then lets the host (or a hook return value) apply newly created selection(s) back into the form.

## Background

The client runtime (`client/`) resolves relationship fields by reading `data-*` attributes, fetching `{value,label}` options, and rendering UI widgets like:
- `typeahead` (single-select)
- `chips` (multi-select)

The runtime has an internal semantic event contract:
- Resolver lifecycle: `formgen:relationship:loading|success|error|validation`
- Internal/UI contract: `formgen:relationship:update` with `detail.kind: "options" | "selection" | "search"`

We also support **inline creation** for search-mode widgets via:
- `data-endpoint-allow-create="true"` (per field)
- `initRelationships({ createOption })` (global hook)

Inline create is best for quick, lightweight values (e.g. tags). For has-one relationships (e.g. Author, Manager), creation often requires a full form (multiple fields), permissions, or complex flows. We need an optional create action that delegates to the host UI (modal/panel/redirect). Inline create and create action can coexist on the same field.

## Goals

1) Provide an optional, reusable “Create …” UI affordance in the **typeahead dropdown** (visible alongside matches).
2) Provide an optional, reusable “Create …” UI affordance in the **chips dropdown footer** (separate from options list).
3) Trigger a host-defined action (modal/panel/redirect) without baking UI framework assumptions into the runtime.
4) Support returning **one or multiple** created items from a single modal session (chips only; typeahead is single-select).
5) Preserve the “native `change` is for external semantics” rule; internal wiring should remain event-driven.
6) Provide enough context (field + endpoint + query) for the host to open the correct create flow and optionally prefill with the query.
7) Allow create action and inline create to coexist without conflicts.
8) Do not require search mode for create action; it should work in default mode too.

## Non‑Goals

- Implementing modal UI, routing, or persistence in the runtime.
- Implementing backend create endpoints or payload schemas.
- Automatically selecting a created record without host confirmation.
- Replacing inline creation (covered by the chips/typeahead inline create hooks). The create action is an explicit “open creator UI” affordance and may coexist.

## User Experience (UX)

### Primary Flow (Has‑One Create Action)
1) User focuses a has-one `typeahead` field and types a query.
2) Dropdown shows matching options.
3) Dropdown also shows a “Create …” action (label configurable).
4) User activates “Create …”:
   - Dropdown closes (recommended).
   - A host-defined handler runs (e.g. open “Create Author” modal).
5) After the host creates the record, the host applies the selection:
   - Refresh options (`registry.resolve(select)`) or directly inject/select the created option.
   - Emit selection updates so UI/mirrors update.

### Primary Flow (Multi‑Select Create Action)
1) User focuses a has-many/many-to-many `chips` field and types a query.
2) Dropdown shows matching options.
3) Dropdown footer shows a “Create …” action (label configurable).
4) User activates “Create …”:
   - A host-defined handler runs (e.g. open “Create Tags” modal).
5) The modal session can create **one or multiple** related items.
6) After the modal closes, newly created items are applied to the select:
   - Default for multi-select is **append**.
   - Replace behavior is configurable (preferred via UI schema → dataset mapping).
7) Chips UI reflects the updated selection.

### Keyboard/Accessibility Expectations
- **Hybrid focus model:** options are highlighted via `aria-activedescendant` while focus stays in the input.
- The create action is a focusable control (`role="button"`) placed outside the listbox.
- ArrowDown from the last option moves focus to the create action; ArrowUp returns focus to the input and re-highlights the last option.
- `Enter` on a highlighted option selects it.
- `Enter` on the create action triggers the create action.
- `Escape` closes the dropdown and restores focus appropriately.

## Configuration Contract

### Field Attributes (DOM)

Proposed `data-*` attributes on the native `<select>`:

- `data-endpoint-create-action="true"`
  - Enables the create action row for this field.
- `data-endpoint-create-action-label="Create Author"`
  - Optional override for the create action label.
  - If omitted, default label is derived from field label and/or query.
- `data-endpoint-create-action-id="author"`
  - Optional identifier the host can use to route to the correct modal/flow.
- `data-endpoint-create-action-select="append|replace"`
  - How returned options are applied to selection.
  - Defaults:
    - `typeahead` (single-select): `replace`
    - `chips` (multi-select): `append`
  - Preferred source-of-truth is UI schema; renderers should emit this as a `data-*` attribute.

Notes:
- This feature targets has-one typeahead fields and multi-select chips fields.
- Create action is not limited to search mode; it should render in default mode as well.
- This is distinct from `data-endpoint-allow-create="true"` which enables inline creation via `createOption`. Both may be enabled together.

### Runtime Hook (GlobalConfig)

Add an optional hook:

```ts
onCreateAction?: (
  context: ResolverContext,
  detail: {
    query: string;
    actionId?: string;
    mode: "typeahead" | "chips";
    selectBehavior: "append" | "replace";
  }
) => Option | Option[] | void | Promise<Option | Option[] | void>;
```

Behavior:
- If provided, the renderer invokes it when the user activates the create action.
- **Typeahead** expects `Option | void` (single-select). **Chips** may return `Option | Option[] | void`.
- If it returns option(s), the renderer applies them according to `selectBehavior`, emits semantic selection updates, and dispatches native `change` for external integration.
- If it returns `void`, the host is expected to apply selection manually (see “Host Integration”).
- If not provided, the renderer dispatches a DOM event (below).

### DOM Event Contract

Dispatch a CustomEvent when the create action is activated:

- Event name: `formgen:relationship:create-action`
- `bubbles: true`
- `detail` payload:

```ts
type RelationshipCreateActionDetail = {
  element: HTMLElement;          // the underlying relationship <select>
  field: FieldConfig;
  endpoint: EndpointConfig;
  query: string;                // current search query (possibly empty)
  actionId?: string;            // from data-endpoint-create-action-id
  mode: "typeahead" | "chips";
  selectBehavior: "append" | "replace";
};
```

Guidance:
- If `onCreateAction` is provided, it takes precedence and the DOM event is **not** emitted.
- The detail should be plain data (no function references) so it remains easy to inspect/debug and safe to forward.

## Host Integration (Post‑Create Selection)

After the modal/panel creates a related record, the host can apply selection in two supported ways.

### Option A: Refresh then select (recommended)

1) Refresh options:
   - `await registry.resolve(select)`
2) Select created record:
   - Set `select.value = createdId` (and ensure an `<option>` exists if the API doesn’t return it yet).
3) Notify runtime/UI (choose one):
   - Dispatch native `change` and let the relationship bridge emit the semantic update, or
   - Dispatch `formgen:relationship:update` with `{ kind:"selection", origin:"program", selectedValues:[createdId] }` (do not also dispatch `change`).

### Option B: Inject and select (works offline / immediate)
1) Add an `<option>` node for the created record.
2) Mark it selected.
3) Emit the same selection events as above (pick one, not both).
4) Optionally call `registry.resolve(select)` afterward to sync labels/metadata.

### Multi‑Select: Append vs Replace

When the create action produces multiple created records, the selection update should:
- De-duplicate created values against existing selection.
- Apply based on `selectBehavior`:
  - `append` (default for multi-select): union of existing selection + created values.
  - `replace`: clear existing selection then select only created values.

Preferred configuration source:
- UI schema controls the behavior.
- Renderers translate UI schema into `data-endpoint-create-action-select="..."` so the runtime stays markup-driven.

## Menu + Query Interaction (Chips Footer Action)

The create action for multi-select chips lives in the **dropdown footer** (outside the listbox). After triggering the modal and applying created items, we need a policy for:
- whether the dropdown stays open
- whether the search query is preserved or cleared
- whether focus returns to the input

Below are viable interaction models with pros/cons.

### Model 1: Close on Action, Clear Query (Recommended Default)
**Behavior**
- User clicks “Create …” in the footer: dropdown closes immediately.
- Modal runs.
- After modal returns: apply selection (append/replace), clear query, keep dropdown closed.

**Pros**
- Least UI jank; avoids dropdown re-rendering while modal opens.
- Simple mental model (similar to selecting an option).
- Works well with “create multiple” sessions (modal can create many; dropdown doesn’t need to remain open).

**Cons**
- Requires an extra click to continue selecting more existing items after create.

### Model 2: Close on Action, Preserve Query (Resume Search)
**Behavior**
- Dropdown closes when action triggers.
- After modal returns: apply selection, keep query in input, reopen dropdown and keep filtering.

**Pros**
- User returns to the same search intent after modal.
- Good when the query is a “topic” and user wants to continue selecting related items.

**Cons**
- Reopening/focus restoration is timing-sensitive and can feel jumpy.
- Query may now literally match the created item and change the list abruptly (especially if the created item is now selected and therefore filtered out).

### Model 3: Keep Open, Clear Query, Refocus Input (Fast Tagging)
**Behavior**
- Trigger action (dropdown may close while modal is open), but after modal returns: reopen dropdown, clear query, focus input.

**Pros**
- Most efficient for rapid multi-select workflows (create + keep adding).

**Cons**
- More complex state management (modal focus transitions, resolver refresh timing, menu stability).
- Highest chance of subtle bugs across browsers.

Recommendation:
- Default to **Model 1**.
- Allow UI schema to select a different model for advanced workflows.
<!-- TODO: UI schema support for menu behavior model selection is deferred. Initial implementation uses Model 1 only. -->

## Implementation Plan (Client)

### Parsing & Field Model
- Extend `FieldConfig` to include:
  - `createAction?: boolean`
  - `createActionLabel?: string`
  - `createActionId?: string`
  - `createActionSelect?: "append" | "replace"`
- Parse these from the dataset in `client/src/index.ts`.

### Typeahead UI
Update `client/src/renderers/typeahead.ts` to:
- Detect `field.createAction` and render a create action row in the dropdown (even when matches exist).
- Use the configured label (or a default derived label).
  <!-- TODO: Define default label derivation logic (e.g., "Create {fieldLabel}..." or "Create new...") -->
- When activated:
  - Close dropdown.
  - Call `config.onCreateAction` if provided, else dispatch `formgen:relationship:create-action`.
  - Include `query` (from `store.searchQuery`) and `actionId` from dataset/field.
- Ensure keyboard navigation can focus the action as the last row without treating it as an option (hybrid model).
  <!-- TODO: Decide if typeahead create action row should be inside listbox (role="option") or outside (role="button" like chips footer). Current recommendation: outside listbox for consistency with chips. -->

### Chips UI
Update `client/src/renderers/chips.ts` to:
- Detect `field.createAction` and render a footer action in the dropdown.
- Use the configured label (or a default derived label).
- When activated:
  - Call `config.onCreateAction` if provided and accept `Option | Option[]` return value.
  - Else dispatch `formgen:relationship:create-action`.
- Apply returned options using `selectBehavior` (append/replace), emit semantic selection updates, and dispatch native `change`.
- Implement one of the “Menu + Query Interaction” models, defaulting to Model 1.
- Support create action in default mode (not search-only).
- Ensure inline create (query-conditional) and create action (footer) can coexist without visual conflict.

### Documentation
- Update `client/README.md` to document:
  - New attributes
  - Hook signature (if used)
  - Event name + payload
  - Example integration snippet (vanilla)

### Tests
Add tests in `client/tests/runtime.test.ts` to assert:
- Create action row renders when enabled.
- Create action row renders even when options are present.
- Create action renders in non-search mode.
- Clicking it dispatches `formgen:relationship:create-action` with expected detail.
- `onCreateAction` suppresses the DOM event.
- Selecting existing options remains unaffected.
- Keyboard navigation can reach the create action row.

### Sandbox (Optional)
Demonstrate the create-action event in `client/dev/vanilla.ts`:
- Attach a listener for `formgen:relationship:create-action`.
- In the handler, open a simple sandbox modal/prompt, then inject/select a fake created option.

## Acceptance Criteria

- A has-one typeahead field with `data-endpoint-create-action="true"` shows a “Create …” action in its dropdown.
- A multi-select chips field with `data-endpoint-create-action="true"` shows a “Create …” action in its dropdown footer.
- Activating the action triggers either:
  - `onCreateAction(...)` hook (if provided), or
  - `formgen:relationship:create-action` event with correct payload.
- The action does not break search, option selection, or internal relationship event wiring.
- Host can programmatically select the newly created record and the typeahead input reflects it correctly.
- For multi-select, the action supports returning multiple created items and they are applied according to the configured selection behavior (append default).
- Create action and inline create may both be enabled without breaking UX.

## Decisions

1) Create action renders even when matches exist (typeahead row, chips footer).
2) Create action is available in default mode (not gated to search mode).
3) `onCreateAction` takes precedence; DOM event fires only when the hook is not provided.
4) Typeahead accepts a single `Option` return; chips can accept `Option[]`.
5) Create action is a focusable control (`role="button"`) and included in arrow-key navigation as the last row.

### Confirmed Decisions (Phase 0.2)

**Create Action + Inline Create Coexistence:**
- Both features may be enabled simultaneously on the same field.
- Inline create (`allowCreate`) renders at the top of the options list when the query doesn't match existing options.
- Create action (`createAction`) renders in the footer (chips) or as a separate row (typeahead).
- No visual or functional conflict: inline create is query-conditional, create action is always visible.

**Hook Precedence:**
- If `onCreateAction` hook is provided in `GlobalConfig`, it is invoked and the DOM event is NOT dispatched.
- If `onCreateAction` is not provided, the `formgen:relationship:create-action` event is dispatched.
- This mirrors the precedence pattern used by `createOption` for inline create.

**Keyboard Semantics:**
- Create action uses the hybrid focus model already implemented in UXI refactor.
- For chips: ArrowDown from last option moves real focus to footer action; ArrowUp returns to last option.
- For typeahead: Create action row is navigable via ArrowDown/ArrowUp like other options but uses `role="button"`.
- Enter on highlighted create action triggers the action (not selection).
- Escape closes dropdown without triggering action.

**Non-Search Availability:**
- Create action renders in both `default` and `search` modes.
- In default mode, `query` in the event payload will be empty string.
- This allows users to trigger "Create Author" even when browsing a pre-populated list.

## Integration Points (Phase 0 Analysis)

This section documents integration points between the create-action feature and the existing relationship runtime.

### FieldConfig Extensions
Location: `client/src/config.ts` (FieldConfig interface)

Existing `allowCreate?: boolean` enables inline creation. New fields needed:
- `createAction?: boolean` – enables the create action row/footer
- `createActionLabel?: string` – custom label override
- `createActionId?: string` – identifier for routing to correct modal/flow
- `createActionSelect?: "append" | "replace"` – selection behavior for returned options

### GlobalConfig Extensions
Location: `client/src/config.ts` (GlobalConfig interface)

Existing `createOption` hook handles inline creation. New hook needed:
- `onCreateAction?: (context, detail) => Option | Option[] | void | Promise<...>` – triggers when user activates create action

### Chips Renderer Integration
Location: `client/src/renderers/chips.ts`

Existing infrastructure:
- `menuFooter` element (line 246-248) – already created and hidden by default
- `footerFocused` state (line 103-104) – hybrid focus model implemented
- Footer keyboard handlers (lines 647-693) – ArrowUp returns to options
- `shouldOfferCreate` / `createAndSelect` – inline create flow (separate from create-action)

Create-action additions:
- Render footer action button when `field.createAction` is true
- Wire activation to `onCreateAction` hook or dispatch `formgen:relationship:create-action` event
- Support returned `Option | Option[]` with append/replace behavior

### Typeahead Renderer Integration
Location: `client/src/renderers/typeahead.ts`

Existing infrastructure:
- `dropdown` element (line 144-147) – options container
- Inline create row at top when no matches (lines 577-597)
- `shouldOfferCreate` / `createAndSelect` – inline create flow

Create-action additions:
- Add create-action row to dropdown (outside listbox or as last row with special handling)
- Wire activation to `onCreateAction` hook or dispatch event
- Restrict returned value to single `Option` (typeahead is single-select)

### Event Contract
Location: `client/src/relationship-events.ts`

Existing events:
- `formgen:relationship:update` with `kind: "options" | "selection" | "search"`

New event needed:
- `formgen:relationship:create-action` with payload:
  - `element`, `field`, `endpoint`, `query`, `actionId`, `mode`, `selectBehavior`

### Dataset Parsing
Location: `client/src/index.ts` (datasetToFieldConfig function)

Currently parses:
- `data-endpoint-allow-create` → `field.allowCreate`

New attributes to parse:
- `data-endpoint-create-action` → `field.createAction`
- `data-endpoint-create-action-label` → `field.createActionLabel`
- `data-endpoint-create-action-id` → `field.createActionId`
- `data-endpoint-create-action-select` → `field.createActionSelect`

### Theme Classes
Location: `client/src/theme/classes.ts`

Already present (from UXI refactor):
- `menuFooter` – pinned footer container
- `menuFooterAction` – footer button styling
- `menuFooterActionFocused` – focused state styling

No new theme classes required for create-action feature.

### UXI Coordination (Phase 0.4)
The UXI refactor (UXI_TSK.md) completed:
- Menu structure with pinned search header + scrollable list + pinned footer
- Hybrid focus model (aria-activedescendant for options, real focus for footer)
- Footer keyboard navigation (ArrowDown from last option → footer, ArrowUp → options)

Create-action leverages this existing infrastructure without requiring structural changes.

**Coordination Notes:**
- `menuFooter` element exists but is hidden by default (`hidden = true` in chips.ts:248)
- Create-action will unhide the footer when `field.createAction` is true
- Theme classes `menuFooterAction` and `menuFooterActionFocused` are already defined
- Keyboard handlers in `menuFooter.addEventListener("keydown", ...)` (chips.ts:647-693) handle ArrowUp/ArrowDown/Escape
- No changes to UXI implementation required; create-action adds content to existing footer structure
