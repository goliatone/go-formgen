import { autoSlug } from "./auto-slug";
import { initBehaviors, registerBehavior, resetBehaviorRegistry } from "./registry";
import { slugify } from "./utils";

registerDefaults();

function registerDefaults(): void {
  registerBehavior("autoSlug", autoSlug);
}

export { initBehaviors, registerBehavior, slugify, autoSlug };
export type { BehaviorContext, BehaviorFactory } from "./types";
export type { BehaviorInitResult } from "./registry";

export function __resetBehaviorsForTests(): void {
  resetBehaviorRegistry();
  registerDefaults();
}
