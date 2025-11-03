declare const __FORMGEN_RUNTIME_VERSION__: string | undefined;

export const RUNTIME_VERSION =
  typeof __FORMGEN_RUNTIME_VERSION__ === "string" && __FORMGEN_RUNTIME_VERSION__.length > 0
    ? __FORMGEN_RUNTIME_VERSION__
    : "dev";

const ANNOUNCE_FLAG = "__formgenRuntimeVersionAnnounced__";

function announceVersion(): void {
  if (typeof globalThis === "undefined") {
    return;
  }
  const globalScope = globalThis as Record<string, unknown>;
  if (globalScope[ANNOUNCE_FLAG]) {
    return;
  }
  globalScope[ANNOUNCE_FLAG] = true;

  if (typeof console === "undefined") {
    return;
  }

  const logger = typeof console.debug === "function" ? console.debug : console.log;
  if (typeof logger === "function") {
    logger(`[formgen-relationships] runtime version ${RUNTIME_VERSION}`);
  }
}

announceVersion();
