import "./version";
import { ResolverRegistry } from "./registry";
import type {
  GlobalConfig,
  EndpointConfig,
  FieldConfig,
  RelationshipCardinality,
  RelationshipKind,
} from "./config";
import {
  locateRelationshipFields,
  readDataset,
} from "./dom";
import { createDebouncedInvoker, createThrottledInvoker } from "./timers";
import { registerChipRenderer, bootstrapChips } from "./renderers/chips";

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

  const roots = Array.from(
    document.querySelectorAll<HTMLElement>("[data-formgen-auto-init]")
  );

  const promises: Promise<void>[] = [];

  for (const root of roots) {
    const fields = locateRelationshipFields(root);
    for (const element of fields) {
      const dataset = readDataset(element);
      const endpoint = datasetToEndpoint(dataset);
      const field = datasetToFieldConfig(element, dataset);

      if (!registry.get(element)) {
        registry.register(element, { field, endpoint });
      }

      if (
        field.renderer === "chips" &&
        element instanceof HTMLSelectElement &&
        element.multiple
      ) {
        bootstrapChips(element);
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
  Option,
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

function datasetToEndpoint(dataset: Record<string, string>): EndpointConfig {
  const endpoint: EndpointConfig = {};
  if (dataset.endpointUrl) {
    endpoint.url = dataset.endpointUrl;
  }
  if (dataset.endpointMethod) {
    endpoint.method = dataset.endpointMethod.toUpperCase();
  }
  if (dataset.endpointLabelField) {
    endpoint.labelField = dataset.endpointLabelField;
  }
  if (dataset.endpointValueField) {
    endpoint.valueField = dataset.endpointValueField;
  }
  if (dataset.endpointResultsPath) {
    endpoint.resultsPath = dataset.endpointResultsPath;
  }
  if (dataset.endpointMode) {
    endpoint.mode = dataset.endpointMode;
  }
  if (dataset.endpointSearchParam) {
    endpoint.searchParam = dataset.endpointSearchParam;
  }
  if (dataset.endpointSubmitAs) {
    endpoint.submitAs = dataset.endpointSubmitAs;
  }

  const params = extractGroup(dataset, "endpointParams");
  if (params) {
    endpoint.params = params;
  }
  const dynamicParams = extractGroup(dataset, "endpointDynamicParams");
  if (dynamicParams) {
    endpoint.dynamicParams = dynamicParams;
  }

  const mapping = extractGroup(dataset, "endpointMapping");
  if (mapping && (mapping.value || mapping.label)) {
    endpoint.mapping = mapping;
  }

  const auth = extractGroup(dataset, "endpointAuth");
  if (auth && (auth.source || auth.header || auth.strategy)) {
    endpoint.auth = auth;
  }

  return endpoint;
}

function datasetToFieldConfig(
  element: HTMLElement,
  dataset: Record<string, string>
): FieldConfig {
  const field: FieldConfig = {
    name: element.getAttribute("name") ?? element.getAttribute("id") ?? undefined,
  };

  if (dataset.relationshipType) {
    field.relationship = dataset.relationshipType as RelationshipKind;
  }
  if (dataset.relationshipCardinality) {
    field.cardinality = dataset.relationshipCardinality as RelationshipCardinality;
  }
  if (dataset.endpointSubmitAs === "json") {
    field.submitAs = "json";
  } else if (dataset.endpointSubmitAs) {
    field.submitAs = "default";
  }
  if (dataset.endpointCacheKey) {
    field.cacheKey = dataset.endpointCacheKey;
  }
  if (dataset.endpointRenderer) {
    field.renderer = dataset.endpointRenderer;
  }
  if (dataset.endpointRefresh) {
    field.refreshMode = dataset.endpointRefresh === "manual" ? "manual" : "auto";
  }
  if (dataset.endpointRefreshOn) {
    field.refreshOn = dataset.endpointRefreshOn
      .split(",")
      .map((value) => value.trim())
      .filter(Boolean);
  }
  if (dataset.endpointMode === "search") {
    field.mode = "search";
  }
  if (dataset.endpointThrottle) {
    field.throttleMs = toNumber(dataset.endpointThrottle);
  }
  if (dataset.endpointDebounce) {
    field.debounceMs = toNumber(dataset.endpointDebounce);
  }
  if (dataset.endpointSearchParam) {
    field.searchParam = dataset.endpointSearchParam;
  }
  if (dataset.relationshipCurrent) {
    field.current = parseCurrent(dataset.relationshipCurrent);
  }

  if (!field.refreshMode) {
    field.refreshMode = "auto";
  }
  if (!field.mode) {
    field.mode = "default";
  }

  return field;
}

function parseCurrent(value: string): string | string[] | null {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed);
    if (Array.isArray(parsed)) {
      return parsed.map((item) => String(item));
    }
    if (parsed == null) {
      return null;
    }
    return String(parsed);
  } catch (_err) {
    if (trimmed.includes(",")) {
      return trimmed.split(",").map((item) => item.trim()).filter(Boolean);
    }
    return trimmed;
  }
}

function extractGroup(
  dataset: Record<string, string>,
  prefix: string
): Record<string, string> | undefined {
  const result: Record<string, string> = {};
  const lowerPrefix = prefix.toLowerCase();

  Object.entries(dataset).forEach(([key, value]) => {
    if (!value) {
      return;
    }
    if (key === prefix || key.toLowerCase() === lowerPrefix) {
      result[""] = value;
      return;
    }
    if (!key.startsWith(prefix)) {
      return;
    }
    const suffix = key.slice(prefix.length);
    if (!suffix) {
      return;
    }
    const paramName = toParamName(suffix);
    if (!paramName) {
      return;
    }
    result[paramName] = value;
  });

  return Object.keys(result).length > 0 ? result : undefined;
}

function toParamName(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) {
    return "";
  }
  const chars = trimmed.split("");
  const transformed: string[] = [];
  chars.forEach((char, index) => {
    if (/[A-Z]/.test(char)) {
      if (index !== 0) {
        transformed.push("-");
      }
      transformed.push(char.toLowerCase());
    } else {
      transformed.push(char);
    }
  });
  return transformed.join("").replace(/^-+/, "");
}

function shouldAutoResolve(field: FieldConfig): boolean {
  if (field.refreshMode === "manual") {
    return false;
  }
  if (field.mode === "search") {
    return false;
  }
  return true;
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

function toNumber(value?: string): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}
