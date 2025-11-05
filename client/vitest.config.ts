import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

const moduleDir = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  resolve: {
    alias: {
      "@formgen": resolve(moduleDir, "src"),
      "@assets": resolve(moduleDir, "..", "pkg", "renderers"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    include: ["tests/**/*.test.ts", "src/**/*.test.ts"],
  },
});
