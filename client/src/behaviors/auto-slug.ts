import type { BehaviorFactory } from "./types";
import { findFieldInput, findNearestInput, slugify } from "./utils";

interface AutoSlugConfig {
  source?: string;
}

export const autoSlug: BehaviorFactory = ({ element, config, root }) => {
  const target = findNearestInput(element);
  if (!target) {
    console.warn("[formgen:behaviors] autoSlug requires an input or textarea target.");
    return;
  }

  const options = normaliseConfig(config);
  if (!options.source) {
    console.warn("[formgen:behaviors] autoSlug config must define a source field.");
    return;
  }

  const source = findFieldInput(root, options.source);
  if (!source) {
    console.warn(`[formgen:behaviors] source field "${options.source}" not found for autoSlug.`);
    return;
  }

  let syncing = false;
  let manual = element.getAttribute("data-behavior-state") === "manual";

  if (!manual && target.value.trim().length > 0) {
    manual = true;
    element.setAttribute("data-behavior-state", "manual");
  }

  const updateSlug = () => {
    if (manual) {
      return;
    }
    const nextValue = slugify(source.value || "");
    if (nextValue === target.value) {
      return;
    }
    syncing = true;
    target.value = nextValue;
    target.dispatchEvent(new Event("input", { bubbles: true }));
    syncing = false;
  };

  const handleSourceInput = () => {
    updateSlug();
  };

  const handleTargetInput = (event: Event) => {
    if (syncing) {
      return;
    }
    const trimmed = target.value.trim();
    if (trimmed.length === 0) {
      manual = false;
      element.removeAttribute("data-behavior-state");
      updateSlug();
      return;
    }
    manual = true;
    element.setAttribute("data-behavior-state", "manual");
  };

  source.addEventListener("input", handleSourceInput);
  target.addEventListener("input", handleTargetInput);

  updateSlug();

  return () => {
    source.removeEventListener("input", handleSourceInput);
    target.removeEventListener("input", handleTargetInput);
  };
};

function normaliseConfig(config: unknown): AutoSlugConfig {
  if (typeof config === "string") {
    return { source: config };
  }
  if (config && typeof config === "object") {
    const record = config as Record<string, unknown>;
    const source = typeof record.source === "string" ? record.source : undefined;
    return { source };
  }
  return {};
}
