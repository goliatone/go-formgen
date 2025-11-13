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
  hydrateFormValues,
  resetGlobalRegistry,
  resetComponentRegistryForTests,
  type ResolverEventDetail,
  type ResolverRegistry,
  registerErrorRenderer,
} from "../src/index";
import { useRelationshipOptions } from "../src/frameworks/preact";
import { setGlobalConfig, getGlobalConfig } from "../src/config";

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
  });

  afterEach(() => {
    resetDom();
    // Clean up global registry
    resetGlobalRegistry();
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

    const searchInput = chipContainer!.querySelector<HTMLInputElement>('input[type="search"]');
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

    const searchInput = container.querySelector<HTMLInputElement>('input[type="search"]')!;
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

    const menu = container.querySelector<HTMLElement>("[role='listbox']");
    expect(menu?.hidden).toBe(true);
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
    expect(clear!.disabled).toBe(false);

    const inputEvent = vi.fn();
    select.addEventListener("input", inputEvent);

    clear!.click();
    await flush();

    expect(select.value).toBe("");
    expect(input!.value).toBe("");
    expect(clear!.disabled).toBe(true);
    if (inputEvent.mock.calls.length === 0) {
      await flush();
    }
    expect(inputEvent).toHaveBeenCalled();
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
  });
});
