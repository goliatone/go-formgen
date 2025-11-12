import { fileUploaderFactory } from "./file-uploader";

type Teardown = (() => void) | void;

export interface ComponentContext {
  element: HTMLElement;
  config?: Record<string, unknown>;
  root: HTMLElement;
}

export type ComponentFactory = (context: ComponentContext) => Teardown;

interface InstanceRecord {
  name: string;
  teardown?: () => void;
}

const factories = new Map<string, ComponentFactory>();
let instances = new WeakMap<HTMLElement, InstanceRecord>();

registerDefaultComponents();

export function registerComponent(name: string, factory: ComponentFactory): void {
  const normalized = normalize(name);
  if (!normalized || typeof factory !== "function") {
    return;
  }
  factories.set(normalized, factory);
}

export function initComponents(root: Document | HTMLElement = document): void {
  const elements = collectComponentRoots(root);
  for (const element of elements) {
    const name = normalize(element.dataset.component ?? "");
    if (!name) {
      continue;
    }

    const previous = instances.get(element);
    if (previous && previous.name === name) {
      continue;
    }

    if (previous?.teardown) {
      previous.teardown();
      instances.delete(element);
    }

    const factory = factories.get(name);
    if (!factory) {
      continue;
    }

    const config = parseConfig(element.getAttribute("data-component-config"));
    const rootElement = resolveRootElement(element, root);

    const teardown = factory({ element, config, root: rootElement });
    instances.set(element, {
      name,
      teardown: typeof teardown === "function" ? teardown : undefined,
    });
  }
}

export function __resetComponentRegistryForTests(): void {
  factories.clear();
  instances = new WeakMap();
  registerDefaultComponents();
}

function collectComponentRoots(root: Document | HTMLElement): HTMLElement[] {
  const scope = root instanceof Document ? root : root;
  const elements = Array.from(scope.querySelectorAll<HTMLElement>("[data-component]"));
  if (root instanceof HTMLElement && root.hasAttribute("data-component")) {
    elements.unshift(root);
  }
  return elements;
}

function parseConfig(raw: string | null): Record<string, unknown> | undefined {
  if (!raw) {
    return undefined;
  }
  try {
    const parsed = JSON.parse(raw);
    return typeof parsed === "object" && parsed !== null ? (parsed as Record<string, unknown>) : undefined;
  } catch (error) {
    console.warn("formgen: failed to parse component config", error);
    return undefined;
  }
}

function resolveRootElement(element: HTMLElement, root: Document | HTMLElement): HTMLElement {
  const formRoot = element.closest<HTMLElement>("[data-formgen-auto-init]");
  if (formRoot) {
    return formRoot;
  }
  if (root instanceof HTMLElement) {
    return root;
  }
  return root.body ?? element.ownerDocument?.body ?? element;
}

function normalize(name: string): string {
  return name.trim().toLowerCase();
}

function registerDefaultComponents(): void {
  if (!factories.has("datetime-range")) {
    factories.set("datetime-range", datetimeRangeFactory);
  }
  if (!factories.has("file_uploader")) {
    factories.set("file_uploader", fileUploaderFactory);
  }
}

function datetimeRangeFactory({ element }: ComponentContext): Teardown {
  const inputs = Array.from(
    element.querySelectorAll<HTMLInputElement>("input[type='datetime-local'], input[type='date'], input[type='time']"),
  );

  if (inputs.length < 2) {
    return;
  }

  const [start, end] = inputs;

  const sync = () => {
    if (!start.value) {
      return;
    }
    if (!end.value || end.value < start.value) {
      end.value = start.value;
    }
  };

  const cleanup = () => {
    start.removeEventListener("change", sync);
    end.removeEventListener("change", sync);
  };

  start.addEventListener("change", sync);
  end.addEventListener("change", sync);
  sync();

  return cleanup;
}
