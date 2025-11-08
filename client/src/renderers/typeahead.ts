import type { Option, RendererContext } from "../config";
import type { ResolverRegistry } from "../registry";
import {
  syncSelectOptions,
  derivePlaceholder,
  deriveSearchPlaceholder,
  buildHighlightedFragment,
} from "./relationship-utils";
import { registerRendererCleanup } from "./relationship-cleanup";
import {
  addElementClasses,
  classesToString,
  getThemeClasses,
  removeElementClasses,
  setElementClasses,
  type TypeaheadClassMap,
} from "../theme/classes";

interface TypeaheadStore {
  select: HTMLSelectElement;
  container: HTMLElement;
  control: HTMLElement;
  input: HTMLInputElement;
  clear: HTMLButtonElement;
  dropdown: HTMLElement;
  options: Option[];
  filtered: Option[];
  placeholder: string;
  searchPlaceholder: string;
  label?: string;
  highlightedIndex: number;
  isOpen: boolean;
  searchMode: boolean;
  searchQuery: string;
  documentHandler: (event: MouseEvent) => void;
  theme: TypeaheadClassMap;
}

const TYPEAHEAD_ROOT_ATTR = "data-fg-typeahead-root";
const TYPEAHEAD_OPTION_ATTR = "data-fg-typeahead-option";
const stores = new WeakMap<HTMLSelectElement, TypeaheadStore>();

export function registerTypeaheadRenderer(registry: ResolverRegistry): void {
  registry.registerRenderer("typeahead", typeaheadRenderer);
}

export function bootstrapTypeahead(select: HTMLSelectElement): void {
  if (select.multiple) {
    return;
  }
  const store = ensureStore(select);
  const selected = syncSelectOptions({
    select: store.select,
    options: store.options,
    placeholder: store.placeholder,
  });
  updateInputFromSelection(store, selected);
  renderOptions(store);
}

const typeaheadRenderer = (context: RendererContext): void => {
  const { element, options } = context;
  if (!(element instanceof HTMLSelectElement) || element.multiple) {
    return;
  }

  const store = ensureStore(element);
  store.options = options;

  const selectedValues = syncSelectOptions({
    select: store.select,
    options,
    placeholder: store.placeholder,
  });
  updateInputFromSelection(store, selectedValues);
  renderOptions(store);
};

function ensureStore(select: HTMLSelectElement): TypeaheadStore {
  const existing = stores.get(select);
  if (existing) {
    return existing;
  }

  const theme = getThemeClasses().typeahead;

  const container = document.createElement("div");
  setElementClasses(container, theme.container);
  container.setAttribute(TYPEAHEAD_ROOT_ATTR, "true");
  container.hidden = true;

  const control = document.createElement("div");
  setElementClasses(control, theme.control);
  control.setAttribute("role", "combobox");
  control.setAttribute("aria-haspopup", "listbox");
  control.setAttribute("aria-expanded", "false");

  const input = document.createElement("input");
  input.type = "text";
  setElementClasses(input, theme.input);
  input.autocomplete = "off";
  input.setAttribute("aria-autocomplete", "list");

  const clear = document.createElement("button");
  clear.type = "button";
  setElementClasses(clear, theme.clear);
  clear.setAttribute("aria-label", "Clear selection");
  clear.innerHTML = '<span aria-hidden="true">&times;</span>';
  clear.disabled = true;

  control.append(input, clear);

  const dropdown = document.createElement("div");
  setElementClasses(dropdown, theme.dropdown);
  dropdown.setAttribute("role", "listbox");
  dropdown.hidden = true;

  const dropdownId = select.id ? `${select.id}__typeahead` : `fg-typeahead-${Math.random().toString(36).slice(2)}`;
  dropdown.id = dropdownId;
  control.setAttribute("aria-controls", dropdownId);
  input.setAttribute("aria-controls", dropdownId);

  // Add dropdown arrow indicator
  const arrow = document.createElement("div");
  arrow.className = "absolute top-1/2 end-3 -translate-y-1/2 pointer-events-none";
  arrow.innerHTML = '<svg class="shrink-0 size-3.5 text-gray-500" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/></svg>';

  container.append(control, arrow, dropdown);

  select.insertAdjacentElement("beforebegin", container);
  addElementClasses(select, theme.nativeSelect);

  const placeholder =
    select.dataset.endpointPlaceholder || derivePlaceholder(select);
  const searchPlaceholder = deriveSearchPlaceholder(
    select,
    select.dataset.endpointSearchPlaceholder
  );
  const label =
    select.dataset.endpointFieldLabel ||
    select.getAttribute("aria-label") ||
    select.getAttribute("name") ||
    select.id ||
    undefined;

  input.placeholder = placeholder;
  input.setAttribute("aria-label", label ?? "Related record");

  const store: TypeaheadStore = {
    select,
    container,
    control,
    input,
    clear,
    dropdown,
    options: [],
    filtered: [],
    placeholder,
    searchPlaceholder,
    label,
    highlightedIndex: -1,
    isOpen: false,
    searchMode: select.dataset.endpointMode === "search",
    searchQuery: "",
    documentHandler: () => {},
    theme,
  };

  input.placeholder = store.searchMode ? store.searchPlaceholder : store.placeholder;

  bindEvents(store);

  stores.set(select, store);

  updateClearState(store);

  if (typeof requestAnimationFrame === "function") {
    requestAnimationFrame(() => {
      container.hidden = false;
      addElementClasses(container, theme.containerReady);
    });
  } else {
    container.hidden = false;
    addElementClasses(container, theme.containerReady);
  }

  return store;
}

function bindEvents(store: TypeaheadStore): void {
  const { input, clear, dropdown } = store;

  input.addEventListener("focus", () => {
    openDropdown(store);
    input.placeholder = store.searchMode ? store.searchPlaceholder : store.placeholder;
  });

  input.addEventListener("click", () => {
    openDropdown(store);
  });

  input.addEventListener("input", () => handleInput(store));
  input.addEventListener("keydown", (event) => handleKeydown(store, event));

  clear.addEventListener("click", () => {
    clearSelection(store);
  });

  dropdown.addEventListener("mousedown", (event) => {
    event.preventDefault();
  });

  store.documentHandler = (event: MouseEvent) => {
    if (!store.container.contains(event.target as Node)) {
      closeDropdown(store);
      resetInputPlaceholder(store);
    }
  };

  document.addEventListener("click", store.documentHandler);
}

function updateInputFromSelection(
  store: TypeaheadStore,
  selectedValues: Set<string>
): void {
  const { select, input } = store;
  const hasSelection = selectedValues.size > 0;
  if (document.activeElement === input && store.searchQuery && !hasSelection) {
    return;
  }
  const value = Array.from(selectedValues)[0] ?? "";
  store.highlightedIndex = -1;

  const option = value
    ? Array.from(select.options).find((item) => item.value === value)
    : undefined;
  input.value = option?.textContent ?? "";

  if (!value) {
    resetInputPlaceholder(store);
  }

  updateClearState(store);
}

function handleInput(store: TypeaheadStore): void {
  const { input, select } = store;
  const trimmed = input.value.trim();
  store.highlightedIndex = -1;
  store.searchQuery = trimmed;
  select.setAttribute("data-endpoint-search-value", trimmed);
  renderOptions(store);
  openDropdown(store);
  if (store.searchMode) {
    select.dispatchEvent(new Event("input", { bubbles: true }));
  }
  updateClearState(store);
}

function handleKeydown(store: TypeaheadStore, event: KeyboardEvent): void {
  const actionableKeys = new Set([
    "ArrowDown",
    "ArrowUp",
    "Enter",
    "Escape",
    "Tab",
  ]);
  if (!actionableKeys.has(event.key)) {
    return;
  }

  const { filtered } = store;

  if (event.key === "Escape") {
    closeDropdown(store);
    resetInputPlaceholder(store);
    return;
  }

  if (event.key === "Tab") {
    if (store.isOpen && store.highlightedIndex >= 0) {
      const option = filtered[store.highlightedIndex];
      if (option) {
        event.preventDefault();
        selectOption(store, option);
      }
    }
    closeDropdown(store);
    resetInputPlaceholder(store);
    return;
  }

  event.preventDefault();

  if (event.key === "ArrowDown") {
    if (!store.isOpen) {
      openDropdown(store);
    }
    moveHighlight(store, 1);
    return;
  }
  if (event.key === "ArrowUp") {
    if (!store.isOpen) {
      openDropdown(store);
    }
    moveHighlight(store, -1);
    return;
  }

  if (event.key === "Enter") {
    if (store.highlightedIndex >= 0) {
      const option = filtered[store.highlightedIndex];
      if (option) {
        selectOption(store, option);
      }
    } else if (filtered.length === 1) {
      selectOption(store, filtered[0]);
    }
  }
}

function moveHighlight(store: TypeaheadStore, delta: number): void {
  const { filtered } = store;
  if (filtered.length === 0) {
    store.highlightedIndex = -1;
    return;
  }
  const next =
    store.highlightedIndex === -1
      ? delta > 0
        ? 0
        : filtered.length - 1
      : (store.highlightedIndex + delta + filtered.length) % filtered.length;
  store.highlightedIndex = next;
  updateHighlightedOption(store);
}

function selectOption(store: TypeaheadStore, option: Option): void {
  const { select, input } = store;
  for (const node of Array.from(select.options)) {
    node.selected = node.value === option.value;
  }
  input.value = option.label ?? option.value;
  resetInputPlaceholder(store);
  select.dispatchEvent(new Event("change", { bubbles: true }));
  closeDropdown(store);
  store.highlightedIndex = -1;
  store.searchQuery = "";
  select.setAttribute("data-endpoint-search-value", "");
  updateClearState(store);
}

function clearSelection(store: TypeaheadStore): void {
  const { select, input } = store;
  for (const node of Array.from(select.options)) {
    node.selected = false;
  }
  input.value = "";
  resetInputPlaceholder(store);
  select.dispatchEvent(new Event("change", { bubbles: true }));
  store.highlightedIndex = -1;
  store.searchQuery = "";
  select.setAttribute("data-endpoint-search-value", "");
  renderOptions(store);
  updateClearState(store);
  if (store.searchMode) {
    select.dispatchEvent(new Event("input", { bubbles: true }));
  }
}

function openDropdown(store: TypeaheadStore): void {
  if (store.isOpen) {
    return;
  }
  store.input.placeholder = store.searchMode ? store.searchPlaceholder : store.placeholder;
  renderOptions(store);
  if (store.searchMode && store.filtered.length === 0 && !store.searchQuery) {
    // When search-driven fields have no data yet (tenant/category filters missing),
    // keep the dropdown collapsed so the empty state doesn't cover other inputs.
    store.dropdown.hidden = true;
    store.control.setAttribute("aria-expanded", "false");
    return;
  }
  store.dropdown.hidden = false;
  store.control.setAttribute("aria-expanded", "true");
  addElementClasses(store.container, store.theme.containerOpen);
  store.isOpen = true;
  if (store.highlightedIndex === -1 && store.filtered.length > 0) {
    store.highlightedIndex = 0;
  }
  updateHighlightedOption(store);
}

function closeDropdown(store: TypeaheadStore): void {
  if (!store.isOpen) {
    return;
  }
  store.dropdown.hidden = true;
  store.control.setAttribute("aria-expanded", "false");
  removeElementClasses(store.container, store.theme.containerOpen);
  store.isOpen = false;
  store.highlightedIndex = -1;
  updateHighlightedOption(store);
}

function renderOptions(store: TypeaheadStore): void {
  const { dropdown, select, theme } = store;
  dropdown.innerHTML = "";
  const trimmed = store.searchQuery.trim().toLowerCase();

  let available = store.options;
  if (trimmed) {
    available = available.filter((option) => {
      const label = option.label ?? option.value;
      return (
        label.toLowerCase().includes(trimmed) ||
        option.value.toLowerCase().includes(trimmed)
      );
    });
  }

  store.filtered = available;

  if (available.length === 0) {
    const empty = document.createElement("div");
    setElementClasses(empty, theme.empty);
    empty.textContent = trimmed ? "No matches" : "No options";
    dropdown.appendChild(empty);
    return;
  }

  const selectedValue = Array.from(select.options).find(
    (option) => option.selected
  )?.value;

  available.forEach((option, index) => {
    const button = document.createElement("button");
    button.type = "button";
    setElementClasses(button, theme.option);
    button.setAttribute("role", "option");
    button.dataset.value = option.value;
    button.setAttribute(TYPEAHEAD_OPTION_ATTR, "true");
    button.dataset.selected = option.value === selectedValue ? "true" : "false";

    // Create label span
    const label = document.createElement("span");
    label.appendChild(
      buildHighlightedFragment(
        option.label ?? option.value,
        trimmed,
        classesToString(theme.highlight)
      )
    );
    button.appendChild(label);

    // Add checkmark icon for selected option
    if (option.value === selectedValue) {
      button.setAttribute("aria-selected", "true");
      const checkmark = document.createElement("span");
      checkmark.innerHTML = '<svg class="shrink-0 size-3.5 text-blue-600" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
      button.appendChild(checkmark);
    }

    button.addEventListener("click", () => selectOption(store, option));
    dropdown.appendChild(button);

    if (store.highlightedIndex === -1 && option.value === selectedValue) {
      store.highlightedIndex = index;
    }
  });

  updateHighlightedOption(store);
}

function updateHighlightedOption(store: TypeaheadStore): void {
  const { dropdown, highlightedIndex, theme } = store;
  const options = Array.from(
    dropdown.querySelectorAll<HTMLElement>(`[${TYPEAHEAD_OPTION_ATTR}]`)
  );

  options.forEach((option, index) => {
    const isActive = index === highlightedIndex;
    if (isActive) {
      addElementClasses(option, theme.optionActive);
    } else {
      removeElementClasses(option, theme.optionActive);
    }
    option.setAttribute(
      "aria-selected",
      isActive ? "true" : option.dataset.selected === "true" ? "true" : "false"
    );
    if (isActive && typeof option.scrollIntoView === "function") {
      option.scrollIntoView({ block: "nearest" });
    }
  });
}

function updateClearState(store: TypeaheadStore): void {
  const { input, select, clear } = store;
  const hasInput = input.value.trim() !== "";
  const hasSelection = Array.from(select.options).some(
    (option) => option.selected && option.value !== ""
  );
  clear.disabled = !(hasInput || hasSelection);
}

function resetInputPlaceholder(store: TypeaheadStore): void {
  if (!store.input.value) {
    store.input.placeholder = store.searchMode
      ? store.searchPlaceholder
      : store.placeholder;
  }
}

function destroyTypeaheadStore(store: TypeaheadStore): void {
  document.removeEventListener("click", store.documentHandler);
}

registerRendererCleanup("typeahead", stores, (_select, store) => {
  destroyTypeaheadStore(store as TypeaheadStore);
});
