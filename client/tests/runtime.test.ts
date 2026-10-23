import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { h } from "preact";
import { render } from "preact";
import { act } from "preact/test-utils";
import { useEffect } from "preact/hooks";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  initRelationships,
  initFormgenRoot,
  hydrateFormValues,
  resetGlobalRegistry,
  resetComponentRegistryForTests,
  type ResolverEventDetail,
  type ResolverRegistry,
  registerErrorRenderer,
  attachFormController,
  registerComponent,
  registerFormErrorRenderer,
  initArrayRepeaters,
  encodeEnumValue,
  RESOLVER_SUPERSEDED_ABORT_REASON,
  RELATIONSHIP_EDIT_ACTION_EVENT,
  emitRelationshipEditAction,
} from "../src/index";
import { useRelationshipOptions } from "../src/frameworks/preact";
import { setGlobalConfig, getGlobalConfig } from "../src/config";
import { datasetToEndpoint, datasetToFieldConfig } from "../src/relationship-config";
import embeddedFixture from "../../testdata/embedded/v1.json";
import type { EmbeddedModel } from "../src/embedded-contract";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

function readFixture<T>(name: string): T {
  const filePath = join(__dirname, "fixtures", name);
  return JSON.parse(readFileSync(filePath, "utf-8")) as T;
}

const fixtures = {
  simplified: readFixture<Array<{ value: string; label: string }>>("options.simplified.json"),
  envelope: readFixture<{ data: Array<Record<string, string>> }>("options.envelope.json"),
} as const;

type FetchResponder = Response & { __calledWith?: RequestInit };

const fetchSpy = vi.fn<Promise<Response>, [string, RequestInit | undefined]>();

function mockResponse(data: unknown, overrides?: Partial<Response>): FetchResponder {
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify(data),
    ...overrides,
  } as FetchResponder;
}

function mockFailure(status: number, body?: unknown): Response {
  return {
    ok: false,
    status,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  } as Response;
}

function createMarkup(extra?: string): HTMLSelectElement {
  document.body.innerHTML = `
    <form data-formgen-auto-init="true">
      <select id="project_owner" name="project[owner_id]" data-endpoint-url="/api/users" data-endpoint-method="GET" data-endpoint-label-field="full_name" data-endpoint-value-field="id" data-relationship-cardinality="one" ${
        extra ?? ""
      }>
        <option value="">Select an option</option>
      </select>
    </form>
  `;
  return document.getElementById("project_owner") as HTMLSelectElement;
}

function resetDom(): void {
  document.body.innerHTML = "";
  document.head.innerHTML = "";
}

async function flush(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe("runtime resolver", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    // Reset global registry to ensure test isolation
    resetGlobalRegistry();
    // Mock scrollIntoView for JSDOM (not implemented)
    Element.prototype.scrollIntoView = vi.fn();
  });

  afterEach(() => {
    resetDom();
    // Clean up global registry
    resetGlobalRegistry();
  });

  it("parses relationship edit-action metadata from endpoint dataset attributes", () => {
    const select = createMarkup(`
      data-endpoint-renderer="typeahead"
      data-endpoint-edit-action="true"
      data-endpoint-edit-action-id="author"
      data-endpoint-edit-action-label="Edit Author"
    `);

    const field = datasetToFieldConfig(select, select.dataset as Record<string, string>);

    expect(field.editAction).toBe(true);
    expect(field.editActionId).toBe("author");
    expect(field.editActionLabel).toBe("Edit Author");
  });

  it("emits relationship edit-action events with selected option detail", () => {
    const select = createMarkup();
    const endpoint = datasetToEndpoint(select.dataset as Record<string, string>);
    const field = datasetToFieldConfig(select, select.dataset as Record<string, string>);
    const listener = vi.fn();

    select.addEventListener(RELATIONSHIP_EDIT_ACTION_EVENT, listener);

    emitRelationshipEditAction(select, {
      element: select,
      field,
      endpoint,
      selectedValue: "author-1",
      selectedLabel: "Ada Lovelace",
      actionId: "author",
      mode: "typeahead",
    });

    expect(listener).toHaveBeenCalledTimes(1);
    const event = listener.mock.calls[0]?.[0] as CustomEvent;
    expect(event.detail).toMatchObject({
      element: select,
      field,
      endpoint,
      selectedValue: "author-1",
      selectedLabel: "Ada Lovelace",
      actionId: "author",
      mode: "typeahead",
    });
  });

  it("invokes global hooks and transforms options", async () => {
    createMarkup();
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    const beforeFetch = vi.fn();
    const afterFetch = vi.fn();
    const buildHeaders = vi.fn().mockResolvedValue({ Authorization: "Bearer 123" });
    const transformOptions = vi
      .fn()
      .mockImplementation((_ctx, options) =>
        options.map((option) => ({ ...option, label: option.label.toUpperCase() }))
      );

    await initRelationships({
      beforeFetch,
      afterFetch,
      buildHeaders,
      transformOptions,
    });

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(beforeFetch).toHaveBeenCalledTimes(1);
    expect(afterFetch).toHaveBeenCalledTimes(1);
    expect(buildHeaders).toHaveBeenCalledTimes(1);
    expect(transformOptions).toHaveBeenCalledTimes(1);

    const option = document.querySelector<HTMLSelectElement>("#project_owner option");
    expect(option?.textContent).toBe("ALICE");
  });

  it("preserves absolute URI schemes until beforeFetch rewrites them", async () => {
    const field = createMarkup();
    field.dataset.endpointUrl = "command-options://search.repair/indexes";
    field.dataset.endpointMethod = "POST";
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));
    let hookUrl = "";

    await initRelationships({
      beforeFetch: ({ request }) => {
        hookUrl = request.url;
        request.url = "/admin/debug/api/panels/commands/actions/resolve_options";
      },
    });

    expect(hookUrl).toBe("command-options://search.repair/indexes?format=options");
    expect(fetchSpy).toHaveBeenCalledWith(
      "/admin/debug/api/panels/commands/actions/resolve_options",
      expect.objectContaining({ method: "POST" })
    );
  });

  it("requests simplified options format by default", async () => {
    const field = createMarkup();
    let requestedUrl: string | undefined;
    fetchSpy.mockImplementation(async (url, init) => {
      requestedUrl = url;
      return mockResponse(fixtures.simplified);
    });

    await initRelationships();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(requestedUrl).toBe("/api/users?format=options");

    const option = field.querySelector<HTMLOptionElement>("option");
    expect(option?.value).toBe("1");
    expect(option?.textContent).toBe("Alice");
  });

  it("preserves rich dynamic option presentation", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse([
      { value: "active", label: "Active", description: "Available now", disabled: false, metadata: { tier: 1 } },
      { value: "retired", label: "Retired", disabled: true },
    ]));

    await initRelationships();

    const active = field.querySelector<HTMLOptionElement>('option[value="active"]')!;
    const retired = field.querySelector<HTMLOptionElement>('option[value="retired"]')!;
    expect(active.dataset.optionDescription).toBe("Available now");
    expect(active.dataset.optionMetadata).toBe('{"tier":1}');
    expect(retired.disabled).toBe(true);
  });

  it("handles legacy envelopes when resultsPath is provided", async () => {
    const field = createMarkup('data-endpoint-results-path="data"');
    let requestedUrl: string | undefined;
    fetchSpy.mockImplementation(async (url, init) => {
      requestedUrl = url;
      return mockResponse(fixtures.envelope);
    });

    await initRelationships();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(requestedUrl).toBe("/api/users");

    const option = field.querySelector<HTMLOptionElement>("option");
    expect(option?.value).toBe("1");
    expect(option?.textContent).toBe("Alice");
  });

  it("emits lifecycle events with resolver detail", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    const events: string[] = [];
    const details: ResolverEventDetail[] = [];
    field.addEventListener("formgen:relationship:loading", (event) => {
      events.push("loading");
      details.push((event as CustomEvent<ResolverEventDetail>).detail);
    });
    field.addEventListener("formgen:relationship:success", (event) => {
      events.push("success");
      details.push((event as CustomEvent<ResolverEventDetail>).detail);
    });

    await initRelationships();

    expect(events).toEqual(["loading", "success"]);
    expect(details[1].options?.[0].label).toBe("Alice");
    expect(field.getAttribute("data-state")).toBe("ready");
  });

  it("avoids duplicate fetches when cache is enabled", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    const registry = await initRelationships({ cache: { strategy: "memory", ttlMs: 1000 } });
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    fetchSpy.mockClear();
    fetchSpy.mockRejectedValue(new Error("network"));

    await registry.resolve(field);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("propagates search input to resolver requests", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tags" name="article[tags][]" multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-relationship-cardinality="many"></select>
      </form>
    `;

    fetchSpy.mockResolvedValue(mockResponse([]));

    await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });

    fetchSpy.mockClear();

    await flush();

    const chipContainer = document.querySelector<HTMLElement>("[data-fg-chip-root='true']");
    expect(chipContainer).not.toBeNull();

    const menu = chipContainer!.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]');
    expect(searchInput).not.toBeNull();
    searchInput!.value = "tag";
    searchInput!.dispatchEvent(new Event("input", { bubbles: true }));

    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const requestUrl = fetchSpy.mock.calls[0]?.[0];
    expect(requestUrl).toContain("q=tag");
  });

  it("filters chip menus during search and resets after clearing", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tags" name="article[tags][]" multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-relationship-cardinality="many"></select>
      </form>
    `;

    const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
    const select = document.getElementById("tags") as HTMLSelectElement;

    fetchSpy.mockResolvedValue(
      mockResponse([
        { value: "design", label: "Product Design" },
        { value: "ai", label: "AI Strategy" },
        { value: "ml", label: "Machine Learning" },
      ])
    );

    await registry.resolve(select);
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
    toggle.click();
    await flush();

    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;
    const menuItems = () =>
      Array.from(container.querySelectorAll<HTMLButtonElement>('[role="option"]'));

    expect(menuItems().length).toBe(3);

    searchInput.value = "ai";
    searchInput.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();

    const filtered = menuItems();
    expect(filtered.length).toBe(1);
    expect(filtered[0]?.dataset.value).toBe("ai");
    const highlight = filtered[0]?.querySelector("mark");
    expect(highlight?.classList.contains("bg-amber-100")).toBe(true);

    searchInput.value = "";
    searchInput.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();

    expect(menuItems().length).toBe(3);

    fetchSpy.mockClear();

    menuItems()[0]?.click();
    await flush();

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(select.getAttribute("data-endpoint-search-value")).toBe("");
    expect(searchInput.value).toBe("");
  });

  it("opens chips menu when typing without requiring extra click", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tags" name="article[tags][]" multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-relationship-cardinality="many"></select>
      </form>
    `;

    fetchSpy.mockResolvedValue(
      mockResponse([
        { value: "design", label: "Product Design" },
        { value: "ai", label: "AI Strategy" },
      ])
    );

    const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
    const select = document.getElementById("tags") as HTMLSelectElement;
    await registry.resolve(select);
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    // Search input is now inside the menu header
    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

    // Menu should be closed initially
    expect(menu.hidden).toBe(true);

    // Open the menu first (search input is inside the menu, so we need to open it)
    const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
    toggle.click();
    await flush();

    // Menu should open and focus search input
    expect(menu.hidden).toBe(false);

    // Close the menu to test typing behavior
    toggle.click();
    await flush();
    expect(menu.hidden).toBe(true);

    // Open and type into the input - menu should stay open
    toggle.click();
    await flush();
    searchInput.value = "des";
    searchInput.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();

    // Menu should still be open
    expect(menu.hidden).toBe(false);

    // Verify filtered options are shown
    const menuItems = Array.from(container.querySelectorAll<HTMLButtonElement>('[role="option"]'));
    expect(menuItems.length).toBe(1);
    expect(menuItems[0]?.dataset.value).toBe("design");
  });

  it("shows create option in chips menu when typing non-matching query", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tags" name="article[tags][]" multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-allow-create="true"
          data-endpoint-search-param="q"
          data-relationship-cardinality="many"></select>
      </form>
    `;

    fetchSpy.mockResolvedValue(mockResponse([]));

    await initRelationships({
      searchThrottleMs: 0,
      searchDebounceMs: 0,
      createOption: async (_context, query) => ({
        value: query.toLowerCase(),
        label: query,
      }),
    });
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const select = document.getElementById("tags") as HTMLSelectElement;
    const container = select.previousElementSibling as HTMLElement;
    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

    // Type a new tag name - menu should open and show create option
    searchInput.focus();
    searchInput.value = "NewTag";
    searchInput.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();

    // Menu should be open
    expect(menu.hidden).toBe(false);

    // Create button should be visible without extra click
    const createButton = container.querySelector<HTMLButtonElement>('[data-fg-create-option="true"]');
    expect(createButton).not.toBeNull();
    expect(createButton?.textContent).toContain("NewTag");
  });

  it("applies auth headers declared in metadata", async () => {
    document.head.innerHTML = '<meta name="formgen-auth" content="secret" />';
    createMarkup(
      'data-endpoint-auth-strategy="header" data-endpoint-auth-header="X-Auth-Token" data-endpoint-auth-source="meta:formgen-auth"'
    );

    let capturedInit: RequestInit | undefined;
    fetchSpy.mockImplementation(async (_url, init) => {
      capturedInit = init;
      return mockResponse([{ value: "1", label: "Alice" }]);
    });

    await initRelationships();
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(capturedInit).toBeDefined();
    expect(capturedInit?.headers).toMatchObject({ "X-Auth-Token": "secret" });
  });

  it("supports custom renderers", async () => {
    const field = createMarkup('data-endpoint-renderer="chips"');
    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

    const registry = await initRelationships();

    const renderer = vi.fn();
    registry.registerRenderer("chips", ({ element, options }) => {
      renderer(options);
      element.setAttribute("data-rendered", options[0]?.label ?? "");
    });

    await registry.resolve(field);

    expect(renderer).toHaveBeenCalled();
    expect(field.getAttribute("data-rendered")).toBe("Alice");
  });

  it("renders chips UI for has-many relationships", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tags" name="article[tags]" multiple data-endpoint-url="/api/tags" data-endpoint-method="GET" data-endpoint-renderer="chips" data-endpoint-submit-as="json" data-relationship-cardinality="many"></select>
      </form>
    `;
    const select = document.getElementById("tags") as HTMLSelectElement;

    fetchSpy.mockResolvedValue(
      mockResponse([
        { value: "design", label: "Product Design" },
        { value: "ai", label: "AI" },
        { value: "year", label: "2025" },
      ])
    );

    await initRelationships();
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    expect(container?.classList.contains("relative")).toBe(true);
    expect(container.classList.contains("flex")).toBe(true);

    const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]');
    expect(toggle).not.toBeNull();
    toggle!.click();
    await flush();

    const optionButton = container.querySelector<HTMLButtonElement>("[role='option'][data-value='design']");
    expect(optionButton).not.toBeNull();
    optionButton!.click();
    await flush();

    expect(Array.from(select.selectedOptions).map((item) => item.value)).toContain("design");

    const chip = container.querySelector("[data-fg-chip-value='design']");
    expect(chip).not.toBeNull();

    const remove = chip!.querySelector<HTMLButtonElement>("button[aria-label^='Remove']");
    expect(remove).not.toBeNull();
    remove!.click();
    await flush();

    expect(Array.from(select.selectedOptions).map((item) => item.value)).not.toContain("design");

    // Menu wrapper is what controls visibility, not the listbox
    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]');
    expect(menu?.hidden).toBe(true);
  });

  it("allows creating new tags in chips search mode", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="tags"
          name="tags"
          multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-allow-create="true"
          data-endpoint-search-param="q"
          data-endpoint-submit-as="json"
          data-relationship-cardinality="many"
        ></select>
      </form>
    `;
    const select = document.getElementById("tags") as HTMLSelectElement;

    fetchSpy.mockImplementation(async (url) => {
      if (url.includes("q=NewTag")) {
        return mockResponse([]);
      }
      return mockResponse([{ value: "typescript", label: "TypeScript" }]);
    });

    await initRelationships({
      searchThrottleMs: 0,
      searchDebounceMs: 0,
      createOption: async (_context, query) => ({
        value: query.toLowerCase(),
        label: query,
      }),
    });
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]');
    expect(toggle).not.toBeNull();
    toggle!.click();
    await flush();

    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]');
    expect(searchInput).not.toBeNull();
    searchInput!.focus();
    searchInput!.value = "NewTag";
    searchInput!.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    await flush();

    const createButton = container.querySelector<HTMLButtonElement>('[data-fg-create-option="true"]');
    expect(createButton).not.toBeNull();
    createButton!.click();
    await flush();
    await flush();

    expect(Array.from(select.selectedOptions).map((item) => item.value)).toContain("newtag");
    expect(container.querySelector("[data-fg-chip-value='newtag']")).not.toBeNull();

    const hidden = select.parentElement?.querySelector<HTMLInputElement>('[data-relationship-hidden] [data-relationship-json]');
    expect(hidden).not.toBeNull();
    expect(hidden?.value).toBe(JSON.stringify(["newtag"]));
  });

  it("creates on Enter from chips input when query is not a literal match", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="tags"
          name="tags"
          multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-allow-create="true"
          data-endpoint-search-param="q"
          data-endpoint-submit-as="json"
          data-relationship-cardinality="many"
        ></select>
      </form>
    `;
    const select = document.getElementById("tags") as HTMLSelectElement;

    const createOption = vi.fn(async (_context, query: string) => ({
      value: query.toLowerCase(),
      label: query,
    }));

    fetchSpy.mockImplementation(async (url) => {
      if (url.includes("q=Type")) {
        return mockResponse([{ value: "typescript", label: "TypeScript" }]);
      }
      return mockResponse([{ value: "typescript", label: "TypeScript" }]);
    });

    await initRelationships({
      searchThrottleMs: 0,
      searchDebounceMs: 0,
      createOption,
    });
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]');
    toggle!.click();
    await flush();

    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;
    searchInput.focus();
    searchInput.value = "Type";
    searchInput.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    await flush();

    searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flush();
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    expect(createOption).toHaveBeenCalledTimes(1);
    expect(Array.from(select.selectedOptions).map((item) => item.value)).toContain("type");
  });

  it("selects existing option on Enter from chips input when query is a literal match", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="tags"
          name="tags"
          multiple
          data-endpoint-url="/api/tags"
          data-endpoint-method="GET"
          data-endpoint-renderer="chips"
          data-endpoint-mode="search"
          data-endpoint-allow-create="true"
          data-endpoint-search-param="q"
          data-endpoint-submit-as="json"
          data-relationship-cardinality="many"
        ></select>
      </form>
    `;
    const select = document.getElementById("tags") as HTMLSelectElement;

    const createOption = vi.fn(async (_context, query: string) => ({
      value: query.toLowerCase(),
      label: query,
    }));

    fetchSpy.mockResolvedValue(mockResponse([{ value: "typescript", label: "TypeScript" }]));

    await initRelationships({
      searchThrottleMs: 0,
      searchDebounceMs: 0,
      createOption,
    });
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]');
    toggle!.click();
    await flush();

    const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
    const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;
    searchInput.focus();
    searchInput.value = "TypeScript";
    searchInput.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();
    await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flush();
    await flush();

    expect(createOption).toHaveBeenCalledTimes(0);
    expect(Array.from(select.selectedOptions).map((item) => item.value)).toContain("typescript");
  });

  it("respects chips defaults before fetching options", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="tags"
          name="article[tags][]"
          multiple
          data-endpoint-url="/api/tags"
          data-endpoint-renderer="chips"
          data-relationship-cardinality="many"
          data-relationship-current="[&quot;design&quot;,&quot;ai&quot;]"
          data-endpoint-refresh="manual"
        >
          <option value="">Select tags</option>
        </select>
      </form>
    `;
    const select = document.getElementById("tags") as HTMLSelectElement;
    fetchSpy.mockResolvedValue(mockResponse([]));

    await initRelationships();
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const chips = Array.from(
      select.previousElementSibling?.querySelectorAll<HTMLElement>("[data-fg-chip-value]") ?? []
    ).map((chip) => chip.getAttribute("data-fg-chip-value"));
    expect(chips).toEqual(["design", "ai"]);
  });

  it("renders typeahead UI for has-one relationships", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="category"
          name="article[category_id]"
          data-endpoint-url="/api/categories"
          data-endpoint-method="GET"
          data-endpoint-renderer="typeahead"
          data-endpoint-label-field="name"
          data-endpoint-value-field="id"
          data-endpoint-search-param="q"
          data-endpoint-mode="search"
          data-relationship-cardinality="one"
        ></select>
      </form>
    `;

    const select = document.getElementById("category") as HTMLSelectElement;

    fetchSpy.mockResolvedValue(
      mockResponse([
        { value: "eng", label: "Engineering" },
        { value: "culture", label: "Culture" },
      ])
    );

    const registry = await initRelationships();
    await registry.resolve(select);
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    expect(container?.classList.contains("relative")).toBe(true);
    expect(container.classList.contains("w-full")).toBe(true);
    expect(container.classList.contains("text-sm")).toBe(true);
    expect(container.classList.contains("block")).toBe(true);

    const input = container.querySelector<HTMLInputElement>('input[type="text"]');
    expect(input).not.toBeNull();

    input!.focus();
    await flush();

    const dropdown = container.querySelector<HTMLElement>('[role="listbox"]');
    expect(dropdown).not.toBeNull();
    expect(dropdown!.hidden).toBe(false);

    const optionButton = dropdown!.querySelector<HTMLButtonElement>("[data-value='culture']");
    expect(optionButton).not.toBeNull();
    optionButton!.click();
    await flush();

    expect(select.value).toBe("culture");
    expect(input!.value).toBe("Culture");
    expect(container.dataset.selectedValue).toBe("culture");
    expect(container.dataset.selectedId).toBe("culture");
    expect(input!.dataset.selectedValue).toBe("culture");
    expect(input!.dataset.selectedLabel).toBe("Culture");
  });

  it("prefills typeahead controls before fetching options", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager_id"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-renderer="typeahead"
          data-relationship-cardinality="one"
          data-relationship-current="manager-1"
          data-endpoint-refresh="manual"
        >
          <option value="">Select manager</option>
        </select>
      </form>
    `;
    const select = document.getElementById("manager_id") as HTMLSelectElement;
    fetchSpy.mockResolvedValue(mockResponse([]));

    await initRelationships();
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const input = select.previousElementSibling?.querySelector<HTMLInputElement>('input[type="text"]');
    expect(input?.value).toBe("manager-1");
  });

  it("prefills typeahead controls with seeded current labels", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager_id"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-renderer="typeahead"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-endpoint-hydrate-param="ids"
          data-relationship-cardinality="one"
          data-relationship-current='{"value":"manager-1","label":"Ada Lovelace"}'
        >
          <option value="">Select manager</option>
        </select>
      </form>
    `;
    const select = document.getElementById("manager_id") as HTMLSelectElement;
    fetchSpy.mockResolvedValue(mockResponse([]));

    await initRelationships();
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const input = select.previousElementSibling?.querySelector<HTMLInputElement>('input[type="text"]');
    expect(input?.value).toBe("Ada Lovelace");
    const container = select.previousElementSibling as HTMLElement;
    expect(container.dataset.selectedValue).toBe("manager-1");
    expect(input?.dataset.selectedValue).toBe("manager-1");
    expect(input?.dataset.selectedLabel).toBe("Ada Lovelace");
    expect(select.selectedOptions[0]?.value).toBe("manager-1");
    expect(select.selectedOptions[0]?.textContent).toBe("Ada Lovelace");
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("keeps seeded current object selected after default select options resolve", async () => {
    const select = createMarkup(
      `data-relationship-current='{"value":"manager-1","label":"Ada Lovelace"}'`
    );
    fetchSpy.mockResolvedValue(
      mockResponse([{ id: "manager-1", full_name: "Ada Lovelace" }])
    );

    await initRelationships();
    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(select.value).toBe("manager-1");
    expect(select.selectedOptions[0]?.textContent).toBe("Ada Lovelace");
  });

  it("prefills chips controls with seeded current labels", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="tags"
          name="article[tags][]"
          multiple
          data-endpoint-url="/api/tags"
          data-endpoint-renderer="chips"
          data-relationship-cardinality="many"
          data-relationship-current='[{"value":"design","label":"Design Systems"},{"value":"ai","label":"AI Research"}]'
          data-endpoint-refresh="manual"
        >
          <option value="">Select tags</option>
        </select>
      </form>
    `;
    const select = document.getElementById("tags") as HTMLSelectElement;
    fetchSpy.mockResolvedValue(mockResponse([]));

    await initRelationships();
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const chipLabels = Array.from(
      select.previousElementSibling?.querySelectorAll<HTMLElement>("[data-fg-chip-value]") ?? []
    ).map((chip) => chip.querySelector("span")?.textContent);
    expect(chipLabels).toEqual(["Design Systems", "AI Research"]);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("hydrates search current values with an explicit hydrate param", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager_id"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-renderer="typeahead"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-endpoint-hydrate-param="ids"
          data-relationship-cardinality="one"
          data-relationship-current="manager-1"
        >
          <option value="">Select manager</option>
        </select>
      </form>
    `;
    const select = document.getElementById("manager_id") as HTMLSelectElement;
    let requestedUrl = "";
    fetchSpy.mockImplementation(async (url) => {
      requestedUrl = String(url);
      return mockResponse([{ value: "manager-1", label: "Ada Lovelace" }]);
    });

    await initRelationships();
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(requestedUrl).toContain("ids=manager-1");
    expect(requestedUrl).not.toContain("q=manager-1");

    const input = select.previousElementSibling?.querySelector<HTMLInputElement>('input[type="text"]');
    expect(input?.value).toBe("Ada Lovelace");
  });

  it("hydrates search current values when blank search state is present", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager_id"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-renderer="typeahead"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-endpoint-search-value=""
          data-endpoint-hydrate-param="ids"
          data-relationship-cardinality="one"
          data-relationship-current="manager-1"
        >
          <option value="">Select manager</option>
        </select>
      </form>
    `;
    const select = document.getElementById("manager_id") as HTMLSelectElement;
    let requestedUrl = "";
    fetchSpy.mockImplementation(async (url) => {
      requestedUrl = String(url);
      return mockResponse([{ value: "manager-1", label: "Ada Lovelace" }]);
    });

    await initRelationships();
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(requestedUrl).toContain("ids=manager-1");
    expect(requestedUrl).not.toContain("q=");

    const input = select.previousElementSibling?.querySelector<HTMLInputElement>('input[type="text"]');
    expect(input?.value).toBe("Ada Lovelace");
  });

  it("does not auto-hydrate current values when refresh mode is manual", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager_id"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-renderer="typeahead"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
          data-endpoint-hydrate-param="ids"
          data-relationship-cardinality="one"
          data-relationship-current="manager-1"
          data-endpoint-refresh="manual"
        >
          <option value="">Select manager</option>
        </select>
      </form>
    `;

    await initRelationships();
    await flush();

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("preserves typed queries while relationship options resolve", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-method="GET"
          data-endpoint-renderer="typeahead"
          data-endpoint-label-field="name"
          data-endpoint-value-field="id"
          data-endpoint-search-param="q"
          data-endpoint-mode="search"
          data-relationship-cardinality="one"
        ></select>
      </form>
    `;

    const select = document.getElementById("manager") as HTMLSelectElement;

    let resolveFetch: (() => void) | undefined;
    const pending = new Promise<Response>((resolve) => {
      resolveFetch = () => resolve(mockResponse([{ value: "radia", label: "Radia Perlman" }]));
    });
    fetchSpy.mockImplementationOnce(() => pending);

    setGlobalConfig({ searchThrottleMs: 0, searchDebounceMs: 0 });
    await initRelationships();
    const container = select.previousElementSibling as HTMLElement;
    const input = container.querySelector<HTMLInputElement>('input[type="text"]');
    expect(input).not.toBeNull();

    input!.focus();
    input!.value = "Ra";
    input!.dispatchEvent(new Event("input", { bubbles: true }));
    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(input!.value).toBe("Ra");

    resolveFetch && resolveFetch();
    await flush();
    await flush();

    expect(input!.value).toBe("Ra");
  });

  it("aborts superseded resolver requests with a stable reason", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="manager"
          name="project[manager_id]"
          data-endpoint-url="/api/managers"
          data-endpoint-method="GET"
          data-endpoint-label-field="name"
          data-endpoint-value-field="id"
          data-endpoint-refresh="manual"
          data-relationship-cardinality="one"
        ></select>
      </form>
    `;

    const select = document.getElementById("manager") as HTMLSelectElement;
    const errorEvents: ResolverEventDetail[] = [];
    select.addEventListener("formgen:relationship:error", (event) => {
      errorEvents.push((event as CustomEvent<ResolverEventDetail>).detail);
    });

    let firstSignal: AbortSignal | undefined;
    let requestCount = 0;
    fetchSpy.mockImplementation((_url, init) => {
      requestCount += 1;
      const signal = init?.signal as AbortSignal | undefined;
      if (requestCount === 1) {
        firstSignal = signal;
        return new Promise<Response>((_resolve, reject) => {
          signal?.addEventListener(
            "abort",
            () => reject((signal as AbortSignal & { reason?: unknown }).reason),
            { once: true }
          );
        });
      }
      return Promise.resolve(mockResponse([{ value: "ada", label: "Ada Lovelace" }]));
    });

    const registry = await initRelationships();
    expect(fetchSpy).not.toHaveBeenCalled();

    const firstResolve = registry.resolve(select);
    await flush();
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    const secondResolve = registry.resolve(select);
    await secondResolve;
    await firstResolve;

    expect(firstSignal?.aborted).toBe(true);
    const reason = (firstSignal as AbortSignal & { reason?: { name?: string; message?: string } })
      ?.reason;
    expect(reason?.name).toBe("AbortError");
    expect(reason?.message).toBe(RESOLVER_SUPERSEDED_ABORT_REASON);
    expect(fetchSpy).toHaveBeenCalledTimes(2);
    expect(errorEvents).toHaveLength(0);
  });

  it("clears typeahead selections and re-issues search", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="publisher"
          name="article[publisher_id]"
          data-endpoint-url="/api/publishing-houses"
          data-endpoint-method="GET"
          data-endpoint-renderer="typeahead"
          data-endpoint-label-field="name"
          data-endpoint-value-field="id"
          data-endpoint-search-param="q"
          data-endpoint-mode="search"
          data-relationship-cardinality="one"
        ></select>
      </form>
    `;

    const select = document.getElementById("publisher") as HTMLSelectElement;

    fetchSpy.mockResolvedValue(
      mockResponse([
        { value: "atlas", label: "Atlas Press" },
        { value: "lumen", label: "Lumen House" },
      ])
    );

    setGlobalConfig({ searchThrottleMs: 0, searchDebounceMs: 0 });
    const registry = await initRelationships();
    await registry.resolve(select);
    await flush();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const container = select.previousElementSibling as HTMLElement;
    const input = container.querySelector<HTMLInputElement>('input[type="text"]');
    const dropdown = container.querySelector<HTMLElement>('[role="listbox"]');
    const clear = container.querySelector<HTMLButtonElement>('[aria-label="Clear selection"]');

    expect(input).not.toBeNull();
    expect(dropdown).not.toBeNull();
    expect(clear).not.toBeNull();
    expect(clear!.disabled).toBe(true);

    input!.focus();
    await flush();

    const optionButton = dropdown!.querySelector<HTMLButtonElement>("[data-value='lumen']");
    expect(optionButton).not.toBeNull();
    optionButton!.click();
    await flush();

    expect(select.value).toBe("lumen");
    expect(input!.value).toBe("Lumen House");
    expect(container.dataset.selectedValue).toBe("lumen");
    expect(clear!.disabled).toBe(false);

    const inputEvent = vi.fn();
    select.addEventListener("input", inputEvent);

    clear!.click();
    await flush();

    expect(select.value).toBe("");
    expect(input!.value).toBe("");
    expect(container.dataset.selectedValue).toBeUndefined();
    expect(input!.dataset.selectedValue).toBeUndefined();
    expect(input!.dataset.selectedLabel).toBeUndefined();
    expect(clear!.disabled).toBe(true);
    if (inputEvent.mock.calls.length === 0) {
      await flush();
    }
    expect(inputEvent).toHaveBeenCalled();
  });

  it("keeps readonly relationship enhancers non-interactive and synchronized", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="readonly_tags"
          name="article[tag_ids][]"
          multiple
          aria-readonly="true"
          data-readonly="true"
          data-endpoint-url="/api/tags"
          data-endpoint-renderer="chips"
          data-endpoint-refresh="manual"
          data-relationship-cardinality="many"
        >
          <option value="alpha" selected>Alpha</option>
          <option value="beta">Beta</option>
        </select>
        <select
          id="readonly_publisher"
          name="article[publisher_id]"
          aria-readonly="true"
          data-readonly="true"
          data-endpoint-url="/api/publishers"
          data-endpoint-renderer="typeahead"
          data-endpoint-refresh="manual"
          data-relationship-cardinality="one"
        >
          <option value="atlas" selected>Atlas Press</option>
          <option value="lumen">Lumen House</option>
        </select>
      </form>
    `;

    await initRelationships();
    await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
    await flush();

    const chipsSelect = document.getElementById("readonly_tags") as HTMLSelectElement;
    const chips = chipsSelect.previousElementSibling as HTMLElement;
    const chipToggle = chips.querySelector<HTMLButtonElement>(
      'button[aria-haspopup="listbox"]',
    );
    const chipRemove = chips.querySelector<HTMLButtonElement>(
      "[data-fg-chip-value] button",
    );
    expect(chips.getAttribute("aria-readonly")).toBe("true");
    expect(chipToggle?.disabled).toBe(true);
    expect(chipRemove?.disabled).toBe(true);
    chipRemove?.click();
    expect(Array.from(chipsSelect.selectedOptions).map((option) => option.value)).toEqual([
      "alpha",
    ]);

    const typeaheadSelect = document.getElementById(
      "readonly_publisher",
    ) as HTMLSelectElement;
    const typeahead = typeaheadSelect.previousElementSibling as HTMLElement;
    const input = typeahead.querySelector<HTMLInputElement>('input[type="text"]');
    const typeaheadToggle = typeahead.querySelector<HTMLButtonElement>(
      'button[aria-haspopup="listbox"]',
    );
    const clear = typeahead.querySelector<HTMLButtonElement>(
      '[aria-label="Clear selection"]',
    );
    expect(typeahead.getAttribute("aria-readonly")).toBe("true");
    expect(input?.readOnly).toBe(true);
    expect(typeaheadToggle?.disabled).toBe(true);
    expect(clear?.disabled).toBe(true);
    input?.click();
    clear?.click();
    expect(typeaheadSelect.value).toBe("atlas");
    expect(
      typeahead.querySelector('[role="listbox"]')?.parentElement?.hasAttribute("hidden"),
    ).toBe(true);

    chipsSelect.removeAttribute("aria-readonly");
    chipsSelect.removeAttribute("data-readonly");
    typeaheadSelect.removeAttribute("aria-readonly");
    typeaheadSelect.removeAttribute("data-readonly");
    await flush();
    expect(chipToggle?.disabled).toBe(false);
    expect(input?.readOnly).toBe(false);
    expect(typeaheadToggle?.disabled).toBe(false);
  });

  it("mirrors relationship loading and terminal state on enhanced controls", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select
          id="stateful_tags"
          multiple
          data-endpoint-url="/api/tags"
          data-endpoint-renderer="chips"
          data-endpoint-refresh="manual"
          data-relationship-cardinality="many"
        ></select>
        <select
          id="stateful_publisher"
          data-endpoint-url="/api/publishers"
          data-endpoint-renderer="typeahead"
          data-endpoint-refresh="manual"
          data-relationship-cardinality="one"
        ></select>
      </form>
    `;

    await initRelationships();
    await flush();

    for (const id of ["stateful_tags", "stateful_publisher"]) {
      const select = document.getElementById(id) as HTMLSelectElement;
      const enhancer = select.previousElementSibling as HTMLElement;
      select.setAttribute("data-state", "loading");
      select.dispatchEvent(new CustomEvent("formgen:relationship:loading"));
      expect(enhancer.getAttribute("data-state")).toBe("loading");

      select.setAttribute("data-state", "error");
      select.dispatchEvent(new CustomEvent("formgen:relationship:error"));
      expect(enhancer.getAttribute("data-state")).toBe("error");
    }
  });

  it("permits custom resolvers to short-circuit fetch", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse([{ value: "server", label: "Server" }]));

    const registry = await initRelationships({ cache: { strategy: "none" } });
    fetchSpy.mockReset();
    fetchSpy.mockRejectedValue(new Error("should not fire"));

    registry.register("projects", {
      matches: (context) => context.endpoint.url?.includes("/users") ?? false,
      resolve: async () => [{ value: "local", label: "Local" }],
    });

    await registry.resolve(field);
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(field.querySelector("option")?.value).toBe("local");
  });

  it("bridges resolver state to the Preact hook", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

    const container = document.createElement("div");
    document.body.appendChild(container);

    const Harness = ({ element }: { element: HTMLElement }) => {
      const state = useRelationshipOptions(element);
      useEffect(() => {
        // Only call refresh if needed (defensive check)
        if (state.options.length === 0 && !state.loading && !state.error) {
          state.refresh();
        }
      }, [state.options.length, state.loading, state.error]);
      return h("div", {
        id: "state",
        "data-loading": String(state.loading),
        "data-error": state.error ? "true" : "false",
        "data-count": String(state.options.length),
      });
    };

    await act(async () => {
      render(h(Harness, { element: field }), container);
      await flush();
    });

    // Wait for hook to initialize registry and resolve
    await act(async () => {
      // Wait for the element to be resolved
      let attempts = 0;
      while (field.getAttribute("data-state") !== "ready" && attempts < 50) {
        await flush();
        attempts++;
      }
      // Give Preact time to process the state update
      await flush();
      await flush();
    });

    const stateNode = container.querySelector<HTMLDivElement>("#state");
    expect(stateNode?.getAttribute("data-count")).toBe("1");
    expect(stateNode?.getAttribute("data-loading")).toBe("false");
    expect(stateNode?.getAttribute("data-error")).toBe("false");
  });

  it("re-fetches when dependent fields change", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tenant" name="tenant_id">
          <option value="1" selected>Tenant 1</option>
          <option value="2">Tenant 2</option>
        </select>
        <select id="project_owner" name="project[owner_id]"
          data-endpoint-url="/api/users"
          data-endpoint-method="GET"
          data-relationship-cardinality="one"
          data-endpoint-dynamic-params-tenant-id="{{field:tenant_id}}"
          data-endpoint-refresh-on="tenant_id"
        ></select>
      </form>
    `;

    fetchSpy.mockResolvedValueOnce(mockResponse([{ value: "1", label: "Alice" }]));

    await initRelationships();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy.mock.calls[0][0]).toContain("tenant-id=1");

    fetchSpy.mockResolvedValueOnce(mockResponse([{ value: "2", label: "Bob" }]));

    const tenant = document.getElementById("tenant") as HTMLSelectElement;
    tenant.value = "2";
    tenant.dispatchEvent(new Event("change", { bubbles: true }));

    await flush();

    expect(fetchSpy.mock.calls[fetchSpy.mock.calls.length - 1][0]).toContain("tenant-id=2");
  });

  it("throttles search mode requests", async () => {
    vi.useFakeTimers();
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <input id="project_lookup" name="project_lookup"
          data-endpoint-url="/api/projects"
          data-endpoint-mode="search"
          data-endpoint-search-param="q"
        />
      </form>
    `;

    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Project" }]));

    await initRelationships();

    expect(fetchSpy).not.toHaveBeenCalled();

    const lookup = document.getElementById("project_lookup") as HTMLInputElement;
    lookup.value = "alpha";
    lookup.dispatchEvent(new Event("input", { bubbles: true }));

    await vi.advanceTimersByTimeAsync(300);
    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(fetchSpy.mock.calls[0][0]).toContain("q=alpha");

    vi.useRealTimers();
  });

  it("maintains JSON hidden inputs when submitAs json", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="project_tags" name="project[tag_ids][]" multiple
          data-endpoint-url="/api/tags"
          data-endpoint-submit-as="json"
          data-relationship-cardinality="many"
        ></select>
      </form>
    `;

    fetchSpy.mockResolvedValue(
      mockResponse([
        { value: "alpha", label: "Alpha" },
        { value: "beta", label: "Beta" },
      ])
    );

    await initRelationships();

    const select = document.getElementById("project_tags") as HTMLSelectElement;
    expect(select.getAttribute("name")).toBeNull();

    select.options[0].selected = true;
    select.dispatchEvent(new Event("change", { bubbles: true }));

    const hidden = select.parentElement?.querySelector<HTMLInputElement>(
      '[data-relationship-hidden] input[data-relationship-json]'
    );
    expect(hidden).toBeTruthy();
    expect(hidden?.name).toBe("project[tag_ids]");
    expect(hidden?.value).toBe("[\"alpha\"]");
  });

  it("retries failed requests before surfacing errors", async () => {
    createMarkup();
    fetchSpy.mockRejectedValueOnce(new Error("network"));
    fetchSpy.mockResolvedValueOnce(mockResponse([{ value: "1", label: "Alice" }]));

    await initRelationships();

    const attempts = fetchSpy.mock.calls.length;
    expect(attempts).toBeGreaterThanOrEqual(2);
    expect(attempts).toBeLessThanOrEqual((getGlobalConfig().retryAttempts ?? 0) + 1);
    const errorNode = document.querySelector("[data-relationship-error]");
    expect(errorNode?.textContent?.trim()).toBe("");
  });

  it("renders error UI when retries are exhausted", async () => {
    createMarkup();
    fetchSpy.mockResolvedValue(mockFailure(500, { error: "boom" }));

    await initRelationships();

    const field = document.getElementById("project_owner")!;
    expect(field.getAttribute("data-state")).toBe("error");
    const errorNode = document.querySelector<HTMLElement>("[data-relationship-error]");
    expect(errorNode?.textContent).toContain("status 500");
  });

  it("defers fetching when refresh mode is manual until triggered", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="project_owner" name="project[owner_id]"
          data-endpoint-url="/api/users"
          data-relationship-cardinality="one"
          data-endpoint-refresh="manual"
        ></select>
        <button type="button" data-endpoint-refresh-target="project[owner_id]">Reload</button>
      </form>
    `;

    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

    await initRelationships();

    expect(fetchSpy).not.toHaveBeenCalled();

    const button = document.querySelector<HTMLButtonElement>(
      '[data-endpoint-refresh-target="project[owner_id]"]'
    )!;
    button.click();

    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });

  it("validates required relationships and surfaces errors", async () => {
    const field = createMarkup('required data-validation-label="Project owner"');
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    const registry = await initRelationships();
    await flush();

    field.innerHTML = "";
    await registry.validate(field);

    const resolver = registry.get(field);
    expect(resolver?.state.validation?.valid).toBe(false);
    expect(field.getAttribute("data-validation-state")).toBe("invalid");
    const errorNode = document.querySelector<HTMLElement>("[data-relationship-error]");
    expect(errorNode?.textContent).toContain("Project owner is required.");
  });

  it("preserves server validation errors until manual validation clears them", async () => {
    const field = createMarkup(
      'required data-validation-label="Project owner" data-validation-state="invalid" data-validation-message="Server rejected this value"'
    );
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    const registry = await initRelationships();
    await flush();

    const resolver = registry.get(field);
    expect(resolver?.state.validation?.valid).toBe(false);
    expect(field.getAttribute("data-validation-state")).toBe("invalid");

    field.value = "1";
    field.dispatchEvent(new Event("change", { bubbles: true }));

    const result = await registry.validate(field);
    expect(result?.valid).toBe(true);
    expect(field.getAttribute("data-validation-state")).toBeNull();
  });

  it("supports custom validation hooks via registry.validate", async () => {
    const field = createMarkup('required data-validation-label="Project owner"');
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    const registry = await initRelationships({
      validateSelection: (_ctx, value) => {
        if (value === "2") {
          return {
            valid: false,
            messages: ["Select a different owner."],
            errors: [{ code: "custom", message: "Select a different owner." }],
          };
        }
        return { valid: true, messages: [], errors: [] };
      },
    });

    field.value = "2";
    field.dispatchEvent(new Event("change", { bubbles: true }));

    const result = await registry.validate(field);
    expect(result?.valid).toBe(false);
    const errorNode = document.querySelector<HTMLElement>("[data-relationship-error]");
    expect(errorNode?.textContent).toContain("Select a different owner.");
  });

  it("invokes onValidationError when validation fails", async () => {
    const field = createMarkup('required data-validation-label="Project owner"');
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));
    const onValidationError = vi.fn();

    const registry = await initRelationships({ onValidationError });
    await flush();

    field.innerHTML = "";
    await registry.validate(field);

    expect(onValidationError).toHaveBeenCalled();
    const [, error] = onValidationError.mock.calls[0];
    expect(error?.code).toBe("required");
  });

  it("uses custom error renderer when configured via data attributes", async () => {
    const field = createMarkup(
      'required data-validation-label="Project owner" data-validation-renderer="toast"'
    );
    fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));

    registerErrorRenderer("toast", ({ element, message }) => {
      let toast = document.querySelector<HTMLElement>(`[data-toast="${element.id}"]`);
      if (!toast) {
        toast = document.createElement("div");
        toast.setAttribute("data-toast", element.id);
        document.body.appendChild(toast);
      }
      toast.textContent = message ?? "";
    });

    const registry = await initRelationships();
    await flush();

    field.innerHTML = "";
    await registry.validate(field);

    const toast = document.querySelector<HTMLElement>(`[data-toast="${field.id}"]`);
    expect(toast?.textContent).toContain("Project owner is required.");
  });

  describe("hydrateFormValues", () => {
    it("applies relationship values from dotted paths", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="tags"
            name="article[tags][]"
            multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-relationship-cardinality="many"
            data-endpoint-refresh="manual"
          ></select>
        </form>
      `;
      const select = document.getElementById("tags") as HTMLSelectElement;
      fetchSpy.mockResolvedValue(mockResponse([]));

      await initRelationships();
      hydrateFormValues(document, {
        values: {
          "article.tags": [{ value: "design" }, "ai"],
        },
      });
      await flush();
      await new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const values = Array.from(select.selectedOptions).map((option) => option.value);
      expect(values).toEqual(["design", "ai"]);
    });

    it("applies server validation errors and clears them", async () => {
      const field = createMarkup('required data-validation-label="Project owner"');
      fetchSpy.mockResolvedValue(mockResponse(fixtures.simplified));
      const registry = await initRelationships();

      hydrateFormValues(document, {
        errors: {
          "project.owner_id": ["Server rejected"],
        },
      });

      const resolver = registry.get(field);
      expect(field.getAttribute("data-validation-state")).toBe("invalid");
      expect(resolver?.state.validation?.valid).toBe(false);

      hydrateFormValues(document, {
        errors: {
          "project.owner_id": [],
        },
      });

      expect(field.getAttribute("data-validation-state")).toBeNull();
      expect(resolver?.state.validation).toBeUndefined();
    });

    it("hydrates primitive inputs and selects without relationship metadata", () => {
      document.body.innerHTML = `
        <form>
          <input type="text" name="title" />
          <textarea name="summary"></textarea>
          <select name="status">
            <option value="">Select status</option>
            <option value="draft">Draft</option>
            <option value="published">Published</option>
          </select>
        </form>
      `;

      hydrateFormValues(document, {
        values: {
          title: "Existing article title",
          summary: "Prefilled summary",
          status: "published",
        },
      });

      const title = document.querySelector<HTMLInputElement>('input[name="title"]')!;
      const summary = document.querySelector<HTMLTextAreaElement>('textarea[name="summary"]')!;
      const status = document.querySelector<HTMLSelectElement>('select[name="status"]')!;

      expect(title.value).toBe("Existing article title");
      expect(summary.value).toBe("Prefilled summary");
      expect(status.value).toBe("published");
    });

    it("applies server errors to primitive inputs", () => {
      document.body.innerHTML = `
        <form>
          <div class="field-wrapper">
            <label for="slug">Slug</label>
            <input id="slug" name="slug" type="text" />
          </div>
        </form>
      `;

      hydrateFormValues(document, {
        errors: {
          slug: ["Slug already taken"],
        },
      });

      const slug = document.getElementById("slug") as HTMLInputElement;
      expect(slug.getAttribute("data-validation-state")).toBe("invalid");
      const inlineError = document.querySelector('[data-relationship-error="true"]');
      expect(inlineError?.textContent).toContain("Slug already taken");

      hydrateFormValues(document, {
        errors: {
          slug: [],
        },
      });

      expect(slug.getAttribute("data-validation-state")).toBeNull();
    });
  });

  describe("form controller", () => {
    it("collects, sets, resets, clears, focuses, and tears down", () => {
      document.body.innerHTML = `
        <div id="root" data-formgen-auto-init="true" data-formgen-render-mode="fields">
          <input name="article[title]" value="Draft" />
          <input type="hidden" name="_csrf" value="token" />
          <textarea name="article[summary]"></textarea>
          <input type="checkbox" name="article[published]" value="yes" />
          <input type="radio" name="article[status]" value="draft" checked />
          <input type="radio" name="article[status]" value="published" />
        </div>
      `;

      const root = document.getElementById("root")!;
      const controller = attachFormController(root);
      expect((globalThis as any).Formgen.attach).toBe(attachFormController);
      expect(controller.getValues()).toEqual({
        article: { title: "Draft", summary: "", published: false, status: "draft" },
        _csrf: "token",
      });

      controller.setValues({ article: { title: "Published", summary: "Done", published: true, status: "published" } });
      expect((document.querySelector('input[name="article[title]"]') as HTMLInputElement).value).toBe("Published");
      expect((document.querySelector('textarea[name="article[summary]"]') as HTMLTextAreaElement).value).toBe("Done");
      expect((document.querySelector('input[name="article[published]"]') as HTMLInputElement).checked).toBe(true);
      expect((document.querySelector('input[name="article[status]"][value="draft"]') as HTMLInputElement).checked).toBe(false);
      expect((document.querySelector('input[name="article[status]"][value="published"]') as HTMLInputElement).checked).toBe(true);

      controller.setErrors({ "article.title": ["Required"] });
      const title = document.querySelector<HTMLInputElement>('input[name="article[title]"]')!;
      expect(title.getAttribute("data-validation-state")).toBe("invalid");
      controller.reset();
      expect(controller.getValues()).toEqual({
        article: { title: "Draft", summary: "", published: false, status: "draft" },
        _csrf: "token",
      });
      expect(title.getAttribute("data-validation-state")).toBeNull();

      controller.setErrors({ "article.title": ["Required"] });
      controller.clearErrors(["article.title"]);
      expect(title.getAttribute("data-validation-state")).toBeNull();
      expect(controller.focus("article.title")).toBe(true);

      const changes = vi.fn();
      controller.onChange(changes);
      title.dispatchEvent(new Event("input", { bubbles: true }));
      expect(changes).toHaveBeenCalledTimes(1);
      controller.destroy();
      title.dispatchEvent(new Event("input", { bubbles: true }));
      expect(changes).toHaveBeenCalledTimes(1);
    });

    it("preserves relationship multi-select arrays instead of collecting native submission mirrors", async () => {
      document.body.innerHTML = `
        <div id="root" data-formgen-render-mode="fields">
          <select name="indexes" multiple data-endpoint-url="/api/indexes" data-endpoint-renderer="chips">
            <option value="site_content" selected>Site Content</option>
            <option value="archive_media">Archive Media</option>
          </select>
          <select name="json_indexes" multiple data-endpoint-url="/api/indexes" data-endpoint-renderer="chips" data-endpoint-submit-as="json">
            <option value="archive_media" selected>Archive Media</option>
          </select>
          <input type="hidden" name="_csrf" value="token" />
        </div>
      `;
      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "site_content", label: "Site Content" },
          { value: "archive_media", label: "Archive Media" },
        ])
      );

      const root = document.getElementById("root")!;
      const registry = await initFormgenRoot(root);
      const controller = attachFormController(root, { registry });

      expect(root.querySelectorAll('[data-relationship-hidden] input[type="hidden"]')).toHaveLength(2);
      expect(controller.getValues()).toEqual({
        indexes: ["site_content"],
        json_indexes: ["archive_media"],
        _csrf: "token",
      });
      expect(controller.getValues({ includeHidden: false })).toEqual({
        indexes: ["site_content"],
        json_indexes: ["archive_media"],
      });

      controller.destroy();
    });

    it("rejects prototype-polluting paths and only traverses own controller values", () => {
      delete (Object.prototype as Record<string, unknown>).polluted;
      delete (Object.prototype as Record<string, unknown>).injected;
      document.body.innerHTML = `
        <div id="root" data-formgen-render-mode="fields">
          <input name="__proto__[polluted]" value="yes" />
          <input name="constructor[prototype][injected]" value="yes" />
          <input name="article[toString]" value="own value" />
          <input name="article[title]" value="Draft" />
        </div>
      `;

      const root = document.getElementById("root")!;
      const controller = attachFormController(root);
      const values = controller.getValues();
      const article = values.article as Record<string, unknown>;

      expect((Object.prototype as Record<string, unknown>).polluted).toBeUndefined();
      expect((Object.prototype as Record<string, unknown>).injected).toBeUndefined();
      expect(Object.prototype.hasOwnProperty.call(article, "toString")).toBe(true);
      expect(article.toString).toBe("own value");
      expect(article.title).toBe("Draft");

      const unsafeValues = JSON.parse(
        '{"__proto__":{"polluted":"hydrated"},"constructor":{"prototype":{"injected":"hydrated"}},"article":{"title":"Published"}}'
      ) as Record<string, unknown>;
      controller.setValues(unsafeValues);
      expect((root.querySelector('input[name="article[title]"]') as HTMLInputElement).value).toBe("Published");
      expect((root.querySelector('input[name="__proto__[polluted]"]') as HTMLInputElement).value).toBe("yes");
      expect((Object.prototype as Record<string, unknown>).polluted).toBeUndefined();
      expect((Object.prototype as Record<string, unknown>).injected).toBeUndefined();

      controller.setErrors({ "__proto__.polluted": ["unsafe"], "article.title": ["required"] });
      expect(root.querySelector('input[name="__proto__[polluted]"]')?.getAttribute("data-validation-state")).toBeNull();
      expect(root.querySelector('input[name="article[title]"]')?.getAttribute("data-validation-state")).toBe("invalid");
      expect(controller.focus("__proto__.polluted")).toBe(false);

      controller.destroy();
      delete (Object.prototype as Record<string, unknown>).polluted;
      delete (Object.prototype as Record<string, unknown>).injected;
    });

    it("initializes and tears down resolvers for an embedded root", async () => {
      document.body.innerHTML = `
        <div id="root" data-formgen-render-mode="fields">
          <select name="source" data-endpoint-url="/api/sources">
            <option value="">Select a source</option>
          </select>
        </div>
      `;
      fetchSpy.mockResolvedValue(mockResponse([{ value: "one", label: "One" }]));

      const root = document.getElementById("root")!;
      const registry = await initFormgenRoot(root);
      const select = root.querySelector("select")!;
      expect(registry.get(select)).toBeDefined();

      const controller = attachFormController(root, { registry });
      controller.destroy();
      expect(registry.get(select)).toBeUndefined();
    });

    it("fully tears down and reinitializes the same embedded root", async () => {
      document.body.innerHTML = `
          <div id="root" data-formgen-render-mode="fields">
            <input name="kind" value="initial" />
            <select name="dependent" data-endpoint-url="/api/dependent" data-endpoint-refresh-on="kind"></select>
            <button type="button" data-endpoint-refresh-target="manual">Refresh manual</button>
            <select name="manual" data-endpoint-url="/api/manual" data-endpoint-refresh="manual"></select>
            <input name="search" data-endpoint-url="/api/search" data-endpoint-mode="search" data-endpoint-debounce="25" />
            <select name="tags" multiple required data-endpoint-renderer="chips">
              <option value="alpha">Alpha</option>
            </select>
            <select name="owner" required data-endpoint-url="/api/owners" data-endpoint-renderer="typeahead">
              <option value="one">One</option>
            </select>
            <div>
              <div data-formgen-array-items="true" data-formgen-array-name="rows" data-formgen-array-next-index="0" data-formgen-array-prototype-path="rows[0]" data-formgen-array-prototype-id-prefix="fg-rows-0">
                <template data-formgen-array-prototype="true"><div data-formgen-array-item="true"><input id="fg-rows-0-name" name="rows[0].name" /></div></template>
              </div>
              <button type="button" data-formgen-array-action="add">Add row</button>
            </div>
          </div>
      `;
      fetchSpy.mockResolvedValue(mockResponse([]));
      const root = document.getElementById("root")!;
      const kind = root.querySelector<HTMLInputElement>('[name="kind"]')!;
      const search = root.querySelector<HTMLInputElement>('[name="search"]')!;
      const manualTrigger = root.querySelector<HTMLButtonElement>('[data-endpoint-refresh-target="manual"]')!;
      const dependent = root.querySelector<HTMLSelectElement>('[name="dependent"]')!;
      const manual = root.querySelector<HTMLSelectElement>('[name="manual"]')!;
      const tags = root.querySelector<HTMLSelectElement>('[name="tags"]')!;
      const owner = root.querySelector<HTMLSelectElement>('[name="owner"]')!;
      const arrayItems = root.querySelector<HTMLElement>('[data-formgen-array-items]')!;
      const addRow = root.querySelector<HTMLButtonElement>('[data-formgen-array-action="add"]')!;
      let firstRequests = 0;
      let secondRequests = 0;

      const firstRegistry = await initFormgenRoot(root, {
        beforeFetch: () => {
          firstRequests += 1;
        },
      });
      const firstController = attachFormController(root, { registry: firstRegistry });
      const firstRequestBaseline = firstRequests;
      search.value = "stale";
      search.dispatchEvent(new Event("input", { bubbles: true }));
      firstController.destroy();

      expect(firstRegistry.get(dependent)).toBeUndefined();
      expect(dependent.dataset.relationshipRefreshBound).toBeUndefined();
      expect(manual.dataset.relationshipManualBound).toBeUndefined();
      expect(manualTrigger.dataset.relationshipRefreshListener).toBeUndefined();
      expect(search.dataset.relationshipSearchBound).toBeUndefined();
      expect(root.querySelectorAll("[data-fg-chip-root]")).toHaveLength(0);
      expect(root.querySelectorAll("[data-fg-typeahead-root]")).toHaveLength(0);
      expect(tags.required).toBe(true);
      expect(owner.required).toBe(true);
      expect(arrayItems.dataset.formgenArrayInitialized).toBeUndefined();

      const secondRegistry = await initFormgenRoot(root, {
        beforeFetch: () => {
          secondRequests += 1;
        },
      });
      const secondController = attachFormController(root, { registry: secondRegistry });
      const secondRequestBaseline = secondRequests;

      expect(root.querySelectorAll("[data-fg-chip-root]")).toHaveLength(1);
      expect(root.querySelectorAll("[data-fg-typeahead-root]")).toHaveLength(1);
      expect(arrayItems.dataset.formgenArrayInitialized).toBe("true");

      kind.dispatchEvent(new Event("input", { bubbles: true }));
      manualTrigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      search.value = "fresh";
      search.dispatchEvent(new Event("input", { bubbles: true }));
      await new Promise((resolve) => setTimeout(resolve, 40));
      await flush();

      expect(firstRequests).toBe(firstRequestBaseline);
      expect(secondRequests).toBe(secondRequestBaseline + 3);

      addRow.click();
      expect(arrayItems.querySelectorAll(':scope > [data-formgen-array-item="true"]')).toHaveLength(1);
      secondController.destroy();

      let thirdRequests = 0;
      const thirdRegistry = await initFormgenRoot(root, {
        beforeFetch: () => {
          thirdRequests += 1;
        },
      });
      const thirdController = attachFormController(root, { registry: thirdRegistry });
      const thirdRequestBaseline = thirdRequests;
      kind.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      expect(firstRequests).toBe(firstRequestBaseline);
      expect(secondRequests).toBe(secondRequestBaseline + 3);
      expect(thirdRequests).toBe(thirdRequestBaseline + 1);
      expect(root.querySelectorAll("[data-fg-chip-root]")).toHaveLength(1);
      expect(root.querySelectorAll("[data-fg-typeahead-root]")).toHaveLength(1);
      thirdController.destroy();
    });

    it("keeps configured embedded roots isolated through hydration and teardown", async () => {
      document.body.innerHTML = `
        <div id="root-a" data-formgen-render-mode="fields">
          <select name="target" data-endpoint-url="/api/a" data-endpoint-mode="search" data-endpoint-hydrate-param="ids">
            <option value="">Select A</option>
          </select>
        </div>
        <div id="root-b" data-formgen-render-mode="fields">
          <select name="target" data-endpoint-url="/api/b" data-endpoint-mode="search" data-endpoint-hydrate-param="ids">
            <option value="">Select B</option>
          </select>
        </div>
      `;
      fetchSpy.mockResolvedValue(mockResponse([]));
      const requestsA: string[] = [];
      const requestsB: string[] = [];
      const globalBefore = getGlobalConfig();
      const rootA = document.getElementById("root-a")!;
      const rootB = document.getElementById("root-b")!;
      const registryA = await initFormgenRoot(rootA, {
        beforeFetch: ({ request }) => {
          requestsA.push(request.url);
        },
      });
      const registryB = await initFormgenRoot(rootB, {
        beforeFetch: ({ request }) => {
          requestsB.push(request.url);
        },
      });

      expect(getGlobalConfig()).toBe(globalBefore);
      expect((globalThis as Record<string, unknown>).formgenRelationships).toBeUndefined();

      const selectA = rootA.querySelector("select")!;
      const selectB = rootB.querySelector("select")!;
      const controllerA = attachFormController(rootA, { registry: registryA });
      const controllerB = attachFormController(rootB, { registry: registryB });
      controllerA.setValues({ target: "alpha" });
      controllerB.setValues({ target: "beta" });
      controllerA.setErrors({ target: ["A is invalid"] });
      controllerB.setErrors({ target: ["B is invalid"] });

      expect(registryA.get(selectA)?.state.validation?.messages).toEqual(["A is invalid"]);
      expect(registryB.get(selectB)?.state.validation?.messages).toEqual(["B is invalid"]);
      await registryA.resolve(selectA);
      expect(requestsA).toHaveLength(1);
      expect(requestsA[0]).toContain("ids=alpha");
      expect(requestsB).toHaveLength(0);
      await registryB.resolve(selectB);
      expect(requestsB).toHaveLength(1);
      expect(requestsB[0]).toContain("ids=beta");

      controllerA.reset();
      expect(registryA.get(selectA)?.state.validation).toBeUndefined();
      expect(registryB.get(selectB)?.state.validation?.messages).toEqual(["B is invalid"]);
      controllerA.destroy();
      expect(registryA.get(selectA)).toBeUndefined();
      expect(registryB.get(selectB)).toBeDefined();
      controllerB.destroy();
    });

    it("keeps global relationship initialization backward compatible", async () => {
      const beforeFetch = vi.fn();
      const registry = await initRelationships({ beforeFetch });

      expect((globalThis as Record<string, unknown>).formgenRelationships).toBe(registry);
      expect(getGlobalConfig().beforeFetch).toBe(beforeFetch);
    });

    it("initializes static chips without requiring a remote endpoint", async () => {
      document.body.innerHTML = `
        <div id="root" data-formgen-render-mode="fields">
          <select name="tags" multiple data-endpoint-renderer="chips">
            <option value="alpha">Alpha</option>
            <option value="beta" data-option-description="Beta choice">Beta</option>
          </select>
        </div>
      `;

      const root = document.getElementById("root")!;
      const registry = await initFormgenRoot(root);
      const select = root.querySelector("select")!;
      expect(registry.get(select)).toBeDefined();
      expect(root.querySelector('[data-fg-chip-root]')).not.toBeNull();
      expect(fetchSpy).not.toHaveBeenCalled();
    });
  });

  describe("chips UX improvements", () => {
    it("positions dropdown below toggle without overlapping", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
          { value: "ai", label: "AI Strategy" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // Menu should use top-full positioning (below toggle)
      expect(menu.classList.contains("top-full")).toBe(true);
      expect(menu.classList.contains("mt-1")).toBe(true);

      // Open the menu
      toggle.click();
      await flush();

      expect(menu.hidden).toBe(false);
    });

    it("highlights option visually during keyboard navigation", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
          { value: "ai", label: "AI Strategy" },
          { value: "ml", label: "Machine Learning" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();

      const getOptions = () => Array.from(container.querySelectorAll<HTMLButtonElement>('[role="option"]'));

      // Press ArrowDown to highlight first option
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      let options = getOptions();
      expect(options[0]?.classList.contains("bg-blue-50")).toBe(true);

      // Press ArrowDown again to highlight second option
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      options = getOptions();
      expect(options[0]?.classList.contains("bg-blue-50")).toBe(false);
      expect(options[1]?.classList.contains("bg-blue-50")).toBe(true);
    });

    it("navigates to first/last option with Home/End keys", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "a", label: "Option A" },
          { value: "b", label: "Option B" },
          { value: "c", label: "Option C" },
          { value: "d", label: "Option D" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();

      const getOptions = () => Array.from(container.querySelectorAll<HTMLButtonElement>('[role="option"]'));

      // Navigate to second option first
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      // Press Home - should go to first option
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "Home", bubbles: true }));
      await flush();

      let options = getOptions();
      expect(options[0]?.classList.contains("bg-blue-50")).toBe(true);
      expect(options[1]?.classList.contains("bg-blue-50")).toBe(false);

      // Press End - should go to last option
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "End", bubbles: true }));
      await flush();

      options = getOptions();
      expect(options[0]?.classList.contains("bg-blue-50")).toBe(false);
      expect(options[options.length - 1]?.classList.contains("bg-blue-50")).toBe(true);
    });

    it("closes dropdown on Tab without blocking focus traversal", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
          <button id="next-button">Next</button>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();
      expect(menu.hidden).toBe(false);

      // Press Tab - should close dropdown (don't preventDefault so focus can move)
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true }));
      await flush();

      // Menu should be closed
      expect(menu.hidden).toBe(true);
    });

    it("closes dropdown on Escape and restores focus to toggle", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();
      expect(menu.hidden).toBe(false);

      // Press Escape - should close dropdown and restore focus
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
      await flush();

      expect(menu.hidden).toBe(true);
      expect(document.activeElement).toBe(toggle);
    });

    it("prevents rapid open/close state mismatches", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // Initial state
      expect(menu.hidden).toBe(true);

      // Rapid clicks - should not desync state
      toggle.click();
      toggle.click();
      toggle.click();
      await flush();

      // State should be consistent (open after odd number of clicks)
      const isOpen = !menu.hidden;
      const ariaExpanded = toggle.getAttribute("aria-expanded");
      expect(ariaExpanded).toBe(isOpen ? "true" : "false");
    });

    it("supports non-search mode keyboard navigation on toggle", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
          { value: "ai", label: "AI Strategy" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // Focus toggle and press ArrowDown to open and navigate
      toggle.focus();
      toggle.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      // Menu should be open
      expect(menu.hidden).toBe(false);

      // Press Escape to close
      toggle.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
      await flush();

      expect(menu.hidden).toBe(true);
    });

    it("uses hybrid focus model with aria-activedescendant in search mode", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
          { value: "ai", label: "AI Strategy" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();

      // Navigate with ArrowDown - focus should stay in input
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      // Focus should remain on the search input (hybrid model)
      expect(document.activeElement).toBe(searchInput);

      // aria-activedescendant should be set
      const activeDescendant = searchInput.getAttribute("aria-activedescendant");
      expect(activeDescendant).not.toBeNull();

      // The referenced element should exist and be highlighted
      if (activeDescendant) {
        const highlightedOption = document.getElementById(activeDescendant);
        expect(highlightedOption).not.toBeNull();
        expect(highlightedOption?.classList.contains("bg-blue-50")).toBe(true);
      }
    });

    it("moves focus between last option and footer action with Arrow keys", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
          { value: "ai", label: "AI Strategy" },
          { value: "ml", label: "Machine Learning" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();

      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      expect(footerAction).not.toBeNull();

      const getOptions = () => Array.from(container.querySelectorAll<HTMLButtonElement>('[role="option"]'));

      // Walk to last option
      for (let i = 0; i < getOptions().length; i += 1) {
        searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
        await flush();
      }

      // ArrowDown from last option should move focus to footer action
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      expect(document.activeElement).toBe(footerAction);
      expect(footerAction.classList.contains("bg-blue-50")).toBe(true);

      // ArrowUp should return to last option and focus the input
      footerAction.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true }));
      await flush();

      const options = getOptions();
      expect(document.activeElement).toBe(searchInput);
      expect(options[options.length - 1]?.classList.contains("bg-blue-50")).toBe(true);
    });
  });

  describe("typeahead edit action", () => {
    function renderAuthorTypeahead(extra = ""): void {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-method="GET"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-edit-action="true"
            data-endpoint-edit-action-id="author"
            data-endpoint-edit-action-label="Edit Author"
            data-relationship-cardinality="one"
            ${extra}
          ></select>
        </form>
      `;
    }

    async function initAuthorTypeahead(config = {}) {
      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "1", label: "Alice Smith" },
          { value: "2", label: "Bob Jones" },
        ])
      );
      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        ...config,
      });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();
      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;
      return { registry, select, container, input };
    }

    it("hides edit action when no option is selected", async () => {
      renderAuthorTypeahead();
      const { container, input } = await initAuthorTypeahead();

      input.click();
      await flush();

      const editAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]');
      expect(editAction).toBeNull();
    });

    it("renders edit action when an option is selected", async () => {
      renderAuthorTypeahead();
      const { container, input } = await initAuthorTypeahead();

      input.click();
      await flush();
      const option = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!;
      option.click();
      await flush();

      input.click();
      await flush();

      const editAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]');
      expect(editAction).not.toBeNull();
      expect(editAction?.textContent).toContain("Edit Author");
      expect(editAction?.dataset.value).toBe("1");
    });

    it("dispatches edit-action event with selected option detail", async () => {
      renderAuthorTypeahead();
      const { select, container, input } = await initAuthorTypeahead();
      const eventSpy = vi.fn();
      select.addEventListener(RELATIONSHIP_EDIT_ACTION_EVENT, eventSpy);

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!.click();
      await flush();

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]')!.click();
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
      const detail = eventSpy.mock.calls[0][0].detail;
      expect(detail.selectedValue).toBe("1");
      expect(detail.selectedLabel).toBe("Alice Smith");
      expect(detail.actionId).toBe("author");
      expect(detail.mode).toBe("typeahead");
      expect(detail.field.name).toBe("article[author_id]");
      expect(detail.endpoint.url).toBe("/api/authors");
      expect(detail.element).toBe(select);
    });

    it("invokes onEditAction hook instead of dispatching event", async () => {
      renderAuthorTypeahead();
      const onEditAction = vi.fn();
      const { select, container, input } = await initAuthorTypeahead({ onEditAction });
      const eventSpy = vi.fn();
      select.addEventListener(RELATIONSHIP_EDIT_ACTION_EVENT, eventSpy);

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!.click();
      await flush();

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]')!.click();
      await flush();
      await flush();

      expect(onEditAction).toHaveBeenCalledTimes(1);
      expect(eventSpy).not.toHaveBeenCalled();
      const detail = onEditAction.mock.calls[0][1];
      expect(detail).toMatchObject({
        selectedValue: "1",
        selectedLabel: "Alice Smith",
        actionId: "author",
        mode: "typeahead",
      });
    });

    it("triggers edit action via Enter on focused action button", async () => {
      renderAuthorTypeahead();
      const { select, container, input } = await initAuthorTypeahead();
      const eventSpy = vi.fn();
      select.addEventListener(RELATIONSHIP_EDIT_ACTION_EVENT, eventSpy);

      input.focus();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!.click();
      await flush();

      input.click();
      await flush();
      const editAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]')!;
      editAction.focus();
      editAction.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
    });

    it("applies returned updated option label while preserving selection", async () => {
      renderAuthorTypeahead();
      const onEditAction = vi.fn().mockResolvedValue({ value: "1", label: "Alice Updated" });
      const { select, container, input } = await initAuthorTypeahead({ onEditAction });
      const selectionSpy = vi.fn();
      select.addEventListener("formgen:relationship:update", selectionSpy);

      input.focus();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!.click();
      await flush();
      selectionSpy.mockClear();

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]')!.click();
      await flush();
      await flush();

      expect(select.value).toBe("1");
      expect(input.value).toBe("Alice Updated");
      expect(select.querySelector<HTMLOptionElement>('option[value="1"]')?.textContent).toBe("Alice Updated");
      expect(selectionSpy).toHaveBeenCalledTimes(1);
      expect(selectionSpy.mock.calls[0][0].detail).toMatchObject({
        kind: "selection",
        origin: "ui",
        selectedValues: ["1"],
      });
    });

    it("leaves selection unchanged when edit hook returns void", async () => {
      renderAuthorTypeahead();
      const onEditAction = vi.fn().mockResolvedValue(undefined);
      const { select, container, input } = await initAuthorTypeahead({ onEditAction });

      input.focus();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!.click();
      await flush();

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]')!.click();
      await flush();
      await flush();

      expect(select.value).toBe("1");
      expect(input.value).toBe("Alice Smith");
      expect(select.querySelector<HTMLOptionElement>('option[value="1"]')?.textContent).toBe("Alice Smith");
    });

    it("leaves selection unchanged when edit hook throws", async () => {
      renderAuthorTypeahead();
      const onEditAction = vi.fn().mockRejectedValue(new Error("save failed"));
      const { select, container, input } = await initAuthorTypeahead({ onEditAction });

      input.focus();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-option="true"]')!.click();
      await flush();

      input.click();
      await flush();
      container.querySelector<HTMLButtonElement>('[data-fg-typeahead-edit-action="true"]')!.click();
      await flush();
      await flush();

      expect(select.value).toBe("1");
      expect(input.value).toBe("Alice Smith");
      expect(select.querySelector<HTMLOptionElement>('option[value="1"]')?.textContent).toBe("Alice Smith");
    });
  });

  describe("typeahead create action", () => {
    it("renders create action row when enabled", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-method="GET"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-endpoint-create-action-label="Create Author"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "1", label: "Alice Smith" },
          { value: "2", label: "Bob Jones" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown
      input.focus();
      await flush();

      const dropdown = container.querySelector<HTMLElement>('[role="listbox"]')!.parentElement as HTMLElement;
      expect(dropdown.hidden).toBe(false);

      // Create action row should be present
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]');
      expect(createAction).not.toBeNull();
      expect(createAction?.textContent).toContain("Create Author");
    });

    it("renders create action even when options exist", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "1", label: "Alice" },
          { value: "2", label: "Bob" },
          { value: "3", label: "Charlie" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown
      input.focus();
      await flush();

      // Verify options are present
      const options = container.querySelectorAll<HTMLButtonElement>('[data-fg-typeahead-option="true"]');
      expect(options.length).toBe(3);

      // Create action should also be present
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]');
      expect(createAction).not.toBeNull();
    });

    it("dispatches create-action event with correct payload when clicked", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-endpoint-create-action-id="author"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Listen for the create-action event
      const eventSpy = vi.fn();
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      // Open dropdown and type a query
      input.focus();
      input.value = "New Author";
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      // Click create action
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;
      createAction.click();
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
      const detail = eventSpy.mock.calls[0][0].detail;
      expect(detail.query).toBe("New Author");
      expect(detail.actionId).toBe("author");
      expect(detail.mode).toBe("typeahead");
      expect(detail.selectBehavior).toBe("replace"); // typeahead defaults to replace
      expect(detail.field.name).toBe("article[author_id]");
      expect(detail.endpoint.url).toBe("/api/authors");
      expect(detail.element).toBe(select);
    });

    it("invokes onCreateAction hook instead of dispatching event when provided", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      const onCreateAction = vi.fn().mockResolvedValue({ value: "new-1", label: "New Author" });
      const eventSpy = vi.fn();

      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        onCreateAction,
      });
      const select = document.getElementById("author") as HTMLSelectElement;
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown and type a query
      input.focus();
      input.value = "New Author";
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      // Click create action
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;
      createAction.click();
      await flush();
      await flush();

      // Hook should be called, event should NOT be dispatched
      expect(onCreateAction).toHaveBeenCalledTimes(1);
      expect(eventSpy).not.toHaveBeenCalled();

      // The returned option should be selected
      expect(select.value).toBe("new-1");
      expect(input.value).toBe("New Author");
    });

    it("restricts returned values to single Option for typeahead", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      // Return array - typeahead should only use the first item
      const onCreateAction = vi.fn().mockResolvedValue([
        { value: "new-1", label: "First Author" },
        { value: "new-2", label: "Second Author" },
      ]);

      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        onCreateAction,
      });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown
      input.focus();
      await flush();

      // Click create action
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;
      createAction.click();
      await flush();
      await flush();

      // Only first option should be selected (typeahead is single-select)
      expect(select.value).toBe("new-1");
      expect(input.value).toBe("First Author");
    });

    it("closes dropdown when create action is activated", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;
      const dropdown = container.querySelector<HTMLElement>('[role="listbox"]')!.parentElement as HTMLElement;

      // Open dropdown
      input.focus();
      await flush();
      expect(dropdown.hidden).toBe(false);

      // Click create action
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;
      createAction.click();
      await flush();

      // Dropdown should be closed
      expect(dropdown.hidden).toBe(true);
    });

    it("navigates to create action via keyboard from last option", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "1", label: "Alice" },
          { value: "2", label: "Bob" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown
      input.focus();
      await flush();

      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;

      // Navigate down through options to create action
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true })); // First option
      await flush();
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true })); // Second option
      await flush();
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true })); // Create action
      await flush();

      expect(document.activeElement).toBe(createAction);
    });

    it("returns focus to options from create action via ArrowUp", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "1", label: "Alice" },
          { value: "2", label: "Bob" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown
      input.focus();
      await flush();

      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;

      // Navigate to create action
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      expect(document.activeElement).toBe(createAction);

      // ArrowUp from create action should return to input with last option highlighted
      createAction.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true }));
      await flush();

      expect(document.activeElement).toBe(input);
    });

    it("triggers create action via Enter on focused action button", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      const eventSpy = vi.fn();
      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown and type query
      input.focus();
      input.value = "Test";
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      // Navigate to create action
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;
      expect(document.activeElement).toBe(createAction);

      // Press Enter on create action
      createAction.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
    });

    it("renders create action in non-search mode (default mode)", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown (non-search mode)
      input.focus();
      await flush();

      // Create action should be present even in non-search mode
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]');
      expect(createAction).not.toBeNull();
    });

    it("passes empty query in non-search mode create action event", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select
            id="author"
            name="article[author_id]"
            data-endpoint-url="/api/authors"
            data-endpoint-renderer="typeahead"
            data-endpoint-create-action="true"
            data-relationship-cardinality="one"
          ></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

      const eventSpy = vi.fn();
      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("author") as HTMLSelectElement;
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open dropdown (non-search mode)
      input.focus();
      await flush();

      // Click create action
      const createAction = container.querySelector<HTMLButtonElement>('[data-fg-typeahead-create-action="true"]')!;
      createAction.click();
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
      expect(eventSpy.mock.calls[0][0].detail.query).toBe("");
    });
  });

  describe("chips create action", () => {
    it("renders create action in dropdown footer when enabled", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-method="GET"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-endpoint-create-action-label="Create Tag"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "design", label: "Product Design" },
          { value: "ai", label: "AI Strategy" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;

      // Open menu
      toggle.click();
      await flush();

      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]');

      expect(footerAction).not.toBeNull();
      expect(footerAction?.textContent).toContain("Create Tag");
    });

    it("renders create action even when options exist", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "a", label: "Option A" },
          { value: "b", label: "Option B" },
          { value: "c", label: "Option C" },
        ])
      );

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;

      // Open menu
      toggle.click();
      await flush();

      // Verify options are present
      const options = container.querySelectorAll<HTMLButtonElement>('[role="option"]');
      expect(options.length).toBe(3);

      // Create action should also be present
      const footerAction = container.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]');
      expect(footerAction).not.toBeNull();
    });

    it("dispatches create-action event with correct payload when clicked", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-endpoint-create-action-id="tags"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Tag 1" }]));

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      const eventSpy = vi.fn();
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      // Open menu and type query
      toggle.click();
      await flush();
      searchInput.value = "New Tag";
      searchInput.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      // Click create action
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
      const detail = eventSpy.mock.calls[0][0].detail;
      expect(detail.query).toBe("New Tag");
      expect(detail.actionId).toBe("tags");
      expect(detail.mode).toBe("chips");
      expect(detail.selectBehavior).toBe("append"); // chips defaults to append
      expect(detail.field.name).toBe("article[tags][]");
      expect(detail.endpoint.url).toBe("/api/tags");
      expect(detail.element).toBe(select);
    });

    it("invokes onCreateAction hook instead of dispatching event when provided", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Tag 1" }]));

      const onCreateAction = vi.fn().mockResolvedValue({ value: "new-1", label: "New Tag" });
      const eventSpy = vi.fn();

      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        onCreateAction,
      });
      const select = document.getElementById("tags") as HTMLSelectElement;
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // Open menu
      toggle.click();
      await flush();

      // Click create action
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();
      await flush();

      // Hook should be called, event should NOT be dispatched
      expect(onCreateAction).toHaveBeenCalledTimes(1);
      expect(eventSpy).not.toHaveBeenCalled();

      // The returned option should be selected
      expect(Array.from(select.selectedOptions).map(o => o.value)).toContain("new-1");
    });

    it("applies append behavior for returned options by default", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "existing", label: "Existing Tag" }]));

      const onCreateAction = vi.fn().mockResolvedValue([
        { value: "new-1", label: "New Tag 1" },
        { value: "new-2", label: "New Tag 2" },
      ]);

      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        onCreateAction,
      });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // First select an existing option
      toggle.click();
      await flush();
      const existingOption = container.querySelector<HTMLButtonElement>('[data-value="existing"]')!;
      existingOption.click();
      await flush();
      await flush();

      expect(Array.from(select.selectedOptions).map(o => o.value)).toContain("existing");

      // Open menu again and click create action
      toggle.click();
      await flush();
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();
      await flush();

      // All options should be selected (append behavior)
      const selectedValues = Array.from(select.selectedOptions).map(o => o.value);
      expect(selectedValues).toContain("existing");
      expect(selectedValues).toContain("new-1");
      expect(selectedValues).toContain("new-2");
    });

    it("applies replace behavior when configured", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-endpoint-create-action-select="replace"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "existing", label: "Existing Tag" }]));

      const onCreateAction = vi.fn().mockResolvedValue([
        { value: "new-1", label: "New Tag 1" },
      ]);

      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        onCreateAction,
      });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // First select an existing option
      toggle.click();
      await flush();
      const existingOption = container.querySelector<HTMLButtonElement>('[data-value="existing"]')!;
      existingOption.click();
      await flush();
      await flush();

      expect(Array.from(select.selectedOptions).map(o => o.value)).toContain("existing");

      // Open menu again and click create action
      toggle.click();
      await flush();
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();
      await flush();

      // Only new options should be selected (replace behavior)
      const selectedValues = Array.from(select.selectedOptions).map(o => o.value);
      expect(selectedValues).not.toContain("existing");
      expect(selectedValues).toContain("new-1");
    });

    it("closes dropdown and clears query on create action (Model 1)", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Tag 1" }]));

      await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu and type query
      toggle.click();
      await flush();
      searchInput.value = "Test Query";
      searchInput.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      expect(menu.hidden).toBe(false);
      expect(searchInput.value).toBe("Test Query");

      // Click create action
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();

      // Menu should be closed
      expect(menu.hidden).toBe(true);
      // Query should be cleared
      expect(searchInput.value).toBe("");
    });

    it("renders create action in non-search mode (default mode)", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Tag 1" }]));

      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // Open menu
      toggle.click();
      await flush();

      // Create action should be present even in non-search mode
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]');
      expect(footerAction).not.toBeNull();
    });

    it("passes empty query in non-search mode create action event", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Tag 1" }]));

      const eventSpy = vi.fn();
      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;

      // Open menu
      toggle.click();
      await flush();

      // Click create action
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
      expect(eventSpy.mock.calls[0][0].detail.query).toBe("");
    });

    it("triggers create action via Enter on focused footer action button", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(
        mockResponse([
          { value: "1", label: "Tag 1" },
          { value: "2", label: "Tag 2" },
        ])
      );

      const eventSpy = vi.fn();
      const registry = await initRelationships({ searchThrottleMs: 0, searchDebounceMs: 0 });
      const select = document.getElementById("tags") as HTMLSelectElement;
      select.addEventListener("formgen:relationship:create-action", eventSpy);

      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu
      toggle.click();
      await flush();

      // Navigate to footer action (ArrowDown past all options)
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      searchInput.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
      await flush();

      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      expect(document.activeElement).toBe(footerAction);

      // Press Enter on footer action
      footerAction.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
      await flush();

      expect(eventSpy).toHaveBeenCalledTimes(1);
    });

    it("inline create and create action can coexist", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-allow-create="true"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      // Return empty results to trigger inline create
      fetchSpy.mockResolvedValue(mockResponse([]));

      const createOption = vi.fn().mockResolvedValue({ value: "inline", label: "Inline Tag" });
      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        createOption,
      });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu and type query
      toggle.click();
      await flush();
      searchInput.value = "NewTag";
      searchInput.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      // Both inline create and footer create action should be present
      const inlineCreate = container.querySelector<HTMLButtonElement>('[data-fg-create-option="true"]');
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]');

      expect(inlineCreate).not.toBeNull();
      expect(footerAction).not.toBeNull();
      expect(inlineCreate?.textContent).toContain("NewTag"); // Inline create shows query
      // Footer action has the static label
    });

    it("adds created options to store.options for consistent filtering", async () => {
      document.body.innerHTML = `
        <form data-formgen-auto-init="true">
          <select id="tags" name="article[tags][]" multiple
            data-endpoint-url="/api/tags"
            data-endpoint-renderer="chips"
            data-endpoint-mode="search"
            data-endpoint-create-action="true"
            data-relationship-cardinality="many"></select>
        </form>
      `;

      fetchSpy.mockResolvedValue(mockResponse([{ value: "existing", label: "Existing" }]));

      const onCreateAction = vi.fn().mockResolvedValue({ value: "created", label: "Created Tag" });

      const registry = await initRelationships({
        searchThrottleMs: 0,
        searchDebounceMs: 0,
        onCreateAction,
      });
      const select = document.getElementById("tags") as HTMLSelectElement;
      await registry.resolve(select);
      await flush();
      await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
      await flush();

      const container = select.previousElementSibling as HTMLElement;
      const toggle = container.querySelector<HTMLButtonElement>('[aria-haspopup="listbox"]')!;
      const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
      const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;

      // Open menu and click create action
      toggle.click();
      await flush();
      const footerAction = menu.querySelector<HTMLButtonElement>('[data-fg-chips-create-action="true"]')!;
      footerAction.click();
      await flush();
      await flush();

      // Open menu again and filter by the created option's label
      toggle.click();
      await flush();
      searchInput.value = "Created";
      searchInput.dispatchEvent(new Event("input", { bubbles: true }));
      await flush();

      // The created option should now appear in the filter (it's selected, so might be hidden from options)
      // Check that it exists in the select element
      const createdOption = Array.from(select.options).find(o => o.value === "created");
      expect(createdOption).not.toBeUndefined();
      expect(createdOption?.textContent).toBe("Created Tag");
    });
  });
});

describe("embedded hydration options", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    resetGlobalRegistry();
    Element.prototype.scrollIntoView = vi.fn();
  });

  afterEach(() => {
    resetDom();
    resetGlobalRegistry();
  });

  // Synthetic enabled/level/colors/note fixture from the preserved integration brief.
  const SAMPLE_MARKUP = `
    <fieldset><legend>Enable feature</legend>
      <label><input type="radio" name="enabled" value="true">Yes</label>
      <label><input type="radio" name="enabled" value="false">No</label>
    </fieldset>
    <input name="level" type="number" min="1" max="10">
    <label><input name="colors[]" type="checkbox" value="blue">Blue</label>
    <label><input name="colors[]" type="checkbox" value="green">Green</label>
    <input name="note" maxlength="80">
  `;

  function renderSample(id = "sample"): HTMLElement {
    document.body.innerHTML = `<form id="${id}">${SAMPLE_MARKUP}</form>`;
    return document.getElementById(id)!;
  }

  function control<T extends HTMLElement>(root: ParentNode, selector: string): T {
    return root.querySelector<T>(selector)!;
  }

  it("reproduces the pinned legacy six-callback re-entry fixture without options", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    let calls = 0;
    let depth = 0;
    let maxDepth = 0;
    const off = controller.onChange((values) => {
      calls += 1;
      depth += 1;
      maxDepth = Math.max(maxDepth, depth);
      if (calls < 3) {
        controller.setValues(values);
      }
      depth -= 1;
    });
    controller.setValues({ enabled: false });
    off();
    // Legacy baseline pinned by the external reproducer on v0.33.2: not a cross-version guarantee.
    expect({ calls, maxDepth }).toEqual({ calls: 6, maxDepth: 3 });

    controller.setValues({ enabled: true, level: 4, colors: ["blue"] });
    expect(controller.getValues()).toEqual({ enabled: "true", level: "4", colors: ["blue"], note: "" });
    controller.destroy();
  });

  it("keeps the legacy callback shape and reports origins for native and legacy events", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    const seen: Array<{ origin: string; paths: string[]; type: string }> = [];
    controller.onChange((_values, event, meta) => {
      seen.push({ origin: meta.origin, paths: meta.paths, type: event.type });
    });
    const note = control<HTMLInputElement>(root, 'input[name="note"]');
    note.value = "typed";
    // jsdom cannot dispatch trusted events; trusted user input reports origin "ui".
    note.dispatchEvent(new Event("input", { bubbles: true }));
    expect(seen).toEqual([{ origin: "program", paths: ["note"], type: "input" }]);

    seen.length = 0;
    controller.setValues({ note: "legacy" });
    expect(seen).toEqual([{ origin: "hydrate", paths: ["note"], type: "input" }]);

    seen.length = 0;
    controller.reset();
    expect(seen.length).toBeGreaterThan(1);
    expect(new Set(seen.map((entry) => entry.origin))).toEqual(new Set(["reset"]));
    controller.destroy();
  });

  it("suppresses subscribers with notify:false while still updating controls", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    const changes = vi.fn();
    controller.onChange(changes);
    controller.setValues({ enabled: true, level: 4, colors: ["blue"], note: "silent" }, { notify: false });
    expect(changes).not.toHaveBeenCalled();
    expect(controller.getValues()).toEqual({ enabled: "true", level: "4", colors: ["blue"], note: "silent" });
    controller.destroy();
  });

  it("settles opted-in hydration into one notification and skips no-op updates", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    const changes = vi.fn();
    controller.onChange(changes);

    controller.setValues({ enabled: true, level: 4, colors: ["blue"] }, {});
    expect(changes).toHaveBeenCalledTimes(1);
    const [values, event, meta] = changes.mock.calls[0];
    expect(values).toEqual({ enabled: "true", level: "4", colors: ["blue"], note: "" });
    expect(event.type).toBe("formgen:controller:change");
    expect(meta).toEqual({ origin: "hydrate", paths: ["enabled", "level", "colors"] });

    controller.setValues({ enabled: true, level: 4, colors: ["blue"] }, { notify: true });
    expect(changes).toHaveBeenCalledTimes(1);

    controller.setValues({ level: 5 }, { notify: true });
    expect(changes).toHaveBeenCalledTimes(2);
    expect(changes.mock.calls[1][2]).toEqual({ origin: "hydrate", paths: ["level"] });
    controller.destroy();
  });

  it("lets a subscriber normalize values without a host re-entry guard (C13)", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    let calls = 0;
    controller.onChange((values, _event, meta) => {
      calls += 1;
      if (meta.origin !== "hydrate") {
        controller.setValues({ note: String(values.note ?? "").trim().toUpperCase() }, { notify: false });
      }
    });
    const note = control<HTMLInputElement>(root, 'input[name="note"]');
    note.value = "  shout  ";
    note.dispatchEvent(new Event("input", { bubbles: true }));
    expect(calls).toBe(1);
    expect(note.value).toBe("SHOUT");

    controller.setValues({ note: " again " }, { notify: true });
    expect(calls).toBe(2);
    expect(note.value).toBe(" again ");
    controller.destroy();
  });

  it("restores a host snapshot with replace semantics without recursion (C08)", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    controller.setValues({ enabled: true, level: 4, colors: ["blue"] }, { notify: false });
    const snapshot = controller.getValues();
    expect(snapshot).toEqual({ enabled: "true", level: "4", colors: ["blue"], note: "" });

    const changes = vi.fn();
    controller.onChange(changes);
    controller.setValues({ enabled: false, level: null, colors: [], note: "edited" }, { notify: false });
    expect(controller.getValues()).toEqual({ enabled: "false", level: "", note: "edited" });

    controller.setValues(snapshot, { mode: "replace", notify: false });
    expect(controller.getValues()).toEqual(snapshot);
    expect(changes).not.toHaveBeenCalled();

    controller.setValues({ enabled: false, note: "cancelled" }, { notify: false });
    controller.setValues(snapshot, { mode: "replace" });
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2]).toEqual({ origin: "hydrate", paths: ["enabled", "note"] });
    expect(controller.getValues()).toEqual(snapshot);
    controller.destroy();
  });

  it("clears omitted managed values in replace mode and treats null as an explicit clear", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    controller.setValues({ enabled: true, level: 4, colors: ["blue", "green"], note: "keep?" }, { notify: false });

    controller.setValues({ enabled: true }, { mode: "replace", notify: false });
    expect(controller.getValues()).toEqual({ enabled: "true", level: "", note: "" });
    expect(control<HTMLInputElement>(root, 'input[value="blue"]').checked).toBe(false);

    controller.setValues({ level: 7, note: "x" }, { notify: false });
    controller.setValues({ level: null, note: null }, { notify: false });
    expect(controller.getValues()).toEqual({ enabled: "true", level: "", note: "" });
    controller.destroy();
  });

  it("leaves disabled controls alone when replace omits them but applies explicit values", () => {
    const root = renderSample();
    const level = control<HTMLInputElement>(root, 'input[name="level"]');
    level.value = "9";
    level.disabled = true;
    const controller = attachFormController(root);
    controller.setValues({ note: "only" }, { mode: "replace", notify: false });
    expect(level.value).toBe("9");
    controller.setValues({ level: 2 }, { mode: "replace", notify: false });
    expect(level.value).toBe("2");
    controller.destroy();
  });

  it("hydrates rendered rows from arrays of objects and fails before mutating on missing rows", () => {
    document.body.innerHTML = `
      <div id="root" data-formgen-render-mode="fields">
        <input name="title" value="t" />
        <div data-formgen-array-items="true" data-formgen-array-name="rows" data-formgen-array-next-index="2" data-formgen-array-prototype-path="rows[2]" data-formgen-array-prototype-id-prefix="fg-rows-2">
          <div data-formgen-array-item="true" data-formgen-array-existing="true">
            <input type="hidden" name="rows[0]._present" value="true" />
            <input name="rows[0].name" value="first" />
          </div>
          <div data-formgen-array-item="true" data-formgen-array-existing="true">
            <input type="hidden" name="rows[1]._present" value="true" />
            <input name="rows[1].name" value="second" />
          </div>
          <template data-formgen-array-prototype="true"><div data-formgen-array-item="true"><input id="fg-rows-2-name" name="rows[2].name" /></div></template>
        </div>
        <button type="button" data-formgen-array-action="add">Add row</button>
      </div>
    `;
    const root = document.getElementById("root")!;
    const controller = attachFormController(root);
    const changes = vi.fn();
    controller.onChange(changes);

    controller.setValues({ rows: [{ name: "alpha" }, { name: "beta" }] }, { mode: "replace" });
    expect(controller.getValues()).toEqual({
      title: "",
      rows: { 0: { _present: "true", name: "alpha" }, 1: { _present: "true", name: "beta" } },
    });
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2]).toEqual({ origin: "hydrate", paths: ["title", "rows[0].name", "rows[1].name"] });

    expect(() =>
      controller.setValues({ title: "late", rows: [{ name: "x" }, { name: "y" }, { name: "z" }] }, { mode: "replace" })
    ).toThrow(/rows\[2\]/);
    expect(controller.getValues()).toEqual({
      title: "",
      rows: { 0: { _present: "true", name: "alpha" }, 1: { _present: "true", name: "beta" } },
    });
    expect(changes).toHaveBeenCalledTimes(1);

    controller.setValues({ rows: [{ name: "p" }, { name: "q" }, { name: "ignored" }] }, { notify: false });
    expect(controller.getValues().rows).toEqual({ 0: { _present: "true", name: "p" }, 1: { _present: "true", name: "q" } });
    controller.destroy();
  });

  it("keeps widget, mirror, dependency, and component synchronization under notify:false", async () => {
    document.body.innerHTML = `
      <div id="root" data-formgen-render-mode="fields">
        <input name="kind" value="initial" />
        <select name="dependent" data-endpoint-url="/api/dependent" data-endpoint-refresh-on="kind"></select>
        <select name="tags" multiple data-endpoint-renderer="chips">
          <option value="alpha">Alpha</option>
          <option value="beta">Beta</option>
        </select>
        <div data-component="probe"><input name="probe" value="" /></div>
      </div>
    `;
    fetchSpy.mockResolvedValue(mockResponse([{ value: "one", label: "One" }]));
    const probeInputs = vi.fn();
    registerComponent("probe", ({ element }) => {
      const input = element.querySelector("input")!;
      input.addEventListener("input", probeInputs);
      return () => input.removeEventListener("input", probeInputs);
    });
    const root = document.getElementById("root")!;
    // Disable option caching so the dependent refresh is observable as a fetch.
    const registry = await initFormgenRoot(root, { cache: { strategy: "none" } });
    const controller = attachFormController(root, { registry });
    const changes = vi.fn();
    controller.onChange(changes);
    const fetchBaseline = fetchSpy.mock.calls.length;

    controller.setValues({ kind: "next", tags: ["beta"], probe: "touched" }, { notify: false });
    await new Promise((resolve) => setTimeout(resolve, 20));
    await flush();

    expect(changes).not.toHaveBeenCalled();
    const chips = Array.from(root.querySelectorAll("[data-fg-chip-value]")).map((chip) =>
      chip.getAttribute("data-fg-chip-value")
    );
    expect(chips).toEqual(["beta"]);
    const mirrors = Array.from(root.querySelectorAll<HTMLInputElement>("[data-relationship-hidden] input")).map(
      (input) => input.value
    );
    expect(mirrors).toEqual(["beta"]);
    expect(fetchSpy.mock.calls.length).toBe(fetchBaseline + 1);
    expect(probeInputs).toHaveBeenCalledTimes(1);
    controller.destroy();
  });

  it("keeps roots isolated and detaches subscribers on destroy (C12)", () => {
    document.body.innerHTML = `<form id="a">${SAMPLE_MARKUP}</form><form id="b">${SAMPLE_MARKUP}</form>`;
    const rootA = document.getElementById("a")!;
    const rootB = document.getElementById("b")!;
    const controllerA = attachFormController(rootA);
    const controllerB = attachFormController(rootB);
    const changesA = vi.fn();
    const changesB = vi.fn();
    controllerA.onChange(changesA);
    controllerB.onChange(changesB);

    controllerA.setValues({ note: "a" }, {});
    expect(changesA).toHaveBeenCalledTimes(1);
    expect(changesB).not.toHaveBeenCalled();
    expect(controllerB.getValues().note).toBe("");

    controllerA.destroy();
    const noteA = control<HTMLInputElement>(rootA, 'input[name="note"]');
    noteA.value = "late";
    noteA.dispatchEvent(new Event("input", { bubbles: true }));
    controllerA.setValues({ note: "ignored" }, {});
    expect(changesA).toHaveBeenCalledTimes(1);
    expect(noteA.value).toBe("late");
    expect(controllerA.getValues()).toEqual({});

    controllerB.setValues({ note: "b" }, {});
    expect(changesB).toHaveBeenCalledTimes(1);
    expect(changesB.mock.calls[0][2]).toEqual({ origin: "hydrate", paths: ["note"] });

    rootA.innerHTML = SAMPLE_MARKUP;
    const fresh = attachFormController(rootA);
    const freshChanges = vi.fn();
    fresh.onChange(freshChanges);
    expect(fresh.getValues()).toEqual({ level: "", note: "" });
    fresh.setValues({ note: "fresh" }, {});
    expect(freshChanges).toHaveBeenCalledTimes(1);
    expect(changesA).toHaveBeenCalledTimes(1);
    fresh.destroy();
    controllerB.destroy();
  });

  it("resets with options once with reset origin and resynchronizes enhanced widgets", async () => {
    document.body.innerHTML = `
      <div id="root" data-formgen-render-mode="fields">
        ${SAMPLE_MARKUP}
        <select name="tags" multiple data-endpoint-renderer="chips">
          <option value="alpha" selected>Alpha</option>
          <option value="beta">Beta</option>
        </select>
      </div>
    `;
    const root = document.getElementById("root")!;
    const registry = await initFormgenRoot(root);
    const controller = attachFormController(root, { registry });
    controller.setValues({ enabled: true, level: 3, colors: ["green"], note: "n", tags: ["beta"] }, { notify: false });
    expect(root.querySelector("[data-fg-chip-value]")?.getAttribute("data-fg-chip-value")).toBe("beta");
    controller.setErrors({ note: ["bad"] });

    const changes = vi.fn();
    controller.onChange(changes);
    controller.reset({});
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2].origin).toBe("reset");
    expect(changes.mock.calls[0][2].paths).toEqual(["enabled", "level", "colors", "note", "tags"]);
    expect(controller.getValues()).toEqual({ level: "", note: "", tags: ["alpha"] });
    expect(root.querySelector("[data-fg-chip-value]")?.getAttribute("data-fg-chip-value")).toBe("alpha");
    expect(control<HTMLInputElement>(root, 'input[name="note"]').getAttribute("data-validation-state")).toBeNull();

    controller.setValues({ note: "again" }, { notify: false });
    controller.reset({ notify: false });
    expect(changes).toHaveBeenCalledTimes(1);
    expect(controller.getValues().note).toBe("");
    controller.reset({});
    expect(changes).toHaveBeenCalledTimes(1);
    controller.destroy();
  });

  it("handles unsubscribe and destroy during notification safely", () => {
    const root = renderSample();
    const controller = attachFormController(root);
    const second = vi.fn();
    let offSecond: () => void = () => undefined;
    const offFirst = controller.onChange(() => {
      offFirst();
      offSecond();
    });
    offSecond = controller.onChange(second);
    controller.setValues({ note: "once" }, {});
    expect(second).not.toHaveBeenCalled();

    const third = vi.fn();
    controller.onChange(() => controller.destroy());
    controller.onChange(third);
    controller.setValues({ note: "twice" }, {});
    expect(third).not.toHaveBeenCalled();
    expect(controller.getValues()).toEqual({});
  });

  it("preserves option labels while hydrating selects", () => {
    document.body.innerHTML = `
      <form id="f">
        <select name="status">
          <option value="">Pick</option>
          <option value="draft">Draft</option>
          <option value="published">Published</option>
        </select>
      </form>
    `;
    const root = document.getElementById("f")!;
    const controller = attachFormController(root);
    controller.setValues({ status: "published" });
    const option = root.querySelector<HTMLOptionElement>('option[value="published"]')!;
    expect(option.selected).toBe(true);
    expect(option.textContent).toBe("Published");
    controller.destroy();
  });
});

describe("embedded model composition", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    resetGlobalRegistry();
  });

  afterEach(() => {
    resetDom();
    resetGlobalRegistry();
  });

  const sharedModel = embeddedFixture.form as unknown as EmbeddedModel;
  const model: EmbeddedModel = {
    fields: [
      ...sharedModel.fields,
      { name: "note", type: "string", required: false, validations: [{ kind: "maxLength", params: { value: "80" } }] },
    ],
  };

  function renderForm(): HTMLFormElement {
    document.body.innerHTML = `
      <form id="sample" action="/submit" method="post">
        <label><input type="radio" name="enabled" value="true">Yes</label>
        <label><input type="radio" name="enabled" value="false">No</label>
        <input name="level" type="text">
        <label><input name="colors[]" type="checkbox" value="blue">Blue</label>
        <label><input name="colors[]" type="checkbox" value="green">Green</label>
        <input name="note" maxlength="80">
      </form>
    `;
    return document.getElementById("sample") as HTMLFormElement;
  }

  it("validates selected canonical paths against the model without fetching or submitting", () => {
    const form = renderForm();
    const submits = vi.fn((event: Event) => event.preventDefault());
    form.addEventListener("submit", submits);
    const controller = attachFormController(form, { model });

    controller.setValues({ colors: ["blue"] }, { notify: false });
    expect(controller.validate({ paths: ["colors"] })).toEqual([]);
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:enabled"]);
    expect(controller.getValues()).toEqual({ level: "", colors: ["blue"], note: "" });

    controller.setValues({ enabled: true, level: 4 }, { notify: false });
    expect(controller.normalize()).toEqual({
      values: { enabled: true, level: 4, colors: ["blue"], note: "" },
      issues: [],
    });
    expect(controller.validate()).toEqual([]);
    expect(controller.getValues().level).toBe("4");

    controller.setValues({ level: 11 }, { notify: false });
    expect(controller.validate({ paths: ["level"] })).toEqual([
      { code: "max", path: "level", rendererPath: "level", message: "level must be at most 10" },
    ]);
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(submits).not.toHaveBeenCalled();
    controller.destroy();
    expect(controller.validate()).toEqual([]);
    expect(controller.normalize()).toEqual({ values: {}, issues: [] });
  });

  it("limits normalization issues to the selected scope", () => {
    const form = renderForm();
    const controller = attachFormController(form, { model });
    controller.setValues({ enabled: true, level: "abc", colors: ["blue"] }, { notify: false });
    expect(controller.validate({ paths: ["colors"] })).toEqual([]);
    expect(controller.validate({ paths: ["level"] }).map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:level"]);
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:level"]);
    controller.destroy();
  });

  it("requires a complete model for model-aware operations", () => {
    const form = renderForm();
    const controller = attachFormController(form);
    expect(() => controller.validate()).toThrow(/model/);
    expect(() => controller.normalize()).toThrow(/model/);
    expect(() => attachFormController(form, { model: { form: {} } as unknown as EmbeddedModel })).toThrow(/fields/);
    expect(controller.getValues()).toEqual({ level: "", note: "" });
    controller.destroy();
  });

  it("accepts the descriptor envelope and exposes helpers on the global", () => {
    const form = renderForm();
    const controller = attachFormController(form, { model: { version: "formgen.descriptor/v1", form: model } });
    controller.setValues({ enabled: false, level: 11, colors: ["green"] }, { notify: false });
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["max:level"]);
    const global = (globalThis as any).Formgen;
    expect(global.normalize(model, { enabled: "true", colors: ["blue"] }).values).toEqual({ enabled: true, colors: ["blue"] });
    expect(global.validate(model, { enabled: true, colors: ["blue"] })).toEqual([]);
    expect(global.validate(model, { enabled: false, colors: ["blue", "blue"] }).map((issue: { code: string }) => issue.code)).toEqual([
      "uniqueItems",
    ]);
    controller.destroy();
  });
});

describe("embedded issue presentation", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    resetGlobalRegistry();
  });

  afterEach(() => {
    resetDom();
    resetGlobalRegistry();
  });

  function renderNested(): HTMLFormElement {
    document.body.innerHTML = `
      <form id="nested" action="/submit" method="post">
        <fieldset data-group="contact">
          <input name="contact[email]" value="a@example.com" />
        </fieldset>
        <div data-formgen-array-items="true" data-formgen-array-name="rows">
          <div data-formgen-array-item="true" data-formgen-array-existing="true">
            <input name="rows[0].name" value="first" />
          </div>
          <div data-formgen-array-item="true" data-formgen-array-existing="true">
            <input name="rows[1].name" value="" />
          </div>
        </div>
        <input name="note" value="keep me" />
        <input type="hidden" name="token" data-field-name="token" value="t" />
      </form>
    `;
    return document.getElementById("nested") as HTMLFormElement;
  }

  function state(root: ParentNode, selector: string): string | null {
    return root.querySelector(selector)!.getAttribute("data-validation-state");
  }

  it("displays, focuses, and clears precise nested and repeated issues while preserving values (C11)", () => {
    const form = renderNested();
    const submits = vi.fn((event: Event) => event.preventDefault());
    form.addEventListener("submit", submits);
    const controller = attachFormController(form);
    const changes = vi.fn();
    controller.onChange(changes);

    const presented = controller.setIssues([
      { code: "required", path: "rows[1].name", rendererPath: "rows.name", message: "Name is required" },
      { code: "pattern", path: "contact.email", rendererPath: "contact.email", message: "Enter a valid email" },
    ]);
    expect(presented.field).toHaveLength(2);
    expect(presented.summary).toEqual([]);
    expect(state(form, 'input[name="rows[1].name"]')).toBe("invalid");
    expect(state(form, 'input[name="rows[0].name"]')).toBeNull();
    expect(state(form, 'input[name="contact[email]"]')).toBe("invalid");
    expect(form.querySelectorAll("[data-formgen-array-item][data-formgen-group-invalid]")).toHaveLength(1);
    expect(form.querySelector("fieldset")?.getAttribute("data-formgen-group-invalid")).toBe("true");
    expect(form.querySelector("[data-formgen-form-errors]")).toBeNull();

    const focus = controller.focusIssue({ code: "required", path: "rows[1].name", message: "Name is required" });
    expect(focus).toEqual({ focused: true, target: "control", path: "rows[1].name" });
    expect(document.activeElement).toBe(form.querySelector('input[name="rows[1].name"]'));

    expect(controller.getValues()).toEqual({
      contact: { email: "a@example.com" },
      rows: { 0: { name: "first" }, 1: { name: "" } },
      note: "keep me",
      token: "t",
    });

    controller.clearIssues();
    expect(state(form, 'input[name="rows[1].name"]')).toBeNull();
    expect(state(form, 'input[name="contact[email]"]')).toBeNull();
    expect(form.querySelectorAll("[data-formgen-group-invalid]")).toHaveLength(0);
    expect(controller.getValues().note).toBe("keep me");
    expect(changes).not.toHaveBeenCalled();
    expect(submits).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
    controller.destroy();
  });

  it("routes form-level and unmapped issues to a replaceable summary and focuses it as a fallback", () => {
    const form = renderNested();
    const controller = attachFormController(form);

    const presented = controller.setIssues([
      { code: "condition", path: "", message: "Definition rejected" },
      { code: "required", path: "ghost", message: "Ghost is required" },
      { code: "required", path: "token", message: "Token is required" },
    ]);
    expect(presented.field.map((issue) => issue.path)).toEqual(["token"]);
    expect(presented.summary.map((issue) => issue.path)).toEqual(["", "ghost"]);
    const summary = form.querySelector<HTMLElement>("[data-formgen-form-errors]")!;
    expect(summary.getAttribute("role")).toBe("alert");
    expect(summary.hidden).toBe(false);
    expect(Array.from(summary.querySelectorAll("li")).map((item) => item.textContent)).toEqual([
      "Definition rejected",
      "Ghost is required",
    ]);
    expect(form.firstElementChild).toBe(summary);

    // The hidden token control has an inline issue but cannot take focus; the summary does.
    const fallback = controller.focusIssue("token");
    expect(fallback).toEqual({ focused: true, target: "summary", path: "token" });
    expect(document.activeElement).toBe(summary);

    controller.setIssues([{ code: "x", message: "Only this" }]);
    expect(Array.from(summary.querySelectorAll("li")).map((item) => item.textContent)).toEqual(["Only this"]);
    controller.setIssues([{ code: "y", message: "And this" }], { append: true });
    expect(Array.from(summary.querySelectorAll("li")).map((item) => item.textContent)).toEqual(["Only this", "And this"]);
    controller.clearIssues();
    expect(summary.hidden).toBe(true);
    expect(controller.focusIssue("ghost")).toEqual({ focused: false, target: null, path: "ghost" });

    const custom = vi.fn();
    registerFormErrorRenderer("custom", custom);
    summary.setAttribute("data-formgen-form-errors-renderer", "custom");
    controller.setIssues([{ code: "z", message: "Custom" }]);
    expect(custom).toHaveBeenCalledTimes(1);
    expect(custom.mock.calls[0][0].messages).toEqual(["Custom"]);
    controller.destroy();
  });

  it("reuses a rendered summary container and keeps server issues across unrelated edits", () => {
    document.body.innerHTML = `
      <form id="rendered">
        <div role="alert" data-formgen-form-errors="true" tabindex="-1" hidden><ul class="list-disc"></ul></div>
        <input name="title" value="" />
        <input name="summary" value="" />
      </form>
    `;
    const form = document.getElementById("rendered") as HTMLFormElement;
    const controller = attachFormController(form);
    controller.setIssues([
      { code: "required", path: "title", message: "Title is required" },
      { code: "condition", path: "", message: "Fix the form" },
    ]);
    const summary = form.querySelector<HTMLElement>("[data-formgen-form-errors]")!;
    expect(form.querySelectorAll("[data-formgen-form-errors]")).toHaveLength(1);
    expect(summary.querySelector("ul")?.className).toBe("list-disc");
    expect(summary.hidden).toBe(false);

    const other = form.querySelector<HTMLInputElement>('input[name="summary"]')!;
    other.value = "typed elsewhere";
    other.dispatchEvent(new Event("input", { bubbles: true }));
    controller.setValues({ summary: "hydrated elsewhere" }, { notify: false });
    expect(state(form, 'input[name="title"]')).toBe("invalid");
    expect(summary.hidden).toBe(false);

    controller.setIssues([], { summary: true });
    expect(state(form, 'input[name="title"]')).toBeNull();
    expect(summary.hidden).toBe(true);
    controller.destroy();
  });

  it("marks and clears group chrome through setErrors and clearErrors", () => {
    const form = renderNested();
    const controller = attachFormController(form);
    controller.setErrors({ "rows.0.name": ["Bad"] });
    const items = form.querySelectorAll("[data-formgen-array-item]");
    expect(items[0].getAttribute("data-formgen-group-invalid")).toBe("true");
    expect(items[1].getAttribute("data-formgen-group-invalid")).toBeNull();
    controller.clearErrors(["rows.0.name"]);
    expect(items[0].getAttribute("data-formgen-group-invalid")).toBeNull();
    controller.destroy();
  });
});

describe("embedded reactive conditions", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    resetGlobalRegistry();
  });

  afterEach(() => {
    resetDom();
    resetGlobalRegistry();
  });

  const sharedModel = embeddedFixture.form as unknown as EmbeddedModel;
  const model: EmbeddedModel = {
    ...sharedModel,
    fields: [...sharedModel.fields, { name: "note", type: "string", required: false }],
  };

  const MARKUP = `
    <div data-component="input" id="enabled-wrap">
      <label><input type="radio" name="enabled" value="true">Yes</label>
      <label><input type="radio" name="enabled" value="false">No</label>
    </div>
    <div data-component="input" id="level-wrap"><input name="level" type="text"></div>
    <div data-component="input">
      <label><input name="colors[]" type="checkbox" value="blue">Blue</label>
      <label><input name="colors[]" type="checkbox" value="green">Green</label>
    </div>
    <div data-component="input"><input name="note"></div>
  `;

  function renderReactive(id = "reactive"): HTMLFormElement {
    document.body.innerHTML = `<form id="${id}">${MARKUP}</form>`;
    return document.getElementById(id) as HTMLFormElement;
  }

  function pick(root: ParentNode, selector: string): HTMLInputElement {
    return root.querySelector<HTMLInputElement>(selector)!;
  }

  function choose(root: ParentNode, value: "true" | "false"): void {
    const radio = pick(root, `input[name="enabled"][value="${value}"]`);
    radio.checked = true;
    radio.dispatchEvent(new Event("change", { bubbles: true }));
  }

  it("settles applicability on attach and user edits, clearing inapplicable values once (C07, C13)", () => {
    const form = renderReactive();
    const controller = attachFormController(form, { model });
    const levelWrap = form.querySelector<HTMLElement>("#level-wrap")!;
    const level = pick(form, 'input[name="level"]');
    expect(levelWrap.hidden).toBe(true);
    expect(levelWrap.getAttribute("data-formgen-applicable")).toBe("false");
    expect(level.required).toBe(false);
    expect(controller.effective().applicable).toEqual({ level: false });

    const changes = vi.fn();
    controller.onChange(changes);
    choose(form, "true");
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2]).toMatchObject({ origin: "program", applicable: { level: true } });
    expect(levelWrap.hidden).toBe(false);
    expect(level.required).toBe(true);
    expect(level.getAttribute("aria-required")).toBe("true");

    level.value = "4";
    level.dispatchEvent(new Event("input", { bubbles: true }));
    expect(changes).toHaveBeenCalledTimes(2);
    expect(controller.effective().values).toEqual({ enabled: true, level: 4, note: "" });

    choose(form, "false");
    expect(changes).toHaveBeenCalledTimes(3);
    expect(level.value).toBe("");
    expect(levelWrap.hidden).toBe(true);
    expect(level.required).toBe(false);
    expect(changes.mock.calls[2][2].paths).toEqual(["enabled", "level"]);
    expect(changes.mock.calls[2][2].applicable).toEqual({ level: false });
    expect(controller.getValues()).toEqual({ enabled: "false", level: "", note: "" });

    choose(form, "true");
    expect(level.value).toBe("");
    expect(levelWrap.hidden).toBe(false);
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:level", "required:colors"]);
    controller.destroy();
  });

  it("settles after silent hydration, replace restoration, and reset (C08)", () => {
    const form = renderReactive();
    const controller = attachFormController(form, { model });
    const levelWrap = form.querySelector<HTMLElement>("#level-wrap")!;
    const level = pick(form, 'input[name="level"]');
    const changes = vi.fn();
    controller.onChange(changes);

    controller.setValues({ enabled: true, level: 4, colors: ["blue"] }, { notify: false });
    expect(changes).not.toHaveBeenCalled();
    expect(levelWrap.hidden).toBe(false);
    expect(level.value).toBe("4");
    const snapshot = controller.getValues();

    controller.setValues({ enabled: false }, { notify: true });
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2]).toEqual({ origin: "hydrate", paths: ["enabled", "level"], applicable: { level: false } });
    expect(level.value).toBe("");
    expect(levelWrap.hidden).toBe(true);

    controller.setValues(snapshot, { mode: "replace", notify: false });
    expect(level.value).toBe("4");
    expect(levelWrap.hidden).toBe(false);
    expect(controller.effective().values).toEqual({ enabled: true, level: 4, colors: ["blue"], note: "" });

    controller.reset({});
    expect(changes).toHaveBeenCalledTimes(2);
    expect(changes.mock.calls[1][2].origin).toBe("reset");
    expect(levelWrap.hidden).toBe(true);
    expect(controller.getValues()).toEqual({ level: "", note: "" });
    controller.destroy();
  });

  it("keeps retained inactive values in the control but out of the effective projection and validation", () => {
    const retainModel: EmbeddedModel = {
      ...model,
      conditions: {
        version: "formgen.conditions/v1",
        rules: [{ path: "level", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" }],
      },
    };
    const form = renderReactive();
    const controller = attachFormController(form, { model: retainModel });
    const level = pick(form, 'input[name="level"]');
    controller.setValues({ enabled: true, level: 4, colors: ["blue"] }, { notify: false });
    choose(form, "false");
    expect(level.value).toBe("4");
    expect(form.querySelector<HTMLElement>("#level-wrap")!.hidden).toBe(true);
    const effective = controller.effective();
    expect(effective.values).toEqual({ enabled: false, colors: ["blue"], note: "" });
    expect(effective.retained).toEqual({ level: 4 });
    expect(controller.validate()).toEqual([]);
    choose(form, "true");
    expect(level.value).toBe("4");
    expect(controller.effective().values.level).toBe(4);
    controller.destroy();
  });

  it("clears obsolete issues on controls that become inapplicable", () => {
    const form = renderReactive();
    const controller = attachFormController(form, { model });
    const level = pick(form, 'input[name="level"]');
    controller.setValues({ enabled: true, level: 11, colors: ["blue"] }, { notify: false });
    controller.setIssues(controller.validate());
    expect(level.getAttribute("data-validation-state")).toBe("invalid");
    choose(form, "false");
    expect(level.getAttribute("data-validation-state")).toBeNull();
    expect(controller.validate()).toEqual([]);
    controller.destroy();
  });

  it("validates applicable scope and the whole form with conditional requiredness (C03, C09)", () => {
    const form = renderReactive();
    const controller = attachFormController(form, { model });
    choose(form, "true");
    expect(controller.validate({ paths: ["colors"] }).map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:colors"]);
    expect(controller.validate({ paths: ["level"] }).map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:level"]);
    pick(form, 'input[value="blue"]').checked = true;
    pick(form, 'input[value="blue"]').dispatchEvent(new Event("change", { bubbles: true }));
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:level"]);
    controller.destroy();
  });

  it("surfaces definition diagnostics as condition issues and keeps legacy behavior (C14)", () => {
    const broken: EmbeddedModel = {
      ...model,
      conditions: embeddedFixture.invalidConditions[0].definition as unknown as EmbeddedModel["conditions"],
    };
    const form = renderReactive();
    const controller = attachFormController(form, { model: broken });
    expect(controller.conditionDiagnostics().map((diagnostic) => diagnostic.code)).toEqual(["conditionReference"]);
    expect(form.querySelector<HTMLElement>("#level-wrap")!.hidden).toBe(false);
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual([
      "condition:level",
      "required:enabled",
      "required:colors",
    ]);
    expect(controller.effective().applicable).toEqual({});
    controller.destroy();
  });

  it("recomputes after supported row changes", () => {
    const rowsModel: EmbeddedModel = {
      fields: [
        { name: "enabled", type: "boolean", required: false },
        {
          name: "rows",
          type: "array",
          required: false,
          items: { name: "row", type: "object", required: false, nested: [{ name: "answer", type: "string", required: true }] },
        },
      ],
      conditions: {
        version: "formgen.conditions/v1",
        rules: [{ path: "rows[1].answer", applicableWhen: { op: "eq", path: "enabled", value: true } }],
      },
    };
    document.body.innerHTML = `
      <form id="rows">
        <div data-component="input"><input type="checkbox" name="enabled" value="true" checked></div>
        <div>
          <div data-formgen-array-items="true" data-formgen-array-name="rows" data-formgen-array-next-index="1" data-formgen-array-prototype-path="rows[1]" data-formgen-array-prototype-id-prefix="fg-rows-1">
            <div data-formgen-array-item="true"><div data-component="input"><input id="fg-rows-0-answer" name="rows[0].answer" value="first"></div></div>
            <template data-formgen-array-prototype="true"><div data-formgen-array-item="true"><div data-component="input"><input id="fg-rows-1-answer" name="rows[1].answer"></div></div></template>
          </div>
          <button type="button" data-formgen-array-action="add">Add row</button>
        </div>
      </form>
    `;
    const form = document.getElementById("rows") as HTMLFormElement;
    initArrayRepeaters(form);
    const controller = attachFormController(form, { model: rowsModel });
    const changes = vi.fn();
    controller.onChange(changes);
    expect(controller.effective().applicable).toEqual({ "rows[1].answer": false });

    form.querySelector<HTMLButtonElement>('[data-formgen-array-action="add"]')!.click();
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2]).toMatchObject({ origin: "program", paths: ["rows"], applicable: { "rows[1].answer": true } });
    const added = form.querySelector<HTMLInputElement>('input[name="rows[1].answer"]')!;
    expect(added.required).toBe(true);
    expect(added.closest<HTMLElement>("[data-component]")!.hidden).toBe(false);
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:rows[1].answer"]);
    controller.destroy();
  });

  it("stops settling after destroy and keeps roots isolated (C12)", () => {
    document.body.innerHTML = `<form id="a">${MARKUP}</form><form id="b">${MARKUP}</form>`;
    const rootA = document.getElementById("a") as HTMLFormElement;
    const rootB = document.getElementById("b") as HTMLFormElement;
    const controllerA = attachFormController(rootA, { model });
    const controllerB = attachFormController(rootB, { model });
    const wrap = (root: ParentNode) => root.querySelector('input[name="level"]')!.closest<HTMLElement>("[data-component]")!;
    choose(rootA, "true");
    expect(wrap(rootA).hidden).toBe(false);
    expect(wrap(rootB).hidden).toBe(true);

    controllerA.destroy();
    choose(rootA, "false");
    expect(wrap(rootA).hidden).toBe(false);
    choose(rootB, "true");
    expect(wrap(rootB).hidden).toBe(false);
    expect(controllerB.effective().applicable).toEqual({ level: true });
    controllerB.destroy();
  });

  it("exposes manual settle for host-driven DOM changes", () => {
    const form = renderReactive();
    const controller = attachFormController(form, { model });
    const radio = pick(form, 'input[name="enabled"][value="true"]');
    radio.checked = true; // no event dispatched
    expect(form.querySelector<HTMLElement>("#level-wrap")!.hidden).toBe(true);
    const projection = controller.settle();
    expect(projection.applicable).toEqual({ level: true });
    expect(form.querySelector<HTMLElement>("#level-wrap")!.hidden).toBe(false);
    controller.destroy();
  });

  it("keeps the rest of a replace payload when settling clears an inapplicable control", () => {
    const form = renderReactive();
    const controller = attachFormController(form, { model });
    controller.setValues({ enabled: true, level: 4, colors: ["blue"], note: "draft" }, { notify: false });
    const changes = vi.fn();
    controller.onChange(changes);

    // enabled=false makes level inapplicable; the clear must patch level only,
    // not re-run the replace against the controls the payload just set.
    controller.setValues({ enabled: false, level: 7, note: "kept" }, { mode: "replace", notify: true });
    expect(controller.getValues()).toEqual({ enabled: "false", level: "", note: "kept" });
    expect(controller.effective().values).toEqual({ enabled: false, note: "kept" });
    expect(changes).toHaveBeenCalledTimes(1);
    expect(changes.mock.calls[0][2].origin).toBe("hydrate");
    expect(changes.mock.calls[0][2].paths).toEqual(expect.arrayContaining(["enabled", "level", "note", "colors"]));

    // restore() replaces by default and must behave the same way.
    controller.setValues({ enabled: true, level: 3, note: "before" }, { mode: "replace", notify: false });
    const snapshot = controller.snapshot();
    snapshot.values.enabled = false;
    controller.restore(snapshot, { notify: false });
    expect(controller.getValues()).toEqual({ enabled: "false", level: "", note: "before" });
    controller.destroy();
  });

  it("does not report rendered defaults of inapplicable controls as inactive input", () => {
    const defaultsModel: EmbeddedModel = {
      fields: [
        { name: "enabled", type: "boolean", required: true },
        { name: "notify", type: "boolean", required: false },
        {
          name: "contact",
          type: "object",
          required: false,
          nested: [{ name: "email", type: "string", required: false }],
        },
      ],
      conditions: {
        version: "formgen.conditions/v1",
        rules: [
          { path: "notify", applicableWhen: { op: "eq", path: "enabled", value: true } },
          { path: "contact", applicableWhen: { op: "eq", path: "enabled", value: true } },
        ],
      },
    };
    document.body.innerHTML = `
      <form id="defaults">
        <div data-component="input">
          <label><input type="radio" name="enabled" value="true">Yes</label>
          <label><input type="radio" name="enabled" value="false">No</label>
        </div>
        <div data-component="input"><input type="checkbox" name="notify" value="true"></div>
        <div data-component="input"><input name="contact.email" type="email"></div>
      </form>
    `;
    const form = document.getElementById("defaults") as HTMLFormElement;
    const controller = attachFormController(form, { model: defaultsModel });
    choose(form, "false");
    // Raw collection yields notify=false and contact={email:""}; neither was supplied.
    expect(controller.normalize().values).toEqual({ enabled: false, notify: false, contact: { email: "" } });
    expect(controller.validate()).toEqual([]);
    expect(controller.validate({ paths: ["notify", "contact"] })).toEqual([]);
    expect(controller.effective().values).toEqual({ enabled: false });

    choose(form, "true");
    const notify = pick(form, 'input[name="notify"]');
    notify.checked = true;
    notify.dispatchEvent(new Event("change", { bubbles: true }));
    expect(controller.validate()).toEqual([]);
    expect(controller.effective().values).toEqual({ enabled: true, notify: true, contact: { email: "" } });
    controller.destroy();
  });

  it("relocates focus only from controls applicability actually hid", () => {
    const visible = renderReactive("visible");
    const keepVisible = attachFormController(visible, { model, conditions: { hideInapplicable: false } });
    choose(visible, "true");
    const level = pick(visible, 'input[name="level"]');
    level.focus();
    expect(document.activeElement).toBe(level);
    choose(visible, "false");
    expect(visible.querySelector<HTMLElement>("#level-wrap")!.getAttribute("data-formgen-applicable")).toBe("false");
    expect(visible.querySelector<HTMLElement>("#level-wrap")!.hidden).toBe(false);
    expect(document.activeElement).toBe(level);
    keepVisible.destroy();

    const hiding = renderReactive("hiding");
    const controller = attachFormController(hiding, { model });
    choose(hiding, "true");
    const hiddenLevel = pick(hiding, 'input[name="level"]');
    hiddenLevel.focus();
    choose(hiding, "false");
    expect(hiding.querySelector<HTMLElement>("#level-wrap")!.hidden).toBe(true);
    expect(document.activeElement).not.toBe(hiddenLevel);
    controller.destroy();
  });
});

describe("embedded choice components", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    resetGlobalRegistry();
  });

  afterEach(() => {
    resetDom();
    resetGlobalRegistry();
  });

  const model: EmbeddedModel = {
    fields: [
      { name: "enabled", type: "boolean", required: true },
      {
        name: "level",
        type: "integer",
        required: false,
        validations: [
          { kind: "min", params: { value: "0" } },
          { kind: "max", params: { value: "10" } },
        ],
      },
      {
        name: "colors",
        type: "array",
        required: true,
        items: { name: "color", type: "string", required: false, enum: ["blue", "green"] },
        validations: [{ kind: "minItems", params: { value: "1" } }, { kind: "uniqueItems" }],
      },
    ],
  };

  // Mirrors the markup emitted by the Go choice/scale/choice-multiple components.
  function choiceGroup(id: string, name: string, kind: string, type: string, options: Array<[string, string]>): string {
    return `
      <div data-component="${kind === "multiple" ? "choice-multiple" : kind === "scale" ? "scale" : "choice"}">
        <fieldset id="${id}" class="formgen-choice" data-formgen-choice="${kind}" data-field-path="${name.replace("[]", "")}"${kind === "multiple" ? "" : ' role="radiogroup"'} aria-labelledby="${id}-legend">
          <legend id="${id}-legend">${name}${kind === "multiple" ? '<span class="sr-only" data-formgen-required-hint="true" hidden>(required)</span>' : ""}</legend>
          <div class="formgen-choice-options" role="presentation">
            ${options
              .map(
                ([value, label], index) => `
            <label class="formgen-choice-option" for="${id}-${index}" data-formgen-choice-option="true">
              <input type="${type}" id="${id}-${index}" name="${name}" value="${value}" class="formgen-choice-input" data-formgen-choice-index="${index}">
              <span class="formgen-choice-text">${label}</span>
            </label>`
              )
              .join("")}
          </div>
          <p data-relationship-error="true" role="status" aria-live="polite" aria-atomic="true" id="${id}-error" class="formgen-error" aria-hidden="true"></p>
        </fieldset>
      </div>`;
  }

  function renderChoices(prefix = "fg"): HTMLFormElement {
    const scale = Array.from({ length: 11 }, (_unused, value) => [encodeEnumValue(value), String(value)] as [string, string]);
    document.body.innerHTML = `
      <form id="${prefix}-form">
        ${choiceGroup(`${prefix}-enabled`, "enabled", "single", "radio", [
          [encodeEnumValue(true), "Yes"],
          [encodeEnumValue(false), "No"],
        ])}
        ${choiceGroup(`${prefix}-level`, "level", "scale", "radio", scale)}
        ${choiceGroup(`${prefix}-colors`, "colors[]", "multiple", "checkbox", [
          ["blue", "Blue"],
          ["green", "Green"],
        ])}
      </form>
    `;
    return document.getElementById(`${prefix}-form`) as HTMLFormElement;
  }

  it("decodes typed selections, keeps false and zero, and starts unanswered (C02, C04, C10)", () => {
    const form = renderChoices();
    const controller = attachFormController(form, { model });
    expect(controller.getValues()).toEqual({});
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:enabled", "required:colors"]);

    const no = form.querySelector<HTMLLabelElement>('label[for="fg-enabled-1"]')!;
    no.click(); // label activation checks the radio
    expect(form.querySelector<HTMLInputElement>("#fg-enabled-1")!.checked).toBe(true);
    const zero = form.querySelector<HTMLInputElement>("#fg-level-0")!;
    zero.checked = true;
    zero.dispatchEvent(new Event("change", { bubbles: true }));
    const blue = form.querySelector<HTMLInputElement>("#fg-colors-0")!;
    blue.checked = true;
    blue.dispatchEvent(new Event("change", { bubbles: true }));

    expect(controller.getValues()).toEqual({ enabled: encodeEnumValue(false), level: encodeEnumValue(0), colors: ["blue"] });
    expect(controller.normalize()).toEqual({ values: { enabled: false, level: 0, colors: ["blue"] }, issues: [] });
    expect(controller.validate()).toEqual([]);
    controller.destroy();
  });

  it("presents group issues on the fieldset and focuses the first option", () => {
    const form = renderChoices();
    const controller = attachFormController(form, { model });
    const presented = controller.setIssues([
      { code: "required", path: "enabled", message: "Enable feature is required" },
      { code: "minItems", path: "colors", message: "Pick at least one color" },
    ]);
    expect(presented.field).toHaveLength(2);
    const enabledGroup = form.querySelector<HTMLElement>("#fg-enabled")!;
    expect(enabledGroup.getAttribute("data-validation-state")).toBe("invalid");
    expect(enabledGroup.getAttribute("aria-invalid")).toBe("true");
    expect(enabledGroup.querySelector("[data-relationship-error]")?.textContent).toBe("Enable feature is required");
    expect(enabledGroup.querySelectorAll("[data-relationship-error]")).toHaveLength(1);
    expect(form.querySelector<HTMLElement>("#fg-colors")!.getAttribute("data-validation-state")).toBe("invalid");

    expect(controller.focusIssue("enabled")).toEqual({ focused: true, target: "control", path: "enabled" });
    expect(document.activeElement).toBe(form.querySelector("#fg-enabled-0"));

    controller.clearIssues();
    expect(enabledGroup.getAttribute("data-validation-state")).toBeNull();
    expect(enabledGroup.querySelector("[data-relationship-error]")?.getAttribute("aria-hidden")).toBe("true");
    controller.destroy();
  });

  it("hydrates and replaces typed group values and keeps two instances independent (C08, C12)", () => {
    document.body.innerHTML = "";
    const formA = renderChoices("a");
    const bHTML = formA.outerHTML.replace(/a-/g, "b-");
    document.body.insertAdjacentHTML("beforeend", bHTML);
    const formB = document.getElementById("b-form") as HTMLFormElement;
    const controllerA = attachFormController(formA, { model });
    const controllerB = attachFormController(formB, { model });

    controllerA.setValues({ enabled: true, level: 4, colors: ["blue", "green"] }, { notify: false });
    expect(controllerA.normalize().values).toEqual({ enabled: true, level: 4, colors: ["blue", "green"] });
    expect(controllerB.getValues()).toEqual({});
    expect(formA.querySelector<HTMLInputElement>("#a-level-4")!.checked).toBe(true);

    const snapshot = controllerA.getValues();
    controllerA.setValues({ enabled: false, colors: [] }, { notify: false });
    expect(controllerA.normalize().values).toEqual({ enabled: false, level: 4 });
    controllerA.setValues(snapshot, { mode: "replace", notify: false });
    expect(controllerA.normalize().values).toEqual({ enabled: true, level: 4, colors: ["blue", "green"] });
    expect(controllerB.getValues()).toEqual({});
    controllerA.destroy();
    controllerB.destroy();
  });

  it("marks required groups through conditions without forcing every checkbox", () => {
    const conditional: EmbeddedModel = {
      ...model,
      // colors is only conditionally required here; static `required` would keep the marker.
      fields: model.fields.map((field) => (field.name === "colors" ? { ...field, required: false } : field)),
      conditions: {
        version: "formgen.conditions/v1",
        rules: [{ path: "colors", requiredWhen: { op: "eq", path: "enabled", value: true } }],
      },
    };
    const form = renderChoices();
    const controller = attachFormController(form, { model: conditional });
    controller.setValues({ enabled: true }, { notify: false });
    const group = form.querySelector<HTMLElement>("#fg-colors")!;
    const hint = group.querySelector<HTMLElement>("[data-formgen-required-hint]")!;
    const members = Array.from(form.querySelectorAll<HTMLInputElement>('input[name="colors[]"]'));
    // A checkbox fieldset is a plain group: aria-required would be invalid on it
    // and would demand every box on a member, so requiredness is text plus a marker.
    expect(group.getAttribute("data-formgen-required")).toBe("true");
    expect(group.hasAttribute("aria-required")).toBe(false);
    expect(hint.hidden).toBe(false);
    expect(members.every((input) => !input.required && !input.hasAttribute("aria-required"))).toBe(true);

    controller.setValues({ enabled: false }, { notify: false });
    expect(group.hasAttribute("data-formgen-required")).toBe(false);
    expect(hint.hidden).toBe(true);
    controller.destroy();
  });

  it("exposes conditional requiredness on radiogroups through aria-required", () => {
    const conditional: EmbeddedModel = {
      ...model,
      conditions: {
        version: "formgen.conditions/v1",
        rules: [{ path: "level", requiredWhen: { op: "eq", path: "enabled", value: true } }],
      },
    };
    const form = renderChoices();
    const controller = attachFormController(form, { model: conditional });
    controller.setValues({ enabled: true }, { notify: false });
    const group = form.querySelector<HTMLElement>("#fg-level")!;
    const members = Array.from(group.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
    expect(members.length).toBeGreaterThan(0);
    expect(group.getAttribute("aria-required")).toBe("true");
    expect(group.getAttribute("data-formgen-required")).toBe("true");
    // Native `required` on any member makes the radio group required; ARIA
    // supports `aria-required` on the radiogroup fieldset, not on `radio`.
    expect(members.every((input) => input.required && !input.hasAttribute("aria-required"))).toBe(true);
    controller.setValues({ enabled: false }, { notify: false });
    expect(group.hasAttribute("aria-required")).toBe(false);
    expect(group.hasAttribute("data-formgen-required")).toBe(false);
    expect(members.every((input) => !input.required && !input.hasAttribute("aria-required"))).toBe(true);
    controller.destroy();
  });
});

describe("embedded typed snapshots", () => {
  beforeEach(() => {
    fetchSpy.mockReset();
    (globalThis as any).fetch = fetchSpy;
    setGlobalConfig();
    resetComponentRegistryForTests();
    resetGlobalRegistry();
  });

  afterEach(() => {
    resetDom();
    resetGlobalRegistry();
  });

  const model: EmbeddedModel = {
    fields: [
      { name: "enabled", type: "boolean", required: true },
      { name: "level", type: "integer", required: false, validations: [{ kind: "min", params: { value: "1" } }, { kind: "max", params: { value: "10" } }] },
      { name: "note", type: "string", required: false },
      { name: "colors", type: "array", required: false, items: { name: "color", type: "string", required: false, enum: ["blue", "green"] } },
    ],
  };

  const MARKUP = `
    <div data-component="input"><input type="checkbox" name="enabled" value="true"></div>
    <div data-component="input"><input name="level" type="text"></div>
    <div data-component="input"><input name="note" type="text"></div>
    <div data-component="input">
      <label><input name="colors[]" type="checkbox" value="blue">Blue</label>
      <label><input name="colors[]" type="checkbox" value="green">Green</label>
    </div>
  `;

  function renderSnapshotForm(id = "snap"): HTMLFormElement {
    document.body.innerHTML = `<form id="${id}">${MARKUP}</form>`;
    return document.getElementById(id) as HTMLFormElement;
  }

  function edit(control: HTMLInputElement, action: () => void): void {
    action();
    control.dispatchEvent(new Event(control.type === "checkbox" ? "change" : "input", { bubbles: true }));
  }

  it("publishes finalized snapshots for hydration, reset, and restore", () => {
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model });
    const seen: ReturnType<typeof controller.snapshot>[] = [];
    controller.onChange(() => seen.push(controller.snapshot()));
    controller.setValues({ note: "saved", enabled: true }, {});
    expect(seen.at(-1)).toEqual(controller.snapshot());
    const saved = controller.snapshot();
    controller.reset({});
    expect(seen.at(-1)).toEqual(controller.snapshot());
    expect(seen.at(-1)!.presence.note).toBe("absent");
    controller.restore(saved);
    expect(seen.at(-1)).toEqual(saved);
    expect(controller.snapshot()).toEqual(saved);
    controller.destroy();
  });

  it("does not overwrite presence from a reentrant settled subscriber", () => {
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model });
    let entered = false;
    controller.onChange(() => {
      if (!entered) {
        entered = true;
        controller.setValues({ note: null }, { notify: false });
      }
    });
    controller.setValues({ note: "outer" }, {});
    expect(controller.snapshot().presence.note).toBe("null");
    expect(controller.snapshot().values.note).toBeNull();
    controller.destroy();
  });

  it("distinguishes absent from false, null from empty string, and explicit empty arrays (C01, C02, C10)", () => {
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model });
    const untouched = controller.snapshot();
    expect(untouched.presence).toEqual({ enabled: "absent", level: "absent", note: "absent", colors: "absent" });
    expect(untouched.values).toEqual({});
    expect(controller.normalize().values).toEqual({ enabled: false, level: null, note: "" });

    const enabled = form.querySelector<HTMLInputElement>('input[name="enabled"]')!;
    edit(enabled, () => (enabled.checked = true));
    edit(enabled, () => (enabled.checked = false));
    const level = form.querySelector<HTMLInputElement>('input[name="level"]')!;
    edit(level, () => (level.value = "5"));
    edit(level, () => (level.value = ""));
    const note = form.querySelector<HTMLInputElement>('input[name="note"]')!;
    edit(note, () => (note.value = "draft"));
    edit(note, () => (note.value = ""));
    const blue = form.querySelector<HTMLInputElement>('input[value="blue"]')!;
    edit(blue, () => (blue.checked = true));
    edit(blue, () => (blue.checked = false));

    const answered = controller.snapshot();
    expect(answered.presence).toEqual({ enabled: "present", level: "null", note: "present", colors: "present" });
    expect(answered.values).toEqual({ enabled: false, level: null, note: "", colors: [] });
    expect(answered.issues).toEqual([]);
    expect(answered.locked).toEqual([]);
    expect(controller.getValues()).toEqual({ enabled: false, level: "", note: "" });
    controller.destroy();
  });

  it("keeps locked values after controls are disabled and returns independent copies (C15)", () => {
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model });
    controller.setValues({ enabled: true, level: 4, colors: ["blue"], note: "keep" }, { notify: false });
    form.querySelectorAll<HTMLInputElement>("input").forEach((input) => (input.disabled = true));
    expect(controller.getValues()).toEqual({});

    const locked = controller.snapshot();
    expect(locked.values).toEqual({ enabled: true, level: 4, note: "keep", colors: ["blue"] });
    expect(locked.locked.sort()).toEqual(["colors", "enabled", "level", "note"]);
    expect(locked.presence.enabled).toBe("present");

    (locked.values.colors as string[]).push("green");
    locked.values.note = "tampered";
    const again = controller.snapshot();
    expect(again.values).toEqual({ enabled: true, level: 4, note: "keep", colors: ["blue"] });
    controller.destroy();
  });

  it("reports unsafe integers and unknown controls without rounding or guessing", () => {
    document.body.innerHTML = `<form id="big">${MARKUP}<input name="extra" value="host"></form>`;
    const form = document.getElementById("big") as HTMLFormElement;
    const controller = attachFormController(form, { model });
    const level = form.querySelector<HTMLInputElement>('input[name="level"]')!;
    edit(level, () => (level.value = "9007199254740993"));
    const snapshot = controller.snapshot();
    expect(snapshot.values.level).toBeNull();
    expect(snapshot.presence.level).toBe("null");
    expect(snapshot.issues.map((issue) => `${issue.code}:${issue.path}`).sort()).toEqual(["type:level", "unknownField:extra"]);
    expect(JSON.stringify(snapshot.values)).not.toContain("9007199254740992");
    controller.destroy();
  });

  it("restores snapshots into the same root with presence intact and keeps roots independent (C08, C12)", () => {
    document.body.innerHTML = `<form id="a">${MARKUP}</form><form id="b">${MARKUP}</form>`;
    const rootA = document.getElementById("a") as HTMLFormElement;
    const rootB = document.getElementById("b") as HTMLFormElement;
    const controllerA = attachFormController(rootA, { model });
    const controllerB = attachFormController(rootB, { model });
    controllerA.setValues({ enabled: true, level: 4, colors: ["blue"], note: null }, { notify: false });
    const snapshot = controllerA.snapshot();
    expect(snapshot.presence).toEqual({ enabled: "present", level: "present", note: "null", colors: "present" });
    expect(snapshot.values).toEqual({ enabled: true, level: 4, note: null, colors: ["blue"] });

    controllerA.setValues({ enabled: false, level: 9, colors: [], note: "edited" }, { notify: false });
    const changes = vi.fn();
    controllerA.onChange(changes);
    const issues = controllerA.restore(snapshot, { notify: false });
    expect(issues).toEqual([]);
    expect(changes).not.toHaveBeenCalled();
    expect(controllerA.snapshot().values).toEqual(snapshot.values);
    expect(controllerA.snapshot().presence).toEqual(snapshot.presence);
    expect(controllerB.snapshot().values).toEqual({});

    const tampered = JSON.parse(
      '{"version":"formgen.snapshot/v1","values":{"__proto__":{"polluted":true},"ghost":"x","enabled":false},"presence":{"__proto__.polluted":"present","ghost":"present","enabled":"present","rows[01].name":"present"},"retained":{},"issues":[],"locked":[]}'
    );
    const reported = controllerA.restore(tampered, { notify: false }).map((issue) => `${issue.code}:${issue.path}`);
    expect(reported).toEqual(["invalidPath:__proto__.polluted", "unknownField:ghost", "invalidPath:rows[01].name"]);
    expect((Object.prototype as Record<string, unknown>).polluted).toBeUndefined();
    expect(controllerA.snapshot().values.enabled).toBe(false);
    controllerA.destroy();
    controllerB.destroy();
  });

  it("drops detached rows and keeps retained inactive values separate", () => {
    const rowsModel: EmbeddedModel = {
      fields: [
        { name: "enabled", type: "boolean", required: false },
        { name: "level", type: "integer", required: false },
        { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "name", type: "string", required: false }] } },
      ],
      conditions: {
        version: "formgen.conditions/v1",
        rules: [{ path: "level", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" }],
      },
    };
    document.body.innerHTML = `
      <form id="rows">
        <div data-component="input"><input type="checkbox" name="enabled" value="true"></div>
        <div data-component="input"><input name="level" type="text"></div>
        <div>
          <div data-formgen-array-items="true" data-formgen-array-name="rows" data-formgen-array-next-index="2" data-formgen-array-prototype-path="rows[2]" data-formgen-array-prototype-id-prefix="fg-rows-2">
            <div data-formgen-array-item="true"><input name="rows[0].name"></div>
            <div data-formgen-array-item="true"><input name="rows[1].name"><button type="button" data-formgen-array-action="remove">x</button></div>
            <template data-formgen-array-prototype="true"><div data-formgen-array-item="true"><input id="fg-rows-2-name" name="rows[2].name"></div></template>
          </div>
          <button type="button" data-formgen-array-action="add">Add</button>
        </div>
      </form>
    `;
    const form = document.getElementById("rows") as HTMLFormElement;
    initArrayRepeaters(form);
    const controller = attachFormController(form, { model: rowsModel });
    controller.setValues({ enabled: true, level: 4, rows: [{ name: "first" }, { name: "second" }] }, { notify: false });
    expect(controller.snapshot().values).toEqual({ enabled: true, level: 4, rows: [{ name: "first" }, { name: "second" }] });

    controller.setValues({ enabled: false }, { notify: false });
    const retained = controller.snapshot();
    expect(retained.values).toEqual({ enabled: false, rows: [{ name: "first" }, { name: "second" }] });
    expect(retained.retained).toEqual({ level: 4 });
    expect(form.querySelector<HTMLInputElement>('input[name="level"]')!.value).toBe("4");

    form.querySelector<HTMLButtonElement>('[data-formgen-array-action="remove"]')!.click();
    const detached = controller.snapshot();
    expect(detached.issues.map((issue) => `${issue.code}:${issue.path}`)).toEqual(["detached:rows[1].name"]);
    expect(detached.values).toEqual({ enabled: false, rows: [{ name: "first" }] });
    expect(controller.snapshot().issues).toEqual([]);

    controller.restore(retained, { notify: false });
    expect(form.querySelector<HTMLInputElement>('input[name="level"]')!.value).toBe("4");
    expect(controller.effective().values).toEqual({ enabled: false, rows: [{ name: "first" }] });
    controller.destroy();
  });

  it("keeps locked conditional values when every control is disabled (C15 with conditions)", () => {
    const conditional: EmbeddedModel = {
      ...model,
      conditions: {
        version: "formgen.conditions/v1",
        rules: [{ path: "level", applicableWhen: { op: "eq", path: "enabled", value: true } }],
      },
    };
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model: conditional });
    controller.setValues({ enabled: true, level: 4, colors: ["blue"] }, { notify: false });
    form.querySelectorAll<HTMLInputElement>("input").forEach((input) => (input.disabled = true));
    expect(controller.getValues()).toEqual({});
    const locked = controller.snapshot();
    expect(locked.values).toEqual({ enabled: true, level: 4, colors: ["blue"] });
    expect(locked.locked.sort()).toEqual(["colors", "enabled", "level", "note"]);
    expect(locked.retained).toEqual({});
    controller.destroy();
  });

  it("seeds presence for controls rendered disabled and for added rows", () => {
    const rowsModel: EmbeddedModel = {
      fields: [
        { name: "note", type: "string", required: false },
        { name: "archived", type: "boolean", required: false },
        {
          name: "rows",
          type: "array",
          required: false,
          items: {
            name: "row",
            type: "object",
            required: false,
            nested: [
              { name: "name", type: "string", required: false },
              { name: "flag", type: "boolean", required: false },
            ],
          },
        },
      ],
    };
    document.body.innerHTML = `
      <form id="seed">
        <div data-component="input"><input name="note" value="locked note" disabled></div>
        <div data-component="input"><input type="checkbox" name="archived" value="true" disabled></div>
        <div>
          <div data-formgen-array-items="true" data-formgen-array-name="rows" data-formgen-array-next-index="1" data-formgen-array-prototype-path="rows[1]" data-formgen-array-prototype-id-prefix="fg-rows-1">
            <div data-formgen-array-item="true"><input name="rows[0].name" value="first"><input type="checkbox" name="rows[0].flag" value="true"></div>
            <template data-formgen-array-prototype="true"><div data-formgen-array-item="true"><input id="fg-rows-1-name" name="rows[1].name"><input type="checkbox" id="fg-rows-1-flag" name="rows[1].flag" value="true"></div></template>
          </div>
          <button type="button" data-formgen-array-action="add">Add</button>
        </div>
      </form>
    `;
    const form = document.getElementById("seed") as HTMLFormElement;
    initArrayRepeaters(form);
    const controller = attachFormController(form, { model: rowsModel });

    // Disabled at attach: the rendered value is locked, the unchecked box is absent.
    const initial = controller.snapshot();
    expect(initial.locked.sort()).toEqual(["archived", "note"]);
    expect(initial.presence.note).toBe("present");
    expect(initial.presence.archived).toBe("absent");
    expect(initial.values).toEqual({ note: "locked note", rows: [{ name: "first" }] });

    // A new row starts unanswered instead of contributing rendered defaults.
    form.querySelector<HTMLButtonElement>('[data-formgen-array-action="add"]')!.click();
    expect(form.querySelector('input[name="rows[1].name"]')).not.toBeNull();
    const added = controller.snapshot();
    expect(added.presence["rows[1].name"]).toBe("absent");
    expect(added.presence["rows[1].flag"]).toBe("absent");
    expect(JSON.stringify(added.values)).not.toContain('"flag"');
    // The row exists but none of its leaves were answered.
    expect(added.values).toEqual({ note: "locked note", rows: [{ name: "first" }, {}] });
    controller.destroy();
  });

  it("marks omitted paths absent on replace even when the control was already empty", () => {
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model });
    const note = form.querySelector<HTMLInputElement>('input[name="note"]')!;
    controller.setValues({ note: "x", level: 5 }, { notify: false });
    expect(controller.snapshot().presence.note).toBe("present");

    note.value = ""; // host-side DOM change without an event
    controller.setValues({ enabled: true }, { mode: "replace", notify: false });
    const replaced = controller.snapshot();
    expect(replaced.presence).toEqual({ enabled: "present", level: "absent", note: "absent", colors: "absent" });
    expect(replaced.values).toEqual({ enabled: true });
    controller.destroy();
  });

  it("round trips nested and indexed retained leaves, including retained containers", () => {
    const retainedModel: EmbeddedModel = {
      fields: [
        { name: "enabled", type: "boolean", required: false },
        { name: "profile", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] },
        { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] } },
      ],
      conditions: { version: "formgen.conditions/v1", rules: [
        { path: "profile.answer", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" },
        { path: "rows[0].answer", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" },
        { path: "rows[1]", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" },
      ] },
    };
    document.body.innerHTML = `<form><div><input type="checkbox" name="enabled"></div>
      <div><input name="profile.answer"></div><div><input name="rows[0].answer"></div><div><input name="rows[1].answer"></div></form>`;
    const form = document.querySelector("form")!;
    const controller = attachFormController(form, { model: retainedModel });
    controller.setValues({ enabled: true, profile: { answer: "saved" }, rows: [{ answer: "first" }, { answer: "second" }] }, { notify: false });
    controller.setValues({ enabled: false }, { notify: false });
    const saved = controller.snapshot();
    expect(saved.retained).toEqual({ "profile.answer": "saved", "rows[0].answer": "first", "rows[1]": { answer: "second" } });
    controller.setValues({ enabled: true, profile: { answer: "edited" }, rows: [{ answer: "edited" }, { answer: "edited" }] }, { notify: false });
    const seen: ReturnType<typeof controller.snapshot>[] = [];
    controller.onChange(() => seen.push(controller.snapshot()));
    controller.restore(saved);
    expect(controller.snapshot()).toEqual(saved);
    expect(seen).toEqual([saved]);
    controller.setValues({ enabled: true }, { notify: false });
    expect(controller.snapshot().values).toEqual({ enabled: true, profile: { answer: "saved" }, rows: [{ answer: "first" }, { answer: "second" }] });
    controller.destroy();
  });

  it("keeps explicit boolean null coherent in snapshots, conditions, and validation", () => {
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model });
    controller.setValues({ enabled: null }, { notify: false });
    expect(controller.getValues().enabled).toBe(false);
    expect(controller.normalize().values.enabled).toBe(false);
    expect(controller.snapshot().presence.enabled).toBe("null");
    expect(controller.snapshot().values.enabled).toBeNull();
    expect(controller.effective().values.enabled).toBeNull();
    expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toContain("required:enabled");
    controller.setValues({ enabled: false }, { notify: false });
    expect(controller.snapshot().values.enabled).toBe(false);
    expect(controller.validate()).toEqual([]);
    controller.destroy();
  });

  it("evaluates explicit null booleans before settling conditions and publishing", () => {
    const conditional: EmbeddedModel = { ...model, conditions: {
      version: "formgen.conditions/v1",
      rules: [{ path: "note", applicableWhen: { op: "eq", path: "enabled", value: false } }],
    } };
    const form = renderSnapshotForm();
    const controller = attachFormController(form, { model: conditional });
    controller.setValues({ enabled: false, note: "answer" }, { notify: false });
    const seen: ReturnType<typeof controller.snapshot>[] = [];
    controller.onChange(() => seen.push(controller.snapshot()));
    controller.setValues({ enabled: null }, {});
    expect(controller.effective().applicable.note).toBe(false);
    expect(controller.snapshot().values).toEqual({ enabled: null });
    expect(controller.snapshot().presence.note).toBe("absent");
    expect(seen).toEqual([controller.snapshot()]);
    const enabled = form.querySelector<HTMLInputElement>('[name="enabled"]')!;
    edit(enabled, () => (enabled.checked = true));
    expect(controller.snapshot().values.enabled).toBe(true);
    edit(enabled, () => (enabled.checked = false));
    expect(controller.effective().applicable.note).toBe(true);
    expect(controller.snapshot().values.enabled).toBe(false);
    controller.destroy();
  });

  it("honors inherited disabled state, nested fieldsets, and the first legend exception", () => {
    document.body.innerHTML = `<form><fieldset>
      <legend><input name="note" value="legend"></legend>
      <input name="level" value="4">
      <fieldset disabled><legend><input name="colors[]" type="checkbox" value="blue" checked></legend></fieldset>
      <legend><input name="enabled" type="checkbox" checked></legend>
    </fieldset></form>`;
    const form = document.querySelector("form")!;
    const controller = attachFormController(form, { model });
    const fieldset = form.querySelector("fieldset")!;
    fieldset.disabled = true;
    form.querySelector<HTMLInputElement>('[name="note"]')!.value = "edited legend";
    form.querySelector<HTMLInputElement>('[name="level"]')!.value = "9";
    expect(controller.getValues()).toEqual({ note: "edited legend" });
    const locked = controller.snapshot();
    expect(locked.locked.sort()).toEqual(["colors", "enabled", "level"]);
    expect(locked.values).toEqual({ note: "edited legend", level: 4, colors: ["blue"], enabled: true });
    controller.setValues({ note: "replaced" }, { mode: "replace", notify: false });
    expect(form.querySelector<HTMLInputElement>('[name="level"]')!.value).toBe("9");
    expect(controller.snapshot().values.level).toBe(4);
    expect(controller.focusIssue("level").focused).toBe(false);
    fieldset.disabled = false;
    expect(controller.getValues().level).toBe("9");
    controller.destroy();
  });

  it("requires a model and clears presence on destroy", () => {
    const form = renderSnapshotForm();
    const plain = attachFormController(form);
    expect(() => plain.snapshot()).toThrow(/model/);
    plain.destroy();
    const controller = attachFormController(form, { model });
    controller.destroy();
    expect(controller.snapshot()).toEqual({ version: "formgen.snapshot/v1", values: {}, presence: {}, retained: {}, issues: [], locked: [] });
  });
});


describe("condition boundary regressions", () => {
  afterEach(() => { resetDom(); resetGlobalRegistry(); });

  it("gates required descendants by inactive ancestors and restores each leaf's constraints", () => {
    document.body.innerHTML = `<form><input type="checkbox" name="enabled">
      <fieldset id="fg-profile">
        <div data-component="input"><input name="profile.answer" required></div>
        <div data-component="input"><input name="profile.optional"></div>
        <div data-component="input"><input name="profile.conditional"></div>
      </fieldset></form>`;
    const form = document.querySelector("form")!;
    const model: EmbeddedModel = {
      fields: [
        { name: "enabled", type: "boolean", required: false },
        { name: "profile", type: "object", required: false, nested: [
          { name: "answer", type: "string", required: true },
          { name: "optional", type: "string", required: false },
          { name: "conditional", type: "string", required: false },
        ] },
      ],
      conditions: { version: "formgen.conditions/v1", rules: [
        { path: "profile", applicableWhen: { op: "eq", path: "enabled", value: true } },
        { path: "profile.conditional", requiredWhen: { op: "eq", path: "enabled", value: true } },
      ] },
    };
    const controller = attachFormController(form, { model });
    const answer = form.querySelector<HTMLInputElement>('[name="profile.answer"]')!;
    const optional = form.querySelector<HTMLInputElement>('[name="profile.optional"]')!;
    const conditional = form.querySelector<HTMLInputElement>('[name="profile.conditional"]')!;
    expect(answer.required).toBe(false);
    expect(conditional.required).toBe(false);
    expect(form.checkValidity()).toBe(true);
    expect(controller.validate()).toEqual([]);
    controller.setValues({ enabled: true }, {});
    expect(answer.required).toBe(true);
    expect(optional.required).toBe(false);
    expect(conditional.required).toBe(true);
    expect(form.checkValidity()).toBe(false);
    controller.setValues({ enabled: false }, {});
    expect(answer.required).toBe(false);
    expect(conditional.required).toBe(false);
    expect(form.checkValidity()).toBe(true);
    controller.destroy();
  });
});

describe("embedded mutation boundary regressions", () => {
  afterEach(() => { document.body.innerHTML = ""; });

  it("notifies settled logical presence changes even when native values stay empty", () => {
    document.body.innerHTML = '<form><input name="answer" type="checkbox"></form>';
    const model: EmbeddedModel = { fields: [{ name: "answer", type: "boolean", required: true }] };
    const controller = attachFormController(document.querySelector("form")!, { model });
    const observed: Array<{ origin: string; paths: string[]; snapshot: ReturnType<typeof controller.snapshot> }> = [];
    controller.onChange((_values, _event, meta) => observed.push({ ...meta, snapshot: controller.snapshot() }));
    const absent = controller.snapshot();
    controller.setValues({ answer: false }, {});
    const answered = controller.snapshot();
    expect(observed).toHaveLength(1);
    expect(observed[0]).toMatchObject({ origin: "hydrate", paths: ["answer"], snapshot: answered });
    controller.setValues({ answer: false }, {});
    expect(observed).toHaveLength(1);
    controller.setValues({ answer: null }, {});
    expect(observed).toHaveLength(2);
    expect(observed[1].snapshot.values.answer).toBeNull();
    controller.reset({});
    expect(observed).toHaveLength(3);
    expect(observed[2]).toMatchObject({ origin: "reset", paths: ["answer"], snapshot: absent });
    controller.restore(answered);
    expect(observed).toHaveLength(4);
    expect(observed[3].snapshot).toEqual(answered);
    controller.restore(answered);
    expect(observed).toHaveLength(4);
    controller.restore(absent, { notify: false });
    expect(observed).toHaveLength(4);
    controller.destroy();
  });

  it.each([{}, null, [], "unrendered"])("rejects an unrendered structural row %j before mutation", (missingRow) => {
    document.body.innerHTML = '<form><input name="note" value="saved"><div data-formgen-array-name="rows"><input name="rows[0].answer" value="first"></div></form>';
    const controller = attachFormController(document.querySelector("form")!);
    const change = vi.fn();
    controller.onChange(change);
    expect(() => controller.setValues({ note: "changed", rows: [{ answer: "one" }, missingRow] }, { mode: "replace" })).toThrow(/rows\[1\]/);
    expect(controller.getValues()).toEqual({ note: "saved", rows: { 0: { answer: "first" } } });
    expect(change).not.toHaveBeenCalled();
    controller.destroy();
  });

  it("checks missing nested rows without treating a multi-select as a repeater", () => {
    document.body.innerHTML = '<form><input name="rows[0].children[0].answer" value="saved"><select name="colors[]" multiple><option value="blue">Blue</option><option value="green">Green</option></select></form>';
    const controller = attachFormController(document.querySelector("form")!);
    expect(() => controller.setValues({ rows: [{ children: [{ answer: "one" }, {}] }] }, { mode: "replace" })).toThrow(/rows\[0\].children\[1\]/);
    controller.setValues({ colors: ["blue", "green"] }, { mode: "replace", notify: false });
    expect(controller.getValues().colors).toEqual(["blue", "green"]);
    controller.destroy();
  });

  it("clears explicit container nulls down to leaf presence, including locked descendants", () => {
    document.body.innerHTML = '<form><input name="profile.answer" value="saved"><fieldset disabled><input name="rows[0].answer" value="locked"></fieldset><input name="note" value="keep"></form>';
    const model: EmbeddedModel = { fields: [
      { name: "profile", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] },
      { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] } },
      { name: "note", type: "string", required: false },
    ] };
    const controller = attachFormController(document.querySelector("form")!, { model });
    controller.setValues({ profile: null, "rows[0]": null }, {});
    const saved = controller.snapshot();
    expect(saved.values).toEqual({ profile: { answer: null }, rows: [{ answer: null }], note: "keep" });
    expect(saved.presence).toMatchObject({ "profile.answer": "null", "rows[0].answer": "null" });
    expect(saved.issues).toEqual([]);
    controller.destroy();
  });
});

describe("embedded snapshot boundary regressions", () => {
  afterEach(() => { document.body.innerHTML = ""; });

  it.each([false, true])("round trips raw-object leaves through ordinary and retained snapshots (retain=%s)", (retain) => {
    document.body.innerHTML = '<form><input type="checkbox" name="gate" checked><textarea name="config">{"answer":4,"tags":["a"]}</textarea><textarea name="rows[0].config">{"zero":0,"false":false}</textarea></form>';
    const model: EmbeddedModel = { fields: [
      { name: "gate", type: "boolean", required: true },
      { name: "config", type: "object", required: true },
      { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "config", type: "object", required: false }] } },
    ], conditions: retain ? { version: "formgen.conditions/v1", rules: [
      { path: "config", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: "retain" },
      { path: "rows[0].config", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: "retain" },
    ] } : undefined };
    const controller = attachFormController(document.querySelector("form")!, { model });
    if (retain) controller.setValues({ gate: false }, { notify: false });
    const saved = controller.snapshot();
    controller.setValues({ gate: true, config: '{"answer":5}', "rows[0].config": '{"zero":1}' }, { notify: false });
    expect(controller.restore(saved, { notify: false })).toEqual([]);
    expect(controller.snapshot()).toEqual(saved);
    controller.setValues({ gate: true, config: { answer: 6 }, rows: [{ config: { nested: { enabled: false } } }] }, { notify: false });
    expect(controller.snapshot().values).toEqual({ gate: true, config: { answer: 6 }, rows: [{ config: { nested: { enabled: false } } }] });
    controller.destroy();
  });

  it("normalizes explicit locked payloads and preserves normalization issues", () => {
    const no = encodeEnumValue(false);
    document.body.innerHTML = `<form><fieldset disabled><input name="level" value="3"><input type="radio" name="answer" value="${no}"><textarea name="config">{}</textarea></fieldset></form>`;
    const model: EmbeddedModel = { fields: [
      { name: "level", type: "integer", required: true }, { name: "answer", type: "boolean", required: true }, { name: "config", type: "object", required: false },
    ] };
    const controller = attachFormController(document.querySelector("form")!, { model });
    controller.setValues({ level: "4", answer: no, config: { answer: false } }, { notify: false });
    expect(controller.snapshot().values).toEqual({ level: 4, answer: false, config: { answer: false } });
    expect(controller.validate()).toEqual([]);
    controller.setValues({ level: "9007199254740993" }, { notify: false });
    const saved = controller.snapshot();
    expect(saved.values.level).toBeNull();
    expect(saved.issues.some((issue) => issue.path === "level" && issue.code === "type")).toBe(true);
    controller.destroy();
  });

  it("merges locked checkbox members with enabled edits without reading locked DOM mutations", () => {
    document.body.innerHTML = '<form><input type="checkbox" name="colors[]" value="red" checked><input type="checkbox" name="colors[]" value="blue" checked></form>';
    const model: EmbeddedModel = { fields: [{ name: "colors", type: "array", required: true, items: { name: "color", type: "string", required: false } }] };
    const form = document.querySelector("form")!;
    const controller = attachFormController(form, { model });
    const [red, blue] = Array.from(form.querySelectorAll("input"));
    expect(controller.snapshot().values.colors).toEqual(["red", "blue"]);
    red.disabled = true;
    red.checked = false;
    red.value = "tampered";
    expect(controller.snapshot().values.colors).toEqual(["red", "blue"]);
    expect(controller.snapshot().locked).toEqual(["colors"]);
    blue.checked = false;
    blue.dispatchEvent(new Event("change", { bubbles: true }));
    expect(controller.snapshot().values.colors).toEqual(["red"]);
    expect(controller.effective().values.colors).toEqual(["red"]);
    expect(controller.validate()).toEqual([]);
    // A deliberate hydration can replace disabled answers; omitted replacement cannot.
    controller.setValues({}, { mode: "replace", notify: false });
    expect(controller.snapshot().values.colors).toEqual(["red"]);
    controller.setValues({ colors: [] }, { notify: false });
    expect(controller.snapshot().values.colors).toEqual([]);
    controller.destroy();
  });

  it("seeds disabled selected options in partial groups and permits a new enabled radio selection", () => {
    document.body.innerHTML = '<form><input type="checkbox" name="colors[]" value="red" checked disabled><input type="checkbox" name="colors[]" value="blue"><input type="radio" name="answer" value="old" checked disabled><input type="radio" name="answer" value="new"></form>';
    const model: EmbeddedModel = { fields: [{ name: "colors", type: "array", required: false, items: { name: "color", type: "string", required: false } }, { name: "answer", type: "string", required: true }] };
    const form = document.querySelector("form")!;
    const controller = attachFormController(form, { model });
    expect(controller.snapshot().values).toEqual({ colors: ["red"], answer: "old" });
    const next = form.querySelector<HTMLInputElement>('[value="new"]')!;
    next.checked = true;
    next.dispatchEvent(new Event("change", { bubbles: true }));
    expect(controller.snapshot().values.answer).toBe("new");
    controller.destroy();
  });

  it("validates retained paths before resolving values and supports valid retained containers", () => {
    document.body.innerHTML = '<form><input name="answer" value="saved"><input name="rows[0].answer" value="row"></form>';
    const model: EmbeddedModel = { fields: [{ name: "answer", type: "string", required: false }, { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] } }] };
    const controller = attachFormController(document.querySelector("form")!, { model });
    const saved = controller.snapshot();
    saved.retained = JSON.parse('{"ghost[100000].answer":"bad","__proto__.polluted":"bad","rows[100000].answer":"bad","rows[0]":{"answer":"retained"}}');
    delete saved.values.rows;
    // Invalid indexes must never be materialized, even temporarily.
    const push = Array.prototype.push;
    let nullPushes = 0;
    Array.prototype.push = function(...args: unknown[]) {
      if (args.length === 1 && args[0] === null) nullPushes += 1;
      return push.apply(this, args);
    };
    let issues: ReturnType<typeof controller.restore>;
    try { issues = controller.restore(saved, { notify: false }); } finally { Array.prototype.push = push; }
    expect(issues!.map((issue) => `${issue.code}:${issue.path}`)).toEqual(["unknownField:ghost[100000].answer", "invalidPath:__proto__.polluted", "unknownField:rows[100000].answer"]);
    expect(nullPushes).toBeLessThan(10);
    expect(controller.snapshot().values).toEqual({ answer: "saved", rows: [{ answer: "retained" }] });
    expect(({} as Record<string, unknown>).polluted).toBeUndefined();
    controller.destroy();
  });
});

// Locked rendered defaults must participate before the first reactive clear.
it("settles conditions from locked defaults before clearing dependent answers", () => {
  document.body.innerHTML = '<form><input type="radio" name="gate" value="true" checked disabled><div><input name="answer" value="saved"></div></form>';
  const model: EmbeddedModel = { fields: [{ name: "gate", type: "boolean", required: true }, { name: "answer", type: "string", required: true }], conditions: { version: "formgen.conditions/v1", rules: [{ path: "answer", applicableWhen: { op: "eq", path: "gate", value: true } }] } };
  const controller = attachFormController(document.querySelector("form")!, { model });
  expect(controller.snapshot().values).toEqual({ gate: true, answer: "saved" });
  expect(controller.effective().applicable.answer).toBe(true);
  controller.destroy();
  document.body.innerHTML = "";
});

it("restores presence through normalized hydration, including absent partially locked groups", () => {
  document.body.innerHTML = '<form><fieldset disabled><input name="level" value="3"></fieldset><input name="colors[]" type="checkbox" value="red"><input name="colors[]" type="checkbox" value="blue"></form>';
  const model: EmbeddedModel = { fields: [{ name: "level", type: "integer", required: false }, { name: "colors", type: "array", required: false, items: { name: "color", type: "string", required: false } }] };
  const form = document.querySelector("form")!;
  const controller = attachFormController(form, { model });
  const saved = controller.snapshot();
  saved.values.level = "4"; // Supported normalization remains identical to setValues.
  controller.setValues({ colors: ["red"] }, { notify: false });
  form.querySelector<HTMLInputElement>('[value="red"]')!.disabled = true;
  controller.restore(saved, { notify: false });
  expect(controller.snapshot().values).toEqual({ level: 4 });
  expect(controller.effective().values).toEqual({ level: 4 });
  expect(controller.snapshot().presence.colors).toBe("absent");
  controller.destroy();
  document.body.innerHTML = "";
});

describe("integrated retained-policy boundaries (IR-2026-10-22-06)", () => {
  afterEach(() => { document.body.innerHTML = ""; });
  for (const parentPolicy of ["clear", "retain"] as const) {
    it(`preserves retained answers with a ${parentPolicy} parent across settlement and restore`, () => {
      const childPolicy = parentPolicy === "clear" ? "retain" : "clear";
      const model: EmbeddedModel = { fields: [
        { name: "gate", type: "boolean", required: false },
        { name: "profile", type: "object", required: false, nested: [
          { name: "answer", type: "string", required: false },
          { name: "other", type: "integer", required: false },
        ] },
      ], conditions: { version: "formgen.conditions/v1", rules: [
        { path: "profile", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: parentPolicy },
        { path: "profile.answer", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: childPolicy },
      ] } };
      document.body.innerHTML = '<form><div><input name="gate" type="checkbox" checked></div><fieldset id="fg-profile"><div><input name="profile.answer" value="saved"></div><div><input name="profile.other" value="4"></div></fieldset></form>';
      const controller = attachFormController(document.querySelector("form")!, { model });
      controller.setValues({ gate: false }, { notify: false });
      const retained = parentPolicy === "retain" ? { profile: { answer: "saved", other: 4 } } : { "profile.answer": "saved" };
      const saved = controller.snapshot();
      expect(saved.retained).toEqual(retained);
      expect(controller.effective().retained).toEqual(retained);
      expect(controller.settle().retained).toEqual(retained);
      expect(controller.snapshot()).toEqual(saved);
      controller.setValues({ gate: true, profile: { answer: "edited", other: 9 } }, { notify: false });
      expect(controller.restore(saved, { notify: false })).toEqual([]);
      expect(controller.snapshot()).toEqual(saved);
      controller.setValues({ gate: true }, { notify: false });
      expect((controller.snapshot().values.profile as Record<string, unknown>)?.answer).toBeUndefined();
      controller.setValues({ gate: true, profile: { answer: "new", other: 6 } }, { notify: false });
      controller.setValues({ gate: false }, { notify: false });
      controller.setValues({ "profile.answer": "replacement" }, { notify: false });
      expect(controller.snapshot().retained).toEqual(parentPolicy === "retain"
        ? { profile: { answer: "replacement", other: 6 } } : { "profile.answer": "replacement" });
      controller.reset({ notify: false });
      expect(controller.snapshot().retained).toEqual({});
      controller.destroy();
    });
  }
});

describe("repeater logical structure and notifications (IR-2026-10-22-06)", () => {
  afterEach(() => { document.body.innerHTML = ""; });
  const row = (index: number, existing: boolean) => `<div data-formgen-array-item data-formgen-array-path="rows[${index}]" data-formgen-array-existing="${existing}"><div><input name="rows[${index}].answer" value="saved"></div>${existing ? `<input name="rows[${index}]._delete" type="hidden" value="false">` : ""}<button type="button" data-formgen-array-action="remove">Remove</button></div>`;
  const markup = (existing: boolean) => `<form><div><input name="gate" type="checkbox" checked></div><div><div data-formgen-array-items data-formgen-array-name="rows" data-formgen-array-next-index="2" data-formgen-array-prototype-path="rows[2]">${row(0, existing)}${row(1, false)}<template data-formgen-array-prototype>${row(2, false)}</template></div><button type="button" data-formgen-array-action="add">Add</button></div></form>`;
  for (const conditional of [false, true]) {
    it(`excludes deleted rows from requiredness/counts and notifies with conditions=${conditional}`, () => {
      const model: EmbeddedModel = { fields: [
        { name: "gate", type: "boolean", required: false },
        { name: "rows", type: "array", required: false, validations: [{ kind: "minItems", params: { value: "2" } }], items: {
          name: "row", type: "object", required: true, nested: [{ name: "answer", type: "string", required: true }],
        } },
      ], ...(conditional ? { conditions: { version: "formgen.conditions/v1" as const, rules: [
        { path: "rows[0].answer", requiredWhen: { op: "present" as const, path: "gate" } },
      ] } } : {}) };
      document.body.innerHTML = markup(true);
      const form = document.querySelector("form")!;
      initArrayRepeaters(form);
      const controller = attachFormController(form, { model });
      const seen: Array<{ values: ReturnType<typeof controller.snapshot>["values"]; paths: string[] }> = [];
      controller.onChange((_values, _event, meta) => seen.push({ values: controller.snapshot().values, paths: meta!.paths }));
      expect(controller.validate()).toEqual([]);
      form.querySelector<HTMLButtonElement>('[data-formgen-array-action="remove"]')!.click();
      expect(seen).toHaveLength(1);
      expect(seen[0]).toEqual({ values: { gate: true, rows: [null, { answer: "saved" }] }, paths: ["rows"] });
      expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["minItems:rows"]);
      expect(controller.validate({ paths: ["rows[0].answer"] })).toEqual([]);
      expect(controller.effective().applicable["rows[0]"]).toBe(false);
      expect(controller.snapshot().presence).not.toHaveProperty("rows[0].answer");
      // Adding a new row leaves the deleted coordinate and its sibling stable.
      form.querySelector<HTMLButtonElement>('[data-formgen-array-action="add"]')!.click();
      expect(seen).toHaveLength(2);
      expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:rows[2].answer"]);
      controller.destroy();
      form.querySelector<HTMLButtonElement>('[data-formgen-array-action="add"]')!.click();
      expect(seen).toHaveLength(2);
    });
  }
  it("keeps physically removed coordinates inactive without treating wizard-hidden rows as deleted", () => {
    document.body.innerHTML = markup(false);
    const form = document.querySelector("form")!;
    initArrayRepeaters(form);
    const model: EmbeddedModel = { fields: [{ name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: true, nested: [{ name: "answer", type: "string", required: true }] } }, { name: "gate", type: "boolean", required: false }] };
    const controller = attachFormController(form, { model });
    form.querySelector<HTMLElement>('[data-formgen-array-path="rows[1]"]')!.hidden = true;
    expect(controller.snapshot().values.rows).toEqual([{ answer: "saved" }, { answer: "saved" }]);
    form.querySelector<HTMLButtonElement>('[data-formgen-array-action="remove"]')!.click();
    expect(controller.effective().values.rows).toEqual([null, { answer: "saved" }]);
    expect(controller.validate()).toEqual([]);
    expect(controller.snapshot().values.rows).toEqual([null, { answer: "saved" }]);
    expect(controller.validate()).toEqual([]);
    controller.destroy();
  });
});

describe("hydration preserves typed inputs before native encoding (IR-2026-10-22-06)", () => {
  afterEach(() => { document.body.innerHTML = ""; });
  for (const tag of ["radio", "checkbox", "select"] as const) {
    it(`restores reserved-prefix strings through ${tag} without confusing string and boolean false`, () => {
      const literal = encodeEnumValue(false); // A valid codec token used as a literal string code.
      const wrapped = encodeEnumValue(literal);
      const isArray = tag === "checkbox";
      const fields: EmbeddedModel["fields"] = [{ name: "code", type: isArray ? "array" : "string", required: false, ...(isArray
        ? { items: { name: "item", type: "string" as const, required: false, enum: [literal, "other", "false"] } }
        : { enum: [literal, "other", "false"] }) }];
      const markup = tag === "select" ? `<select name="code"><option value="${wrapped}" selected>Code</option><option value="other">Other</option><option value="false">False string</option></select>`
        : `<label><input type="${tag}" name="code${isArray ? "[]" : ""}" value="${wrapped}" checked></label><label><input type="${tag}" name="code${isArray ? "[]" : ""}" value="other"></label><label><input type="${tag}" name="code${isArray ? "[]" : ""}" value="false"></label>`;
      document.body.innerHTML = `<form>${markup}</form>`;
      const form = document.querySelector("form")!;
      const controller = attachFormController(form, { model: { fields } });
      const saved = controller.snapshot();
      expect(saved.values.code).toEqual(isArray ? [literal] : literal);
      controller.setValues({ code: isArray ? ["other"] : "other" }, { notify: false });
      expect(controller.restore(saved, { notify: false })).toEqual([]);
      expect(controller.snapshot()).toEqual(saved);
      expect(controller.validate()).toEqual([]);
      controller.setValues({ code: isArray ? [literal] : literal }, { notify: false });
      expect(controller.snapshot()).toEqual(saved);
      controller.setValues({ code: isArray ? ["false"] : "false" }, { notify: false });
      expect(controller.snapshot().values.code).toEqual(isArray ? ["false"] : "false");
      controller.destroy();
    });
  }
  for (const disabled of [false, true]) {
    it(`keeps lossy numeric payload diagnostics with disabled=${disabled} until an observed correction`, () => {
      document.body.innerHTML = `<form><fieldset ${disabled ? "disabled" : ""}><input name="n" type="number" value="3"></fieldset></form>`;
      const form = document.querySelector("form")!;
      const controller = attachFormController(form, { model: { fields: [{ name: "n", type: "integer", required: false }] } });
      const seen: ReturnType<typeof controller.snapshot>[] = [];
      controller.onChange(() => seen.push(controller.snapshot()));
      controller.setValues({ n: "bad" }, {});
      expect(seen).toHaveLength(1);
      expect(seen[0].values).toEqual({ n: null });
      expect(seen[0].issues.map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:n"]);
      expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:n"]);
      controller.setValues({ n: "bad" }, {});
      expect(seen).toHaveLength(1);
      controller.setValues({ n: 0 }, { notify: false });
      expect(controller.snapshot().values).toEqual({ n: 0 });
      expect(controller.validate()).toEqual([]);
      controller.setValues({ n: "bad" }, { notify: false });
      controller.reset({ notify: false });
      expect(controller.snapshot().values).toEqual({ n: 3 });
      expect(controller.validate()).toEqual([]);
      controller.destroy();
    });
  }
  it("diagnoses unmatched choices while preserving the intended logical value", () => {
    document.body.innerHTML = '<form><input name="color" type="radio" value="red" checked><input name="color" type="radio" value="blue"></form>';
    const form = document.querySelector("form")!;
    const controller = attachFormController(form, { model: { fields: [{ name: "color", type: "string", required: false, enum: ["red", "blue"] }] } });
    controller.setValues({ color: "green" }, { notify: false });
    expect(controller.snapshot().values).toEqual({ color: "green" });
    expect(controller.snapshot().issues.map((issue) => issue.code)).toEqual(["unrepresentable"]);
    expect(controller.validate().map((issue) => issue.code)).toEqual(["unrepresentable", "enum"]);
    const red = form.querySelector<HTMLInputElement>('[value="red"]')!;
    red.checked = true; red.dispatchEvent(new Event("change", { bubbles: true }));
    expect(controller.snapshot().values).toEqual({ color: "red" });
    expect(controller.validate()).toEqual([]);
    controller.destroy();
  });
});

it("round trips opaque JSON members through hydration, retention, locking and restore", () => {
  const raw = JSON.parse('{"constructor":"builder","prototype":{"a":1},"__proto__":{"flag":true},"nested":[{"constructor":{"b":2}}]}');
  document.body.innerHTML = '<form><div><input name="gate" type="checkbox" checked></div><div><textarea name="config"></textarea></div></form>';
  const model: EmbeddedModel = { fields: [{ name: "gate", type: "boolean", required: false }, { name: "config", type: "object", required: false }], conditions: { version: "formgen.conditions/v1", rules: [{ path: "config", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: "retain" }] } };
  const form = document.querySelector("form")!;
  const controller = attachFormController(form, { model });
  controller.setValues({ config: raw }, { notify: false });
  expect(JSON.stringify(controller.snapshot().values.config)).toBe(JSON.stringify(raw));
  controller.setValues({ gate: false }, { notify: false });
  const saved = controller.snapshot();
  expect(JSON.stringify(saved.retained.config)).toBe(JSON.stringify(raw));
  form.querySelector("textarea")!.disabled = true;
  controller.setValues({ config: { edited: true } }, { notify: false });
  expect(controller.restore(saved, { notify: false })).toEqual([]);
  expect(JSON.stringify(controller.snapshot().retained.config)).toBe(JSON.stringify(raw));
  expect(controller.validate()).toEqual([]);
  expect(({} as Record<string, unknown>).flag).toBeUndefined();
  controller.destroy(); document.body.innerHTML = "";
});

describe("snapshot normalization issue fidelity (IR-2026-10-22-06)", () => {
  afterEach(() => { document.body.innerHTML = ""; });
  for (const disabled of [false, true]) {
    it(`restores invalid typed state and issues before callbacks with disabled=${disabled}`, () => {
      document.body.innerHTML = `<form><fieldset ${disabled ? "disabled" : ""}><input name="n" value="bad"></fieldset><input name="other" value="kept"></form>`;
      const form = document.querySelector("form")!;
      const controller = attachFormController(form, { model: { fields: [{ name: "n", type: "integer", required: false }, { name: "other", type: "string", required: false }] } });
      const saved = controller.snapshot();
      expect(saved.issues.map((issue) => issue.code)).toEqual(["type"]);
      controller.setValues({ n: 5 }, { notify: false });
      const seen: ReturnType<typeof controller.snapshot>[] = [];
      controller.onChange(() => seen.push(controller.snapshot()));
      expect(controller.restore(saved)).toEqual([]);
      expect(controller.snapshot()).toEqual(saved);
      expect(seen).toEqual([saved]);
      expect(controller.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:n"]);
      expect(controller.validate({ paths: ["other"] })).toEqual([]);
      controller.restore(saved);
      expect(seen).toHaveLength(1);
      controller.setValues({ n: 0 }, { notify: false });
      expect(controller.validate()).toEqual([]);
      expect(controller.snapshot().issues).toEqual([]);
      controller.destroy();
    });
  }
});

describe("native choice presentation state (IR-2026-10-22-06)", () => {
  afterEach(() => { document.body.innerHTML = ""; });
  it("follows checked/disabled state across native edits, hydration, reset and restore", () => {
    document.body.innerHTML = '<form><fieldset data-formgen-choice="single"><label class="formgen-choice-option" data-selected="true"><input class="formgen-choice-input" type="radio" name="code" value="a" checked></label><label class="formgen-choice-option"><input class="formgen-choice-input" type="radio" name="code" value="b"></label></fieldset></form>';
    const form = document.querySelector("form")!;
    const controller = attachFormController(form, { model: { fields: [{ name: "code", type: "string", required: false, enum: ["a", "b"] }] } });
    const selected = () => Array.from(form.querySelectorAll<HTMLLabelElement>('.formgen-choice-option:has(.formgen-choice-input:checked)')).map((label) => label.querySelector("input")!.value);
    const disabled = () => form.querySelectorAll('.formgen-choice-option:has(.formgen-choice-input:disabled)').length;
    const saved = controller.snapshot();
    expect(selected()).toEqual(["a"]);
    form.querySelector<HTMLInputElement>('[value="b"]')!.click();
    expect(selected()).toEqual(["b"]);
    controller.setValues({ code: "a" }, { notify: false });
    expect(selected()).toEqual(["a"]);
    controller.setValues({ code: "b" }, { notify: false });
    expect(selected()).toEqual(["b"]);
    controller.reset({ notify: false });
    expect(selected()).toEqual(["a"]);
    controller.setValues({ code: "b" }, { notify: false });
    controller.restore(saved, { notify: false });
    expect(selected()).toEqual(["a"]);
    form.querySelector("fieldset")!.disabled = true;
    expect(disabled()).toBe(2);
    form.querySelector("fieldset")!.disabled = false;
    expect(disabled()).toBe(0);
    controller.destroy();
  });
});

it("normalizes model-backed option-like array hydration and omits repeater transport from typed state", () => {
  document.body.innerHTML = '<form><select name="colors[]" multiple><option value="red">Red</option><option value="blue">Blue</option></select><input name="rows__present" type="hidden" value="true"><input name="rows__complete" type="hidden" value="true"><input name="rows__clear" type="hidden" value="false"><div data-formgen-array-items data-formgen-array-name="rows"><div data-formgen-array-item><input name="rows[0]._row_key" type="hidden" value="key"><input name="rows[0].answer" value="saved"></div></div></form>';
  const m: EmbeddedModel = { fields: [{ name: "colors", type: "array", required: false, items: { name: "color", type: "string", required: false } }, { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] } }] };
  const c = attachFormController(document.querySelector("form")!, { model: m });
  c.setValues({ colors: [{ value: "red", label: "Red" }, { id: "blue" }] }, { notify: false });
  expect(c.snapshot().values).toEqual({ colors: ["red", "blue"], rows: [{ answer: "saved" }] });
  expect(c.snapshot().issues).toEqual([]);
  expect(Object.keys(c.snapshot().presence).sort()).toEqual(["colors", "rows[0].answer"]);
  c.destroy(); document.body.innerHTML = "";
});
