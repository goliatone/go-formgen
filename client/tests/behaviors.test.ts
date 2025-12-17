import { describe, it, beforeEach, afterEach, expect, vi } from "vitest";
import { initBehaviors, registerBehavior, __resetBehaviorsForTests } from "../src/behaviors";

beforeEach(() => {
  __resetBehaviorsForTests();
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("behaviors runtime", () => {
  it("autoSlug keeps slug in sync until the field is edited manually", () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <input id="fg-title" name="title" value="">
        <input
          id="fg-slug"
          name="slug"
          data-behavior="autoSlug"
          data-behavior-config='{"source":"title"}'
        >
      </form>
    `;

    const { dispose } = initBehaviors();
    const title = document.getElementById("fg-title") as HTMLInputElement;
    const slug = document.getElementById("fg-slug") as HTMLInputElement;

    title.value = "Hello World";
    title.dispatchEvent(new Event("input", { bubbles: true }));
    expect(slug.value).toBe("hello-world");

    slug.value = "custom-slug";
    slug.dispatchEvent(new Event("input", { bubbles: true }));
    expect(slug.getAttribute("data-behavior-state")).toBe("manual");

    title.value = "Updated Title";
    title.dispatchEvent(new Event("input", { bubbles: true }));
    expect(slug.value).toBe("custom-slug");

    slug.value = "";
    slug.dispatchEvent(new Event("input", { bubbles: true }));
    expect(slug.getAttribute("data-behavior-state")).toBeNull();

    title.value = "Final Value";
    title.dispatchEvent(new Event("input", { bubbles: true }));
    expect(slug.value).toBe("final-value");

    dispose();
  });

  it("autoResize adjusts textarea height based on content", () => {
    document.body.innerHTML = `
      <form data-formgen-auto-init="true">
        <textarea
          id="fg-notes"
          name="notes"
          rows="2"
          data-behavior="autoResize"
          data-behavior-config='{"minRows":2,"maxRows":6}'
          style="line-height: 20px; padding: 0; border: 0;"
        ></textarea>
      </form>
    `;

    const { dispose } = initBehaviors();
    const notes = document.getElementById("fg-notes") as HTMLTextAreaElement;

    Object.defineProperty(notes, "scrollHeight", { configurable: true, get: () => 40 });
    notes.value = "a";
    notes.dispatchEvent(new Event("input", { bubbles: true }));
    expect(notes.style.height).toBe("40px");

    Object.defineProperty(notes, "scrollHeight", { configurable: true, get: () => 120 });
    notes.value = "multi\nline\ncontent";
    notes.dispatchEvent(new Event("input", { bubbles: true }));
    expect(notes.style.height).toBe("120px");

    Object.defineProperty(notes, "scrollHeight", { configurable: true, get: () => 999 });
    notes.value = "clamped";
    notes.dispatchEvent(new Event("input", { bubbles: true }));
    expect(notes.style.height).toBe("120px");

    dispose();
  });

  it("passes behavior-specific configs when multiple behaviors share an element", () => {
    const alphaSpy = vi.fn();
    const betaSpy = vi.fn();

    registerBehavior("alpha", ({ config }) => {
      alphaSpy(config);
    });
    registerBehavior("beta", ({ config }) => {
      betaSpy(config);
    });

    document.body.innerHTML = `
      <input
        data-behavior="alpha beta"
        data-behavior-config='{"alpha":{"source":"title"},"beta":{"delay":200}}'
      >
    `;

    initBehaviors();
    expect(alphaSpy).toHaveBeenCalledWith({ source: "title" });
    expect(betaSpy).toHaveBeenCalledWith({ delay: 200 });
  });

  it("disposes custom behaviors registered at runtime", () => {
    const teardown = vi.fn();
    const factory = vi.fn(() => teardown);

    registerBehavior("custom", factory);

    document.body.innerHTML = `<input data-behavior="custom">`;
    const result = initBehaviors();

    expect(factory).toHaveBeenCalledTimes(1);
    result.dispose();
    expect(teardown).toHaveBeenCalledTimes(1);
  });
});
