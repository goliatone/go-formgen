import { readFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it, beforeAll } from "vitest";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const themeOutput = resolve(projectRoot, "dist", "themes", "formgen.css");

beforeAll(() => {
  const nodeBin = resolve(process.execPath);
  const result = spawnSync(nodeBin, ["scripts/build-theme.mjs"], {
    cwd: projectRoot,
    stdio: "inherit",
  });
  if (result.status !== 0) {
    throw new Error("Theme build failed");
  }
});

describe("theme build", () => {
  it("emits the default theme stylesheet", async () => {
    const css = await readFile(themeOutput, "utf8");
    expect(css.length).toBeGreaterThan(0);
    expect(css).toContain(".max-w-4xl");
    expect(css).toContain(".ring-blue-500");
    expect(css).toContain(".shadow-xl");
  });
});
