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
});
