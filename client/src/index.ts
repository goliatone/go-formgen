import "./version";
import { ResolverRegistry } from "./registry";
import type {
  GlobalConfig,
  FieldConfig,
} from "./config";
import {
  locateRelationshipFields,
  readDataset,
  syncHiddenInputs,
  syncJsonInput,
} from "./dom";
import { datasetToEndpoint, datasetToFieldConfig } from "./relationship-config";
import { createDebouncedInvoker, createThrottledInvoker } from "./timers";
import { registerChipRenderer, bootstrapChips } from "./renderers/chips";
import { registerTypeaheadRenderer, bootstrapTypeahead } from "./renderers/typeahead";
import { initComponents } from "./components/registry";
import { clearFieldError, renderFieldError } from "./errors";
import { emitRelationshipUpdate } from "./relationship-events";

/**
 * initRelationships bootstraps the runtime resolver registry. The initial phase
 * provides a no-op implementation that resolves immediately while the full
 * runtime logic is developed across subsequent tasks.
 */
let activeRegistry: ResolverRegistry | null = null;

export async function initRelationships(
  config: GlobalConfig = {}
): Promise<ResolverRegistry> {
  const hasOverrides = Object.keys(config ?? {}).length > 0;
  const reuseExisting = activeRegistry && !hasOverrides ? activeRegistry : null;
  const registry = reuseExisting ?? new ResolverRegistry(hasOverrides ? config : undefined);

  if (!reuseExisting) {
    activeRegistry = registry;
    (globalThis as Record<string, unknown>).formgenRelationships = registry;
  }

  registerChipRenderer(registry);
  registerTypeaheadRenderer(registry);

  const roots = Array.from(
    document.querySelectorAll<HTMLElement>("[data-formgen-auto-init]")
  );

  const promises: Promise<void>[] = [];

  for (const root of roots) {
    initComponents(root);
    const fields = locateRelationshipFields(root);
    for (const element of fields) {
      const dataset = readDataset(element);
      const endpoint = datasetToEndpoint(dataset);
      const field = datasetToFieldConfig(element, dataset);
      applyInitialSelection(element, field);

      if (!registry.get(element)) {
        registry.register(element, { field, endpoint });
      }

      if (
        field.renderer === "chips" &&
        element instanceof HTMLSelectElement &&
        element.multiple
      ) {
        bootstrapChips(element, registry);
      }
      if (
        field.renderer === "typeahead" &&
        element instanceof HTMLSelectElement &&
        !element.multiple
      ) {
        bootstrapTypeahead(element, registry);
      }

      setupDependentRefresh(element, field, root, registry);
      setupManualRefresh(element, field, root, registry);
      setupSearchMode(element, field, registry);

      if (shouldAutoResolve(field)) {
        promises.push(registry.resolve(element));
      }
    }
  }

  if (promises.length > 0) {
    await Promise.all(promises);
  }

  return registry;
}

/**
 * Reset the global registry. Intended for testing only.
 * @internal
 */
export function resetGlobalRegistry(): void {
  activeRegistry = null;
  delete (globalThis as Record<string, unknown>).formgenRelationships;
}

export type {
  GlobalConfig,
  EndpointConfig,
  EndpointOverride,
  EndpointMapping,
  EndpointAuth,
  FieldConfig,
  FieldValidationRule,
  Option,
  ValidationError,
  ValidationResult,
  CreateActionDetail,
} from "./config";
export { ResolverRegistry } from "./registry";
export {
  Resolver,
  type ResolverEventDetail,
  type ResolverEventName,
  type Renderer,
  type CustomResolver,
} from "./resolver";
export { RUNTIME_VERSION } from "./version";
export {
  registerComponent,
  initComponents,
  __resetComponentRegistryForTests as resetComponentRegistryForTests,
} from "./components/registry";
export { registerErrorRenderer } from "./errors";
export {
  registerThemeClasses,
  getThemeClasses,
  type ThemeClassMap,
  type ChipsClassMap,
  type TypeaheadClassMap,
  type SwitchClassMap,
  type WysiwygClassMap,
} from "./theme/classes";
export { renderSwitch, type SwitchStore } from "./renderers/switch";
export { renderWysiwyg, autoInitWysiwyg, type WysiwygStore, type WysiwygConfig } from "./renderers/wysiwyg";
export {
  RELATIONSHIP_UPDATE_EVENT,
  RELATIONSHIP_CREATE_ACTION_EVENT,
  emitRelationshipUpdate,
  emitRelationshipCreateAction,
  type RelationshipUpdateDetail,
  type RelationshipUpdateOrigin,
  type RelationshipCreateActionDetail,
} from "./relationship-events";


function applyInitialSelection(element: HTMLElement, field: FieldConfig): void {
  if (!field || field.current == null) {
    return;
  }
  if (element.dataset.relationshipCurrentApplied === "true") {
    return;
  }
  if (element instanceof HTMLSelectElement) {
    const values = normalizeCurrentValues(field.current, element.multiple);
    const changed = applySelectValues(element, values);
    if (changed) {
      syncRelationshipMirrors(element, field.submitAs);
      element.dataset.relationshipCurrentApplied = "true";
    }
    return;
  }
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    const values = normalizeCurrentValues(field.current, false);
    if (values.length > 0) {
      element.value = values[0] ?? "";
      element.dataset.relationshipCurrentApplied = "true";
    }
  }
}

function normalizeCurrentValues(
  current: string | string[] | null,
  allowMultiple: boolean
): string[] {
  if (current == null) {
    return [];
  }
  if (Array.isArray(current)) {
    return allowMultiple ? current.filter(Boolean).map(String) : [String(current[0] ?? "")].filter(Boolean);
  }
  const value = String(current);
  return value ? [value] : [];
}

function applySelectValues(select: HTMLSelectElement, values: string[]): boolean {
  const unique = select.multiple ? Array.from(new Set(values)) : values.slice(0, 1);
  const targetValues = new Set(unique.filter(Boolean));
  let changed = false;

  Array.from(select.options).forEach((option) => {
    const shouldSelect = targetValues.has(option.value);
    if (option.selected !== shouldSelect) {
      option.selected = shouldSelect;
      changed = true;
    }
    if (shouldSelect) {
      targetValues.delete(option.value);
    }
  });

  targetValues.forEach((value) => {
    if (!value) {
      return;
    }
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    option.selected = true;
    select.appendChild(option);
    changed = true;
  });

  if (!select.multiple && unique.length === 0) {
    if (select.value !== "") {
      select.value = "";
      changed = true;
    }
  }

  return changed;
}

function syncRelationshipMirrors(select: HTMLSelectElement, submitAs?: FieldConfig["submitAs"]): void {
  if (submitAs === "json") {
    syncJsonInput(select);
    return;
  }
  if (select.multiple) {
    syncHiddenInputs(select);
  }
}

function shouldAutoResolve(field: FieldConfig): boolean {
  if (field.refreshMode === "manual") {
    return false;
  }
  if (field.mode === "search") {
    return hasCurrentValue(field.current);
  }
  return true;
}

function hasCurrentValue(current: FieldConfig["current"]): boolean {
  if (current == null) {
    return false;
  }
  if (Array.isArray(current)) {
    return current.some((value) => String(value).trim() !== "");
  }
  return String(current).trim() !== "";
}

export interface HydrationPayload {
  values?: Record<string, unknown>;
  errors?: Record<string, string | string[]>;
}

export function hydrateFormValues(
  root: Document | HTMLElement = document,
  payload: HydrationPayload = {}
): void {
  const scope = root ?? document;
  const fields = locateHydratableFields(scope);
  if (fields.length === 0) {
    return;
  }

  const valueIndex = buildPayloadIndex(payload.values);
  const errorIndex = buildPayloadIndex(payload.errors);

  fields.forEach((element) => {
    applyHydratedValue(element, valueIndex);
    applyHydratedErrors(element, errorIndex);
  });
}

function locateHydratableFields(root: Document | HTMLElement): HTMLElement[] {
  const nodes = new Set<HTMLElement>();
  const scope = root instanceof Document ? root : root;
  if (root instanceof HTMLElement && isHydratableElement(root)) {
    nodes.add(root);
  }

  const selector = "input, select, textarea, [data-field-name], [data-field-path]";
  scope.querySelectorAll<HTMLElement>(selector).forEach((element) => {
    if (isHydratableElement(element)) {
      nodes.add(element);
    }
  });

  locateRelationshipFields(scope).forEach((element) => nodes.add(element));
  return Array.from(nodes);
}

function isHydratableElement(element: Element): element is HTMLElement {
  if (element instanceof HTMLInputElement) {
    if (element.type === "hidden") {
      if (element.hasAttribute("data-relationship-hidden") || element.hasAttribute("data-relationship-json")) {
        return false;
      }
      return Boolean(element.dataset.fieldName || element.dataset.fieldPath);
    }
    return true;
  }
  return element instanceof HTMLSelectElement || element instanceof HTMLTextAreaElement;
}

function buildPayloadIndex(
  source?: Record<string, unknown>
): Map<string, unknown> {
  const index = new Map<string, unknown>();
  if (!source) {
    return index;
  }
  const flattened = new Map<string, unknown>();
  flattenPayload(source, flattened, "");
  flattened.forEach((value, key) => {
    addKeyVariants(index, key, value);
  });
  return index;
}

function flattenPayload(
  input: Record<string, unknown>,
  target: Map<string, unknown>,
  prefix: string
): void {
  Object.entries(input).forEach(([key, value]) => {
    const trimmedKey = key.trim();
    if (!trimmedKey) {
      return;
    }
    const nextKey = prefix ? `${prefix}.${trimmedKey}` : trimmedKey;
    if (
      value &&
      typeof value === "object" &&
      !Array.isArray(value)
    ) {
      flattenPayload(value as Record<string, unknown>, target, nextKey);
      return;
    }
    target.set(nextKey, value);
  });
}

function addKeyVariants(
  index: Map<string, unknown>,
  key: string,
  value: unknown
): void {
  const variants = new Set<string>();
  const canonical = key.trim();
  if (!canonical) {
    return;
  }
  const stripped = stripArraySuffix(canonical);
  const dotted = toDotKey(stripped);
  const bracket = toBracketKey(dotted);

  [canonical, stripped, dotted, bracket].forEach((entry) => {
    if (entry) {
      variants.add(entry);
    }
  });

  if (Array.isArray(value)) {
    [canonical, stripped, dotted, bracket].forEach((entry) => {
      if (entry) {
        variants.add(`${entry}[]`);
      }
    });
  }

  variants.forEach((variant) => {
    if (variant) {
      index.set(variant, value);
    }
  });
}

function stripArraySuffix(value: string): string {
  return value.endsWith("[]") ? value.slice(0, -2) : value;
}

function toDotKey(value: string): string {
  if (!value.includes("[")) {
    return value.replace(/\[\]/g, "");
  }
  return value
    .replace(/\[\]/g, "")
    .replace(/\[([^\]]+)\]/g, ".$1")
    .replace(/^\./, "");
}

function toBracketKey(value: string): string {
  if (!value.includes(".")) {
    return value;
  }
  const segments = value.split(".").filter(Boolean);
  if (segments.length === 0) {
    return value;
  }
  const [first, ...rest] = segments;
  return `${first}${rest.map((segment) => `[${segment}]`).join("")}`;
}

function collectFieldKeys(element: HTMLElement): string[] {
  const keys = new Set<string>();
  const candidates = [
    element.getAttribute("name"),
    element.dataset.fieldName,
    element.getAttribute("data-field-path"),
    element.id,
  ];

  candidates.forEach((value) => {
    if (!value) {
      return;
    }
    keys.add(value);
    keys.add(stripArraySuffix(value));
    const dotted = toDotKey(value);
    keys.add(dotted);
    keys.add(stripArraySuffix(dotted));
    keys.add(toBracketKey(dotted));
  });

  return Array.from(keys).filter(Boolean);
}

function resolvePayloadEntry(
  index: Map<string, unknown>,
  element: HTMLElement
): { found: boolean; value: unknown } {
  const keys = collectFieldKeys(element);
  for (const key of keys) {
    if (index.has(key)) {
      return { found: true, value: index.get(key) };
    }
  }
  return { found: false, value: undefined };
}

function applyHydratedValue(
  element: HTMLElement,
  index: Map<string, unknown>
): void {
  const entry = resolvePayloadEntry(index, element);
  if (!entry.found) {
    return;
  }
  const normalized = normalizeHydratedSelection(entry.value, element);
  if (element instanceof HTMLSelectElement) {
    const values = Array.isArray(normalized)
      ? normalized
      : normalized != null
        ? [normalized]
        : [];
    const changed = applySelectValues(element, values);
    const submitMode =
      element.getAttribute("data-relationship-submit-mode") === "json" ||
      element.dataset.endpointSubmitAs === "json"
        ? "json"
        : undefined;
    if (changed) {
      syncRelationshipMirrors(element, submitMode as FieldConfig["submitAs"]);
      emitRelationshipUpdate(element, {
        kind: "selection",
        origin: "hydrate",
        selectedValues: Array.from(element.selectedOptions)
          .map((option) => option.value)
          .filter((value) => value !== ""),
      });
      element.dispatchEvent(new Event("change", { bubbles: true }));
    }
  } else if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    if (Array.isArray(normalized)) {
      element.value = normalized[0] ?? "";
    } else if (normalized == null) {
      element.value = "";
    } else {
      element.value = normalized;
    }
    element.dispatchEvent(new Event("input", { bubbles: true }));
  }

  updateRelationshipCurrentAttribute(element, normalized);

  const resolver = activeRegistry?.get(element);
  if (resolver) {
    resolver.setCurrentValue(normalized);
  }
}

function applyHydratedErrors(
  element: HTMLElement,
  index: Map<string, unknown>
): void {
  const entry = resolvePayloadEntry(index, element);
  if (!entry.found) {
    return;
  }
  const messages = normalizeErrorMessages(entry.value);
  const resolver = activeRegistry?.get(element);

  if (messages.length === 0) {
    clearFieldError(element);
    element.removeAttribute("data-validation-state");
    element.removeAttribute("data-validation-message");
    resolver?.setServerValidation(undefined);
    return;
  }

  const message = messages[0];
  renderFieldError(element, message);
  element.setAttribute("data-validation-state", "invalid");
  element.setAttribute("data-validation-message", messages.join("; "));
  resolver?.setServerValidation({
    valid: false,
    messages,
    errors: messages.map((text) => ({
      code: "server",
      message: text,
    })),
  });
}

function normalizeHydratedSelection(
  value: unknown,
  element: HTMLElement
): string | string[] | null {
  if (value == null) {
    return null;
  }
  if (Array.isArray(value)) {
    const tokens = value
      .map((item) => coerceHydratedValue(item))
      .filter((token): token is string => typeof token === "string" && token !== "");
    if (element instanceof HTMLSelectElement && element.multiple) {
      return tokens;
    }
    return tokens[0] ?? null;
  }
  const token = coerceHydratedValue(value);
  return token ?? null;
}

function coerceHydratedValue(value: unknown): string | null {
  if (value == null) {
    return null;
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    for (const key of ["value", "id", "slug"]) {
      const candidate = record[key];
      if (candidate != null) {
        return String(candidate);
      }
    }
    return null;
  }
  return String(value);
}

function updateRelationshipCurrentAttribute(
  element: HTMLElement,
  value: string | string[] | null
): void {
  if (value == null || (Array.isArray(value) && value.length === 0)) {
    element.removeAttribute("data-relationship-current");
    return;
  }
  const payload = serializeCurrentValue(value);
  if (payload) {
    element.setAttribute("data-relationship-current", payload);
  }
}

function serializeCurrentValue(
  value: string | string[] | null
): string | undefined {
  if (value == null) {
    return undefined;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return undefined;
    }
    try {
      return JSON.stringify(value);
    } catch (_err) {
      return undefined;
    }
  }
  const trimmed = String(value).trim();
  return trimmed || undefined;
}

function normalizeErrorMessages(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value
      .map((item) => (typeof item === "string" ? item.trim() : ""))
      .filter((item) => item.length > 0);
  }
  if (typeof value === "string") {
    const direct = value.trim();
    if (direct.includes(";")) {
      return direct
        .split(";")
        .map((item) => item.trim())
        .filter((item) => item.length > 0);
    }
    return direct ? [direct] : [];
  }
  return [];
}

function setupDependentRefresh(
  element: HTMLElement,
  field: FieldConfig,
  root: HTMLElement,
  registry: ResolverRegistry
): void {
  if (!field.refreshOn || field.refreshOn.length === 0) {
    return;
  }
  if (field.refreshMode === "manual") {
    return;
  }
  if (element.dataset.relationshipRefreshBound === "true") {
    return;
  }
  element.dataset.relationshipRefreshBound = "true";

  const form = element.closest("form");
  const scope = form ?? root ?? element.ownerDocument ?? document;

  const trigger = () => {
    registry.resolve(element).catch(() => undefined);
  };

  field.refreshOn.forEach((reference) => {
    const targets = findDependencyTargets(scope, reference);
    targets.forEach((target) => {
      const eventType = target instanceof HTMLInputElement && target.type === "text" ? "input" : "change";
      target.addEventListener(eventType, trigger);
    });
  });
}

function setupManualRefresh(
  element: HTMLElement,
  field: FieldConfig,
  root: HTMLElement,
  registry: ResolverRegistry
): void {
  if (field.refreshMode !== "manual") {
    return;
  }
  if (element.dataset.relationshipManualBound === "true") {
    return;
  }
  element.dataset.relationshipManualBound = "true";

  const name = field.name ?? element.getAttribute("id");
  if (!name) {
    return;
  }

  const escaped = safeSelectorValue(name);
  const doc = root.ownerDocument ?? element.ownerDocument ?? document;
  const triggers = new Set<HTMLElement>();

  const selectors = [
    `[data-endpoint-refresh-target="${escaped}"]`,
    `[data-endpoint-refresh-for="${escaped}"]`,
  ];

  selectors.forEach((selector) => {
    doc.querySelectorAll<HTMLElement>(selector).forEach((node) => triggers.add(node));
  });

  const container = element.closest("[data-relationship-type]") ?? element.parentElement;
  container
    ?.querySelectorAll<HTMLElement>("[data-endpoint-refresh-trigger]")
    .forEach((node) => triggers.add(node));

  triggers.forEach((trigger) => {
    if (trigger.dataset.relationshipRefreshListener === "true") {
      return;
    }
    trigger.dataset.relationshipRefreshListener = "true";
    trigger.addEventListener("click", (event) => {
      event.preventDefault();
      registry.resolve(element).catch(() => undefined);
    });
  });
}

function setupSearchMode(
  element: HTMLElement,
  field: FieldConfig,
  registry: ResolverRegistry
): void {
  if (field.mode !== "search") {
    return;
  }
  if (element.dataset.relationshipSearchBound === "true") {
    return;
  }
  element.dataset.relationshipSearchBound = "true";

  const config = registry.getConfig();
  const throttleMs = field.throttleMs ?? config.searchThrottleMs;
  const debounceMs = field.debounceMs ?? config.searchDebounceMs;

  const invoke = () => {
    registry.resolve(element).catch(() => undefined);
  };

  let trigger = () => invoke();

  if (debounceMs > 0) {
    trigger = createDebouncedInvoker(trigger, debounceMs);
  }

  if (throttleMs > 0) {
    trigger = createThrottledInvoker(trigger, throttleMs);
  }

  const updateSearchValue = () => {
    if (
      element instanceof HTMLInputElement ||
      element instanceof HTMLTextAreaElement
    ) {
      const trimmed = element.value.trim();
      element.setAttribute("data-endpoint-search-value", trimmed);
    }
  };

  const handleSearchEvent = () => {
    updateSearchValue();
    trigger();
  };

  element.addEventListener("input", handleSearchEvent);

  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    element.addEventListener("change", handleSearchEvent);
  }
}

function findDependencyTargets(scope: Document | HTMLElement, reference: string): HTMLElement[] {
  const matches: HTMLElement[] = [];
  const seen = new Set<HTMLElement>();

  const form = scope instanceof HTMLFormElement ? scope : scope instanceof HTMLElement ? scope.closest("form") : null;
  const doc = scope instanceof Document ? scope : scope.ownerDocument ?? document;

  const sources: HTMLElement[] = [];
  if (form) {
    const elements = Array.from(form.elements) as Array<Element>;
    elements.forEach((el) => {
      if (el instanceof HTMLElement) {
        sources.push(el);
      }
    });
  } else {
    sources.push(...Array.from(doc.querySelectorAll<HTMLElement>("[name], [data-field-name], [id]")));
  }

  sources.forEach((candidate) => {
    if (seen.has(candidate)) {
      return;
    }
    const datasetName = (candidate.dataset as Record<string, string | undefined>).fieldName;
    if (datasetName && datasetName === reference) {
      matches.push(candidate);
      seen.add(candidate);
      return;
    }
    const nameAttr = (candidate as HTMLInputElement).name;
    if (matchesFieldName(nameAttr, reference)) {
      matches.push(candidate);
      seen.add(candidate);
      return;
    }
    const idAttr = candidate.id;
    if (idAttr && idAttr === reference) {
      matches.push(candidate);
      seen.add(candidate);
    }
  });

  return matches;
}

function matchesFieldName(candidate: string | undefined, reference: string): boolean {
  if (!candidate) {
    return false;
  }
  if (candidate === reference) {
    return true;
  }
  if (candidate === `${reference}[]`) {
    return true;
  }
  if (candidate.endsWith(`[${reference}]`)) {
    return true;
  }
  if (candidate.endsWith(`[${reference}][]`)) {
    return true;
  }
  const sanitizedCandidate = candidate.replace(/[^a-z0-9]/gi, "").toLowerCase();
  const sanitizedReference = reference.replace(/[^a-z0-9]/gi, "").toLowerCase();
  return sanitizedCandidate.endsWith(sanitizedReference);
}

function safeSelectorValue(value: string): string {
  if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
    return CSS.escape(value);
  }
  return value.replace(/(["\\])/g, "\\$1");
}
