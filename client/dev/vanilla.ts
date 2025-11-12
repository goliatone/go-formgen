import "preline/preline";
import "../src/theme/index.css";
import { initRelationships, autoInitWysiwyg, getThemeClasses } from "../src/index";
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

  // Auto-initialize WYSIWYG editors
  const theme = getThemeClasses();
  autoInitWysiwyg(theme.wysiwyg);
}

bootstrap().catch((error) => {
  console.error("[formgen:sandbox] failed to bootstrap vanilla sandbox:", error);
});
