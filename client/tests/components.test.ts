import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  registerComponent,
  initComponents,
  resetComponentRegistryForTests,
} from "../src/index";

describe("component registry", () => {
  beforeEach(() => {
    resetComponentRegistryForTests();
    document.body.innerHTML = "";
  });

  it("registers and boots components", () => {
    const factory = vi.fn();
    registerComponent("custom", ({ element, config }) => {
      factory(config);
      element.setAttribute("data-enhanced", "true");
    });

    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="custom" data-component-config='{"foo":"bar"}'></div>
      </form>
    `;

    initComponents(document);

    expect(factory).toHaveBeenCalledTimes(1);
    expect(factory.mock.calls[0][0]).toEqual({ foo: "bar" });
    expect(document.querySelector("[data-enhanced='true']")).not.toBeNull();
  });

  it("avoids duplicate initialisation", () => {
    const factory = vi.fn();
    registerComponent("custom", factory);

    const root = document.createElement("div");
    root.innerHTML = `<div data-component="custom"></div>`;
    document.body.appendChild(root);

    initComponents(root);
    initComponents(root);

    expect(factory).toHaveBeenCalledTimes(1);
  });

  it("ignores unknown component names", () => {
    document.body.innerHTML = `<div data-component="missing"></div>`;
    expect(() => initComponents(document)).not.toThrow();
  });

  it("boots file uploader component via registry", () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="file_uploader" data-component-config='{"uploadEndpoint":"/api/uploads"}'>
          <input type="text" name="avatar" id="avatar">
        </div>
      </form>
    `;

    expect(() => initComponents(document)).not.toThrow();
    const hidden = document.querySelector("[data-fg-uploader-hidden]");
    expect(hidden).not.toBeNull();
  });

  it("boots media picker component via registry", async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/api/media/capabilities")) {
        return new Response(
          JSON.stringify({
            operations: { list: true, resolve: true },
            upload: { direct_upload: false, presign: false },
            picker: { value_modes: ["url"], default_value_mode: "url" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      if (url.endsWith("/api/media/resolve")) {
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
        <div data-component="media_picker" data-component-config='{"libraryPath":"/api/media/library","resolveEndpoint":"/api/media/resolve","capabilitiesEndpoint":"/api/media/capabilities"}'>
          <input type="text" name="hero" id="hero" value="/assets/hero.jpg">
        </div>
      </form>
    `;

    initComponents(document);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(document.querySelector("[data-media-picker-selections]")).not.toBeNull();
    expect(document.querySelector("[data-media-picker-selection]")).not.toBeNull();
  });
});
