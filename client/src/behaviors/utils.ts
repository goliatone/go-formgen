export function slugify(input: string): string {
  if (!input) {
    return "";
  }
  return input
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-zA-Z0-9\s-]/g, " ")
    .trim()
    .replace(/[\s_-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .toLowerCase();
}

export function normalizeBehaviorName(name: string): string {
  return name?.trim().toLowerCase() ?? "";
}

export function collectBehaviorElements(root: Document | HTMLElement): HTMLElement[] {
  const scope = root instanceof Document ? root : root;
  const elements = Array.from(scope.querySelectorAll<HTMLElement>("[data-behavior]"));
  if (root instanceof HTMLElement && root.hasAttribute("data-behavior")) {
    elements.unshift(root);
  }
  return elements;
}

export function parseBehaviorNames(raw: string | null): string[] {
  if (!raw) {
    return [];
  }
  const tokens = raw
    .split(/[\s,]+/)
    .map((token) => normalizeBehaviorName(token))
    .filter(Boolean);
  return Array.from(new Set(tokens));
}

export function parseBehaviorConfig(raw: string | null): unknown {
  if (!raw) {
    return undefined;
  }
  try {
    return JSON.parse(raw);
  } catch (error) {
    console.warn("[formgen:behaviors] failed to parse data-behavior-config:", error);
    return undefined;
  }
}

export function selectBehaviorConfig(parsed: unknown, name: string, total: number): unknown {
  if (parsed && typeof parsed === "object" && parsed !== null) {
    const record = parsed as Record<string, unknown>;
    if (Object.prototype.hasOwnProperty.call(record, name)) {
      return record[name];
    }
    if (total === 1) {
      return parsed;
    }
    return undefined;
  }
  if (total === 1) {
    return parsed;
  }
  return undefined;
}

export function resolveRootElement(element: HTMLElement, scope: Document | HTMLElement): HTMLElement {
  const nearest = element.closest<HTMLElement>("[data-formgen-auto-init]");
  if (nearest) {
    return nearest;
  }
  if (scope instanceof HTMLElement) {
    return scope;
  }
  return scope.body ?? element.ownerDocument?.body ?? element;
}

export function isInputControl(
  node: Element | null,
): node is HTMLInputElement | HTMLTextAreaElement {
  return (
    !!node &&
    (node instanceof HTMLInputElement || node instanceof HTMLTextAreaElement)
  );
}

export function findNearestInput(element: HTMLElement): HTMLInputElement | HTMLTextAreaElement | null {
  if (isInputControl(element)) {
    return element;
  }
  return element.querySelector<HTMLInputElement | HTMLTextAreaElement>("input, textarea");
}

export function findFieldInput(
  root: HTMLElement,
  key: string,
): HTMLInputElement | HTMLTextAreaElement | null {
  if (!key) {
    return null;
  }
  const attrSelector = `[name="${key}"]`;
  const idSelector = `#${buildElementID(key)}`;
  return (
    root.querySelector<HTMLInputElement | HTMLTextAreaElement>(attrSelector) ??
    root.querySelector<HTMLInputElement | HTMLTextAreaElement>(idSelector)
  );
}

export function buildElementID(key: string): string {
  const escaped = key.replace(/[^a-zA-Z0-9_-]/g, "-");
  return escaped.startsWith("fg-") ? escaped : `fg-${escaped}`;
}
