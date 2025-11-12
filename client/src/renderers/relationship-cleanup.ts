type StoreMap<T> = WeakMap<HTMLSelectElement, T>;

interface Registration<T> {
  renderer: string;
  stores: StoreMap<T>;
  onDestroy: (select: HTMLSelectElement, store: T) => void;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const registrations: Registration<any>[] = [];
const HAS_HTML_ELEMENT = typeof HTMLElement !== "undefined";

let observer: MutationObserver | null = null;

export function registerRendererCleanup<T>(
  renderer: string,
  stores: StoreMap<T>,
  onDestroy: (select: HTMLSelectElement, store: T) => void
): void {
  registrations.push({ renderer, stores, onDestroy });
  ensureObserver();
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
  if (!HAS_HTML_ELEMENT || !(node instanceof HTMLElement)) {
    return;
  }

  for (const registration of registrations) {
    const { renderer, stores, onDestroy } = registration;
    const selects = collectMatchingSelects(node, renderer);
    for (const select of selects) {
      const store = stores.get(select);
      if (store) {
        onDestroy(select, store);
        stores.delete(select);
      }
    }
  }
}

function collectMatchingSelects(node: HTMLElement, renderer: string): HTMLSelectElement[] {
  const results = new Set<HTMLSelectElement>();
  if (
    node instanceof HTMLSelectElement &&
    node.dataset.endpointRenderer === renderer
  ) {
    results.add(node);
  }
  node
    .querySelectorAll<HTMLSelectElement>(`select[data-endpoint-renderer='${renderer}']`)
    .forEach((select) => results.add(select));
  return Array.from(results);
}
