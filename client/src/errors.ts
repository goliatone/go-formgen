export class ResolverError extends Error {
  readonly status?: number;
  readonly detail?: unknown;

  constructor(message: string, status?: number, detail?: unknown) {
    super(message);
    this.name = "ResolverError";
    this.status = status;
    this.detail = detail;
  }
}

export class ResolverAbortError extends Error {
  constructor() {
    super("Resolver request aborted");
    this.name = "ResolverAbortError";
  }
}

const FIELD_CONTAINER_SELECTOR = "[data-relationship-type], [data-formgen-choice]";
const CHOICE_GROUP_SELECTOR = "[data-formgen-choice]";
const ERROR_ATTR = "data-relationship-error";
const DEFAULT_ERROR_RENDERER = "inline";

export interface FieldErrorRenderContext {
  element: HTMLElement;
  message: string | null;
  code?: string;
}

type FieldErrorRenderer = (context: FieldErrorRenderContext) => void;

const errorRenderers = new Map<string, FieldErrorRenderer>();
errorRenderers.set(DEFAULT_ERROR_RENDERER, inlineErrorRenderer);

export function registerErrorRenderer(name: string, renderer: FieldErrorRenderer): void {
  if (!name || typeof renderer !== "function") {
    return;
  }
  errorRenderers.set(name, renderer);
}

export function renderFieldError(
  element: HTMLElement,
  message: string | null,
  code?: string
): void {
  const rendererName = element.dataset.validationRenderer || DEFAULT_ERROR_RENDERER;
  const renderer =
    errorRenderers.get(rendererName) ??
    errorRenderers.get(DEFAULT_ERROR_RENDERER) ??
    inlineErrorRenderer;
  renderer({ element, message, code });
}

export function clearFieldError(element: HTMLElement): void {
  renderFieldError(element, null);
}

function inlineErrorRenderer(context: FieldErrorRenderContext): void {
  // Find the appropriate container for the error message
  const container =
    context.element.closest(FIELD_CONTAINER_SELECTOR) ??
    context.element.parentElement ??
    context.element;
  if (!container) {
    return;
  }

  // If the container is a "relative" wrapper (for icons), insert error after it
  // to prevent icon shifting when error message appears/disappears
  let errorParent = container;
  if (container.classList.contains('relative') && container.parentElement) {
    errorParent = container.parentElement;
  }

  let target = errorParent.querySelector<HTMLElement>(`[${ERROR_ATTR}]`);
  if (!target) {
    target = document.createElement("p");
    target.setAttribute(ERROR_ATTR, "true");
    target.className = "formgen-error text-xs text-red-600 mt-2 dark:text-red-400";
    target.setAttribute("role", "status");
    target.setAttribute("aria-live", "polite");
    target.setAttribute("aria-atomic", "true");

    // Insert after the icon wrapper if it exists, or append to container
    if (container.classList.contains('relative') && container.parentElement) {
      container.parentElement.insertBefore(target, container.nextSibling);
    } else {
      errorParent.appendChild(target);
    }
  }

  const group = context.element.closest<HTMLElement>(CHOICE_GROUP_SELECTOR);
  if (context.message && context.message.trim() !== "") {
    target.textContent = context.message;
    target.removeAttribute("aria-hidden");
    markElementInvalid(context.element, context.message);
    if (group) {
      if (group.getAttribute("role") === "radiogroup") {
        group.setAttribute("aria-invalid", "true");
      }
      group.setAttribute("data-validation-state", "invalid");
      group.setAttribute("data-validation-message", context.message);
    }
  } else {
    target.textContent = "";
    target.setAttribute("aria-hidden", "true");
    clearInvalidState(context.element);
    if (group && !group.querySelector('input[aria-invalid="true"]')) {
      group.removeAttribute("aria-invalid");
      group.removeAttribute("data-validation-state");
      group.removeAttribute("data-validation-message");
    }
  }
}

function markElementInvalid(element: HTMLElement, message: string): void {
  element.setAttribute("aria-invalid", "true");
  element.setAttribute("data-validation-state", "invalid");
  element.setAttribute("data-validation-message", message);

  // Add Preline validation border classes dynamically
  addValidationClasses(element, true);
}

function clearInvalidState(element: HTMLElement): void {
  element.removeAttribute("aria-invalid");
  element.removeAttribute("data-validation-state");
  element.removeAttribute("data-validation-message");

  // Remove Preline validation border classes
  addValidationClasses(element, false);
}

function addValidationClasses(element: HTMLElement, isInvalid: boolean): void {
  // Find the actual input/textarea/select element
  let target: HTMLElement | null = element;

  if (!(element instanceof HTMLInputElement ||
        element instanceof HTMLTextAreaElement ||
        element instanceof HTMLSelectElement)) {
    // If element is a container, find the input inside
    target = element.querySelector<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(
      'input, textarea, select'
    );
  }

  if (!target) {
    return;
  }

  const invalidClasses = ['border-red-500', 'focus:border-red-500', 'focus:ring-red-500', 'dark:border-red-500'];
  const validClasses = ['border-gray-200', 'focus:border-blue-500', 'focus:ring-blue-500', 'dark:border-gray-700', 'dark:focus:ring-gray-600'];

  if (isInvalid) {
    // Remove valid classes, add invalid classes
    validClasses.forEach(cls => target!.classList.remove(cls));
    invalidClasses.forEach(cls => target!.classList.add(cls));
  } else {
    // Remove invalid classes, add valid classes
    invalidClasses.forEach(cls => target!.classList.remove(cls));
    validClasses.forEach(cls => target!.classList.add(cls));
  }
}

// ---------------------------------------------------------------------------
// Form-level summaries and group chrome
// ---------------------------------------------------------------------------

/** Container hook for form-level issue summaries. */
export const FORM_ERRORS_ATTR = "data-formgen-form-errors";
/** Group containers (repeater items, fieldsets) with an invalid descendant. */
export const GROUP_INVALID_ATTR = "data-formgen-group-invalid";
const FORM_ERRORS_RENDERER_ATTR = "data-formgen-form-errors-renderer";
const DEFAULT_FORM_ERROR_RENDERER = "summary";

export interface FormErrorRenderContext {
  root: Document | HTMLElement;
  /** Existing summary container inside the root, when the markup rendered one. */
  container: HTMLElement | null;
  messages: string[];
}

export type FormErrorRenderer = (context: FormErrorRenderContext) => void;

const formErrorRenderers = new Map<string, FormErrorRenderer>();
formErrorRenderers.set(DEFAULT_FORM_ERROR_RENDERER, summaryFormErrorRenderer);

/** Replace the form-level summary presentation (messages stay host-localizable). */
export function registerFormErrorRenderer(name: string, renderer: FormErrorRenderer): void {
  if (!name || typeof renderer !== "function") {
    return;
  }
  formErrorRenderers.set(name, renderer);
}

export function findFormErrorContainer(root: Document | HTMLElement): HTMLElement | null {
  if (root instanceof HTMLElement && root.hasAttribute(FORM_ERRORS_ATTR)) {
    return root;
  }
  return root.querySelector<HTMLElement>(`[${FORM_ERRORS_ATTR}]`);
}

/** Render (or clear, with an empty list) the form-level issue summary for a root. */
export function renderFormErrors(root: Document | HTMLElement, messages: string[], rendererName?: string): void {
  const container = findFormErrorContainer(root);
  const name =
    rendererName || container?.getAttribute(FORM_ERRORS_RENDERER_ATTR) || DEFAULT_FORM_ERROR_RENDERER;
  const renderer =
    formErrorRenderers.get(name) ??
    formErrorRenderers.get(DEFAULT_FORM_ERROR_RENDERER) ??
    summaryFormErrorRenderer;
  renderer({ root, container, messages: messages.filter((message) => message && message.trim() !== "") });
}

/** Messages currently shown by the default summary renderer. */
export function currentFormErrors(root: Document | HTMLElement): string[] {
  const container = findFormErrorContainer(root);
  if (!container || container.hidden) {
    return [];
  }
  return Array.from(container.querySelectorAll("li")).map((item) => item.textContent ?? "").filter(Boolean);
}

function summaryFormErrorRenderer(context: FormErrorRenderContext): void {
  let target = context.container;
  if (!target) {
    if (context.messages.length === 0) {
      return;
    }
    const host = context.root instanceof Document ? context.root.body : context.root;
    if (!host) {
      return;
    }
    target = document.createElement("div");
    target.setAttribute(FORM_ERRORS_ATTR, "true");
    target.setAttribute("role", "alert");
    target.className = "formgen-form-errors";
    host.insertBefore(target, host.firstChild);
  }
  if (!target.hasAttribute("tabindex")) {
    target.tabIndex = -1;
  }
  let list = target.querySelector("ul");
  if (!list) {
    list = document.createElement("ul");
    target.appendChild(list);
  }
  while (list.firstChild) {
    list.removeChild(list.firstChild);
  }
  if (context.messages.length === 0) {
    target.hidden = true;
    return;
  }
  for (const message of context.messages) {
    const item = document.createElement("li");
    item.textContent = message;
    list.appendChild(item);
  }
  target.hidden = false;
}

/**
 * Mark repeater items and fieldsets that contain an invalid control so group
 * chrome can reflect nested issues; clear the marker when none remain.
 */
export function syncInvalidGroups(root: Document | HTMLElement): void {
  const groups = new Set<HTMLElement>();
  root.querySelectorAll<HTMLElement>(`[${GROUP_INVALID_ATTR}]`).forEach((group) => groups.add(group));
  root.querySelectorAll<HTMLElement>('[data-validation-state="invalid"]').forEach((control) => {
    let node: HTMLElement | null = control.parentElement;
    while (node && node !== root) {
      if (node.matches("[data-formgen-array-item], fieldset")) {
        groups.add(node);
      }
      node = node.parentElement;
    }
  });
  groups.forEach((group) => {
    if (group.querySelector('[data-validation-state="invalid"]')) {
      group.setAttribute(GROUP_INVALID_ATTR, "true");
    } else {
      group.removeAttribute(GROUP_INVALID_ATTR);
    }
  });
}
