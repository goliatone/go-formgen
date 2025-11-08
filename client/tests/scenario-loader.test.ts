import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

import { loadSandboxScenario } from "../dev/scenario-loader";

const fixturesDir = resolve(__dirname, "..", "dev");
const originalFetch = globalThis.fetch;

beforeAll(() => {
  vi.stubGlobal("fetch", async (input: RequestInfo | URL): Promise<Response> => {
    const target = typeof input === "string"
      ? input
      : input instanceof URL
        ? input.pathname ?? input.href
        : input.url;

    if (!target) {
      throw new Error("Unsupported fetch target");
    }

    if (target.endsWith("ui-schema.json")) {
      const payload = await readFile(resolve(fixturesDir, "ui-schema.json"), "utf-8");
      return {
        ok: true,
        status: 200,
        json: async () => JSON.parse(payload),
      } as unknown as Response;
    }

    if (target.endsWith("schema.json")) {
      const payload = await readFile(resolve(fixturesDir, "schema.json"), "utf-8");
      return {
        ok: true,
        status: 200,
        json: async () => JSON.parse(payload),
      } as unknown as Response;
    }

    throw new Error(`Unhandled fetch call for ${target}`);
  });
});

afterAll(() => {
  if (originalFetch) {
    vi.stubGlobal("fetch", originalFetch);
  } else {
    vi.unstubAllGlobals();
  }
});

describe("sandbox scenario loader", () => {
  it("preloads fixtures", async () => {
    const response = await fetch("./ui-schema.json");
    const doc = await response.json();
    expect(doc.operations).toBeDefined();
    expect(Object.keys(doc.operations ?? {})).toContain("createArticle");
  });

  it("hydrates relationship metadata and refresh hints", async () => {
    const scenario = await loadSandboxScenario();

    expect(scenario.sections.length).toBeGreaterThanOrEqual(3);

    const managerField = scenario.fieldMap["manager_id"];
    expect(managerField).toBeDefined();
    expect(managerField.refresh?.mode).toBe("manual");
    expect(managerField.refresh?.triggers).toContain("manager-refresh");
    expect(managerField.endpoint?.dynamicParams?.author_id).toBe("{{field:author_id}}");

    const tagsField = scenario.fieldMap["tags"];
    expect(tagsField.endpoint?.dynamicParams).toMatchObject({
      tenant_id: "{{field:tenant_id}}",
      category_id: "{{field:category_id}}",
    });

    const contributors = scenario.fieldMap["contributors"];
    expect(contributors.component).toBe("array");
    expect(contributors.nestedKeys).toEqual([
      "contributors[].person_id",
      "contributors[].role",
      "contributors[].notes",
    ]);

    const contributorPerson = scenario.fieldMap["contributors[].person_id"];
    expect(contributorPerson.endpoint?.dynamicParams?.status).toBe("{{field:status}}");
    expect(contributorPerson.relationship?.cardinality).toBe("one");
  });
});
