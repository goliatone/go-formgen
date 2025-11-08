import "preline/preline";
import "../src/theme/index.css";
import { initRelationships, renderSwitch, autoInitWysiwyg, getThemeClasses } from "../src/index";
import { initBehaviors } from "../src/behaviors";
import { installMockApi } from "./mock-api";
import { vanillaFormHtml } from "./templates";

function renderVanillaMarkup(container: HTMLElement): void {
  const template = document.createElement("template");
  template.innerHTML = vanillaFormHtml.trim();

  const fragment = template.content.cloneNode(true);
  container.replaceChildren();
  container.appendChild(fragment);
}

function setupViewSelector(): void {
  const selector = document.getElementById("view-select") as HTMLSelectElement | null;
  if (!selector) {
    console.warn("[formgen:sandbox] view selector not found; skipping view binding.");
    return;
  }

  selector.value = "vanilla";
  selector.addEventListener("change", (event) => {
    const target = event.target as HTMLSelectElement;
    if (target.value === "preact") {
      window.location.href = "/preact/";
    }
  });
}

function setupSwitchDemos(): void {
  // Create a demo section for switches at the top of the form
  const host = document.getElementById("app");
  if (!host) return;

  const form = host.querySelector("form");
  if (!form) return;

  // Create demo section
  const demoSection = document.createElement("div");
  demoSection.className = "mb-8 p-6 bg-white rounded-lg border border-gray-200";
  demoSection.innerHTML = `
    <h3 class="text-lg font-semibold mb-4">Switch Component Demo</h3>
    <div class="space-y-4">
      <div class="flex items-center gap-x-3">
        <input type="checkbox" id="demo-switch-1" class="demo-checkbox">
        <label for="demo-switch-1" class="text-sm text-gray-800">Enable notifications</label>
      </div>
      <div class="flex items-center gap-x-3">
        <input type="checkbox" id="demo-switch-2" class="demo-checkbox" checked>
        <label for="demo-switch-2" class="text-sm text-gray-800">Auto-save (checked by default)</label>
      </div>
      <div class="flex items-center gap-x-3">
        <input type="checkbox" id="demo-switch-3" class="demo-checkbox" disabled>
        <label for="demo-switch-3" class="text-sm text-gray-500">Disabled switch</label>
      </div>
    </div>
  `;

  // Insert before the form
  form.parentElement?.insertBefore(demoSection, form);

  // Convert all demo checkboxes to switches
  const theme = getThemeClasses();
  const checkboxes = demoSection.querySelectorAll<HTMLInputElement>('.demo-checkbox');
  checkboxes.forEach((checkbox) => {
    renderSwitch(checkbox, theme.switch);
  });
}

function enhanceFeaturedSwitch(): void {
  const host = document.getElementById("app");
  if (!host) return;

  const featuredInput = host.querySelector<HTMLInputElement>("#fg-featured");
  if (!featuredInput || featuredInput.type !== "checkbox") {
    return;
  }

  const theme = getThemeClasses();
  renderSwitch(featuredInput, theme.switch);
}

async function bootstrap(): Promise<void> {
  const host = document.getElementById("app");
  if (!host) {
    throw new Error("[formgen:sandbox] expected #app container to exist.");
  }

  setupViewSelector();
  installMockApi();

  renderVanillaMarkup(host);

  await initRelationships({
    searchThrottleMs: 150,
    searchDebounceMs: 150,
  });
  initBehaviors();

  setupSwitchDemos();
  enhanceFeaturedSwitch();

  // Auto-initialize WYSIWYG editors
  const theme = getThemeClasses();
  autoInitWysiwyg(theme.wysiwyg);
}

bootstrap().catch((error) => {
  console.error("[formgen:sandbox] failed to bootstrap vanilla sandbox:", error);
});
