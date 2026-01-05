import { autoSlug } from "./auto-slug";
import { autoResize } from "./auto-resize";
import { initBehaviors as initBehaviorsCore, registerBehavior, resetBehaviorRegistry } from "./registry";
import type { BehaviorInitResult } from "./registry";
import { slugify } from "./utils";
import { initIcons, registerIconProvider, __resetIconProvidersForTests } from "../icons";
import { initJSONEditors } from "../editors";

registerDefaults();

function registerDefaults(): void {
  registerBehavior("autoSlug", autoSlug);
  registerBehavior("autoResize", autoResize);
}

export function initBehaviors(root: Document | HTMLElement = document): BehaviorInitResult {
  const result = initBehaviorsCore(root);
  initIcons(root);
  initJSONEditors();
  return result;
}

export { registerBehavior, registerIconProvider, initIcons, initJSONEditors, slugify, autoSlug, autoResize };
export type { BehaviorContext, BehaviorFactory } from "./types";
export type { BehaviorInitResult } from "./registry";

export function __resetBehaviorsForTests(): void {
  resetBehaviorRegistry();
  __resetIconProvidersForTests();
  registerDefaults();
}
