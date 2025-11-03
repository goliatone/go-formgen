import type { Option, RendererContext } from "../config";
import type { ResolverRegistry } from "../registry";

interface ChipStore {
  select: HTMLSelectElement;
  container: HTMLElement;
  chips: HTMLElement;
  menu: HTMLElement;
  toggle: HTMLButtonElement;
  clear: HTMLButtonElement;
  placeholder: string;
  options: Option[];
  documentHandler: (event: MouseEvent) => void;
  isOpen: boolean;
}

const CHIP_ROOT_ATTR = "data-fg-chip-root";
const CHIP_DATA_VALUE = "data-fg-chip-value";
const stores = new WeakMap<HTMLSelectElement, ChipStore>();

export function registerChipRenderer(registry: ResolverRegistry): void {
  registry.registerRenderer("chips", chipsRenderer);
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
  const store: ChipStore = {
    select,
    container,
    chips,
    menu,
    toggle,
    clear,
    placeholder,
    options: [],
    documentHandler: () => {},
    isOpen: false,
  };

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
      optionByValue.set(value, { value, label: value });
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
  return new Set(
    Array.from(select.options)
      .filter((option) => option.selected && option.value !== "")
      .map((option) => option.value)
  );
}

function renderChips(store: ChipStore, selectedValues: Set<string>): void {
  const { chips, select, placeholder } = store;
  chips.innerHTML = "";

  const selectedOptions = Array.from(select.options).filter(
    (option) => option.selected && option.value !== ""
  );

  if (selectedOptions.length === 0) {
    const placeholderNode = document.createElement("span");
    placeholderNode.className = "fg-chip-select__placeholder";
    placeholderNode.textContent = placeholder || "Select an option";
    chips.appendChild(placeholderNode);
    return;
  }

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
    chips.appendChild(chip);
  }
}

function renderMenu(store: ChipStore, selectedValues: Set<string>): void {
  const { menu, options } = store;
  menu.innerHTML = "";

  menu.setAttribute("role", "listbox");

  const available = options.filter((option) => !selectedValues.has(option.value));
  if (available.length === 0) {
    const empty = document.createElement("div");
    empty.className = "fg-chip-select__menu-empty";
    empty.textContent = "No more options";
    menu.appendChild(empty);
    return;
  }

  for (const option of available) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "fg-chip-select__menu-item";
    button.setAttribute("role", "option");
    button.dataset.value = option.value;
    button.textContent = option.label;
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
  const selectedValues = new Set(
    Array.from(select.options)
      .filter((option) => option.selected && option.value !== "")
      .map((option) => option.value)
  );
  renderChips(store, selectedValues);
  renderMenu(store, selectedValues);
  updateClearState(store, selectedValues);
}

function updateClearState(store: ChipStore, selectedValues: Set<string>): void {
  store.clear.disabled = selectedValues.size === 0;
}

function toggleMenu(store: ChipStore, open: boolean): void {
  store.menu.hidden = !open;
  store.toggle.setAttribute("aria-expanded", open ? "true" : "false");
  if (open) {
    store.container.classList.add("fg-chip-select--open");
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
