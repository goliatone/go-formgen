import { defineConfig } from "vite";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { runtimeEntryPoints } from "./build.config";

const moduleDir = dirname(fileURLToPath(import.meta.url));

const entryRuntime = resolve(moduleDir, runtimeEntryPoints.runtime);
const entryPreact = resolve(moduleDir, runtimeEntryPoints.preact);

/**
 * Optional Vite configuration for local development.
 * Shares entry points with the esbuild bundle so the dev server mirrors production.
 *
 * TODO: layer additional plugins (code splitting, manifest generation) once bundler
 * requirements expand beyond the current esbuild pipeline.
 */
export default defineConfig({
  root: "dev",
  publicDir: false,
  esbuild: {
    jsxFactory: "h",
    jsxFragment: "Fragment",
    jsxInject: `import { h, Fragment } from 'preact'`,
  },
  build: {
    outDir: "dist/vite",
    sourcemap: true,
    lib: {
      entry: {
        runtime: entryRuntime,
        preact: entryPreact,
      },
      formats: ["es"],
      fileName: (_format, entryName) =>
        entryName === "runtime" ? "index.js" : `frameworks/${entryName}.js`,
    },
    rollupOptions: {
      treeshake: true,
    },
  },
  server: {
    port: 5173,
    host: "localhost",
    open: true,
    fs: {
      // Allow serving files from parent directories (Go package assets)
      allow: [resolve(moduleDir, "..")],
    },
  },
  resolve: {
    alias: {
      "@formgen": resolve(moduleDir, "src"),
      // Alias to serve CSS from Go package
      "@assets": resolve(moduleDir, "..", "pkg", "renderers"),
    },
  },
});
