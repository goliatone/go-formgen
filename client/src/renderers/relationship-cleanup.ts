type StoreMap<T> = WeakMap<HTMLSelectElement, T>;

interface Registration<T> {
  renderer: string;
  stores: StoreMap<T>;
  onDestroy: (select: HTMLSelectElement, store: T) => void;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const registrations: Registration<any>[] = [];

let observer: MutationObserver | null = null;

export function registerRendererCleanup<T>(
  renderer: string,
  stores: StoreMap<T>,
  onDestroy: (select: HTMLSelectElement, store: T) => void
): void {
  registrations.push({ renderer, stores, onDestroy });
  ensureObserver();
}

export function destroyRendererWidgets(root: Document | HTMLElement): void {
  for (const registration of registrations) {
    destroyRegistrationStores(root, registration);
  }
}

function ensureObserver(): void {
  if (observer || typeof window === "undefined" || typeof document === "undefined") {
    return;
  }

  observer = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      mutation.removedNodes.forEach((node) => handleRemovedNode(node));
    }
  });

  observer.observe(document.documentElement, {
    childList: true,
    subtree: true,
  });
}

function handleRemovedNode(node: Node): void {
  const HTMLElementCtor =
    typeof globalThis !== "undefined"
      ? (globalThis as { HTMLElement?: typeof HTMLElement }).HTMLElement
      : undefined;
  if (typeof HTMLElementCtor !== "function" || !(node instanceof HTMLElementCtor)) {
    return;
  }

  for (const registration of registrations) {
    destroyRegistrationStores(node, registration);
  }
}

function destroyRegistrationStores<T>(root: Document | HTMLElement, registration: Registration<T>): void {
  const { renderer, stores, onDestroy } = registration;
  const selects = collectMatchingSelects(root, renderer);
  for (const select of selects) {
    const store = stores.get(select);
    if (store) {
      onDestroy(select, store);
      stores.delete(select);
    }
  }
}

function collectMatchingSelects(root: Document | HTMLElement, renderer: string): HTMLSelectElement[] {
  const HTMLSelectElementCtor =
    typeof globalThis !== "undefined"
      ? (globalThis as { HTMLSelectElement?: typeof HTMLSelectElement }).HTMLSelectElement
      : undefined;
  const results = new Set<HTMLSelectElement>();
  if (
    typeof HTMLSelectElementCtor === "function" &&
    root instanceof HTMLSelectElementCtor &&
    root.dataset.endpointRenderer === renderer
  ) {
    results.add(root);
  }
  root
    .querySelectorAll<HTMLSelectElement>(`select[data-endpoint-renderer='${renderer}']`)
    .forEach((select) => results.add(select));
  return Array.from(results);
}
