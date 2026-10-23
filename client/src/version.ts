declare const __FORMGEN_RUNTIME_VERSION__: string | undefined;
declare const __FORMGEN_RUNTIME_BUILD__: string | undefined;

export const RUNTIME_VERSION = typeof __FORMGEN_RUNTIME_VERSION__ === 'string' ? __FORMGEN_RUNTIME_VERSION__ : 'dev';
export const RUNTIME_BUILD = typeof __FORMGEN_RUNTIME_BUILD__ === 'string' ? __FORMGEN_RUNTIME_BUILD__ : 'dev';
export const RUNTIME_INFO = Object.freeze({ version: RUNTIME_VERSION, fingerprint: RUNTIME_BUILD });

const key = '__formgenRuntimeIdentities__';
const scope = globalThis as typeof globalThis & { [key]?: Set<string> };
const identities = scope[key] ??= new Set<string>();
const identity = `${RUNTIME_VERSION}+${RUNTIME_BUILD}`;
if (!identities.has(identity)) {
  if (identities.size) console.warn('[formgen] multiple runtime identities loaded', [...identities, identity]);
  identities.add(identity);
  console.info(`[formgen] runtime version ${RUNTIME_VERSION} build ${RUNTIME_BUILD.slice(0, 12)}`);
}
