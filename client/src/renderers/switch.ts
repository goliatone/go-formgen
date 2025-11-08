/**
 * Switch Renderer
 *
 * Renders boolean fields as toggle switches instead of checkboxes.
 * Based on Preline UI switch component design.
 */

import { setElementClasses } from "../theme/classes.js";
import type { SwitchClassMap } from "../theme/classes.js";

// Force Vite to reload this module

export interface SwitchStore {
  input: HTMLInputElement;
  container: HTMLLabelElement;
  track: HTMLSpanElement;
  toggle: HTMLSpanElement;
  theme: SwitchClassMap;
}

const SWITCH_ROOT_ATTR = "data-fg-switch-root";

/**
 * Enhance a boolean input field as a toggle switch.
 *
 * @param input - The checkbox input element to enhance
 * @param theme - Theme classes for switch components
 */
export function renderSwitch(
  input: HTMLInputElement,
  theme: SwitchClassMap
): SwitchStore {
  if (input.type !== "checkbox") {
    throw new Error("Switch renderer requires input[type=checkbox]");
  }

  // Create container label
  const container = document.createElement("label");
  container.setAttribute(SWITCH_ROOT_ATTR, "true");
  setElementClasses(container, theme.container);

  // Set up the checkbox for accessibility
  setElementClasses(input, theme.input);

  // Create track (background)
  const track = document.createElement("span");
  setElementClasses(track, theme.track);

  // Create toggle (sliding circle)
  const toggle = document.createElement("span");
  setElementClasses(toggle, theme.toggle);

  // Replace original input with container FIRST
  const parent = input.parentElement;
  if (parent) {
    parent.replaceChild(container, input);
  }

  // Then assemble structure
  container.append(input, track, toggle);

  const store: SwitchStore = {
    input,
    container,
    track,
    toggle,
    theme,
  };

  // Set initial state
  updateSwitchState(store);

  // Listen for changes
  input.addEventListener("change", () => updateSwitchState(store));

  return store;
}

/**
 * Update visual state based on checkbox checked state
 */
function updateSwitchState(store: SwitchStore): void {
  // The CSS handles the visual state via peer-checked: pseudo-class
  // This function is available for any additional state management needed
  store.container.setAttribute("aria-checked", String(store.input.checked));
}

/**
 * Get the switch store for an input element
 */
export function getSwitchStore(input: HTMLInputElement): SwitchStore | null {
  const container = input.closest(`[${SWITCH_ROOT_ATTR}]`);
  if (!container) return null;

  const track = container.querySelector("span:first-of-type") as HTMLSpanElement;
  const toggle = container.querySelector("span:last-of-type") as HTMLSpanElement;

  if (!track || !toggle) return null;

  return {
    input,
    container: container as HTMLLabelElement,
    track,
    toggle,
    theme: {} as SwitchClassMap, // Would need to be reconstructed
  };
}
