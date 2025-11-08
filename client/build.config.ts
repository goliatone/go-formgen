import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

/**
 * Shared build configuration for the relationship runtime bundles.
 * esbuild scripts and Vite both import this file to keep entry points aligned.
 */
const moduleDir = dirname(fileURLToPath(import.meta.url));
export const projectRoot = resolve(moduleDir);

export const runtimeEntryPoints = {
  runtime: "src/index.ts",
  preact: "src/frameworks/preact.ts",
  behaviors: "src/behaviors/index.ts",
};

export const buildOutput = {
  root: "dist",
  esm: "dist/esm",
  iife: "dist/browser",
};

export const esbuildTarget = ["es2019"];
export const iifeGlobalName = "FormgenRelationships";
export const behaviorsGlobalName = "FormgenBehaviors";

export const banner = `/**
 * formgen relationship runtime
 * DO NOT EDIT: generated via \`npm run build\`
 */`;

/**
 * TODO: Revisit once we add tree-shaking and code splitting support;
 * future bundler work may introduce additional outputs.
 */
export const TODO_FUTURE_ENHANCEMENTS = true;

export type BuildFormat = "esm" | "iife";
