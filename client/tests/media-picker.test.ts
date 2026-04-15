import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { initComponents, resetComponentRegistryForTests } from "../src/index";

function setInputFiles(input: HTMLInputElement, files: File[]): void {
  const fileList: FileList = {
    length: files.length,
    item: (index: number) => files[index] ?? null,
    [Symbol.iterator]() {
      let pointer = 0;
      return {
        next: () => ({
          done: pointer >= files.length,
          value: files[pointer++],
        }),
      };
    },
  } as unknown as FileList;

  Object.defineProperty(input, "files", {
    configurable: true,
    get: () => fileList,
  });
}

async function flushAsync(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
}

function requestURL(input: RequestInfo | URL): string {
  return String(input);
}

describe("media picker component", () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    resetComponentRegistryForTests();
    document.body.innerHTML = "";
    if (originalFetch) {
      globalThis.fetch = originalFetch.bind(globalThis);
    }
  });

  afterEach(() => {
    vi.restoreAllMocks();
    if (originalFetch) {
      globalThis.fetch = originalFetch.bind(globalThis);
    }
  });

  it("hydrates URL-backed values through media.resolve without scanning the library", async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = requestURL(input);
      if (url.endsWith("/api/media/capabilities")) {
        return new Response(
          JSON.stringify({
            operations: { list: true, resolve: true, upload: false, presign: false, confirm: false },
            upload: { direct_upload: false, presign: false },
            picker: { value_modes: ["url"], default_value_mode: "url" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.endsWith("/api/media/resolve")) {
        expect(init?.method).toBe("POST");
        expect(String(init?.body)).toContain(`"url":"/assets/hero.jpg"`);
        return new Response(
          JSON.stringify({
            id: "hero-1",
            name: "Hero",
            url: "/assets/hero.jpg",
            thumbnail: "/assets/hero-thumb.jpg",
            type: "image",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    });
    vi.stubGlobal("fetch", fetchSpy as unknown as typeof fetch);

    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="media_picker" data-component-config='{"libraryPath":"/api/media/library","resolveEndpoint":"/api/media/resolve","capabilitiesEndpoint":"/api/media/capabilities","valueMode":"url"}'>
          <input type="text" name="hero" value="/assets/hero.jpg">
        </div>
      </form>
    `;

    initComponents(document);
    await flushAsync();

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    const hidden = document.querySelector<HTMLInputElement>('input[type="hidden"][name="hero"]');
    expect(hidden?.value).toBe("/assets/hero.jpg");
    expect(document.querySelector("[data-media-picker-selection]")?.textContent).toContain("Hero");
  });

  it("hydrates ID-backed values through media.item when enabled", async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL) => {
      const url = requestURL(input);
      if (url.endsWith("/api/media/capabilities")) {
        return new Response(
          JSON.stringify({
            operations: { get: true, resolve: true, upload: false, presign: false, confirm: false },
            upload: { direct_upload: false, presign: false },
            picker: { value_modes: ["url", "id"], default_value_mode: "id" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.endsWith("/api/media/library/123")) {
        return new Response(
          JSON.stringify({
            id: "123",
            name: "Archive Hero",
            url: "/assets/archive-hero.jpg",
            thumbnail: "/assets/archive-hero-thumb.jpg",
            type: "image",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    });
    vi.stubGlobal("fetch", fetchSpy as unknown as typeof fetch);

    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="media_picker" data-component-config='{"itemEndpoint":"/api/media/library/:id","capabilitiesEndpoint":"/api/media/capabilities","valueMode":"id"}'>
          <input type="text" name="hero_id" value="123">
        </div>
      </form>
    `;

    initComponents(document);
    await flushAsync();

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    const hidden = document.querySelector<HTMLInputElement>('input[type="hidden"][name="hero_id"]');
    expect(hidden?.value).toBe("123");
    expect(document.querySelector("[data-media-picker-selection]")?.textContent).toContain("Archive Hero");
  });

  it("selects upload mode from capabilities and uses presign+confirm when multipart is disabled", async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = requestURL(input);
      if (url.endsWith("/api/media/capabilities")) {
        return new Response(
          JSON.stringify({
            operations: { upload: false, presign: true, confirm: true },
            upload: { direct_upload: false, presign: true },
            picker: { value_modes: ["url"], default_value_mode: "url" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.endsWith("/api/media/presign")) {
        expect(init?.method).toBe("POST");
        return new Response(
          JSON.stringify({
            upload_url: "https://uploads.example.com/object",
            method: "PUT",
            upload_id: "up-1",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url === "https://uploads.example.com/object") {
        expect(init?.method).toBe("PUT");
        return new Response("", { status: 200 });
      }
      if (url.endsWith("/api/media/confirm")) {
        expect(init?.method).toBe("POST");
        return new Response(
          JSON.stringify({
            id: "new-asset",
            name: "new-hero.jpg",
            url: "/assets/new-hero.jpg",
            thumbnail: "/assets/new-hero-thumb.jpg",
            type: "image",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    });
    vi.stubGlobal("fetch", fetchSpy as unknown as typeof fetch);

    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="media_picker" data-component-config='{"uploadEndpoint":"/api/media/upload","presignEndpoint":"/api/media/presign","confirmEndpoint":"/api/media/confirm","capabilitiesEndpoint":"/api/media/capabilities"}'>
          <input type="text" name="hero">
        </div>
      </form>
    `;

    initComponents(document);
    await flushAsync();

    const fileInput = document.querySelector<HTMLInputElement>('[data-component="media_picker"] input[type="file"]');
    expect(fileInput).not.toBeNull();

    const file = new File(["binary"], "new-hero.jpg", { type: "image/jpeg" });
    setInputFiles(fileInput!, [file]);
    fileInput!.dispatchEvent(new Event("change"));
    await flushAsync();

    const requested = fetchSpy.mock.calls.map(([input]) => requestURL(input as RequestInfo | URL));
    expect(requested).toContain("https://uploads.example.com/object");
    expect(requested.some((url) => url.endsWith("/api/media/upload"))).toBe(false);

    const hidden = document.querySelector<HTMLInputElement>('input[type="hidden"][name="hero"]');
    expect(hidden?.value).toBe("/assets/new-hero.jpg");
    const preview = document.querySelector<HTMLImageElement>("[data-media-picker-selection] img");
    expect(preview?.getAttribute("src")).toBe("/assets/new-hero-thumb.jpg");
  });
});
