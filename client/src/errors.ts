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
  const container =
    context.element.closest(FIELD_CONTAINER_SELECTOR) ??
    context.element.parentElement ??
    context.element;
  if (!container) {
    return;
  }

  let target = container.querySelector<HTMLElement>(`[${ERROR_ATTR}]`);
  if (!target) {
    target = document.createElement("p");
    target.setAttribute(ERROR_ATTR, "true");
    target.className = "formgen-error text-sm text-red-600 dark:text-red-400";
    target.setAttribute("role", "status");
    target.setAttribute("aria-live", "polite");
    target.setAttribute("aria-atomic", "true");
    container.appendChild(target);
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
}

function clearInvalidState(element: HTMLElement): void {
  element.removeAttribute("aria-invalid");
  element.removeAttribute("data-validation-state");
  element.removeAttribute("data-validation-message");
}
