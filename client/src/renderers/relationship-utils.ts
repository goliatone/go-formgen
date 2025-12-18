import type { Option } from "../config";

export interface SyncOptionsConfig {
  select: HTMLSelectElement;
  options: Option[];
  placeholder?: string;
}

/**
 * Synchronises the native <select> options with the resolver output while
 * preserving existing selections and labels for stale values.
 *
 * Important: this is intentionally side-effect free (no event dispatch). Use
 * `formgen:relationship:update` (`kind: "options"`) to notify renderers/internal
 * listeners after syncing.
 */
export function syncSelectOptions(config: SyncOptionsConfig): Set<string> {
  const { select, options, placeholder } = config;

  const existingLabels = new Map(
    Array.from(select.options)
      .filter((option) => option.value !== "")
      .map((option) => [option.value, option.textContent || option.value])
  );

  const currentSelection = new Set(
    Array.from(select.options)
      .filter((option) => option.selected && option.value !== "")
      .map((option) => option.value)
  );

  select.innerHTML = "";
  if (placeholder) {
    const placeholderOption = document.createElement("option");
    placeholderOption.value = "";
    placeholderOption.textContent = placeholder;
    select.appendChild(placeholderOption);
  }

  const optionByValue = new Map(options.map((option) => [option.value, option]));

  for (const value of currentSelection) {
    if (!optionByValue.has(value)) {
      const label = existingLabels.get(value) || value;
      optionByValue.set(value, { value, label });
    }
  }

  for (const option of optionByValue.values()) {
    const node = document.createElement("option");
    node.value = option.value;
    node.textContent = option.label;
    node.selected = currentSelection.has(option.value);
    select.appendChild(node);
  }

  return getSelectedValues(select);
}

export function derivePlaceholder(select: HTMLSelectElement): string {
  const option = Array.from(select.options).find((item) => item.value === "");
  if (option && option.textContent) {
    return option.textContent;
  }
  if (select.getAttribute("placeholder")) {
    return select.getAttribute("placeholder") ?? "";
  }
  if (select.getAttribute("aria-label")) {
    return select.getAttribute("aria-label") ?? "";
  }
  return "Select an option";
}

export function deriveSearchPlaceholder(
  select: HTMLSelectElement,
  explicit?: string
): string {
  if (explicit) {
    return explicit;
  }
  const override = select.getAttribute("data-endpoint-search-placeholder");
  if (override) {
    return override;
  }
  const label =
    select.getAttribute("data-endpoint-field-label") ??
    select.getAttribute("aria-label") ??
    select.getAttribute("placeholder") ??
    select.getAttribute("name") ??
    select.id;
  if (label) {
    return `Search ${label}`.trim();
  }
  return "Search options";
}

export function getSelectedValues(select: HTMLSelectElement): Set<string> {
  return new Set(
    Array.from(select.options)
      .filter((option) => option.selected && option.value !== "")
      .map((option) => option.value)
  );
}

export function buildHighlightedFragment(
  label: string,
  query: string,
  className: string
): DocumentFragment {
  const fragment = document.createDocumentFragment();
  const lowerLabel = label.toLowerCase();
  const lowerQuery = query.toLowerCase();

  if (!lowerQuery) {
    fragment.append(document.createTextNode(label));
    return fragment;
  }

  let cursor = 0;
  let index = lowerLabel.indexOf(lowerQuery);
  const length = lowerQuery.length;

  while (index !== -1) {
    if (index > cursor) {
      fragment.append(document.createTextNode(label.slice(cursor, index)));
    }

    const match = label.slice(index, index + length);
    const highlight = document.createElement("mark");
    highlight.className = className;
    highlight.textContent = match;
    fragment.append(highlight);

    cursor = index + length;
    index = lowerLabel.indexOf(lowerQuery, cursor);
  }

  if (cursor < label.length) {
    fragment.append(document.createTextNode(label.slice(cursor)));
  }

  return fragment;
}
