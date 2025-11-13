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

const FIELD_CONTAINER_SELECTOR = "[data-relationship-type]";
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

  if (context.message && context.message.trim() !== "") {
    target.textContent = context.message;
    target.removeAttribute("aria-hidden");
    markElementInvalid(context.element, context.message);
  } else {
    target.textContent = "";
    target.setAttribute("aria-hidden", "true");
    clearInvalidState(context.element);
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
