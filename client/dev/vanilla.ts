import "preline/preline";
import "../src/theme/index.css";
import {
  initRelationships,
  hydrateFormValues,
  autoInitWysiwyg,
  getThemeClasses,
  registerErrorRenderer,
  RELATIONSHIP_CREATE_ACTION_EVENT,
  type ResolverRegistry,
  type RelationshipCreateActionDetail,
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

function wireToolbarActions(
  toolbar: HTMLElement,
  registry: ResolverRegistry,
  form?: HTMLFormElement | null
): void {
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
        const nextState = !toggleState.recordLoaded;
        hydrateFormValues(document, {
          values: nextState ? SAMPLE_RECORD_VALUES : RESET_RECORD_VALUES,
        });
        toggleState.recordLoaded = nextState;
        toggleSampleRecordMethod(form, nextState);
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

function toggleSampleRecordMethod(form: HTMLFormElement | null | undefined, active: boolean): void {
  if (!form) {
    return;
  }
  rememberOriginalFormState(form);
  if (active) {
    applyFormMethod(form, "PATCH");
    form.dataset.sandboxMode = "edit";
  } else {
    restoreOriginalFormMethod(form);
    delete form.dataset.sandboxMode;
  }
}

function rememberOriginalFormState(form: HTMLFormElement): void {
  if (!form.dataset.sandboxOriginalMethod) {
    const declared = form.getAttribute("method");
    form.dataset.sandboxOriginalMethod = normalizeMethod(declared);
  }
  if (!form.dataset.sandboxOriginalOverride) {
    const override = form.querySelector<HTMLInputElement>('input[name="_method"]')?.value;
    if (override) {
      form.dataset.sandboxOriginalOverride = normalizeMethod(override, "POST");
    }
  }
}

function restoreOriginalFormMethod(form: HTMLFormElement): void {
  const originalOverride = form.dataset.sandboxOriginalOverride;
  if (originalOverride) {
    applyFormMethod(form, originalOverride);
    return;
  }
  const originalMethod = form.dataset.sandboxOriginalMethod ?? "POST";
  applyFormMethod(form, originalMethod);
}

function applyFormMethod(form: HTMLFormElement, method: string): void {
  const normalized = normalizeMethod(method, "POST");
  if (!normalized || normalized === "GET") {
    form.setAttribute("method", "get");
    removeMethodOverride(form);
    return;
  }
  if (normalized === "POST") {
    form.setAttribute("method", "post");
    removeMethodOverride(form);
    return;
  }

  form.setAttribute("method", "post");
  let hidden = form.querySelector<HTMLInputElement>('input[name="_method"]');
  if (!hidden) {
    hidden = document.createElement("input");
    hidden.type = "hidden";
    hidden.name = "_method";
    form.prepend(hidden);
  }
  hidden.value = normalized;
}

function removeMethodOverride(form: HTMLFormElement): void {
  const hidden = form.querySelector<HTMLInputElement>('input[name="_method"]');
  if (!hidden) {
    return;
  }
  if (form.dataset.sandboxOriginalOverride) {
    hidden.value = form.dataset.sandboxOriginalOverride;
    return;
  }
  hidden.remove();
}

function normalizeMethod(value: string | null | undefined, fallback: string = "POST"): string {
  if (!value) {
    return fallback;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return fallback;
  }
  const normalized = trimmed.toUpperCase();
  const allowed = new Set(["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]);
  if (allowed.has(normalized)) {
    return normalized;
  }
  if (normalized === "TRUE") {
    return "POST";
  }
  return fallback;
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

  // Sandbox-only: make tags creatable so the dev environment exercises the
  // `data-endpoint-allow-create` + `createOption` integration.
  const tags = container.querySelector<HTMLSelectElement>("#fg-tags");
  if (tags) {
    tags.dataset.endpointAllowCreate = "true";
  }

  // Sandbox-only: enable create-action on author field to demonstrate the
  // `data-endpoint-create-action` + `onCreateAction` / event integration.
  // This shows how typeahead fields can delegate creation to a modal/panel.
  const author = container.querySelector<HTMLSelectElement>("#fg-author_id");
  if (author) {
    author.dataset.endpointCreateAction = "true";
    author.dataset.endpointCreateActionLabel = "Create Author";
    author.dataset.endpointCreateActionId = "author";
  }

  // Sandbox-only: enable create-action on related articles (chips) to show
  // multi-select create action with append behavior.
  const relatedArticles = container.querySelector<HTMLSelectElement>("#fg-related_article_ids");
  if (relatedArticles) {
    relatedArticles.dataset.endpointCreateAction = "true";
    relatedArticles.dataset.endpointCreateActionLabel = "Create Article";
    relatedArticles.dataset.endpointCreateActionId = "article";
    relatedArticles.dataset.endpointCreateActionSelect = "append";
  }

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

/**
 * Demonstrates the create-action event flow for relationship fields.
 *
 * This listener handles `formgen:relationship:create-action` events dispatched
 * when the user clicks "Create ..." in a typeahead/chips dropdown (and no
 * `onCreateAction` hook is provided in `initRelationships`).
 *
 * In a real application, you would:
 * 1. Open a modal/panel based on `detail.actionId`
 * 2. Prefill the create form with `detail.query` if applicable
 * 3. After creation, inject the new option and select it
 *
 * This sandbox demo uses `window.prompt` to simulate the modal flow.
 */
function setupCreateActionListener(): void {
  document.addEventListener(RELATIONSHIP_CREATE_ACTION_EVENT, async (event) => {
    const detail = (event as CustomEvent<RelationshipCreateActionDetail>).detail;
    const { element, actionId, query, mode, selectBehavior } = detail;

    console.log("[sandbox] create-action triggered:", {
      actionId,
      query,
      mode,
      selectBehavior,
      fieldName: detail.field.name,
    });

    // Simulate a modal dialog using window.prompt
    // In production, you'd open a proper modal component here
    const defaultLabel =
      query.trim() ||
      (actionId === "author" ? "New Author" : actionId === "article" ? "New Article" : "New Item");

    const label = window.prompt(
      `[Sandbox Demo] Create ${actionId ?? "item"}:\n\nEnter a label for the new record (prefilled from query).`,
      defaultLabel
    );

    if (!label || label.trim() === "") {
      console.log("[sandbox] create-action cancelled by user");
      return;
    }

    // Generate a fake ID for the created record
    const newId = `created-${Date.now()}`;
    const newLabel = label.trim();

    console.log("[sandbox] simulating record creation:", { value: newId, label: newLabel });

    // Inject the new option into the select element
    if (element instanceof HTMLSelectElement) {
      const optionExists = Array.from(element.options).some((opt) => opt.value === newId);
      if (!optionExists) {
        const option = document.createElement("option");
        option.value = newId;
        option.textContent = newLabel;
        element.appendChild(option);
      }

      // Apply selection based on selectBehavior
      if (selectBehavior === "replace") {
        // Clear existing selection, then select new option
        Array.from(element.options).forEach((opt) => {
          opt.selected = opt.value === newId;
        });
      } else {
        // Append: just select the new option (keep existing selections for multi-select)
        const newOption = Array.from(element.options).find((opt) => opt.value === newId);
        if (newOption) {
          newOption.selected = true;
        }
      }

      // Dispatch change event to notify the runtime
      element.dispatchEvent(new Event("change", { bubbles: true }));

      console.log("[sandbox] create-action completed:", { value: newId, label: newLabel });
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
  setupCreateActionListener(); // Wire up create-action event handler

  const toolbar = renderVanillaMarkup(host);
  const form = host.querySelector("form");

  const registry = await initRelationships({
    searchThrottleMs: 150,
    searchDebounceMs: 150,
    createOption: async (context, query) => {
      // Demo: POST /api/tags with the same dynamic params used for fetching options.
      // The mock API returns `{value,label}`.
      const response = await fetch(context.request.url, {
        ...context.request.init,
        method: "POST",
        body: JSON.stringify({ label: query }),
      });
      if (!response.ok) {
        throw new Error(`Failed to create tag (${response.status})`);
      }
      const payload = (await response.json()) as { value: string; label: string };
      return { value: payload.value, label: payload.label };
    },
    // Note: We intentionally do NOT provide onCreateAction here so that the
    // DOM event is dispatched and handled by setupCreateActionListener() above.
    // If you want to handle create-action via hook instead of events, you can
    // provide onCreateAction here:
    //
    // onCreateAction: async (context, detail) => {
    //   const label = await openCreateModal(detail.actionId, detail.query);
    //   if (label) {
    //     return { value: `hook-${Date.now()}`, label };
    //   }
    // },
  });
  wireToolbarActions(toolbar, registry, form);
  initBehaviors();

  // Auto-initialize WYSIWYG editors
  const theme = getThemeClasses();
  autoInitWysiwyg(theme.wysiwyg);
}

bootstrap().catch((error) => {
  console.error("[formgen:sandbox] failed to bootstrap vanilla sandbox:", error);
});
