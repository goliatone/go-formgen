import { describe, expect, it, beforeEach, afterEach, vi } from "vitest";
import {
  initRelationships,
  initComponents,
  resetGlobalRegistry,
  registerThemeClasses,
  resetComponentRegistryForTests,
} from "../src/index";
import { __resetThemeClassesForTests } from "../src/theme/classes";

const originalFetch = globalThis.fetch;

function mockResponse(options: Array<{ value: string; label: string }>): Response {
  return new Response(JSON.stringify(options), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  document.body.innerHTML = "";
  resetGlobalRegistry();
  resetComponentRegistryForTests();
  __resetThemeClassesForTests();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  if (originalFetch) {
    globalThis.fetch = originalFetch.bind(globalThis);
  }
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  if (originalFetch) {
    globalThis.fetch = originalFetch.bind(globalThis);
  }
  document.body.innerHTML = "";
  resetGlobalRegistry();
  resetComponentRegistryForTests();
  __resetThemeClassesForTests();
});

describe("theme classes", () => {
  it("applies default tailwind utilities to chips container", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="chips" multiple data-endpoint-renderer="chips" data-endpoint-mode="search" data-endpoint-url="/api/mock" data-endpoint-method="GET"></select>
      </form>
    `;

    await initRelationships();

    const container = document.querySelector<HTMLElement>("[data-fg-chip-root='true']");
    expect(container).not.toBeNull();
    expect(container!.classList.contains("relative")).toBe(true);
    expect(container!.classList.contains("w-full")).toBe(true);
    expect(container!.classList.contains("text-sm")).toBe(true);
  });

  it("applies default tailwind utilities to typeahead container", async () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="manager" data-endpoint-renderer="typeahead" data-endpoint-mode="search" data-endpoint-url="/api/mock" data-endpoint-method="GET"></select>
      </form>
    `;

    await initRelationships();

    const container = document.querySelector<HTMLElement>("[data-fg-typeahead-root='true']");
    expect(container).not.toBeNull();
    expect(container!.classList.contains("relative")).toBe(true);
    expect(container!.classList.contains("w-full")).toBe(true);
    expect(container!.classList.contains("text-sm")).toBe(true);
  });

  it("accepts custom class overrides that persist after resolver updates", async () => {
    registerThemeClasses({
      chips: {
        container: ["bg-slate-50", "border", "border-slate-300"],
        actionClear: ["hover:text-red-500"],
      },
    });

    const fetchSpy = vi.fn().mockResolvedValue(
      mockResponse([
        { value: "design", label: "Design" },
        { value: "ops", label: "Operations" },
      ])
    );
    vi.stubGlobal("fetch", fetchSpy as unknown as typeof fetch);

    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <select id="tags" multiple data-endpoint-renderer="chips" data-endpoint-url="/api/tags" data-endpoint-method="GET"></select>
      </form>
    `;

    const select = document.getElementById("tags") as HTMLSelectElement;
    const registry = await initRelationships();

    await registry.resolve(select);

    const container = document.querySelector<HTMLElement>("[data-fg-chip-root='true']");
    expect(container).not.toBeNull();
    expect(container!.classList.contains("bg-slate-50")).toBe(true);

    const clearButton = container!.querySelector<HTMLButtonElement>('[aria-label="Clear selection"]');
    expect(clearButton).not.toBeNull();
    expect(clearButton!.classList.contains("hover:text-red-500")).toBe(true);

    fetchSpy.mockResolvedValue(mockResponse([{ value: "ux", label: "UX" }]));
    await registry.resolve(select);

    expect(container!.classList.contains("bg-slate-50")).toBe(true);
  });

  it("applies file uploader theme overrides", () => {
    registerThemeClasses({
      fileUploader: {
        dropzone: ["bg-red-100"],
      },
    });

    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <div data-component="file_uploader" data-component-config='{"variant":"dropzone","uploadEndpoint":"/api/files"}'>
          <input type="text" name="attachments">
        </div>
      </form>
    `;

    initComponents(document);

    const dropzone = document.querySelector<HTMLElement>('[data-component="file_uploader"] [role="button"]');
    expect(dropzone).not.toBeNull();
    expect(dropzone!.classList.contains("bg-red-100")).toBe(true);
  });
});
