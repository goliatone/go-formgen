import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import postcss from "postcss";
import chokidar from "chokidar";
import postcssImport from "postcss-import";
import tailwindcss from "tailwindcss";
import autoprefixer from "autoprefixer";
import cssnano from "cssnano";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const inputFile = resolve(projectRoot, "src", "theme", "index.css");
const tailwindConfig = resolve(projectRoot, "tailwind.config.js");
const outputDir = resolve(projectRoot, "dist", "themes");
const outputFile = resolve(outputDir, "formgen.css");
const vanillaStyles = resolve(projectRoot, "..", "pkg", "renderers", "vanilla", "assets", "formgen-vanilla.css");
const preactStyles = resolve(projectRoot, "..", "pkg", "renderers", "preact", "assets", "formgen-preact.min.css");

const args = new Set(process.argv.slice(2));
const watch = args.has("--watch");
const skipMinify = args.has("--no-minify");

async function buildTheme() {
  const css = await readFile(inputFile, "utf8");

  const plugins = [
    postcssImport(),
    tailwindcss({ config: tailwindConfig }),
    autoprefixer(),
  ];

  if (!skipMinify && !watch) {
    plugins.push(cssnano({ preset: "default" }));
  }

  const result = await postcss(plugins).process(css, {
    from: inputFile,
    to: outputFile,
    map: watch ? { inline: true } : false,
  });

  await mkdir(outputDir, { recursive: true });
  await writeFile(outputFile, result.css, "utf8");
  await writeFile(vanillaStyles, result.css, "utf8");
  await writeFile(preactStyles, result.css, "utf8");

  console.log(`[formgen:theme] wrote ${outputFile.replace(`${projectRoot}/`, "")}`);
  console.log("[formgen:theme] copied to vanilla renderer assets");
  console.log("[formgen:theme] copied to preact renderer assets");
}

async function run() {
  if (!watch && !skipMinify) {
    process.env.NODE_ENV = "production";
  }

  try {
    await buildTheme();
  } catch (error) {
    console.error("[formgen:theme] build failed:", error);
    process.exitCode = 1;
    return;
  }

  if (!watch) {
    return;
  }

  console.log("[formgen:theme] watching for changes…");
  const watcher = chokidar.watch(
    [
      inputFile,
      tailwindConfig,
      resolve(projectRoot, "src", "**/*.ts"),
      resolve(projectRoot, "src", "**/*.tsx"),
      resolve(projectRoot, "src", "**/*.css"),
    ],
    { ignoreInitial: true },
  );

  watcher.on("change", async (changedPath) => {
    try {
      console.log(`[formgen:theme] rebuild triggered by ${changedPath.replace(`${projectRoot}/`, "")}`);
      await buildTheme();
    } catch (error) {
      console.error("[formgen:theme] rebuild failed:", error);
    }
  });
}

run().catch((error) => {
  console.error("[formgen:theme] unexpected error:", error);
  process.exit(1);
});
