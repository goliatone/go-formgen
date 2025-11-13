import "preline/preline";
import "../src/theme/index.css";
import {
  initRelationships,
  hydrateFormValues,
  autoInitWysiwyg,
  getThemeClasses,
  registerErrorRenderer,
  type ResolverRegistry,
} from "../src/index";
import { initBehaviors } from "../src/behaviors";
import { installMockApi } from "./mock-api";
import { vanillaFormHtml } from "./templates";
import { locateRelationshipFields } from "../src/dom";
import { clearFieldError } from "../src/errors";

const SAMPLE_RECORD_VALUES: Record<string, unknown> = {
  title: "Existing article title",
  slug: "existing-article-title",
  summary: "Updated teaser copy for the story.",
  tenant_id: "garden",
  status: "scheduled",
  read_time_minutes: 7,
  author_id: "1",
  manager_id: "m1",
  category_id: "news",
  tags: ["design", "ai"],
  related_article_ids: ["a1"],
  published_at: "2024-03-01T10:00:00Z",
  "cta.headline": "Ready to dig deeper?",
  "cta.url": "https://example.com/cta",
  "cta.button_text": "Explore guides",
  "seo.title": "Existing article title | Northwind Editorial",
  "seo.description": "Updated description for SEO block.",
};

const RESET_RECORD_VALUES: Record<string, unknown> = deriveResetValues(SAMPLE_RECORD_VALUES);

const SAMPLE_SERVER_ERRORS: Record<string, string[]> = {
  slug: ["Slug already taken"],
  manager_id: ["Manager must belong to the selected author"],
  tags: ["Select at least one tag", "Tags must match the tenant"],
  title: ["Title cannot be empty"],
  related_article_ids: ["Replace duplicate related articles"],
};

const CLEAR_SERVER_ERRORS: Record<string, string[]> = deriveClearErrors(SAMPLE_SERVER_ERRORS);

const toggleState = {
  recordLoaded: false,
  errorsInjected: false,
};

function deriveResetValues(values: Record<string, unknown>): Record<string, unknown> {
  const reset: Record<string, unknown> = {};
  Object.entries(values).forEach(([key, value]) => {
    reset[key] = Array.isArray(value) ? [] : null;
  });
  return reset;
}

function deriveClearErrors(errors: Record<string, string[]>): Record<string, string[]> {
  const cleared: Record<string, string[]> = {};
  Object.keys(errors).forEach((key) => {
    cleared[key] = [];
  });
  return cleared;
}

function registerDemoErrorRenderer(): void {
  registerErrorRenderer("banner", ({ element, message }) => {
    const container =
      element.closest("[data-relationship-type]") ?? element.parentElement ?? element;
    if (!container) {
      return;
    }

    let banner = container.querySelector<HTMLElement>("[data-demo-error-banner]");
    if (!banner) {
      banner = document.createElement("div");
      banner.setAttribute("data-demo-error-banner", "true");
      banner.setAttribute("role", "status");
      banner.setAttribute("aria-live", "polite");
      banner.className =
        "mt-2 flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700";
      const icon = document.createElement("span");
      icon.setAttribute("aria-hidden", "true");
      icon.textContent = "⚠️";
      const text = document.createElement("span");
      text.className = "flex-1";
      banner.append(icon, text);
      container.appendChild(banner);
    }

    const textNode = banner.querySelector("span:last-child");
    if (!message || message.trim() === "") {
      banner.setAttribute("hidden", "true");
      if (textNode) {
        textNode.textContent = "";
      }
      element.removeAttribute("aria-invalid");
      return;
    }

    banner.removeAttribute("hidden");
    if (textNode) {
      textNode.textContent = message;
    }
    element.setAttribute("aria-invalid", "true");
  });
}

function createToolbar(): HTMLElement {
  const toolbar = document.createElement("section");
  toolbar.dataset.sandboxToolbar = "true";
  toolbar.className =
    "rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-gray-700 dark:bg-slate-900";
  toolbar.innerHTML = `
    <div class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p class="text-sm font-semibold text-gray-900 dark:text-white">Sandbox controls</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">Toggle runtime behaviors while iterating on CSS.</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            data-sandbox-action="show-errors"
            class="inline-flex items-center rounded-md border border-transparent bg-rose-600 px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-rose-500 focus:outline-none focus:ring-2 focus:ring-rose-600 focus:ring-offset-2 dark:focus:ring-offset-slate-900"
          >
            Show validation errors
          </button>
          <button
            type="button"
            data-sandbox-action="clear-errors"
            class="inline-flex items-center rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:border-gray-600 dark:bg-slate-800 dark:text-gray-100"
          >
            Clear validation state
          </button>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            data-sandbox-action="load-record"
            class="inline-flex items-center rounded-md border border-indigo-200 bg-indigo-50 px-3 py-2 text-sm font-medium text-indigo-800 shadow-sm transition hover:bg-indigo-100 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 dark:border-indigo-500/50 dark:bg-slate-800 dark:text-indigo-100"
          >
            Load sample record
          </button>
          <button
            type="button"
            data-sandbox-action="inject-errors"
            class="inline-flex items-center rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 shadow-sm transition hover:bg-amber-100 focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 dark:border-amber-500/50 dark:bg-slate-800 dark:text-amber-100"
          >
            Inject server errors
          </button>
        </div>
      </div>
      <div data-sandbox-validation-summary class="rounded-md border border-dashed border-gray-300 bg-gray-50/60 px-3 py-2 text-xs text-gray-600 dark:border-gray-700 dark:bg-slate-800/70 dark:text-gray-300">
        Click “Show validation errors” to preview the current invalid fields.
      </div>
    </div>
  `;
  return toolbar;
}

function wireToolbarActions(toolbar: HTMLElement, registry: ResolverRegistry): void {
  toolbar.addEventListener("click", async (event) => {
    const trigger = (event.target as HTMLElement | null)?.closest<HTMLButtonElement>(
      "[data-sandbox-action]"
    );
    if (!trigger) {
      return;
    }
    const action = trigger.dataset.sandboxAction;
    if (!action) {
      return;
    }
    trigger.disabled = true;
    try {
      if (action === "show-errors") {
        await showValidationErrors(toolbar, registry);
      } else if (action === "clear-errors") {
        clearValidationState(toolbar);
      } else if (action === "load-record") {
        if (!toggleState.recordLoaded) {
          hydrateFormValues(document, { values: SAMPLE_RECORD_VALUES });
        } else {
          hydrateFormValues(document, { values: RESET_RECORD_VALUES });
        }
        toggleState.recordLoaded = !toggleState.recordLoaded;
        updateToggleLabel(toolbar, "load-record", toggleState.recordLoaded);
      } else if (action === "inject-errors") {
        if (!toggleState.errorsInjected) {
          hydrateFormValues(document, { errors: SAMPLE_SERVER_ERRORS });
        } else {
          hydrateFormValues(document, { errors: CLEAR_SERVER_ERRORS });
        }
        toggleState.errorsInjected = !toggleState.errorsInjected;
        updateToggleLabel(toolbar, "inject-errors", toggleState.errorsInjected);
      }
    } finally {
      trigger.disabled = false;
    }
  });
}

async function showValidationErrors(toolbar: HTMLElement, registry: ResolverRegistry): Promise<void> {
  const fields = locateRelationshipFields();
  const results = await Promise.all(fields.map((field) => registry.validate(field)));
  renderValidationSummary(toolbar, fields, results);
}

function clearValidationState(toolbar: HTMLElement): void {
  const fields = locateRelationshipFields();
  fields.forEach((field) => clearFieldError(field));
  renderValidationSummary(toolbar, [], []);
  toggleState.errorsInjected = false;
  updateToggleLabel(toolbar, "inject-errors", false);
}

function renderValidationSummary(
  toolbar: HTMLElement,
  fields: HTMLElement[],
  results: Array<{ valid: boolean; messages: string[] } | undefined>
): void {
  const summary = toolbar.querySelector<HTMLElement>("[data-sandbox-validation-summary]");
  if (!summary) {
    return;
  }

  const invalid: Array<{ label: string; message: string }> = [];
  fields.forEach((field, index) => {
    const result = results[index];
    if (!result || result.valid) {
      return;
    }
    const label = getFieldLabel(field);
    const message = result.messages[0] ?? "Invalid selection.";
    invalid.push({ label, message });
  });

  if (invalid.length === 0) {
    summary.innerHTML =
      '<span class="text-green-700 dark:text-green-400">No validation errors detected. Try clearing a required field and click “Show validation errors”.</span>';
    return;
  }

  const list = invalid
    .map(
      (item) =>
        `<li><strong class="text-gray-900 dark:text-white">${item.label}:</strong> <span class="text-gray-700 dark:text-gray-200">${item.message}</span></li>`
    )
    .join("");
  summary.innerHTML = `
    <p class="mb-1 font-semibold text-gray-900 dark:text-white">Validation summary (${invalid.length})</p>
    <ul class="list-disc space-y-1 pl-5 text-gray-700 dark:text-gray-200">${list}</ul>
  `;
}

function updateToggleLabel(toolbar: HTMLElement, action: string, active: boolean): void {
  const button = toolbar.querySelector<HTMLButtonElement>(`[data-sandbox-action="${action}"]`);
  if (!button) {
    return;
  }
  if (action === "load-record") {
    button.textContent = active ? "Clear sample record" : "Load sample record";
    return;
  }
  if (action === "inject-errors") {
    button.textContent = active ? "Clear server errors" : "Inject server errors";
  }
}

function getFieldLabel(element: HTMLElement): string {
  return (
    element.dataset.validationLabel ||
    element.getAttribute("aria-label") ||
    element.getAttribute("name") ||
    element.id ||
    "Field"
  );
}

function renderVanillaMarkup(container: HTMLElement): HTMLElement {
  const layout = document.createElement("div");
  layout.className = "space-y-4";

  const toolbar = createToolbar();
  layout.appendChild(toolbar);

  const template = document.createElement("template");
  template.innerHTML = vanillaFormHtml.trim();
  layout.appendChild(template.content.cloneNode(true));

  container.replaceChildren(layout);
  return toolbar;
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
  registerDemoErrorRenderer();

  const toolbar = renderVanillaMarkup(host);

  const registry = await initRelationships({
    searchThrottleMs: 150,
    searchDebounceMs: 150,
  });
  wireToolbarActions(toolbar, registry);
  initBehaviors();

  // Auto-initialize WYSIWYG editors
  const theme = getThemeClasses();
  autoInitWysiwyg(theme.wysiwyg);
}

bootstrap().catch((error) => {
  console.error("[formgen:sandbox] failed to bootstrap vanilla sandbox:", error);
});
