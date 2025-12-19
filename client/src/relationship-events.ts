import type { EndpointConfig, FieldConfig } from "./config";

/**
 * Relationship widgets and the resolver communicate via CustomEvents so internal
 * wiring does not depend on native DOM events like `change`.
 *
 * Event model
 * - Resolver lifecycle: `formgen:relationship:loading|success|error|validation`
 * - UI/internal state: `formgen:relationship:update` (this module)
 * - Create action: `formgen:relationship:create-action` (delegated creation)
 *
 * `formgen:relationship:update` is intended to be the single internal contract
 * for syncing:
 * - selection mirrors (hidden inputs / json input)
 * - renderer UI (chips/typeahead) when selection changes externally
 * - resolver-driven option refreshes (so UI can react without `change`)
 *
 * Native `change` should remain reserved for external integration and form
 * semantics. Internally, we bridge `change` -> `formgen:relationship:update`
 * for safety, but we also dedupe so widgets that emit `selection` updates do
 * not cause duplicate internal work when they also dispatch `change`.
 */
export const RELATIONSHIP_UPDATE_EVENT = "formgen:relationship:update" as const;

/**
 * Event dispatched when the user activates the "Create …" action in a
 * relationship widget (typeahead or chips).
 *
 * This event is dispatched ONLY when `GlobalConfig.onCreateAction` is not
 * provided. If the hook is provided, it takes precedence and this event is
 * not emitted.
 *
 * Hosts can listen for this event to:
 * - Open a modal/panel for creating a new related record
 * - Prefill the create form with the current query
 * - Route to a different page for creation
 *
 * After creation, the host should apply the selection using one of:
 * - `registry.resolve(select)` to refresh options and select the new record
 * - Direct DOM manipulation: inject `<option>` + dispatch `change`
 * - Dispatch `formgen:relationship:update` with `kind: "selection"`
 */
export const RELATIONSHIP_CREATE_ACTION_EVENT = "formgen:relationship:create-action" as const;

/**
 * Payload for the `formgen:relationship:create-action` event.
 *
 * Contains all context needed for the host to:
 * - Open the correct modal/panel (via `actionId`)
 * - Prefill the create form (via `query`)
 * - Apply the correct selection behavior after creation (via `selectBehavior`)
 */
export interface RelationshipCreateActionDetail {
  /** The underlying relationship `<select>` element. */
  element: HTMLElement;
  /** Field configuration harvested from DOM attributes. */
  field: FieldConfig;
  /** Endpoint configuration for the relationship. */
  endpoint: EndpointConfig;
  /** Current search query (empty string in default mode). */
  query: string;
  /** Optional identifier for routing to the correct modal/flow. */
  actionId?: string;
  /** Which renderer triggered the action. */
  mode: "typeahead" | "chips";
  /** How returned options should be applied to selection. */
  selectBehavior: "append" | "replace";
}

export type RelationshipUpdateOrigin = "resolver" | "hydrate" | "ui" | "program";

export type RelationshipUpdateDetail =
  | {
      kind: "options";
      origin: Exclude<RelationshipUpdateOrigin, "ui">;
      selectedValues: string[];
      query?: string;
    }
  | {
      kind: "selection";
      origin: RelationshipUpdateOrigin;
      selectedValues: string[];
    }
  | {
      kind: "search";
      origin: "ui";
      query: string;
    };

export function emitRelationshipUpdate(
  element: HTMLElement,
  detail: RelationshipUpdateDetail
): void {
  const selectionKey =
    detail.kind === "selection" ? serializeSelection(detail.selectedValues) : null;
  try {
    element.dispatchEvent(
      new CustomEvent<RelationshipUpdateDetail>(RELATIONSHIP_UPDATE_EVENT, {
        bubbles: true,
        detail,
      })
    );
    if (selectionKey) {
      lastSelectionByElement.set(element, selectionKey);
    }
  } catch (_err) {
    // Ignore dispatch failures (e.g. CustomEvent not supported in some environments).
  }
}

/**
 * Dispatch the create-action event when the user activates the "Create …"
 * action in a relationship widget.
 *
 * This function should only be called when `GlobalConfig.onCreateAction` is
 * NOT provided. The caller (renderer) is responsible for checking hook
 * precedence before invoking this.
 *
 * @param element - The relationship `<select>` element
 * @param detail - The create action context (field, endpoint, query, etc.)
 */
export function emitRelationshipCreateAction(
  element: HTMLElement,
  detail: RelationshipCreateActionDetail
): void {
  try {
    element.dispatchEvent(
      new CustomEvent<RelationshipCreateActionDetail>(RELATIONSHIP_CREATE_ACTION_EVENT, {
        bubbles: true,
        detail,
      })
    );
  } catch (_err) {
    // Ignore dispatch failures (e.g. CustomEvent not supported in some environments).
  }
}

/**
 * Ensure relationship selects always emit semantic selection updates, even if
 * something mutates the native <select> and only fires `change`.
 *
 * This lets internal consumers subscribe to `formgen:relationship:update`
 * exclusively.
 */
export function ensureRelationshipSelectionBridge(
  element: HTMLElement
): void {
  if (!(element instanceof HTMLSelectElement)) {
    return;
  }
  const select = element;
  if (select.dataset.formgenRelationshipSelectionBridge === "true") {
    return;
  }
  select.dataset.formgenRelationshipSelectionBridge = "true";

  select.addEventListener("change", (event) => {
    const selectedValues = readSelectedValues(select);
    const key = serializeSelection(selectedValues);
    if (lastSelectionByElement.get(select) === key) {
      return;
    }
    emitRelationshipUpdate(select, {
      kind: "selection",
      origin: (event as Event).isTrusted ? "ui" : "program",
      selectedValues,
    });
  });
}

const lastSelectionByElement = new WeakMap<HTMLElement, string>();

function readSelectedValues(select: HTMLSelectElement): string[] {
  return Array.from(select.selectedOptions)
    .map((option) => option.value)
    .filter((value) => value !== "");
}

function serializeSelection(values: string[]): string {
  return values
    .slice()
    .sort((left, right) => left.localeCompare(right))
    .join("\u0000");
}
