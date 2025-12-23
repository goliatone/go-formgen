import type { Option, RendererContext, FieldConfig, EndpointConfig, CreateActionDetail } from "../config";
import { readDataset } from "../dom";
import { datasetToEndpoint, datasetToFieldConfig } from "../relationship-config";
import type { ResolverRegistry } from "../registry";
import {
  RELATIONSHIP_UPDATE_EVENT,
  emitRelationshipUpdate,
  emitRelationshipCreateAction,
  type RelationshipUpdateDetail,
} from "../relationship-events";
import {
  syncSelectOptions,
  derivePlaceholder,
  deriveSearchPlaceholder,
  buildHighlightedFragment,
  getSelectedValues,
} from "./relationship-utils";
import { registerRendererCleanup } from "./relationship-cleanup";
import {
  addElementClasses,
  classesToString,
  combineClasses,
  getThemeClasses,
  removeElementClasses,
  setElementClasses,
  type TypeaheadClassMap,
} from "../theme/classes";
import { createIconElement, readIconConfig, type IconConfig } from "./icons";

interface TypeaheadStore {
  select: HTMLSelectElement;
  container: HTMLElement;
  control: HTMLElement;
  input: HTMLInputElement;
  actions: HTMLElement;
  clear: HTMLButtonElement;
  toggle: HTMLButtonElement;
  dropdown: HTMLElement;
  dropdownList: HTMLElement;
  options: Option[];
  filtered: Option[];
  placeholder: string;
  searchPlaceholder: string;
  allowCreate: boolean;
  createLabel: (query: string) => string;
  createOption?: (query: string) => Promise<Option | undefined>;
  label?: string;
  highlightedIndex: number;
  isOpen: boolean;
  searchMode: boolean;
  searchQuery: string;
  documentHandler: (event: MouseEvent) => void;
  theme: TypeaheadClassMap;
  icon: IconConfig | null;
  iconElement: HTMLElement | null;
  validationHandler?: (event: Event) => void;
  validationObserver?: MutationObserver;
  updateHandler?: (event: Event) => void;
  // Loading state
  loading: boolean;
  loadingHandler?: (event: Event) => void;
  successHandler?: (event: Event) => void;
  // Create action state
  createActionEnabled: boolean;
  createActionLabel: string;
  createActionId?: string;
  createActionSelect: "append" | "replace";
  createActionFocused: boolean;
  createActionElement: HTMLElement | null;
  // References for hook invocation
  field: FieldConfig;
  endpoint: EndpointConfig;
  registry: ResolverRegistry;
}

const TYPEAHEAD_ROOT_ATTR = "data-fg-typeahead-root";
const TYPEAHEAD_OPTION_ATTR = "data-fg-typeahead-option";
const stores = new WeakMap<HTMLSelectElement, TypeaheadStore>();

export function registerTypeaheadRenderer(registry: ResolverRegistry): void {
  registry.registerRenderer("typeahead", (context) =>
    typeaheadRenderer(context, registry)
  );
}

export function bootstrapTypeahead(select: HTMLSelectElement, registry: ResolverRegistry): void {
  if (select.multiple) {
    return;
  }
  const store = ensureStore(select);
  ensureCreateIntegration(store, registry);
  const selected = syncSelectOptions({
    select: store.select,
    options: store.options,
    placeholder: store.placeholder,
  });
  emitRelationshipUpdate(store.select, {
    kind: "options",
    origin: "hydrate",
    selectedValues: Array.from(selected),
    query: store.searchQuery,
  });
  updateInputFromSelection(store, selected);
  renderOptions(store);
}

const typeaheadRenderer = (context: RendererContext, registry: ResolverRegistry): void => {
  const { element, options } = context;
  if (!(element instanceof HTMLSelectElement) || element.multiple) {
    return;
  }

  const store = ensureStore(element);
  ensureCreateIntegration(store, registry, context);
  store.options = options;

  const selectedValues = syncSelectOptions({
    select: store.select,
    options,
    placeholder: store.placeholder,
  });
  emitRelationshipUpdate(store.select, {
    kind: "options",
    origin: "resolver",
    selectedValues: Array.from(selectedValues),
    query: store.searchQuery,
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

  // Actions container with clear and toggle buttons (matching chips renderer)
  const actions = document.createElement("div");
  setElementClasses(actions, theme.actions);

  const clear = document.createElement("button");
  clear.type = "button";
  setElementClasses(clear, combineClasses(theme.action, theme.actionClear));
  clear.setAttribute("aria-label", "Clear selection");
  clear.innerHTML = '<span aria-hidden="true">&times;</span>';
  clear.disabled = true;

  const toggle = document.createElement("button");
  toggle.type = "button";
  setElementClasses(toggle, combineClasses(theme.action, theme.actionToggle));
  toggle.setAttribute("aria-haspopup", "listbox");
  toggle.setAttribute("aria-expanded", "false");
  toggle.innerHTML = '<svg class="shrink-0 size-3.5 text-gray-500" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/></svg>';

  actions.append(clear, toggle);
  control.append(input, actions);

  const dropdown = document.createElement("div");
  setElementClasses(dropdown, theme.dropdown);
  dropdown.hidden = true;

  const dropdownId = select.id ? `${select.id}__typeahead` : `fg-typeahead-${Math.random().toString(36).slice(2)}`;
  const dropdownList = document.createElement("div");
  setElementClasses(dropdownList, theme.dropdownList);
  dropdownList.setAttribute("role", "listbox");
  dropdownList.id = dropdownId;
  control.setAttribute("aria-controls", dropdownId);
  input.setAttribute("aria-controls", dropdownId);

  dropdown.appendChild(dropdownList);
  container.append(control, dropdown);

  select.insertAdjacentElement("beforebegin", container);
  addElementClasses(select, theme.nativeSelect);
  handleRequiredAttribute(select, control, input);

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

  const iconConfig = readIconConfig(select);

  // Derive create action label from field label
  const createActionLabelFromAttr = select.dataset.endpointCreateActionLabel;
  const defaultCreateActionLabel = label ? `Create ${label}…` : "Create new…";

  const store: TypeaheadStore = {
    select,
    container,
    control,
    input,
    actions,
    clear,
    toggle,
    dropdown,
    dropdownList,
    options: [],
    filtered: [],
    placeholder,
    searchPlaceholder,
    allowCreate: select.dataset.endpointAllowCreate === "true",
    createLabel: (query) => `Create "${query}"`,
    createOption: undefined,
    label,
    highlightedIndex: -1,
    isOpen: false,
    searchMode: select.dataset.endpointMode === "search",
    searchQuery: "",
    documentHandler: () => {},
    theme,
    icon: iconConfig,
    iconElement: null,
    // Loading state
    loading: false,
    // Create action state
    createActionEnabled: select.dataset.endpointCreateAction === "true",
    createActionLabel: createActionLabelFromAttr || defaultCreateActionLabel,
    createActionId: select.dataset.endpointCreateActionId,
    createActionSelect: (select.dataset.endpointCreateActionSelect === "append" ? "append" : "replace"),
    createActionFocused: false,
    createActionElement: null,
    // References (will be set in ensureCreateIntegration)
    field: {} as FieldConfig,
    endpoint: {} as EndpointConfig,
    registry: null as unknown as ResolverRegistry,
  };

  input.placeholder = store.searchMode ? store.searchPlaceholder : store.placeholder;

  const renderedIcon = createIconElement(iconConfig, {
    wrapperClasses: theme.icon,
    svgClasses: theme.iconSvg,
  });
  if (renderedIcon) {
    control.insertBefore(renderedIcon, input);
    store.iconElement = renderedIcon;
    addElementClasses(input, theme.inputWithIcon);
  }

  bindEvents(store);

  bindValidationState(store);
  bindLoadingState(store);
  bindSelectionListener(store);
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
  const { input, clear, toggle, dropdown } = store;

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

  // Toggle button opens/closes dropdown
  toggle.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    if (store.isOpen) {
      closeDropdown(store);
      resetInputPlaceholder(store);
    } else {
      openDropdown(store);
      input.focus();
    }
  });

  // Keyboard navigation on toggle button
  toggle.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (store.isOpen) {
        closeDropdown(store);
        resetInputPlaceholder(store);
      } else {
        openDropdown(store);
        input.focus();
      }
      return;
    }

    if (event.key === "Escape" && store.isOpen) {
      event.preventDefault();
      closeDropdown(store);
      resetInputPlaceholder(store);
      return;
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!store.isOpen) {
        openDropdown(store);
      }
      input.focus();
      return;
    }
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
  if (document.activeElement === input && store.searchQuery) {
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
  store.createActionFocused = false;
  store.searchQuery = trimmed;
  select.setAttribute("data-endpoint-search-value", trimmed);
  emitRelationshipUpdate(select, { kind: "search", origin: "ui", query: trimmed });
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

  if (event.key === "Enter") {
    if (store.highlightedIndex >= 0) {
      const option = filtered[store.highlightedIndex];
      if (option) {
        selectOption(store, option);
      }
      return;
    }

    if (filtered.length === 1) {
      selectOption(store, filtered[0]);
      return;
    }

    const query = store.searchQuery.trim();
    const createOption = store.createOption;
    if (
      store.searchMode &&
      store.allowCreate &&
      createOption &&
      shouldOfferCreate(store, query)
    ) {
      createAndSelect(store, query).catch(() => undefined);
    }
    return;
  }

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

}

function ensureCreateIntegration(
  store: TypeaheadStore,
  registry: ResolverRegistry,
  context?: RendererContext
): void {
  const allowCreate =
    context?.field.allowCreate ?? store.select.dataset.endpointAllowCreate === "true";
  store.allowCreate = allowCreate;
  store.createOption = async (query: string) => {
    return registry.create(store.select, query);
  };

  const dataset = readDataset(store.select);
  const fallbackField = datasetToFieldConfig(store.select, dataset);
  const fallbackEndpoint = datasetToEndpoint(dataset);

  // Store registry and context references for create action
  store.registry = registry;
  store.field = context?.field ?? fallbackField;
  store.endpoint = fallbackEndpoint;

  if (context) {
    // Update create action config from context if available
    if (context.field.createAction !== undefined) {
      store.createActionEnabled = context.field.createAction;
    }
    if (context.field.createActionLabel) {
      store.createActionLabel = context.field.createActionLabel;
    }
    if (context.field.createActionId) {
      store.createActionId = context.field.createActionId;
    }
    if (context.field.createActionSelect) {
      store.createActionSelect = context.field.createActionSelect;
    }
  }
}

function shouldOfferCreate(store: TypeaheadStore, query: string): boolean {
  const trimmed = query.trim();
  if (!trimmed) {
    return false;
  }
  if (!store.allowCreate || !store.createOption) {
    return false;
  }
  const lower = trimmed.toLowerCase();
  if (
    store.options.some((option) => option.value.toLowerCase() === lower)
  ) {
    return false;
  }
  if (
    store.options.some((option) => (option.label ?? option.value).toLowerCase() === lower)
  ) {
    return false;
  }
  return true;
}

async function createAndSelect(store: TypeaheadStore, query: string): Promise<void> {
  const create = store.createOption;
  if (!create) {
    return;
  }
  const trimmed = query.trim();
  if (!trimmed) {
    return;
  }

  const dropdown = store.dropdown;
  const createButton = dropdown.querySelector<HTMLButtonElement>("[data-fg-create-option='true']");
  if (createButton) {
    createButton.disabled = true;
    createButton.textContent = "Creating…";
  }

  const option = await create(trimmed);
  if (!option) {
    return;
  }

  const existing = Array.from(store.select.options).find(
    (node) => node.value === option.value
  );
  if (!existing) {
    const node = document.createElement("option");
    node.value = option.value;
    node.textContent = option.label ?? option.value;
    store.select.appendChild(node);
  }

  if (!store.options.some((item) => item.value === option.value)) {
    store.options = [...store.options, option];
  }

  selectOption(store, option);
}

function moveHighlight(store: TypeaheadStore, delta: number): void {
  const { filtered, createActionEnabled, createActionElement } = store;
  const hasCreateAction = createActionEnabled && createActionElement;

  // If currently focused on create action
  if (store.createActionFocused) {
    if (delta < 0) {
      // ArrowUp from create action: go to last option or stay if no options
      store.createActionFocused = false;
      if (filtered.length > 0) {
        store.highlightedIndex = filtered.length - 1;
      }
      store.input.focus();
      updateHighlightedOption(store);
    }
    // ArrowDown from create action: stay on create action (it's the last item)
    return;
  }

  // No options case
  if (filtered.length === 0) {
    if (hasCreateAction && delta > 0) {
      // Move to create action
      store.highlightedIndex = -1;
      store.createActionFocused = true;
      createActionElement.focus();
      updateHighlightedOption(store);
    }
    return;
  }

  // Calculate next index
  let next: number;
  if (store.highlightedIndex === -1) {
    next = delta > 0 ? 0 : filtered.length - 1;
  } else {
    next = store.highlightedIndex + delta;
  }

  // Handle boundaries with create action
  if (next >= filtered.length && hasCreateAction && delta > 0) {
    // ArrowDown from last option: move to create action
    store.highlightedIndex = -1;
    store.createActionFocused = true;
    createActionElement.focus();
    updateHighlightedOption(store);
    return;
  }

  // Standard wrapping behavior (without create action or when not at boundary)
  if (next < 0) {
    next = filtered.length - 1;
  } else if (next >= filtered.length) {
    next = 0;
  }

  store.highlightedIndex = next;
  store.createActionFocused = false;
  updateHighlightedOption(store);
}

function selectOption(store: TypeaheadStore, option: Option): void {
  const { select, input } = store;
  for (const node of Array.from(select.options)) {
    node.selected = node.value === option.value;
  }
  input.value = option.label ?? option.value;
  resetInputPlaceholder(store);
  closeDropdown(store);
  store.highlightedIndex = -1;
  store.searchQuery = "";
  select.setAttribute("data-endpoint-search-value", "");
  emitRelationshipUpdate(select, {
    kind: "selection",
    origin: "ui",
    selectedValues: Array.from(getSelectedValues(select)),
  });
  select.dispatchEvent(new Event("change", { bubbles: true }));
  updateClearState(store);
}

function clearSelection(store: TypeaheadStore): void {
  const { select, input } = store;
  for (const node of Array.from(select.options)) {
    node.selected = false;
  }
  input.value = "";
  resetInputPlaceholder(store);
  store.highlightedIndex = -1;
  store.searchQuery = "";
  select.setAttribute("data-endpoint-search-value", "");
  emitRelationshipUpdate(select, {
    kind: "selection",
    origin: "ui",
    selectedValues: [],
  });
  select.dispatchEvent(new Event("change", { bubbles: true }));
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
    store.toggle.setAttribute("aria-expanded", "false");
    return;
  }
  store.dropdown.hidden = false;
  store.control.setAttribute("aria-expanded", "true");
  store.toggle.setAttribute("aria-expanded", "true");
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
  store.toggle.setAttribute("aria-expanded", "false");
  removeElementClasses(store.container, store.theme.containerOpen);
  store.isOpen = false;
  store.highlightedIndex = -1;
  store.createActionFocused = false;
  updateHighlightedOption(store);
}

function renderOptions(store: TypeaheadStore): void {
  const { dropdownList, select, theme } = store;
  dropdownList.innerHTML = "";
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
    // Show loading indicator when fetching and no options yet
    if (store.loading) {
      const loadingRow = document.createElement("div");
      setElementClasses(loadingRow, theme.loading);
      loadingRow.setAttribute("aria-live", "polite");
      loadingRow.setAttribute("role", "status");

      const spinner = document.createElement("span");
      setElementClasses(spinner, theme.loadingSpinner);
      spinner.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>';
      loadingRow.appendChild(spinner);

      const text = document.createElement("span");
      text.textContent = "Loading…";
      loadingRow.appendChild(text);

      dropdownList.appendChild(loadingRow);
      renderCreateAction(store);
      return;
    }

    const rawQuery = store.searchQuery.trim();
    if (store.searchMode && shouldOfferCreate(store, rawQuery)) {
      const create = document.createElement("button");
      create.type = "button";
      setElementClasses(create, theme.option);
      create.setAttribute("role", "option");
      create.setAttribute(TYPEAHEAD_OPTION_ATTR, "true");
      create.setAttribute("data-fg-create-option", "true");
      create.textContent = store.createLabel(rawQuery);
      create.addEventListener("click", () => {
        createAndSelect(store, rawQuery).catch(() => undefined);
      });
      dropdownList.appendChild(create);
    }
    const empty = document.createElement("div");
    setElementClasses(empty, theme.empty);
    empty.textContent = trimmed ? "No matches" : "No options";
    dropdownList.appendChild(empty);
    // Render create action row if enabled (always visible, even with no matches)
    renderCreateAction(store);
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
    dropdownList.appendChild(button);

    if (store.highlightedIndex === -1 && option.value === selectedValue) {
      store.highlightedIndex = index;
    }
  });

  // Render create action row if enabled (always visible, not query-dependent)
  renderCreateAction(store);

  updateHighlightedOption(store);
}

function updateHighlightedOption(store: TypeaheadStore): void {
  const { dropdownList, highlightedIndex, theme } = store;
  const options = Array.from(
    dropdownList.querySelectorAll<HTMLElement>(`[${TYPEAHEAD_OPTION_ATTR}]`)
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

  // Update create action focus state
  updateCreateActionFocus(store);
}

const TYPEAHEAD_CREATE_ACTION_ATTR = "data-fg-typeahead-create-action";

/**
 * Render the create action row in the dropdown footer.
 * The create action is always visible when enabled, regardless of search query or matches.
 */
function renderCreateAction(store: TypeaheadStore): void {
  if (store.createActionElement) {
    store.createActionElement.remove();
    store.createActionElement = null;
  }
  if (!store.createActionEnabled) {
    return;
  }

  const { dropdown, theme } = store;

  const actionButton = document.createElement("button");
  actionButton.type = "button";
  setElementClasses(actionButton, theme.createAction);
  actionButton.setAttribute("role", "button");
  actionButton.setAttribute(TYPEAHEAD_CREATE_ACTION_ATTR, "true");
  actionButton.setAttribute("tabindex", "-1");

  // Add plus icon
  const iconSpan = document.createElement("span");
  iconSpan.innerHTML = '<svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>';
  actionButton.appendChild(iconSpan);

  // Add label
  const labelSpan = document.createElement("span");
  labelSpan.textContent = store.createActionLabel;
  actionButton.appendChild(labelSpan);

  actionButton.addEventListener("click", () => {
    triggerCreateAction(store);
  });

  // Handle keyboard events on the action button
  actionButton.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      triggerCreateAction(store);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      // Return focus to options list
      store.createActionFocused = false;
      if (store.filtered.length > 0) {
        store.highlightedIndex = store.filtered.length - 1;
      }
      store.input.focus();
      updateHighlightedOption(store);
    } else if (event.key === "Escape") {
      event.preventDefault();
      closeDropdown(store);
      resetInputPlaceholder(store);
    }
  });

  dropdown.appendChild(actionButton);
  store.createActionElement = actionButton;
}

/**
 * Update the visual focus state of the create action button.
 */
function updateCreateActionFocus(store: TypeaheadStore): void {
  const { createActionElement, createActionFocused, theme } = store;
  if (!createActionElement) {
    return;
  }

  if (createActionFocused) {
    addElementClasses(createActionElement, theme.createActionFocused);
    createActionElement.setAttribute("aria-current", "true");
  } else {
    removeElementClasses(createActionElement, theme.createActionFocused);
    createActionElement.removeAttribute("aria-current");
  }
}

/**
 * Trigger the create action: invoke hook or dispatch event.
 */
async function triggerCreateAction(store: TypeaheadStore): Promise<void> {
  const config = store.registry.getConfig();
  const query = store.searchQuery.trim();

  const detail: CreateActionDetail = {
    query,
    actionId: store.createActionId,
    mode: "typeahead",
    selectBehavior: store.createActionSelect,
  };

  // Close dropdown before triggering action
  closeDropdown(store);
  resetInputPlaceholder(store);

  // Check if hook is provided
  if (config.onCreateAction) {
    try {
      // Build a minimal context for the hook
      const context = {
        element: store.select,
        field: store.field,
        endpoint: store.endpoint,
        request: { url: store.endpoint.url ?? "", init: {} },
        fromCache: false,
        config,
      };
      const result = await config.onCreateAction(context, detail);

      // Apply returned option (typeahead only accepts single option)
      if (result) {
        const option = Array.isArray(result) ? result[0] : result;
        if (option) {
          applyCreatedOption(store, option);
        }
      }
    } catch (_err) {
      // Ignore hook errors
    }
  } else {
    // Dispatch DOM event
    emitRelationshipCreateAction(store.select, {
      element: store.select,
      field: store.field,
      endpoint: store.endpoint,
      query,
      actionId: store.createActionId,
      mode: "typeahead",
      selectBehavior: store.createActionSelect,
    });
  }
}

/**
 * Apply a created option from the create action hook.
 * For typeahead (single-select), this replaces the current selection.
 */
function applyCreatedOption(store: TypeaheadStore, option: Option): void {
  // Add option to the native select if not present
  const existing = Array.from(store.select.options).find(
    (node) => node.value === option.value
  );
  if (!existing) {
    const node = document.createElement("option");
    node.value = option.value;
    node.textContent = option.label ?? option.value;
    store.select.appendChild(node);
  }

  // Add to store options if not present
  if (!store.options.some((item) => item.value === option.value)) {
    store.options = [...store.options, option];
  }

  // Select the option
  selectOption(store, option);
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

function bindValidationState(store: TypeaheadStore): void {
  const syncState = () => {
    const state = store.select.getAttribute("data-validation-state");
    if (state === "invalid") {
      store.container.setAttribute("data-validation-state", "invalid");
      store.control.setAttribute("aria-invalid", "true");
      store.input.setAttribute("aria-invalid", "true");
    } else {
      store.container.removeAttribute("data-validation-state");
      store.control.removeAttribute("aria-invalid");
      store.input.removeAttribute("aria-invalid");
    }
  };
  store.validationHandler = () => syncState();
  store.select.addEventListener(
    "formgen:relationship:validation",
    store.validationHandler as EventListener
  );
  if (typeof MutationObserver !== "undefined") {
    store.validationObserver = new MutationObserver(() => syncState());
    store.validationObserver.observe(store.select, {
      attributes: true,
      attributeFilter: ["data-validation-state"],
    });
  }
  syncState();
}

function bindSelectionListener(store: TypeaheadStore): void {
  // Listen to semantic selection updates (not native `change`).
  const handler = (event: Event) => {
    const detail = (event as CustomEvent<RelationshipUpdateDetail>).detail;
    if (detail.kind !== "selection") {
      return;
    }
    if (detail.origin === "ui") {
      return;
    }
    const selected = getSelectedValues(store.select);
    updateInputFromSelection(store, selected);
    updateClearState(store);
  };
  store.updateHandler = handler;
  store.select.addEventListener(RELATIONSHIP_UPDATE_EVENT, handler as EventListener);
}

function bindLoadingState(store: TypeaheadStore): void {
  const loadingHandler = () => {
    store.loading = true;
    renderOptions(store);
  };
  const successHandler = () => {
    const wasCreateFocused = store.createActionFocused;
    const wasInputFocused = document.activeElement === store.input;
    store.loading = false;
    renderOptions(store);
    if (wasCreateFocused && store.createActionElement) {
      store.createActionElement.focus();
    } else if (wasInputFocused) {
      store.input.focus();
    }
  };
  store.loadingHandler = loadingHandler;
  store.successHandler = successHandler;
  store.select.addEventListener("formgen:relationship:loading", loadingHandler);
  store.select.addEventListener("formgen:relationship:success", successHandler);
  store.select.addEventListener("formgen:relationship:error", successHandler);
}

function destroyTypeaheadStore(store: TypeaheadStore): void {
  document.removeEventListener("click", store.documentHandler);
  if (store.validationHandler) {
    store.select.removeEventListener(
      "formgen:relationship:validation",
      store.validationHandler as EventListener
    );
  }
  store.validationObserver?.disconnect();
  if (store.updateHandler) {
    store.select.removeEventListener(RELATIONSHIP_UPDATE_EVENT, store.updateHandler as EventListener);
  }
  if (store.loadingHandler) {
    store.select.removeEventListener("formgen:relationship:loading", store.loadingHandler);
  }
  if (store.successHandler) {
    store.select.removeEventListener("formgen:relationship:success", store.successHandler);
    store.select.removeEventListener("formgen:relationship:error", store.successHandler);
  }
}

function handleRequiredAttribute(
  select: HTMLSelectElement,
  control: HTMLElement,
  input: HTMLInputElement
): void {
  if (!select.hasAttribute("required")) {
    return;
  }
  select.dataset.validationRequiredNative = "true";
  select.removeAttribute("required");
  control.setAttribute("aria-required", "true");
  input.setAttribute("aria-required", "true");
}

registerRendererCleanup("typeahead", stores, (_select, store) => {
  destroyTypeaheadStore(store as TypeaheadStore);
});
