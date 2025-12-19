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

// Optional Floating UI import - loaded dynamically when needed
type ComputePositionFn = typeof import("@floating-ui/dom").computePosition;
type OffsetFn = typeof import("@floating-ui/dom").offset;
type FlipFn = typeof import("@floating-ui/dom").flip;
type ShiftFn = typeof import("@floating-ui/dom").shift;

interface FloatingUIModule {
  computePosition: ComputePositionFn;
  offset: OffsetFn;
  flip: FlipFn;
  shift: ShiftFn;
}

let floatingUIModule: FloatingUIModule | null = null;
let floatingUILoadPromise: Promise<FloatingUIModule | null> | null = null;

/**
 * Dynamically load Floating UI when needed. Returns null if unavailable.
 * This keeps the module optional and doesn't affect default behavior.
 */
async function loadFloatingUI(): Promise<FloatingUIModule | null> {
  if (floatingUIModule) {
    return floatingUIModule;
  }
  if (floatingUILoadPromise) {
    return floatingUILoadPromise;
  }
  floatingUILoadPromise = (async () => {
    try {
      const mod = await import("@floating-ui/dom");
      floatingUIModule = {
        computePosition: mod.computePosition,
        offset: mod.offset,
        flip: mod.flip,
        shift: mod.shift,
      };
      return floatingUIModule;
    } catch {
      // Floating UI not available - use CSS positioning
      return null;
    }
  })();
  return floatingUILoadPromise;
}

interface ChipStore {
  select: HTMLSelectElement;
  container: HTMLElement;
  inner: HTMLElement;
  chips: HTMLElement;
  chipsContent: HTMLElement;
  menu: HTMLElement;
  menuSearch: HTMLElement | null;
  menuList: HTMLElement;
  menuFooter: HTMLElement;
  toggle: HTMLButtonElement;
  clear: HTMLButtonElement;
  placeholder: string;
  allowCreate: boolean;
  createLabel: (query: string) => string;
  createOption?: (query: string) => Promise<Option | undefined>;
  options: Option[];
  lastRenderedSelectionKey: string;
  highlightedIndex: number;
  creatingQuery: string;
  documentHandler: (event: MouseEvent) => void;
  isOpen: boolean;
  searchMode: boolean;
  searchInput?: HTMLInputElement;
  searchValue: string;
  theme: ChipsClassMap;
  icon: IconConfig | null;
  iconElement: HTMLElement | null;
  validationHandler?: (event: Event) => void;
  validationObserver?: MutationObserver;
  updateHandler?: (event: Event) => void;
  /** When true, footer action has keyboard focus (hybrid model) */
  footerFocused: boolean;
  /** Prevents rapid open/close races during animations */
  animationInProcess: boolean;
  /** When true, use Floating UI for viewport-aware positioning (optional enhancement) */
  useFloatingUI: boolean;
  /** Hidden span for measuring input text width (dynamic input sizing) */
  widthMeasurer?: HTMLSpanElement;
  /** When true, dynamically size input to fit content */
  dynamicInputWidth: boolean;
  /** Unique instance id for stable option ids */
  instanceId: number;
  /** Pending toggle state when an animation lock is active */
  pendingToggle: { open: boolean; restoreFocus: boolean } | null;
  // Create action state
  /** When true, show the create action button in the footer */
  createActionEnabled: boolean;
  /** Label for the create action button */
  createActionLabel: string;
  /** Optional identifier for routing to the correct modal/flow */
  createActionId?: string;
  /** How returned options are applied to selection */
  createActionSelect: "append" | "replace";
  /** Reference to the create action button element */
  createActionElement: HTMLButtonElement | null;
  /** Field configuration for context */
  field: FieldConfig;
  /** Endpoint configuration for context */
  endpoint: EndpointConfig;
  /** Registry reference for hook invocation */
  registry: ResolverRegistry;
}

const CHIP_ROOT_ATTR = "data-fg-chip-root";
const CHIP_DATA_VALUE = "data-fg-chip-value";
const CHIP_MENU_ATTR = "data-fg-chip-menu";
const stores = new WeakMap<HTMLSelectElement, ChipStore>();
let chipInstanceId = 0;

export function registerChipRenderer(registry: ResolverRegistry): void {
  registry.registerRenderer("chips", (context) => chipsRenderer(context, registry));
}

export function bootstrapChips(element: HTMLSelectElement, registry: ResolverRegistry): void {
  if (!element.multiple) {
    return;
  }
  const store = ensureStore(element);
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
    query: store.searchValue,
  });
  renderChips(store, selected);
  renderMenu(store, selected);
  updateClearState(store, selected);
}

const chipsRenderer = (context: RendererContext, registry: ResolverRegistry): void => {
  const { element, options } = context;
  if (!(element instanceof HTMLSelectElement) || !element.multiple) {
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
    query: store.searchValue,
  });
  renderChips(store, selectedValues);
  // Keep the menu stable while a create request is in-flight so the click target
  // does not move due to resolver-driven re-renders.
  if (!store.creatingQuery) {
    renderMenu(store, selectedValues);
  }
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

  // Build menu with pinned search header + scrollable options list + pinned footer
  const menu = document.createElement("div");
  setElementClasses(menu, theme.menu);
  menu.hidden = true;
  menu.setAttribute(CHIP_MENU_ATTR, "true");

  // Menu list (scrollable options area with role="listbox")
  const menuList = document.createElement("div");
  setElementClasses(menuList, theme.menuList);
  menuList.setAttribute("role", "listbox");

  // Menu footer (pinned, for actions like create-action)
  const menuFooter = document.createElement("div");
  setElementClasses(menuFooter, theme.menuFooter);
  menuFooter.hidden = true; // Hidden by default, shown when footer actions exist

  container.append(inner, menu);

  select.insertAdjacentElement("beforebegin", container);
  addElementClasses(select, theme.nativeSelect);
  handleRequiredAttribute(select, container);

  const placeholder = derivePlaceholder(select);
  const searchMode = select.dataset.endpointMode === "search";

  // Build menu search header if in search mode
  let menuSearch: HTMLElement | null = null;
  let searchInput: HTMLInputElement | undefined;

  if (searchMode) {
    // Prevent input blur only on options list in search mode.
    menuList.addEventListener("mousedown", (event) => event.preventDefault());

    menuSearch = document.createElement("div");
    setElementClasses(menuSearch, theme.menuSearch);

    searchInput = document.createElement("input");
    searchInput.type = "text";
    setElementClasses(searchInput, theme.menuSearchInput);
    searchInput.setAttribute("placeholder", deriveSearchPlaceholder(select));
    searchInput.setAttribute("autocomplete", "off");
    searchInput.setAttribute("aria-label", "Search options");

    menuSearch.appendChild(searchInput);
  }

  // Assemble menu structure: search header (if any) + list + footer
  if (menuSearch) {
    menu.appendChild(menuSearch);
  }
  menu.appendChild(menuList);
  menu.appendChild(menuFooter);

  // Optional enhancements - gated by data attributes
  const useFloatingUI = select.dataset.endpointUseFloatingUi === "true";
  const dynamicInputWidth = select.dataset.endpointDynamicInput === "true";

  // Derive create action label from field label
  const label =
    select.dataset.endpointFieldLabel ||
    select.getAttribute("aria-label") ||
    select.getAttribute("name") ||
    select.id ||
    undefined;
  const createActionLabelFromAttr = select.dataset.endpointCreateActionLabel;
  const defaultCreateActionLabel = label ? `Create ${label}…` : "Create new…";

  const store: ChipStore = {
    select,
    container,
    inner,
    chips,
    chipsContent,
    menu,
    menuSearch,
    menuList,
    menuFooter,
    toggle,
    clear,
    placeholder,
    allowCreate: select.dataset.endpointAllowCreate === "true",
    createLabel: (query) => `Create "${query}"`,
    createOption: undefined,
    options: [],
    lastRenderedSelectionKey: "",
    highlightedIndex: -1,
    creatingQuery: "",
    documentHandler: () => {},
    isOpen: false,
    searchMode,
    searchInput,
    searchValue: "",
    theme,
    icon: iconConfig,
    iconElement: renderedIcon,
    footerFocused: false,
    animationInProcess: false,
    useFloatingUI,
    dynamicInputWidth,
    instanceId: chipInstanceId++,
    pendingToggle: null,
    // Create action state
    createActionEnabled: select.dataset.endpointCreateAction === "true",
    createActionLabel: createActionLabelFromAttr || defaultCreateActionLabel,
    createActionId: select.dataset.endpointCreateActionId,
    createActionSelect: (select.dataset.endpointCreateActionSelect === "replace" ? "replace" : "append"),
    createActionElement: null,
    // References (will be set in ensureCreateIntegration)
    field: {} as FieldConfig,
    endpoint: {} as EndpointConfig,
    registry: null as unknown as ResolverRegistry,
  };

  if (searchMode && searchInput) {
    select.setAttribute("data-endpoint-search-value", "");

    const propagateSearch = () => {
      const trimmed = searchInput!.value.trim();
      store.searchValue = trimmed;
      select.setAttribute("data-endpoint-search-value", trimmed);
      emitRelationshipUpdate(select, { kind: "search", origin: "ui", query: trimmed });
      const selectedValues = getSelectedValues(select);
      renderMenu(store, selectedValues);

      // Open the menu when there's a query or options to show
      const hasContent = trimmed.length > 0 || store.options.length > 0;
      if (hasContent && !store.isOpen) {
        toggleMenu(store, true);
      } else if (!hasContent && store.isOpen) {
        toggleMenu(store, false);
      }

      select.dispatchEvent(new Event("input", { bubbles: true }));
    };

    searchInput.addEventListener("input", () => {
      propagateSearch();
      // Update input width if dynamic sizing is enabled (optional enhancement)
      if (store.dynamicInputWidth) {
        updateInputWidth(store);
      }
    });
    searchInput.addEventListener("keydown", (event) => {
      const query = store.searchValue.trim();

      if (event.key === "Tab") {
        // Close dropdown on Tab but allow natural focus traversal
        toggleMenu(store, false);
        return;
      }

      if (event.key === "Escape") {
        toggleMenu(store, false, true);
        return;
      }

      if (event.key === "Home") {
        event.preventDefault();
        focusMenuOption(store, 0);
        return;
      }

      if (event.key === "End") {
        event.preventDefault();
        const options = getMenuOptions(store);
        // If there's a footer action, focus that on End; otherwise last option
        const footerAction = getFooterAction(store);
        if (footerAction) {
          focusFooterAction(store);
        } else if (options.length > 0) {
          focusMenuOption(store, options.length - 1);
        }
        return;
      }

      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        if (!store.isOpen) {
          toggleMenu(store, true);
        }
        event.preventDefault();
        const options = getMenuOptions(store);

        if (event.key === "ArrowDown") {
          // Hybrid model: navigate through options, then to footer
          if (store.highlightedIndex === -1) {
            // Start from first option
            if (options.length > 0) {
              focusMenuOption(store, 0);
            } else {
              // No options, check for footer action
              const footerAction = getFooterAction(store);
              if (footerAction) {
                focusFooterAction(store);
              }
            }
          } else if (store.highlightedIndex >= options.length - 1) {
            // At last option, move to footer if available
            const footerAction = getFooterAction(store);
            if (footerAction) {
              focusFooterAction(store);
            } else {
              // Wrap to first option
              focusMenuOption(store, 0);
            }
          } else {
            // Move to next option
            focusMenuOption(store, store.highlightedIndex + 1);
          }
        } else {
          // ArrowUp: navigate backwards
          if (store.highlightedIndex === -1) {
            // Start from last option
            if (options.length > 0) {
              focusMenuOption(store, options.length - 1);
            }
          } else if (store.highlightedIndex === 0) {
            // At first option, clear highlight (stay in input)
            clearHighlight(store);
          } else {
            // Move to previous option
            focusMenuOption(store, store.highlightedIndex - 1);
          }
        }
        return;
      }

      if (event.key !== "Enter") {
        return;
      }

      // Handle Enter key - select highlighted option or create new
      if (store.highlightedIndex >= 0) {
        event.preventDefault();
        const options = getMenuOptions(store);
        const highlightedOption = options[store.highlightedIndex];
        if (highlightedOption) {
          highlightedOption.click();
        }
        return;
      }

      if (!query) {
        return;
      }

      const selectedValues = getSelectedValues(select);
      const match = findLiteralMatch(select, query);
      event.preventDefault();

      if (match) {
        const updated = new Set(selectedValues);
        updated.add(match.value);
        updateSelected(store, updated);
        toggleMenu(store, false);
        return;
      }

      if (canCreate(store)) {
        toggleMenu(store, true);
        createAndSelect(store, query).catch(() => undefined);
      }
    });
  }

  // Click on chips area opens menu and focuses search input (if search mode)
  chips.addEventListener("click", () => {
    if (!store.isOpen) {
      toggleMenu(store, true);
    }
  });

  // Click on toggle area opens menu and focuses search input (if search mode)
  inner.addEventListener("click", (event) => {
    // Don't interfere with clear button clicks
    if ((event.target as HTMLElement).closest("button")) {
      return;
    }
    if (!store.isOpen) {
      toggleMenu(store, true);
    }
  });

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

  // Keyboard navigation on toggle (for non-search mode)
  toggle.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      toggleMenu(store, !store.isOpen);
      return;
    }

    if (event.key === "Escape" && store.isOpen) {
      event.preventDefault();
      toggleMenu(store, false, true);
      return;
    }

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!store.isOpen) {
        toggleMenu(store, true);
      }
      // In non-search mode, navigate directly to options
      if (!store.searchMode) {
        const options = getMenuOptions(store);
        if (options.length > 0) {
          const startIndex = event.key === "ArrowDown" ? 0 : options.length - 1;
          focusMenuOption(store, startIndex);
        } else {
          // No options, check for footer action
          const footerAction = getFooterAction(store);
          if (footerAction && event.key === "ArrowDown") {
            focusFooterAction(store);
          }
        }
      }
      return;
    }

    if (event.key === "Home" && store.isOpen) {
      event.preventDefault();
      focusMenuOption(store, 0);
      return;
    }

    if (event.key === "End" && store.isOpen) {
      event.preventDefault();
      const footerAction = getFooterAction(store);
      if (footerAction) {
        focusFooterAction(store);
      } else {
        const options = getMenuOptions(store);
        if (options.length > 0) {
          focusMenuOption(store, options.length - 1);
        }
      }
      return;
    }
  });

  document.addEventListener("click", store.documentHandler);

  // Keyboard navigation in menu list (for non-search mode or direct option focus)
  menuList.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      toggleMenu(store, false, true);
      return;
    }

    if (event.key === "Tab") {
      // Close dropdown on Tab but allow natural focus traversal
      toggleMenu(store, false);
      return;
    }

    if (event.key === "Home") {
      event.preventDefault();
      focusMenuOption(store, 0);
      return;
    }

    if (event.key === "End") {
      event.preventDefault();
      // Focus footer action if available, otherwise last option
      const footerAction = getFooterAction(store);
      if (footerAction) {
        focusFooterAction(store);
      } else {
        const options = getMenuOptions(store);
        if (options.length > 0) {
          focusMenuOption(store, options.length - 1);
        }
      }
      return;
    }

    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") {
      return;
    }
    event.preventDefault();
    const options = getMenuOptions(store);
    if (options.length === 0) {
      // No options but might have footer
      if (event.key === "ArrowDown") {
        const footerAction = getFooterAction(store);
        if (footerAction) {
          focusFooterAction(store);
        }
      }
      return;
    }
    const currentIndex = options.findIndex((node) => node === document.activeElement);
    const delta = event.key === "ArrowDown" ? 1 : -1;
    let next: number;
    if (currentIndex === -1) {
      next = delta > 0 ? 0 : options.length - 1;
    } else {
      next = currentIndex + delta;
      // Handle boundaries
      if (next < 0) {
        // Move focus back to search input (or toggle in non-search mode)
        if (store.searchMode && store.searchInput) {
          store.searchInput.focus();
        } else {
          store.toggle.focus();
        }
        clearHighlight(store);
        return;
      }
      if (next >= options.length) {
        // At last option going down - move to footer if available
        const footerAction = getFooterAction(store);
        if (footerAction) {
          focusFooterAction(store);
          return;
        }
        // Wrap to first option if no footer
        next = 0;
      }
    }
    focusMenuOption(store, next);
  });

  // Keyboard navigation on footer (for ArrowUp to return to options)
  menuFooter.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      toggleMenu(store, false, true);
      return;
    }

    if (event.key === "Tab") {
      // Close dropdown on Tab but allow natural focus traversal
      toggleMenu(store, false);
      return;
    }

    if (event.key === "Home") {
      event.preventDefault();
      focusMenuOption(store, 0);
      return;
    }

    if (event.key === "ArrowUp") {
      event.preventDefault();
      const options = getMenuOptions(store);
      if (options.length > 0) {
        // Return to last option
        focusMenuOption(store, options.length - 1);
      } else {
        // No options, return to input/toggle
        if (store.searchMode && store.searchInput) {
          store.searchInput.focus();
        } else {
          store.toggle.focus();
        }
        clearHighlight(store);
      }
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      // Wrap to first option
      const options = getMenuOptions(store);
      if (options.length > 0) {
        focusMenuOption(store, 0);
      }
      return;
    }
  });

  bindValidationState(store);
  bindSelectionListener(store);
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

function ensureCreateIntegration(
  store: ChipStore,
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

function canCreate(store: ChipStore): boolean {
  return store.allowCreate && typeof store.createOption === "function";
}

function getMenuOptions(store: ChipStore): HTMLButtonElement[] {
  return Array.from(
    store.menuList.querySelectorAll<HTMLButtonElement>("[data-fg-chip-option='true']")
  );
}

function clearHighlight(store: ChipStore): void {
  const options = getMenuOptions(store);
  for (const opt of options) {
    removeElementClasses(opt, store.theme.menuItemHighlighted);
  }
  store.highlightedIndex = -1;
  store.footerFocused = false;
  // Clear aria-activedescendant
  if (store.searchInput) {
    store.searchInput.removeAttribute("aria-activedescendant");
  }
  // Remove footer highlight if present
  const footerAction = getFooterAction(store);
  if (footerAction) {
    removeElementClasses(footerAction, store.theme.menuFooterActionFocused);
  }
}

/**
 * Get the footer action button if one exists.
 */
function getFooterAction(store: ChipStore): HTMLButtonElement | null {
  return store.menuFooter.querySelector<HTMLButtonElement>("button");
}

/**
 * Focus the footer action (hybrid model: real focus moves to footer).
 */
function focusFooterAction(store: ChipStore): void {
  const footerAction = getFooterAction(store);
  if (!footerAction) {
    return;
  }

  // Clear option highlights
  const options = getMenuOptions(store);
  for (const opt of options) {
    removeElementClasses(opt, store.theme.menuItemHighlighted);
  }
  store.highlightedIndex = -1;

  // Remove aria-activedescendant from input (footer gets real focus)
  if (store.searchInput) {
    store.searchInput.removeAttribute("aria-activedescendant");
  }

  // Add focus styling and focus the button
  addElementClasses(footerAction, store.theme.menuFooterActionFocused);
  footerAction.focus();
  store.footerFocused = true;
}

/**
 * Focus menu option using hybrid model:
 * - In search mode, focus stays in the input and aria-activedescendant points to the option
 * - In non-search mode, direct focus moves to the option
 */
function focusMenuOption(store: ChipStore, index: number): void {
  const options = getMenuOptions(store);
  if (options.length === 0) {
    store.highlightedIndex = -1;
    return;
  }

  // Clear footer focus state if we're highlighting an option
  store.footerFocused = false;
  const footerAction = getFooterAction(store);
  if (footerAction) {
    removeElementClasses(footerAction, store.theme.menuFooterActionFocused);
  }

  // Remove highlight from all options
  for (const opt of options) {
    removeElementClasses(opt, store.theme.menuItemHighlighted);
  }

  const target = Math.max(0, Math.min(index, options.length - 1));
  store.highlightedIndex = target;

  // Add highlight class
  const targetOption = options[target];
  if (targetOption) {
    addElementClasses(targetOption, store.theme.menuItemHighlighted);
    targetOption.scrollIntoView({ block: "nearest" });

    // Hybrid model: in search mode, keep focus in input and use aria-activedescendant
    if (store.searchMode && store.searchInput) {
      // Ensure the option has an id for aria-activedescendant
      if (!targetOption.id) {
        const baseId = store.select.id
          ? `${store.select.id}-${store.instanceId}`
          : `anon-${store.instanceId}`;
        targetOption.id = `fg-chip-option-${baseId}-${target}`;
      }
      store.searchInput.setAttribute("aria-activedescendant", targetOption.id);
      // Keep focus in input
      if (document.activeElement !== store.searchInput) {
        store.searchInput.focus();
      }
    } else {
      // Non-search mode: direct focus on option
      targetOption.focus();
    }
  }
}

function findLiteralMatch(
  select: HTMLSelectElement,
  query: string
): { value: string; label: string } | null {
  const trimmed = query.trim();
  if (!trimmed) {
    return null;
  }
  const lower = trimmed.toLowerCase();
  for (const option of Array.from(select.options)) {
    if (!option.value) {
      continue;
    }
    const value = option.value;
    const label = option.textContent ?? value;
    if (value.toLowerCase() === lower || label.toLowerCase() === lower) {
      return { value, label };
    }
  }
  return null;
}

function shouldOfferCreate(
  store: ChipStore,
  query: string,
  selectedValues: Set<string>
): boolean {
  const trimmed = query.trim();
  if (!trimmed) {
    return false;
  }
  if (!canCreate(store)) {
    return false;
  }
  const lower = trimmed.toLowerCase();
  const allValues = new Set<string>([
    ...Array.from(selectedValues),
    ...store.options.map((option) => option.value),
  ]);
  if (Array.from(allValues).some((value) => value.toLowerCase() === lower)) {
    return false;
  }
  if (
    store.options.some((option) => (option.label ?? option.value).toLowerCase() === lower)
  ) {
    return false;
  }
  return true;
}

async function createAndSelect(store: ChipStore, query: string): Promise<void> {
  const create = store.createOption;
  if (!create) {
    return;
  }
  const trimmed = query.trim();
  if (!trimmed) {
    return;
  }

  if (store.creatingQuery) {
    return;
  }
  store.creatingQuery = trimmed;
  store.highlightedIndex = -1;
  renderMenu(store, getSelectedValues(store.select));

  try {
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

    // Ensure the created option is available in local filtering immediately.
    if (!store.options.some((item) => item.value === option.value)) {
      store.options = [...store.options, option];
    }

    const selected = getSelectedValues(store.select);
    const updated = new Set(selected);
    updated.add(option.value);
    updateSelected(store, updated);
    toggleMenu(store, false);
  } finally {
    store.creatingQuery = "";
  }
}

function renderChips(store: ChipStore, selectedValues: Set<string>): void {
  const { chipsContent, select, placeholder, searchInput, theme } = store;

  // Avoid re-rendering while the user types, but still update immediately when
  // selection changes (e.g. clicking an option or creating a tag).
  const selectionKey = serializeSelection(selectedValues);
  if (
    searchInput &&
    document.activeElement === searchInput &&
    selectionKey === store.lastRenderedSelectionKey
  ) {
    return;
  }
  store.lastRenderedSelectionKey = selectionKey;

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

      // Look up option data for avatar (from store.options)
      const optionData = store.options.find((item) => item.value === value);
      if (optionData?.avatar) {
        const avatar = document.createElement("img");
        avatar.src = optionData.avatar;
        avatar.alt = "";
        avatar.loading = "lazy";
        setElementClasses(avatar, theme.chipAvatar);
        chip.appendChild(avatar);
      }

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
}

function serializeSelection(values: Set<string>): string {
  return Array.from(values)
    .slice()
    .sort((left, right) => left.localeCompare(right))
    .join("\u0000");
}

function renderMenu(store: ChipStore, selectedValues: Set<string>): void {
  const { menuList, options, searchMode, searchValue, theme } = store;

  // Clear only the menu list content (preserve search header and footer structure)
  clearHighlight(store);
  menuList.innerHTML = "";

  if (store.creatingQuery) {
    const create = document.createElement("button");
    create.type = "button";
    setElementClasses(create, theme.menuItem);
    create.setAttribute("role", "option");
    create.setAttribute("data-fg-chip-option", "true");
    create.setAttribute("data-fg-create-option", "true");
    create.disabled = true;
    create.textContent = `Creating "${store.creatingQuery}"…`;
    menuList.appendChild(create);
    return;
  }

  const rawQuery = searchMode ? searchValue.trim() : "";
  const query = rawQuery.toLowerCase();
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

  // Render inline create button at top of list when applicable (search mode only)
  if (shouldOfferCreate(store, rawQuery, selectedValues)) {
    const create = document.createElement("button");
    create.type = "button";
    setElementClasses(create, theme.menuItem);
    create.setAttribute("role", "option");
    create.setAttribute("data-fg-chip-option", "true");
    create.setAttribute("data-fg-create-option", "true");
    create.textContent = store.createLabel(rawQuery);
    create.addEventListener("click", () => {
      createAndSelect(store, rawQuery).catch(() => undefined);
    });
    menuList.appendChild(create);
  }

  if (available.length === 0) {
    // Only show empty state if create action is not enabled
    if (!store.createActionEnabled) {
      const empty = document.createElement("div");
      setElementClasses(empty, theme.menuEmpty);
      empty.textContent = query ? "No matches" : "No more options";
      menuList.appendChild(empty);
    }
    // Render create action footer even with no options
    renderCreateActionFooter(store);
    return;
  }

  for (const option of available) {
    const button = document.createElement("button");
    button.type = "button";
    setElementClasses(button, theme.menuItem);
    button.setAttribute("role", "option");
    button.dataset.value = option.value;
    button.setAttribute("data-fg-chip-option", "true");

    const label = option.label ?? option.value;
    const hasRichContent = option.avatar || option.icon || option.description;

    if (hasRichContent) {
      // Rich option: icon/avatar + text container with title and description
      if (option.avatar || option.icon) {
        const iconWrapper = document.createElement("span");
        setElementClasses(iconWrapper, theme.menuItemIcon);

        if (option.avatar) {
          const img = document.createElement("img");
          img.src = option.avatar;
          img.alt = "";
          img.loading = "lazy";
          setElementClasses(img, theme.menuItemAvatar);
          iconWrapper.appendChild(img);
        } else if (option.icon) {
          // Use safe icon registry lookup (no raw HTML)
          const iconEl = createIconElement({ name: option.icon }, {
            wrapperClasses: [],
            svgClasses: theme.menuItemAvatar,
          });
          if (iconEl) {
            // Remove wrapper, keep inner content
            while (iconEl.firstChild) {
              iconWrapper.appendChild(iconEl.firstChild);
            }
          }
        }

        button.appendChild(iconWrapper);
      }

      // Text container with title and optional description
      const textContainer = document.createElement("div");
      setElementClasses(textContainer, theme.menuItemText);

      const titleSpan = document.createElement("span");
      setElementClasses(titleSpan, theme.menuItemTitle);
      titleSpan.appendChild(
        buildHighlightedFragment(label, query, classesToString(theme.searchHighlight))
      );
      textContainer.appendChild(titleSpan);

      if (option.description) {
        const descSpan = document.createElement("span");
        setElementClasses(descSpan, theme.menuItemDescription);
        descSpan.textContent = option.description;
        textContainer.appendChild(descSpan);
      }

      button.appendChild(textContainer);
    } else {
      // Simple option: label only
      const labelSpan = document.createElement("span");
      labelSpan.appendChild(
        buildHighlightedFragment(label, query, classesToString(theme.searchHighlight))
      );
      button.appendChild(labelSpan);
    }

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
    menuList.appendChild(button);
  }

  // Render create action footer (always visible when enabled, regardless of options)
  renderCreateActionFooter(store);
}

function updateSelected(store: ChipStore, values: Set<string>): void {
  const { select } = store;
  for (const option of Array.from(select.options)) {
    option.selected = values.has(option.value);
  }
  const selectedValues = getSelectedValues(select);
  emitRelationshipUpdate(select, {
    kind: "selection",
    origin: "ui",
    selectedValues: Array.from(selectedValues),
  });
  select.dispatchEvent(new Event("change", { bubbles: true }));
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

const CHIPS_CREATE_ACTION_ATTR = "data-fg-chips-create-action";

/**
 * Render the create action button in the dropdown footer.
 * The create action is always visible when enabled, regardless of search query or matches.
 */
function renderCreateActionFooter(store: ChipStore): void {
  const { menuFooter, theme } = store;

  // Clear existing footer content
  menuFooter.innerHTML = "";
  store.createActionElement = null;

  if (!store.createActionEnabled) {
    menuFooter.hidden = true;
    return;
  }

  // Show footer
  menuFooter.hidden = false;

  const actionButton = document.createElement("button");
  actionButton.type = "button";
  setElementClasses(actionButton, theme.menuFooterAction);
  actionButton.setAttribute("role", "button");
  actionButton.setAttribute(CHIPS_CREATE_ACTION_ATTR, "true");
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
      store.footerFocused = false;
      const options = getMenuOptions(store);
      if (options.length > 0) {
        focusMenuOption(store, options.length - 1);
      } else {
        // No options, return to input/toggle
        if (store.searchMode && store.searchInput) {
          store.searchInput.focus();
        } else {
          store.toggle.focus();
        }
        clearHighlight(store);
      }
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      // Wrap to first option
      const options = getMenuOptions(store);
      if (options.length > 0) {
        store.footerFocused = false;
        focusMenuOption(store, 0);
      }
    } else if (event.key === "Escape") {
      event.preventDefault();
      toggleMenu(store, false, true);
    }
  });

  menuFooter.appendChild(actionButton);
  store.createActionElement = actionButton;
}

/**
 * Trigger the create action: invoke hook or dispatch event.
 * Implements Model 1 behavior: close dropdown + clear query on activation.
 */
async function triggerCreateAction(store: ChipStore): Promise<void> {
  const config = store.registry.getConfig();
  const query = store.searchValue.trim();

  const detail: CreateActionDetail = {
    query,
    actionId: store.createActionId,
    mode: "chips",
    selectBehavior: store.createActionSelect,
  };

  // Model 1: Close dropdown before triggering action
  toggleMenu(store, false);

  // Model 1: Clear query
  if (store.searchMode && store.searchInput) {
    store.searchInput.value = "";
    store.searchValue = "";
    store.select.setAttribute("data-endpoint-search-value", "");
  }

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

      // Apply returned options (chips can accept Option | Option[])
      if (result) {
        const options = Array.isArray(result) ? result : [result];
        if (options.length > 0) {
          applyCreatedOptions(store, options, store.createActionSelect);
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
      mode: "chips",
      selectBehavior: store.createActionSelect,
    });
  }
}

/**
 * Apply created options from the create action hook.
 * For chips (multi-select), supports append (default) or replace behavior.
 */
function applyCreatedOptions(
  store: ChipStore,
  options: Option[],
  selectBehavior: "append" | "replace"
): void {
  // Add options to the native select if not present
  for (const option of options) {
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
  }

  // Build the new selection based on behavior
  let newSelection: Set<string>;
  if (selectBehavior === "replace") {
    // Replace: clear existing selection, use only created values
    newSelection = new Set(options.map((opt) => opt.value));
  } else {
    // Append (default): union of existing selection + created values
    newSelection = getSelectedValues(store.select);
    for (const option of options) {
      newSelection.add(option.value);
    }
  }

  // Update selection
  updateSelected(store, newSelection);
}

/**
 * Position dropdown using Floating UI for viewport-aware positioning.
 * Only called when useFloatingUI is enabled via data-endpoint-use-floating-ui="true".
 * Falls back gracefully to CSS positioning if Floating UI is not available.
 */
async function positionDropdownWithFloatingUI(store: ChipStore): Promise<void> {
  if (!store.useFloatingUI) {
    return;
  }

  const floatingUI = await loadFloatingUI();
  if (!floatingUI) {
    // Floating UI not available - CSS positioning is used as fallback
    return;
  }

  const { computePosition, offset, flip, shift } = floatingUI;

  try {
    const { x, y } = await computePosition(store.inner, store.menu, {
      placement: "bottom-start",
      middleware: [
        offset(4), // 4px gap
        flip(), // Flip to top if no room below
        shift({ padding: 8 }), // Keep within viewport with 8px padding
      ],
    });

    // Apply computed position (override CSS positioning)
    Object.assign(store.menu.style, {
      position: "absolute",
      left: `${x}px`,
      top: `${y}px`,
    });
  } catch {
    // Positioning failed - CSS positioning is used as fallback
  }
}

/**
 * Update search input width to fit content (dynamic input sizing).
 * Only called when dynamicInputWidth is enabled via data-endpoint-dynamic-input="true".
 */
function updateInputWidth(store: ChipStore): void {
  if (!store.dynamicInputWidth || !store.searchInput) {
    return;
  }

  // Create measurer span if it doesn't exist
  if (!store.widthMeasurer) {
    store.widthMeasurer = document.createElement("span");
    store.widthMeasurer.style.cssText = `
      position: absolute;
      visibility: hidden;
      white-space: pre;
      font: inherit;
      pointer-events: none;
    `;
    store.searchInput.parentElement?.appendChild(store.widthMeasurer);
  }

  // Measure text width
  const text = store.searchInput.value || store.searchInput.placeholder || "";
  store.widthMeasurer.textContent = text;
  const measuredWidth = store.widthMeasurer.offsetWidth;

  // Set input width with minimum and padding
  const minWidth = 50;
  const padding = 16; // Account for padding/cursor
  const width = Math.max(minWidth, measuredWidth + padding);
  store.searchInput.style.width = `${width}px`;
}

function toggleMenu(store: ChipStore, open: boolean, restoreFocus = false): void {
  if (store.animationInProcess) {
    if (store.isOpen === open) {
      return;
    }
    store.pendingToggle = { open, restoreFocus };
    return;
  }
  // Skip if already in the requested state - this prevents redundant operations
  // and serves as the primary guard against rapid toggle races.
  if (store.isOpen === open) {
    return;
  }

  store.animationInProcess = true;
  store.menu.hidden = !open;
  store.toggle.setAttribute("aria-expanded", open ? "true" : "false");
  if (open) {
    addElementClasses(store.container, store.theme.containerOpen);
    // Position dropdown using Floating UI if enabled (optional enhancement)
    if (store.useFloatingUI) {
      positionDropdownWithFloatingUI(store).catch(() => {
        // Positioning failed - CSS fallback is already in place
      });
    }
    if (store.searchMode && store.searchInput) {
      store.searchInput.focus();
      // Update input width if dynamic sizing is enabled
      if (store.dynamicInputWidth) {
        updateInputWidth(store);
      }
    }
  } else {
    removeElementClasses(store.container, store.theme.containerOpen);
    // Clear highlight state when closing
    clearHighlight(store);
    // Restore focus when requested - prefer toggle as it's the main interactive element
    if (restoreFocus) {
      store.toggle.focus();
    }
  }
  store.isOpen = open;

  const releaseLock = () => {
    store.animationInProcess = false;
    if (store.pendingToggle && store.pendingToggle.open !== store.isOpen) {
      const next = store.pendingToggle;
      store.pendingToggle = null;
      toggleMenu(store, next.open, next.restoreFocus);
      return;
    }
    store.pendingToggle = null;
  };
  const transitionMs = getTransitionDurationMs(store.menu);
  if (transitionMs > 0 && typeof setTimeout === "function") {
    setTimeout(releaseLock, transitionMs);
  } else if (typeof queueMicrotask === "function") {
    queueMicrotask(releaseLock);
  } else {
    Promise.resolve().then(releaseLock);
  }
}

function getTransitionDurationMs(element: HTMLElement): number {
  if (typeof window === "undefined" || typeof window.getComputedStyle !== "function") {
    return 0;
  }
  const style = window.getComputedStyle(element);
  const durations = style.transitionDuration.split(",");
  const delays = style.transitionDelay.split(",");
  const count = Math.max(durations.length, delays.length);
  let maxMs = 0;
  for (let i = 0; i < count; i += 1) {
    const duration = parseDurationMs(durations[i] ?? durations[0] ?? "0s");
    const delay = parseDurationMs(delays[i] ?? delays[0] ?? "0s");
    const total = duration + delay;
    if (total > maxMs) {
      maxMs = total;
    }
  }
  return maxMs;
}

function parseDurationMs(value: string): number {
  const trimmed = value.trim();
  if (!trimmed) {
    return 0;
  }
  const numeric = parseFloat(trimmed);
  if (Number.isNaN(numeric)) {
    return 0;
  }
  return trimmed.endsWith("ms") ? numeric : numeric * 1000;
}

function bindValidationState(store: ChipStore): void {
  const syncState = () => {
    const state = store.select.getAttribute("data-validation-state");
    if (state === "invalid") {
      store.container.setAttribute("data-validation-state", "invalid");
      store.container.setAttribute("aria-invalid", "true");
    } else {
      store.container.removeAttribute("data-validation-state");
      store.container.removeAttribute("aria-invalid");
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

function bindSelectionListener(store: ChipStore): void {
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
    renderChips(store, selected);
    renderMenu(store, selected);
    updateClearState(store, selected);
  };
  store.updateHandler = handler;
  store.select.addEventListener(RELATIONSHIP_UPDATE_EVENT, handler as EventListener);
}

function destroyChipStore(store: ChipStore): void {
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
}

function handleRequiredAttribute(select: HTMLSelectElement, target: HTMLElement): void {
  if (!select.hasAttribute("required")) {
    return;
  }
  select.dataset.validationRequiredNative = "true";
  select.removeAttribute("required");
  target.setAttribute("aria-required", "true");
}

registerRendererCleanup("chips", stores, (_select, store) => {
  destroyChipStore(store as ChipStore);
});
