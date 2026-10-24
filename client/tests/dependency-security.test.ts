import { afterEach, describe, expect, it } from "vitest";
import { h, isValidElement, render } from "preact";
import postcss from "postcss";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const temporaryDirectories: string[] = [];
afterEach(async () => {
  render(null, document.body);
  document.body.innerHTML = "";
  await Promise.all(temporaryDirectories.splice(0).map(path => rm(path, { recursive: true, force: true })));
});

describe("dependency security boundaries", () => {
  it("rejects JSON objects masquerading as Preact VNodes", () => {
    // GHSA-36hm-qxxp-pg3m: JSON null must not impersonate VNode's undefined constructor.
    const injected = JSON.parse(JSON.stringify({
      ...h("img", { src: "invalid:test", "data-injected": "yes" }),
      constructor: null,
    }));
    expect(isValidElement(injected)).toBe(false);
    try {
      render(h("div", null, injected), document.body);
    } catch {
      // Throwing on invalid child data is also a safe rejection.
    }
    expect(document.querySelector("[data-injected]")).toBeNull();
    render(h("div", null, "normal text"), document.body);
    expect(document.body.textContent).toBe("normal text");
  });

  it("does not load an absolute source map when CSS has no source file", async () => {
    // GHSA-fxqj-rqcc-2cmp: use a synthetic canary, never a real credential file.
    const directory = await mkdtemp(join(tmpdir(), "formgen-source-map-"));
    temporaryDirectories.push(directory);
    const mapPath = join(directory, "outside.map");
    const marker = "FORMGEN_SYNTHETIC_SOURCE_MAP_CANARY";
    await writeFile(mapPath, JSON.stringify({
      version: 3, sources: ["outside-source.ts"], names: [], mappings: "AAAA",
      sourcesContent: [marker],
    }));
    const result = await postcss([]).process(`a{color:red}\n/*# sourceMappingURL=${mapPath} */`, {
      from: undefined, map: { inline: false, annotation: false },
    });
    expect(JSON.stringify(result.map?.toJSON())).not.toContain(marker);
    expect(result.css).toContain("color:red");
  });
});
