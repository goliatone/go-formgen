import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { initRelationships } from "../src/index";

const fetchSpy = vi.fn();

function mockResponse(data: unknown): Response {
  return {
    ok: true,
    status: 200,
    text: async () => JSON.stringify(data),
  } as unknown as Response;
}

function createMarkup(): void {
  document.body.innerHTML = `
    <form data-formgen-auto-init="true">
      <select id="project_owner" name="project[owner_id]" data-endpoint-url="/api/users" data-endpoint-method="GET" data-relationship-cardinality="one"></select>
      <select id="project_tags" name="project[tag_ids]" data-endpoint-url="/api/tags" data-relationship-cardinality="many" multiple>
        <option value="alpha" selected>Alpha</option>
      </select>
    </form>
  `;
}

describe("initRelationships", () => {
  beforeEach(() => {
    createMarkup();
    fetchSpy.mockClear();
    fetchSpy.mockResolvedValue(mockResponse([{ value: "alpha", label: "Alpha" }]));
    (globalThis as any).fetch = fetchSpy;
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("returns a registry and augments multi-select fields", async () => {
    const registry = await initRelationships();
    const owner = document.getElementById("project_owner")!;
    const tags = document.getElementById("project_tags") as HTMLSelectElement;

    expect(registry.get(owner)).toBeDefined();
    expect(registry.get(tags)).toBeDefined();

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    expect(owner.querySelectorAll("option").length).toBe(1);
    expect(owner.querySelector("option")?.value).toBe("alpha");

    const hidden = tags.parentElement!.querySelectorAll(
      '[data-relationship-hidden] input[type="hidden"]'
    );
    expect(hidden.length).toBe(1);
    expect(hidden[0].value).toBe("alpha");
  });
});
