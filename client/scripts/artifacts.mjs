import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFile, writeFile, readdir, readlink } from 'node:fs/promises';
import { resolve } from 'node:path';
import { verifyBundledDependencies, dependencyInventoryPath } from './package-dependencies.mjs';
import { assertInstalledDependencies } from './check-dependencies.mjs';

export const validVersion = version => /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*)?$/.test(version ?? '');
export const manifestPath = 'client/runtime-manifest.json';
export const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const canonical = value => Array.isArray(value) ? value.map(canonical) : value && typeof value === 'object'
  ? Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([key, item]) => [key, canonical(item)])) : value;
export const packageMetadataHash = pkg => sha256(JSON.stringify(canonical(pkg)));
export const json = async path => JSON.parse(await readFile(path, 'utf8'));
export async function files(root, dir) {
  const out = [];
  for (const entry of await readdir(resolve(root, dir), { withFileTypes: true })) {
    // Exclude generated/cache directories by name, never all hidden source files.
    if (entry.isDirectory() && ['.git', '.cache', '.gocache', '.gomodcache', '.tmp', '.vite', '.turbo', 'node_modules', 'dist', 'coverage'].includes(entry.name)) continue;
    const path = `${dir}/${entry.name}`;
    if (entry.isSymbolicLink()) {
      // The development stylesheet aliases a generated output, not a build input.
      if (path === 'client/dev/formgen-vanilla.css' && await readlink(resolve(root, path)) === '../../pkg/renderers/vanilla/assets/formgen-vanilla.css') continue;
      throw Error(`Symbolic input paths require explicit materialization: ${path}`);
    }
    if (entry.isDirectory()) out.push(...await files(root, path));
    else if (entry.isFile()) out.push(path);
  }
  return out.sort();
}
export async function versionInfo(root) {
  const version = (await readFile(resolve(root, '.version'), 'utf8')).trim();
  if (!validVersion(version)) throw Error('Invalid shared version');
  const pkg = await json(resolve(root, 'client/package.json'));
  const lock = await json(resolve(root, 'client/package-lock.json'));
  if ([pkg.version, lock.version, lock.packages?.['']?.version].some(v => v !== version)) throw Error('Shared/package/lock versions differ; run release:prepare VERSION');
  return { version, pkg, lock };
}
export async function buildInputs(root) {
  // Tailwind scans tests, examples and Go templates too: include its real inputs.
  // Conservatively include every client source/config asset, including hidden
  // modules and loader inputs with arbitrary extensions. Exclusions are explicit.
  return ['.version', '.node-version', ...(await files(root, 'client')).filter(p => p !== manifestPath && !p.endsWith('/.DS_Store')),
    ...(await files(root, 'pkg')).filter(p => /\.(go|tmpl|html|json|ya?ml)$/.test(p)),
    ...(await files(root, 'examples')).filter(p => /\.(go|tmpl|html)$/.test(p)),
    ...(await files(root, 'scripts')).filter(p => /\.(mjs|go)$/.test(p))].sort();
}
export async function buildInfo(root) {
  const { version } = await versionInfo(root);
  const node = (await readFile(resolve(root, '.node-version'), 'utf8')).trim();
  if (process.versions.node !== node) throw Error(`Production builds require Node ${node}; got ${process.versions.node}`);
  const inputs = await buildInputs(root);
  const hash = createHash('sha256');
  for (const path of inputs) hash.update(path + '\0').update(await readFile(resolve(root, path))).update('\0');
  return { version, fingerprint: hash.digest('hex'), node };
}
// Explicit copy inventory: include the Preact hook, but not the separate renderer application.
export const copies = {
  'client/dist/browser/formgen-relationships.min.js': ['pkg/runtime/assets/formgen-relationships.min.js', 'pkg/renderers/vanilla/assets/formgen-relationships.min.js'],
  'client/dist/browser/formgen-relationships.min.js.map': ['pkg/runtime/assets/formgen-relationships.min.js.map', 'pkg/renderers/vanilla/assets/formgen-relationships.min.js.map'],
  'client/dist/browser/formgen-behaviors.min.js': ['pkg/runtime/assets/formgen-behaviors.min.js'],
  'client/dist/browser/formgen-behaviors.min.js.map': ['pkg/runtime/assets/formgen-behaviors.min.js.map'],
  'client/dist/browser/frameworks/preact.min.js': ['pkg/runtime/assets/frameworks/preact.min.js'],
  'client/dist/browser/frameworks/preact.min.js.map': ['pkg/runtime/assets/frameworks/preact.min.js.map'],
  'client/dist/themes/formgen.css': ['pkg/renderers/vanilla/assets/formgen-vanilla.css', 'pkg/renderers/preact/assets/formgen-preact.min.css'],
};
export async function artifactManifest(root) {
  const info = await buildInfo(root);
  const pkg = await json(resolve(root, 'client/package.json'));
  if (pkg.bundleDependencies?.length) await verifyBundledDependencies(resolve(root, 'client'));
  const outputs = [];
  if (pkg.bundleDependencies?.length) outputs.push(`client/${dependencyInventoryPath}`);
  for (const dir of ['client/dist/esm', 'client/dist/browser', 'client/dist/types', 'client/dist/themes']) {
    // files() deliberately excludes nested dist directories, not these explicit roots.
    outputs.push(...await files(root, dir));
  }
  outputs.push(...Object.values(copies).flat());
  const artifacts = {};
  for (const path of outputs.sort()) artifacts[path] = sha256(await readFile(resolve(root, path)));
  for (const [source, targets] of Object.entries(copies)) {
    if (!artifacts[source] || targets.some(p => artifacts[p] !== artifacts[source])) throw Error(`Stale embedded copy of ${source}`);
  }
  if (!outputs.some(p => p.endsWith('index.d.ts')) || !artifacts['client/dist/esm/index.js']) throw Error('Missing package entry points');
  return { format: 'formgen.artifacts/v1', ...info, packageMetadataHash: packageMetadataHash(pkg), artifacts };
}
export async function writeManifest(root) {
  const manifest = await artifactManifest(root);
  await writeFile(resolve(root, manifestPath), JSON.stringify(manifest, null, 2) + '\n');
  return manifest;
}
export async function checkManifest(root) {
  const actual = await artifactManifest(root);
  const expected = await json(resolve(root, manifestPath));
  if (JSON.stringify(actual) !== JSON.stringify(expected)) throw Error('Artifact manifest differs: rebuild and prepare the candidate');
  return actual;
}

export async function assertBuildTools(root) {
  const {pkg,lock}=await versionInfo(root);
  await assertInstalledDependencies(resolve(root, 'client'));
  const npm=execFileSync('npm',['--version'],{encoding:'utf8'}).trim();
  if(pkg.packageManager!==`npm@${npm}`)throw Error(`Production builds require ${pkg.packageManager}; got npm@${npm}`);
  for(const name of ['esbuild','typescript','tsx','postcss','tailwindcss','autoprefixer','cssnano']) {
    const installed=await json(resolve(root,'client/node_modules',name,'package.json'));
    if(installed.version!==lock.packages?.[`node_modules/${name}`]?.version)throw Error(`Installed ${name} differs from the lockfile; run npm ci`);
  }
}
