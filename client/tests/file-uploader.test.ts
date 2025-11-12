import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
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
}

describe("file uploader component", () => {
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

  it("uploads immediately when autoUpload=true", async () => {
    const mockResponse = {
      name: "uploads/avatar.jpg",
      originalName: "avatar.jpg",
      size: 1234,
      contentType: "image/jpeg",
      url: "/uploads/avatar.jpg",
    };

    const fetchSpy = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(mockResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );
    vi.stubGlobal("fetch", fetchSpy as unknown as typeof fetch);

    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="file_uploader" data-component-config='{"uploadEndpoint":"/api/uploads"}'>
          <input type="text" name="avatar" id="avatar">
        </div>
      </form>
    `;

    initComponents(document);

    const fileInput = document.querySelector<HTMLInputElement>('input[type="file"]');
    expect(fileInput).not.toBeNull();

    const file = new File(["hello"], "avatar.jpg", { type: "image/jpeg" });
    setInputFiles(fileInput!, [file]);
    fileInput!.dispatchEvent(new Event("change"));

    await flushAsync();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const hidden = document.querySelector<HTMLInputElement>('input[name="avatar"]');
    expect(hidden?.value).toBe("/uploads/avatar.jpg");
  });

  it("queues uploads when autoUpload=false and submits sequentially", async () => {
    const mockResponse = {
      url: "/uploads/doc.pdf",
      originalName: "doc.pdf",
      size: 20,
      contentType: "application/pdf",
      name: "doc.pdf",
    };
    const fetchSpy = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(mockResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );
    vi.stubGlobal("fetch", fetchSpy as unknown as typeof fetch);

    document.body.innerHTML = `
      <form data-formgen-auto-init>
        <div data-component="file_uploader" data-component-config='{"uploadEndpoint":"/api/uploads","autoUpload":false}'>
          <input type="text" name="resume" id="resume">
        </div>
      </form>
    `;

    initComponents(document);

    const fileInput = document.querySelector<HTMLInputElement>('input[type="file"]');
    const form = document.querySelector("form") as HTMLFormElement;
    const submitSpy = vi.fn();
    Object.defineProperty(form, "submit", {
      configurable: true,
      value: submitSpy,
    });

    const file = new File(["resume"], "resume.pdf", { type: "application/pdf" });
    setInputFiles(fileInput!, [file]);
    fileInput!.dispatchEvent(new Event("change"));
    await flushAsync();

    const submitEvent = new Event("submit", { bubbles: true, cancelable: true });
    form.dispatchEvent(submitEvent);

    await flushAsync();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(submitSpy).toHaveBeenCalledTimes(1);
    const hidden = document.querySelector<HTMLInputElement>('input[name="resume"]');
    expect(hidden?.value).toBe("/uploads/doc.pdf");
  });
});
