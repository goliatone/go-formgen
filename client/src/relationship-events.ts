/**
 * Relationship widgets and the resolver communicate via CustomEvents so internal
 * wiring does not depend on native DOM events like `change`.
 *
 * Event model
 * - Resolver lifecycle: `formgen:relationship:loading|success|error|validation`
 * - UI/internal state: `formgen:relationship:update` (this module)
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
