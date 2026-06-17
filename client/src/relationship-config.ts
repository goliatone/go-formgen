import type {
  CurrentOption,
  EndpointConfig,
  FieldConfig,
  FieldValidationRule,
  RelationshipCurrent,
  RelationshipCurrentItem,
  RelationshipCardinality,
  RelationshipKind,
} from "./config";

export function datasetToEndpoint(dataset: Record<string, string>): EndpointConfig {
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
  if (dataset.endpointHydrateParam) {
    endpoint.hydrateParam = dataset.endpointHydrateParam;
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

export function datasetToFieldConfig(
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
  if (dataset.endpointAllowCreate === "true") {
    field.allowCreate = true;
  }
  if (dataset.endpointCreateAction === "true") {
    field.createAction = true;
  }
  if (dataset.endpointCreateActionLabel) {
    field.createActionLabel = dataset.endpointCreateActionLabel;
  }
  if (dataset.endpointCreateActionId) {
    field.createActionId = dataset.endpointCreateActionId;
  }
  if (dataset.endpointCreateActionSelect === "append" || dataset.endpointCreateActionSelect === "replace") {
    field.createActionSelect = dataset.endpointCreateActionSelect;
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
  if (dataset.icon) {
    field.icon = dataset.icon;
  }
  if (dataset.iconSource) {
    field.iconSource = dataset.iconSource;
  }
  if (dataset.iconRaw) {
    field.iconRaw = dataset.iconRaw;
  }

  field.required = element.hasAttribute("required") || dataset.validationRequired === "true";

  const label =
    dataset.validationLabel ||
    dataset.endpointFieldLabel ||
    element.getAttribute("aria-label") ||
    element.getAttribute("placeholder") ||
    element.getAttribute("name") ||
    element.id ||
    undefined;
  if (label) {
    field.label = label;
  }

  if (dataset.validationRules) {
    try {
      const parsed = JSON.parse(dataset.validationRules);
      if (Array.isArray(parsed)) {
        field.validations = parsed.filter(isValidValidationRule);
      }
    } catch (_err) {
      // Ignore malformed validation metadata to avoid breaking auto-init.
    }
  }

  if (!field.refreshMode) {
    field.refreshMode = "auto";
  }
  if (!field.mode) {
    field.mode = "default";
  }

  return field;
}

function parseCurrent(value: string): RelationshipCurrent {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed);
    if (Array.isArray(parsed)) {
      return parsed
        .map(parseCurrentItem)
        .filter((item): item is RelationshipCurrentItem => item != null);
    }
    if (parsed == null) {
      return null;
    }
    return parseCurrentItem(parsed);
  } catch (_err) {
    if (trimmed.includes(",")) {
      return trimmed.split(",").map((item) => item.trim()).filter(Boolean);
    }
    return trimmed;
  }
}

function parseCurrentItem(value: unknown): RelationshipCurrentItem | null {
  if (value == null) {
    return null;
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    const option = parseCurrentOption(record);
    if (option) {
      return option;
    }
    const rawValue = firstPresent(record, ["value", "id", "slug"]);
    if (rawValue != null) {
      return String(rawValue);
    }
    return null;
  }
  return String(value);
}

function parseCurrentOption(value: Record<string, unknown>): CurrentOption | null {
  const rawValue = firstPresent(value, ["value", "id", "slug"]);
  if (rawValue == null) {
    return null;
  }
  const rawLabel = firstPresent(value, ["label", "name", "title"]);
  if (rawLabel == null) {
    return null;
  }
  const optionValue = String(rawValue);
  const optionLabel = String(rawLabel);
  if (!optionValue || !optionLabel) {
    return null;
  }
  return { value: optionValue, label: optionLabel };
}

function firstPresent(record: Record<string, unknown>, keys: string[]): unknown {
  for (const key of keys) {
    const value = record[key];
    if (value != null && value !== "") {
      return value;
    }
  }
  return undefined;
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

function isValidValidationRule(candidate: unknown): candidate is FieldValidationRule {
  if (!candidate || typeof candidate !== "object") {
    return false;
  }
  const rule = candidate as FieldValidationRule;
  return typeof rule.kind === "string" && rule.kind.length > 0;
}

function toNumber(value?: string): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}
