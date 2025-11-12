import type { Option, RendererContext } from "../config";
import type { ResolverRegistry } from "../registry";
import {
  syncSelectOptions,
  derivePlaceholder,
  deriveSearchPlaceholder,
  getSelectedValues,
  buildHighlightedFragment,
} from "./relationship-utils";
import { registerRendererCleanup } from "./relationship-cleanup";
import {
  addElementClasses,
  classesToString,
  combineClasses,
  getThemeClasses,
  removeElementClasses,
  setElementClasses,
  type ChipsClassMap,
} from "../theme/classes";
import { createIconElement, readIconConfig, type IconConfig } from "./icons";

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
  theme: ChipsClassMap;
  icon: IconConfig | null;
  iconElement: HTMLElement | null;
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
  const selected = syncSelectOptions({
    select: store.select,
    options: store.options,
    placeholder: store.placeholder,
  });
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

  const selectedValues = syncSelectOptions({
    select: store.select,
    options,
    placeholder: store.placeholder,
  });
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

  const theme = getThemeClasses().chips;

  const container = document.createElement("div");
  setElementClasses(container, theme.container);
  container.style.width = "100%";
  container.setAttribute(CHIP_ROOT_ATTR, "true");
  container.hidden = true;

  const chips = document.createElement("div");
  setElementClasses(chips, theme.chips);
  const chipsContent = document.createElement("div");
  setElementClasses(chipsContent, theme.chipsContent);
  chips.appendChild(chipsContent);

  const iconConfig = readIconConfig(select);
  let renderedIcon: HTMLElement | null = null;
  if (iconConfig) {
    renderedIcon = createIconElement(iconConfig, {
      wrapperClasses: theme.icon,
    });
    if (renderedIcon) {
      chips.insertBefore(renderedIcon, chipsContent);
    }
  }

  const actions = document.createElement("div");
  setElementClasses(actions, theme.actions);

  const clear = document.createElement("button");
  clear.type = "button";
  setElementClasses(clear, combineClasses(theme.action, theme.actionClear));
  clear.setAttribute("aria-label", "Clear selection");
  clear.innerHTML = '<span aria-hidden="true">&times;</span>';

  const toggle = document.createElement("button");
  toggle.type = "button";
  setElementClasses(toggle, combineClasses(theme.action, theme.actionToggle));
  toggle.setAttribute("aria-haspopup", "listbox");
  toggle.setAttribute("aria-expanded", "false");
  toggle.innerHTML = '<svg class="shrink-0 size-3.5 text-gray-500" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/></svg>';

  actions.append(clear, toggle);

  const inner = document.createElement("div");
  setElementClasses(inner, theme.inner);
  inner.append(chips, actions);

  const menu = document.createElement("div");
  setElementClasses(menu, theme.menu);
  menu.hidden = true;

  container.append(inner, menu);

  select.insertAdjacentElement("beforebegin", container);
  addElementClasses(select, theme.nativeSelect);

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
    theme,
    icon: iconConfig,
    iconElement: renderedIcon,
  };

  if (searchMode) {
    const searchContainer = document.createElement("div");
    setElementClasses(searchContainer, theme.search);

    const searchInput = document.createElement("input");
    searchInput.type = "search";
    setElementClasses(searchInput, theme.searchInput);
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
      // Only open menu if there are options available or if user has typed something
      const hasQuery = store.searchValue.trim().length > 0;
      const hasOptions = store.options.length > 0;
      if (hasQuery || hasOptions) {
        toggleMenu(store, true);
      }
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
      addElementClasses(container, theme.containerReady);
    });
  } else {
    container.hidden = false;
    addElementClasses(container, theme.containerReady);
  }

  return store;
}

function renderChips(store: ChipStore, selectedValues: Set<string>): void {
  const { chipsContent, chips, select, placeholder, searchInput, theme } = store;

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
    setElementClasses(placeholderNode, theme.placeholder);
    placeholderNode.textContent = placeholder || "Select an option";
    chipsContent.appendChild(placeholderNode);
  } else {
    for (const option of selectedOptions) {
      const value = option.value;
      const label = option.textContent ?? value;

      const chip = document.createElement("span");
      setElementClasses(chip, theme.chip);
      chip.setAttribute(CHIP_DATA_VALUE, value);

      const text = document.createElement("span");
      setElementClasses(text, theme.chipLabel);
      text.textContent = label;

      const remove = document.createElement("button");
      remove.type = "button";
      setElementClasses(remove, theme.chipRemove);
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
  const { menu, options, searchMode, searchValue, theme } = store;
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
    setElementClasses(empty, theme.menuEmpty);
    empty.textContent = query ? "No matches" : "No more options";
    menu.appendChild(empty);
    return;
  }

  for (const option of available) {
    const button = document.createElement("button");
    button.type = "button";
    setElementClasses(button, theme.menuItem);
    button.setAttribute("role", "option");
    button.dataset.value = option.value;

    // Create label span
    const labelSpan = document.createElement("span");
    const label = option.label ?? option.value;
    labelSpan.appendChild(
      buildHighlightedFragment(label, query, classesToString(theme.searchHighlight))
    );
    button.appendChild(labelSpan);

    // Add checkmark for already selected options
    const isSelected = selectedValues.has(option.value);
    if (isSelected) {
      button.setAttribute("aria-selected", "true");
      const checkmark = document.createElement("span");
      checkmark.innerHTML = '<svg class="shrink-0 size-3.5 text-blue-600" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
      button.appendChild(checkmark);
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
    addElementClasses(store.container, store.theme.containerOpen);
    if (store.searchMode && store.searchInput) {
      store.searchInput.focus();
    }
  } else {
    removeElementClasses(store.container, store.theme.containerOpen);
  }
  store.isOpen = open;
}

function destroyChipStore(store: ChipStore): void {
  document.removeEventListener("click", store.documentHandler);
}

registerRendererCleanup("chips", stores, (_select, store) => {
  destroyChipStore(store as ChipStore);
});
