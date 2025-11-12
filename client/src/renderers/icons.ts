import type { ClassToken } from "../theme/classes";
import { setElementClasses, addElementClasses } from "../theme/classes";

export interface IconConfig {
  name?: string;
  source?: string;
  raw?: string;
}

export interface IconRenderOptions {
  wrapperClasses?: ClassToken[];
  svgClasses?: ClassToken[];
}

export function readIconConfig(element: HTMLElement): IconConfig | null {
  const { icon, iconSource, iconRaw } = element.dataset;
  if (!icon && !iconSource && !iconRaw) {
    return null;
  }
  const config: IconConfig = {};
  if (icon) {
    config.name = icon;
  }
  if (iconSource) {
    config.source = iconSource;
  }
  if (iconRaw) {
    config.raw = iconRaw;
  }
  if (!config.name && !config.raw) {
    return null;
  }
  return config;
}

export function createIconElement(
  config: IconConfig | null,
  options: IconRenderOptions = {}
): HTMLElement | null {
  if (!config) {
    return null;
  }

  const wrapper = document.createElement("span");
  wrapper.setAttribute("aria-hidden", "true");
  setElementClasses(wrapper, options.wrapperClasses ?? []);

  if (config.raw) {
    wrapper.innerHTML = config.raw;
    if (options.svgClasses && options.svgClasses.length > 0) {
      const svg = wrapper.querySelector("svg");
      if (svg) {
        addElementClasses(svg, options.svgClasses);
      }
    }
    return wrapper;
  }

  if (config.name) {
    const fallback = document.createElement("span");
    fallback.textContent = config.name.slice(0, 2).toUpperCase();
    fallback.className = "text-xs font-semibold text-current";
    wrapper.appendChild(fallback);
    return wrapper;
  }

  return null;
}
