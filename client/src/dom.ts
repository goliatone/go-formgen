const FIELD_SELECTOR = "[data-endpoint-url]";
const HIDDEN_CONTAINER_ATTR = "data-relationship-hidden";
const HIDDEN_INITIALISED_ATTR = "data-relationship-hidden-initialised";
const JSON_INITIALISED_ATTR = "data-relationship-json-initialised";
const JSON_INPUT_ATTR = "data-relationship-json";
const SUBMIT_MODE_ATTR = "data-relationship-submit-mode";
const ORIGINAL_NAME_ATTR = "data-relationship-original-name";

export function locateRelationshipFields(
  root: Document | HTMLElement = document
): HTMLElement[] {
  const scope = root instanceof Document ? root : root;
  const candidates = Array.from(scope.querySelectorAll<HTMLElement>(FIELD_SELECTOR));

  if (root instanceof HTMLElement && root.matches(FIELD_SELECTOR)) {
    candidates.unshift(root);
  }

  return Array.from(new Set(candidates));
}

export function readDataset(element: HTMLElement): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, value] of Object.entries(element.dataset)) {
    if (typeof value === "string") {
      result[key] = value;
    }
  }
  return result;
}

export function isMultiSelect(element: Element): element is HTMLSelectElement {
  return element instanceof HTMLSelectElement && element.multiple;
}

export function attachHiddenInputSync(select: HTMLSelectElement): void {
  if (select.getAttribute(SUBMIT_MODE_ATTR) === "json") {
    return;
  }

  if (select.hasAttribute(HIDDEN_INITIALISED_ATTR)) {
    return;
  }
  select.setAttribute(HIDDEN_INITIALISED_ATTR, "true");
  select.setAttribute(SUBMIT_MODE_ATTR, "hidden-array");
  syncHiddenInputs(select);
  select.addEventListener("change", () => syncHiddenInputs(select));
}

export function syncHiddenInputs(select: HTMLSelectElement): void {
  if (select.getAttribute(SUBMIT_MODE_ATTR) === "json") {
    syncJsonInput(select);
    return;
  }
  const container = ensureHiddenContainer(select);
  while (container.firstChild) {
    container.removeChild(container.firstChild);
  }

  const baseName = select.name || select.id;
  if (!baseName) {
    return;
  }
  const name = baseName.endsWith("[]") ? baseName : `${baseName}[]`;

  Array.from(select.selectedOptions).forEach((option) => {
    const input = document.createElement("input");
    input.type = "hidden";
    input.name = name;
    input.value = option.value;
    container.appendChild(input);
  });
}

function ensureHiddenContainer(select: HTMLSelectElement): HTMLElement {
  const existing = select.parentElement?.querySelector<HTMLElement>(
    `[${HIDDEN_CONTAINER_ATTR}]`
  );
  if (existing) {
    return existing;
  }
  const container = document.createElement("div");
  container.setAttribute(HIDDEN_CONTAINER_ATTR, "true");
  container.style.display = "none";
  if (select.parentElement) {
    select.parentElement.appendChild(container);
  } else if (select.nextSibling) {
    select.parentNode?.insertBefore(container, select.nextSibling);
  } else {
    select.parentNode?.appendChild(container);
  }
  return container;
}

export function attachJsonInputSync(select: HTMLSelectElement): void {
  if (select.hasAttribute(JSON_INITIALISED_ATTR)) {
    return;
  }
  select.setAttribute(JSON_INITIALISED_ATTR, "true");
  select.setAttribute(SUBMIT_MODE_ATTR, "json");
  const originalName = select.getAttribute("name");
  if (originalName) {
    select.setAttribute(ORIGINAL_NAME_ATTR, originalName);
    select.removeAttribute("name");
  }
  syncJsonInput(select);
  select.addEventListener("change", () => syncJsonInput(select));
  select.addEventListener("blur", () => syncJsonInput(select));
}

export function syncJsonInput(select: HTMLSelectElement): void {
  const container = ensureHiddenContainer(select);
  let input = container.querySelector<HTMLInputElement>(`[${JSON_INPUT_ATTR}]`);
  if (!input) {
    input = document.createElement("input");
    input.type = "hidden";
    input.setAttribute(JSON_INPUT_ATTR, "true");
    container.appendChild(input);
  }

  const originalName = select.getAttribute(ORIGINAL_NAME_ATTR) ?? select.getAttribute("name");
  if (originalName) {
    const trimmed = originalName.endsWith("[]")
      ? originalName.slice(0, originalName.length - 2)
      : originalName;
    input.name = trimmed;
  }

  const values = Array.from(select.selectedOptions).map((option) => option.value);
  if (select.multiple) {
    input.value = JSON.stringify(values);
  } else {
    const value = values[0] ?? "";
    input.value = value ? JSON.stringify(value) : "null";
  }
}

export function readElementValue(element: HTMLElement | null): string | string[] | null {
  if (!element) {
    return null;
  }

  if (element instanceof HTMLInputElement) {
    if (element.type === "checkbox" || element.type === "radio") {
      if (!element.checked) {
        return null;
      }
      return element.value;
    }
    return element.value;
  }

  if (element instanceof HTMLSelectElement) {
    if (element.multiple) {
      return Array.from(element.selectedOptions).map((option) => option.value);
    }
    const option = element.selectedOptions[0];
    return option ? option.value : null;
  }

  if (element instanceof HTMLTextAreaElement) {
    return element.value;
  }

  return element.textContent;
}
