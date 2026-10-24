// @ts-ignore Node ESM build helper; runtime-only, not a published declaration.
import { buildInfo, assertBuildTools, copies } from "./artifacts.mjs";
import { build, context, analyzeMetafile, BuildOptions } from "esbuild";
import { copyFile, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  runtimeEntryPoints,
  buildOutput,
  esbuildTarget,
  iifeGlobalName,
  behaviorsGlobalName,
  banner,
} from "../build.config";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const repoRoot = resolve(projectRoot, "..");

const args = new Set(process.argv.slice(2));
const watch = args.has("--watch");
const analyze = args.has("--analyze");

const distRoot = resolve(projectRoot, buildOutput.root);
const esmOutDir = resolve(projectRoot, buildOutput.esm);
const iifeOutDir = resolve(projectRoot, buildOutput.iife);
const themesOutDir = resolve(projectRoot, "dist", "themes");
const viteOutDir = resolve(projectRoot, "dist", "vite");

// Use the manifest's copy inventory so build synchronization and verification agree.
const runtimeAssetCopies = Object.entries(copies)
  .filter(([source]) => source.startsWith("client/dist/browser/"))
  .map(([source, targets]) => ({
    source: resolve(repoRoot, source),
    targets: targets.map((target) => resolve(repoRoot, target)),
  }));

const esmOptions: BuildOptions = {
  absWorkingDir: projectRoot,
  entryPoints: Object.values(runtimeEntryPoints),
  outbase: "src",
  outdir: esmOutDir,
  bundle: true,
  // Hooks must share the consumer renderer's Preact instance.
  external: ["preact", "preact/*"],
  splitting: true,
  format: "esm",
  sourcemap: true,
  minify: false,
  treeShaking: true,
  target: esbuildTarget,
  platform: "browser",
  legalComments: "none",
  banner: { js: banner },
};

const iifeRuntimeOptions: BuildOptions = {
  absWorkingDir: projectRoot,
  entryPoints: [runtimeEntryPoints.runtime],
  outfile: resolve(iifeOutDir, "formgen-relationships.min.js"),
  bundle: true,
  format: "iife",
  sourcemap: true,
  minify: true,
  target: esbuildTarget,
  platform: "browser",
  globalName: iifeGlobalName,
  legalComments: "none",
  banner: { js: banner },
};

const iifePreactOptions: BuildOptions = {
  absWorkingDir: projectRoot,
  entryPoints: [runtimeEntryPoints.preact],
  outfile: resolve(iifeOutDir, "frameworks/preact.min.js"),
  bundle: true,
  plugins: [{
    name: "host-preact-hooks",
    setup(builder) {
      builder.onResolve({ filter: /^preact\/hooks$/ }, () => ({ path: "preact/hooks", namespace: "host-preact" }));
      builder.onLoad({ filter: /.*/, namespace: "host-preact" }, () => ({
        contents: `function hostHooks() {
          const hooks = globalThis.preactHooks;
          if (!hooks || !hooks.useState || !hooks.useEffect) {
            throw new Error("Load the host Preact and preact/hooks UMD scripts before using the Formgen Preact adapter");
          }
          return hooks;
        }
        export function useState(...args) { return hostHooks().useState(...args); }
        export function useEffect(...args) { return hostHooks().useEffect(...args); }`,
        loader: "js",
      }));
    },
  }],
  format: "iife",
  sourcemap: true,
  minify: true,
  target: esbuildTarget,
  platform: "browser",
  globalName: `${iifeGlobalName}Preact`,
  legalComments: "none",
  banner: { js: banner },
};

const iifeBehaviorsOptions: BuildOptions = {
  absWorkingDir: projectRoot,
  entryPoints: [runtimeEntryPoints.behaviors],
  outfile: resolve(iifeOutDir, "formgen-behaviors.min.js"),
  bundle: true,
  format: "iife",
  sourcemap: true,
  minify: true,
  target: esbuildTarget,
  platform: "browser",
  globalName: behaviorsGlobalName,
  legalComments: "none",
  banner: { js: banner },
};

async function ensureOutDirs() {
  if (!watch) {
    await Promise.all([
      rm(esmOutDir, { recursive: true, force: true }),
      rm(iifeOutDir, { recursive: true, force: true }),
      rm(viteOutDir, { recursive: true, force: true }),
    ]);
  }
  await Promise.all([
    mkdir(distRoot, { recursive: true }),
    mkdir(esmOutDir, { recursive: true }),
    mkdir(resolve(iifeOutDir, "frameworks"), { recursive: true }),
    mkdir(themesOutDir, { recursive: true }),
  ]);
}

async function syncRuntimeAssets() {
  for (const copy of runtimeAssetCopies) {
    for (const target of copy.targets) {
      await mkdir(dirname(target), { recursive: true });
      await copyFile(copy.source, target);
    }
  }
  console.log("[formgen:runtime] synced browser bundles to Go assets");
}

async function run() {
  if (!watch) await assertBuildTools(repoRoot);
  await ensureOutDirs();

  const info = watch ? { version: "dev", fingerprint: "dev" } : await buildInfo(repoRoot);
  const define = {
    __FORMGEN_RUNTIME_VERSION__: JSON.stringify(watch ? "dev" : info.version),
    __FORMGEN_RUNTIME_BUILD__: JSON.stringify(watch ? "dev" : info.fingerprint),
  };
  esmOptions.define = { ...(esmOptions.define ?? {}), ...define };
  iifeRuntimeOptions.define = { ...(iifeRuntimeOptions.define ?? {}), ...define };
  iifePreactOptions.define = { ...(iifePreactOptions.define ?? {}), ...define };
  iifeBehaviorsOptions.define = { ...(iifeBehaviorsOptions.define ?? {}), ...define };

  if (watch) {
    const contexts = await Promise.all([
      context(esmOptions),
      context(iifeRuntimeOptions),
      context(iifePreactOptions),
      context(iifeBehaviorsOptions),
    ]);
    await Promise.all(contexts.map((ctx) => ctx.watch()));
    console.log("Watching relationship runtime sources for changes…");
    return;
  }

  const builds: Array<{ label: string; options: BuildOptions }> = [
    { label: "esm", options: esmOptions },
    { label: "runtime", options: iifeRuntimeOptions },
    { label: "preact", options: iifePreactOptions },
    { label: "behaviors", options: iifeBehaviorsOptions },
  ];

  for (const buildTarget of builds) {
    const options = analyze
      ? { ...buildTarget.options, metafile: true }
      : buildTarget.options;

    const result = await build(options);
    if (options.metafile && result.metafile) {
      const report = await analyzeMetafile(result.metafile, { verbose: true });
      const fileName = join(
        distRoot,
        `stats-${buildTarget.label}.txt`,
      );
      await writeFile(fileName, report, "utf8");
    }
  }

  await syncRuntimeAssets();
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
