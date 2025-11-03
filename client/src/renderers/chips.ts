import type { Option, RendererContext } from "../config";
import type { ResolverRegistry } from "../registry";

interface ChipStore {
  select: HTMLSelectElement;
  container: HTMLElement;
  chips: HTMLElement;
  chipsContent: HTMLElement;
  menu: HTMLElement;
  toggle: HTMLButtonElement;
  clear: HTMLButtonElement;
  placeholder: string;
  options: Option[];
  documentHandler: (event: MouseEvent) => void;
  isOpen: boolean;
  searchMode: boolean;
  searchInput?: HTMLInputElement;
  searchValue: string;
}

const CHIP_ROOT_ATTR = "data-fg-chip-root";
const CHIP_DATA_VALUE = "data-fg-chip-value";
const stores = new WeakMap<HTMLSelectElement, ChipStore>();

export function registerChipRenderer(registry: ResolverRegistry): void {
  registry.registerRenderer("chips", chipsRenderer);
}

export function bootstrapChips(element: HTMLSelectElement): void {
  if (!element.multiple) {
    return;
  }
  const store = ensureStore(element);
  const selected = syncNativeOptions(store);
  renderChips(store, selected);
  renderMenu(store, selected);
  updateClearState(store, selected);
}

const chipsRenderer = (context: RendererContext): void => {
  const { element, options } = context;
  if (!(element instanceof HTMLSelectElement) || !element.multiple) {
    return;
  }

  const store = ensureStore(element);
  store.options = options;

  const selectedValues = syncNativeOptions(store);
  renderChips(store, selectedValues);
  renderMenu(store, selectedValues);
  if (!store.isOpen) {
    toggleMenu(store, false);
  }
  updateClearState(store, selectedValues);
};

function ensureStore(select: HTMLSelectElement): ChipStore {
  const existing = stores.get(select);
  if (existing) {
    return existing;
  }

  const container = document.createElement("div");
  container.className = "fg-chip-select";
  container.style.width = "100%";
  container.setAttribute(CHIP_ROOT_ATTR, "true");
  container.hidden = true;

  const chips = document.createElement("div");
  chips.className = "fg-chip-select__chips";
  const chipsContent = document.createElement("div");
  chipsContent.className = "fg-chip-select__chips-content";
  chips.appendChild(chipsContent);

  const actions = document.createElement("div");
  actions.className = "fg-chip-select__actions";

  const clear = document.createElement("button");
  clear.type = "button";
  clear.className = "fg-chip-select__action fg-chip-select__action--clear";
  clear.setAttribute("aria-label", "Clear selection");
  clear.innerHTML = '<span aria-hidden="true">&times;</span>';

  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = "fg-chip-select__action fg-chip-select__action--toggle";
  toggle.setAttribute("aria-haspopup", "listbox");
  toggle.setAttribute("aria-expanded", "false");
  toggle.innerHTML = '<span aria-hidden="true">&#x2304;</span>';

  actions.append(clear, toggle);

  const inner = document.createElement("div");
  inner.className = "fg-chip-select__inner";
  inner.append(chips, actions);

  const menu = document.createElement("div");
  menu.className = "fg-chip-select__menu";
  menu.hidden = true;

  container.append(inner, menu);

  select.insertAdjacentElement("beforebegin", container);
  select.classList.add("fg-chip-select__native");

  const placeholder = derivePlaceholder(select);
  const searchMode = select.dataset.endpointMode === "search";
  const store: ChipStore = {
    select,
    container,
    chips,
    chipsContent,
    menu,
    toggle,
    clear,
    placeholder,
    options: [],
    documentHandler: () => {},
    isOpen: false,
    searchMode,
    searchValue: "",
  };

  if (searchMode) {
    const searchContainer = document.createElement("div");
    searchContainer.className = "fg-chip-select__search";

    const searchInput = document.createElement("input");
    searchInput.type = "search";
    searchInput.className = "fg-chip-select__search-input";
    searchInput.setAttribute("placeholder", deriveSearchPlaceholder(select));
    searchInput.setAttribute("autocomplete", "off");
    searchInput.setAttribute("aria-label", "Search options");

    searchContainer.appendChild(searchInput);
    chips.appendChild(searchContainer);

    store.searchInput = searchInput;
    store.searchValue = "";
    select.setAttribute("data-endpoint-search-value", "");

    const propagateSearch = () => {
      const trimmed = searchInput.value.trim();
      store.searchValue = trimmed;
      select.setAttribute("data-endpoint-search-value", trimmed);
      const selectedValues = getSelectedValues(select);
      renderMenu(store, selectedValues);
      select.dispatchEvent(new Event("input", { bubbles: true }));
    };

    searchInput.addEventListener("input", () => propagateSearch());
    searchInput.addEventListener("focus", () => {
      toggleMenu(store, true);
    });

    chips.addEventListener("click", () => {
      searchInput.focus();
    });
  }

  store.documentHandler = (event: MouseEvent) => {
    if (!container.contains(event.target as Node)) {
      toggleMenu(store, false);
    }
  };

  clear.addEventListener("click", () => {
    updateSelected(store, new Set());
  });
  toggle.addEventListener("click", (event) => {
    event.preventDefault();
    toggleMenu(store, menu.hidden);
  });

  document.addEventListener("click", store.documentHandler);

  stores.set(select, store);
  if (typeof requestAnimationFrame === "function") {
    requestAnimationFrame(() => {
      container.hidden = false;
      container.classList.add("fg-chip-select--ready");
    });
  } else {
    container.hidden = false;
    container.classList.add("fg-chip-select--ready");
  }

  return store;
}

function syncNativeOptions(store: ChipStore): Set<string> {
  const { select, options } = store;

  // Preserve existing labels from pre-rendered options
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
  if (store.placeholder) {
    const placeholder = document.createElement("option");
    placeholder.value = "";
    placeholder.textContent = store.placeholder;
    select.appendChild(placeholder);
  }

  const optionByValue = new Map(options.map((option) => [option.value, option]));
  for (const value of currentSelection) {
    if (!optionByValue.has(value)) {
      // Use preserved label instead of value as fallback
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

  select.dispatchEvent(new Event("change", { bubbles: true }));
  return getSelectedValues(select);
}

function renderChips(store: ChipStore, selectedValues: Set<string>): void {
  const { chipsContent, chips, select, placeholder, searchInput } = store;

  // Don't re-render chips if user is actively typing in search input
  if (searchInput && document.activeElement === searchInput) {
    return;
  }

  chipsContent.innerHTML = "";

  const selectedOptions = Array.from(select.options).filter(
    (option) => option.selected && option.value !== ""
  );

  if (selectedOptions.length === 0) {
    const placeholderNode = document.createElement("span");
    placeholderNode.className = "fg-chip-select__placeholder";
    placeholderNode.textContent = placeholder || "Select an option";
    chipsContent.appendChild(placeholderNode);
  } else {
    for (const option of selectedOptions) {
      const value = option.value;
      const label = option.textContent ?? value;

      const chip = document.createElement("span");
      chip.className = "fg-chip-select__chip";
      chip.setAttribute(CHIP_DATA_VALUE, value);

      const text = document.createElement("span");
      text.className = "fg-chip-select__chip-label";
      text.textContent = label;

      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "fg-chip-select__chip-remove";
      remove.setAttribute("aria-label", `Remove ${label}`);
      remove.innerHTML = "&times;";
      remove.addEventListener("click", () => {
        const updated = new Set(selectedValues);
        updated.delete(value);
        updateSelected(store, updated);
      });

      chip.append(text, remove);
      chipsContent.appendChild(chip);
    }
  }

  // Only append search input if it's not already in the correct position
  if (store.searchMode && searchInput && searchInput.parentElement) {
    const searchContainer = searchInput.parentElement;
    if (searchContainer.parentElement !== chips) {
      chips.appendChild(searchContainer);
    }
  }
}

function renderMenu(store: ChipStore, selectedValues: Set<string>): void {
  const { menu, options, searchMode, searchValue } = store;
  menu.innerHTML = "";

  menu.setAttribute("role", "listbox");

  const query = searchMode ? searchValue.trim().toLowerCase() : "";
  const available = options
    .filter((option) => !selectedValues.has(option.value))
    .filter((option) => {
      if (!query) {
        return true;
      }
      const label = option.label ?? option.value;
      return (
        label.toLowerCase().includes(query) ||
        option.value.toLowerCase().includes(query)
      );
    });

  if (available.length === 0) {
    const empty = document.createElement("div");
    empty.className = "fg-chip-select__menu-empty";
    empty.textContent = query ? "No matches" : "No more options";
    menu.appendChild(empty);
    return;
  }

  for (const option of available) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "fg-chip-select__menu-item";
    button.setAttribute("role", "option");
    button.dataset.value = option.value;
    const label = option.label ?? option.value;
    if (query) {
      button.appendChild(buildHighlightedLabel(label, query));
    } else {
      button.textContent = label;
    }
    button.addEventListener("click", () => {
      const updated = new Set(selectedValues);
      updated.add(option.value);
      updateSelected(store, updated);
      toggleMenu(store, false);
    });
    menu.appendChild(button);
  }
}

function updateSelected(store: ChipStore, values: Set<string>): void {
  const { select } = store;
  for (const option of Array.from(select.options)) {
    option.selected = values.has(option.value);
  }
  select.dispatchEvent(new Event("change", { bubbles: true }));
  const selectedValues = getSelectedValues(select);
  renderChips(store, selectedValues);

  if (store.searchMode) {
    if (store.searchInput) {
      store.searchInput.value = "";
    }
    store.searchValue = "";
    select.setAttribute("data-endpoint-search-value", "");
  }

  renderMenu(store, selectedValues);
  updateClearState(store, selectedValues);

  if (store.searchMode) {
    select.dispatchEvent(new Event("input", { bubbles: true }));
  }
}

function updateClearState(store: ChipStore, selectedValues: Set<string>): void {
  store.clear.disabled = selectedValues.size === 0;
}

function toggleMenu(store: ChipStore, open: boolean): void {
  store.menu.hidden = !open;
  store.toggle.setAttribute("aria-expanded", open ? "true" : "false");
  if (open) {
    store.container.classList.add("fg-chip-select--open");
    if (store.searchMode && store.searchInput) {
      store.searchInput.focus();
    }
  } else {
    store.container.classList.remove("fg-chip-select--open");
  }
  store.isOpen = open;
}

function derivePlaceholder(select: HTMLSelectElement): string {
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

function deriveSearchPlaceholder(select: HTMLSelectElement): string {
  const explicit = select.getAttribute("data-endpoint-search-placeholder");
  if (explicit) {
    return explicit;
  }
  const label =
    select.getAttribute("aria-label") ??
    select.getAttribute("placeholder") ??
    select.getAttribute("name") ??
    select.id;
  if (label) {
    return `Search ${label}`.trim();
  }
  return "Search options";
}

function getSelectedValues(select: HTMLSelectElement): Set<string> {
  return new Set(
    Array.from(select.options)
      .filter((option) => option.selected && option.value !== "")
      .map((option) => option.value)
  );
}

function buildHighlightedLabel(label: string, query: string): DocumentFragment {
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
    highlight.className = "fg-chip-select__search-highlight";
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

// Ensure cleanup when elements are removed from the DOM.
const observer = new MutationObserver((mutations) => {
  for (const mutation of mutations) {
    mutation.removedNodes.forEach((node) => {
      if (!(node instanceof HTMLElement)) {
        return;
      }
      const select = node.matches("select[data-endpoint-renderer='chips']")
        ? (node as HTMLSelectElement)
        : (node.querySelector("select[data-endpoint-renderer='chips']") as HTMLSelectElement | null);
      if (select && stores.has(select)) {
        const store = stores.get(select);
        if (store) {
          document.removeEventListener("click", store.documentHandler);
          stores.delete(select);
        }
      }
    });
  }
});

if (typeof window !== "undefined" && typeof document !== "undefined") {
  observer.observe(document.documentElement, { childList: true, subtree: true });
}
