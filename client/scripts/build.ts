import { build, context, analyzeMetafile, BuildOptions } from "esbuild";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  runtimeEntryPoints,
  buildOutput,
  esbuildTarget,
  iifeGlobalName,
  banner,
} from "../build.config";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");

const args = new Set(process.argv.slice(2));
const watch = args.has("--watch");
const analyze = args.has("--analyze");

const distRoot = resolve(projectRoot, buildOutput.root);
const esmOutDir = resolve(projectRoot, buildOutput.esm);
const iifeOutDir = resolve(projectRoot, buildOutput.iife);

const esmOptions: BuildOptions = {
  absWorkingDir: projectRoot,
  entryPoints: Object.values(runtimeEntryPoints),
  outbase: "src",
  outdir: esmOutDir,
  bundle: true,
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
  format: "iife",
  sourcemap: true,
  minify: true,
  target: esbuildTarget,
  platform: "browser",
  globalName: `${iifeGlobalName}Preact`,
  legalComments: "none",
  banner: { js: banner },
};

async function ensureOutDirs() {
  if (!watch) {
    await rm(distRoot, { recursive: true, force: true });
  }
  await Promise.all([
    mkdir(esmOutDir, { recursive: true }),
    mkdir(resolve(iifeOutDir, "frameworks"), { recursive: true }),
  ]);
}

async function run() {
  await ensureOutDirs();

  if (watch) {
    const contexts = await Promise.all([
      context(esmOptions),
      context(iifeRuntimeOptions),
      context(iifePreactOptions),
    ]);
    await Promise.all(contexts.map((ctx) => ctx.watch()));
    console.log("Watching relationship runtime sources for changes…");
    return;
  }

  const builds: Array<{ label: string; options: BuildOptions }> = [
    { label: "esm", options: esmOptions },
    { label: "runtime", options: iifeRuntimeOptions },
    { label: "preact", options: iifePreactOptions },
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
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
