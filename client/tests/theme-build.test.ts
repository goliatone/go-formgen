import { readFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it, beforeAll } from "vitest";
import {
  GRID_LAYOUT_FILE,
  GRID_MAX_TRACKS,
  renderGridLayoutCSS,
} from "../scripts/grid-layout.mjs";

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
    expect(css).toContain(".formgen-form");
    expect(css).toContain(
      "max-width:var(--form-container-max-width,var(--container-max-width,56rem))",
    );
    expect(css).toContain(".ring-blue-500");
    expect(css).toContain(".shadow-xl");
  });

  it("ships the data-attribute grid layout rules", async () => {
    const css = await readFile(themeOutput, "utf8");
    // Container: gutter -> gap variable, column count -> template columns.
    expect(css).toContain("[data-formgen-gutter=sm]{--formgen-grid-gap:1rem}");
    expect(css).toContain("[data-formgen-gutter=md]{--formgen-grid-gap:1.5rem}");
    expect(css).toContain("[data-formgen-gutter=lg]{--formgen-grid-gap:2rem}");
    expect(css).toContain(".formgen-grid{display:grid;gap:var(--formgen-grid-gap,1.5rem)}");
    expect(css).toContain('[data-formgen-columns="1"]{grid-template-columns:repeat(1,minmax(0,1fr))}');
    expect(css).toContain('[data-formgen-columns="12"]{grid-template-columns:repeat(12,minmax(0,1fr))}');
    expect(css).toContain(
      `[data-formgen-columns="${GRID_MAX_TRACKS}"]{grid-template-columns:repeat(${GRID_MAX_TRACKS},minmax(0,1fr))}`,
    );
    // Field wrappers: span/start/row at the base and inside each breakpoint.
    expect(css).toContain('[data-formgen-span="6"]{grid-column-end:span 6}');
    expect(css).toContain('[data-formgen-start="3"]{grid-column-start:3}');
    expect(css).toContain('[data-formgen-row="2"]{grid-row-start:2}');
    expect(css).toContain(`[data-formgen-span="${GRID_MAX_TRACKS}"]{grid-column-end:span ${GRID_MAX_TRACKS}}`);
    for (const [breakpoint, minWidth] of [["sm", 640], ["md", 768], ["lg", 1024], ["xl", 1280], ["2xl", 1536]] as const) {
      expect(css).toContain(`@media (min-width:${minWidth}px){[data-formgen-span-${breakpoint}="1"]{grid-column-end:span 1}`);
      expect(css).toContain(`[data-formgen-start-${breakpoint}="7"]{grid-column-start:7}`);
      expect(css).toContain(`[data-formgen-row-${breakpoint}="2"]{grid-row-start:2}`);
    }
    // Cascade order: base rules precede breakpoints and breakpoints ascend, so a
    // larger breakpoint overrides a smaller one at equal specificity.
    const order = ["", "-sm", "-md", "-lg", "-xl", "-2xl"].map((suffix) =>
      css.indexOf(`[data-formgen-span${suffix}="6"]{grid-column-end:span 6}`),
    );
    for (const index of order) {
      expect(index).toBeGreaterThan(-1);
    }
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    // No selector in the contract depends on the replaceable chrome class.
    expect(css).not.toContain(".formgen-grid[data-formgen");
    expect(css).not.toContain(".formgen-grid>[data-formgen");
  });

  it("keeps the committed grid layout stylesheet in sync with its generator", async () => {
    const committed = await readFile(GRID_LAYOUT_FILE, "utf8");
    expect(committed).toBe(renderGridLayoutCSS());
  });
});
