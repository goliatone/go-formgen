import type { BehaviorFactory, BehaviorTeardown } from "./types";
import {
  collectBehaviorElements,
  normalizeBehaviorName,
  parseBehaviorConfig,
  parseBehaviorNames,
  resolveRootElement,
  selectBehaviorConfig,
} from "./utils";

type DisposeFn = (() => void) | undefined;

interface BehaviorRecord {
  element: HTMLElement;
  name: string;
  dispose?: DisposeFn;
}

export interface BehaviorInitResult {
  dispose(): void;
  records: BehaviorRecord[];
}

const factories = new Map<string, BehaviorFactory>();
let instances = new WeakMap<HTMLElement, Map<string, DisposeFn>>();

export function registerBehavior(name: string, factory: BehaviorFactory): void {
  const normalized = normalizeBehaviorName(name);
  if (!normalized || typeof factory !== "function") {
    return;
  }
  factories.set(normalized, factory);
}

export function initBehaviors(root: Document | HTMLElement = document): BehaviorInitResult {
  const elements = collectBehaviorElements(root);
  const records: BehaviorRecord[] = [];

  for (const element of elements) {
    const names = parseBehaviorNames(element.getAttribute("data-behavior"));
    if (names.length === 0) {
      continue;
    }
    const configPayload = parseBehaviorConfig(element.getAttribute("data-behavior-config"));
    const scopeRoot = resolveRootElement(element, root);

    for (const rawName of names) {
      const normalized = normalizeBehaviorName(rawName);
      if (!normalized) {
        continue;
      }

      if (hasActiveInstance(element, normalized)) {
        continue;
      }

      const factory = factories.get(normalized);
      if (!factory) {
        console.warn(`[formgen:behaviors] behavior "${normalized}" is not registered.`);
        continue;
      }

      const contextConfig = selectBehaviorConfig(configPayload, normalized, names.length);
      const dispose = invokeFactory(factory, {
        element,
        name: normalized,
        root: scopeRoot,
        config: contextConfig,
      });

      setActiveInstance(element, normalized, dispose);
      records.push({ element, name: normalized, dispose });
    }
  }

  return {
    records,
    dispose: () => {
      for (const record of records.splice(0)) {
        if (record.dispose) {
          try {
            record.dispose();
          } catch (error) {
            console.warn(`[formgen:behaviors] dispose failed for ${record.name}:`, error);
          }
        }
        clearActiveInstance(record.element, record.name);
      }
    },
  };
}

export function resetBehaviorRegistry(): void {
  factories.clear();
  instances = new WeakMap();
}

function invokeFactory(factory: BehaviorFactory, context: Parameters<BehaviorFactory>[0]): DisposeFn {
  let teardown: BehaviorTeardown;
  try {
    teardown = factory(context);
  } catch (error) {
    console.warn(`[formgen:behaviors] factory for "${context.name}" failed:`, error);
    return undefined;
  }
  if (typeof teardown === "function") {
    return teardown;
  }
  if (teardown && typeof teardown === "object" && typeof teardown.dispose === "function") {
    return () => teardown.dispose();
  }
  return undefined;
}

function getInstanceMap(element: HTMLElement): Map<string, DisposeFn> {
  let map = instances.get(element);
  if (!map) {
    map = new Map();
    instances.set(element, map);
  }
  return map;
}

function hasActiveInstance(element: HTMLElement, name: string): boolean {
  const map = instances.get(element);
  return map ? map.has(name) : false;
}

function setActiveInstance(element: HTMLElement, name: string, dispose: DisposeFn): void {
  getInstanceMap(element).set(name, dispose);
}

function clearActiveInstance(element: HTMLElement, name: string): void {
  const map = instances.get(element);
  if (!map) {
    return;
  }
  map.delete(name);
  if (map.size === 0) {
    instances.delete(element);
  }
}
