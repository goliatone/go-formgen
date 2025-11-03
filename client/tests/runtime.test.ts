import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { h } from "preact";
import { render } from "preact";
import { act } from "preact/test-utils";
import { useEffect } from "preact/hooks";
import {
  initRelationships,
  type ResolverEventDetail,
  type ResolverRegistry,
} from "../src/index";
import { useRelationshipOptions } from "../src/frameworks/preact";

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
      <select id="project_owner" name="project[owner_id]" data-endpoint-url="/api/users" data-endpoint-method="GET" data-relationship-cardinality="one" ${
        extra ?? ""
      }></select>
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
  });

  afterEach(() => {
    resetDom();
  });

  it("invokes global hooks and transforms options", async () => {
    createMarkup();
    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

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

  it("emits lifecycle events with resolver detail", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

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
    expect(details[1].options?.[0].label).toBe("ALICE");
    expect(field.getAttribute("data-state")).toBe("ready");
  });

  it("avoids duplicate fetches when cache is enabled", async () => {
    const field = createMarkup();
    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

    const registry = await initRelationships({ cache: { strategy: "memory", ttlMs: 1000 } });
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    fetchSpy.mockClear();
    fetchSpy.mockRejectedValue(new Error("network"));

    await registry.resolve(field);
    expect(fetchSpy).not.toHaveBeenCalled();
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
    expect(container?.classList.contains("fg-chip-select")).toBe(true);
    expect(container.classList.contains("fg-chip-select--ready")).toBe(true);

    const toggle = container.querySelector<HTMLButtonElement>(".fg-chip-select__action--toggle");
    expect(toggle).not.toBeNull();
    toggle!.click();
    await flush();

    const optionButton = container.querySelector<HTMLButtonElement>(
      ".fg-chip-select__menu-item[data-value='design']"
    );
    expect(optionButton).not.toBeNull();
    optionButton!.click();
    await flush();

    expect(Array.from(select.selectedOptions).map((item) => item.value)).toContain("design");

    const chip = container.querySelector("[data-fg-chip-value='design']");
    expect(chip).not.toBeNull();

    const remove = chip!.querySelector<HTMLButtonElement>(".fg-chip-select__chip-remove");
    expect(remove).not.toBeNull();
    remove!.click();
    await flush();

    expect(Array.from(select.selectedOptions).map((item) => item.value)).not.toContain("design");

    const menu = container.querySelector<HTMLElement>(".fg-chip-select__menu");
    expect(menu?.hidden).toBe(true);
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
        state.refresh();
      }, [element]);
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
          data-endpoint-dynamic-params-q="{{self}}"
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

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    const errorNode = document.querySelector("[data-relationship-error]");
    expect(errorNode).toBeNull();
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
        <button type="button" data-endpoint-refresh-target="project_owner">Reload</button>
      </form>
    `;

    fetchSpy.mockResolvedValue(mockResponse([{ value: "1", label: "Alice" }]));

    await initRelationships();

    expect(fetchSpy).not.toHaveBeenCalled();

    const button = document.querySelector<HTMLButtonElement>(
      '[data-endpoint-refresh-target="project_owner"]'
    )!;
    button.click();

    await flush();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
  });
});
