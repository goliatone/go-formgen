import { readFile } from 'node:fs/promises';
import { resolve, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

/** Check metadata before loading build/test tools or candidate package code. */
export async function assertInstalledDependencies(client) {
  const lock = JSON.parse(await readFile(resolve(client, 'package-lock.json'), 'utf8'));
  const pkg = JSON.parse(await readFile(resolve(client, 'package.json'), 'utf8'));
  const direct = new Set([...Object.keys(pkg.dependencies ?? {}), ...Object.keys(pkg.devDependencies ?? {})]
    .map(name => `node_modules/${name}`));
  const entries = record => JSON.stringify(Object.entries(record ?? {}).sort(([a], [b]) => a.localeCompare(b)));
  const problems = [];
  for (const group of ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies']) {
    if (entries(pkg[group]) !== entries(lock.packages?.['']?.[group])) problems.push(`package.json ${group} differs from the lockfile root`);
  }
  for (const [path, expected] of Object.entries(lock.packages)) {
    if (!path || !expected.version) continue;
    let installed;
    try { installed = JSON.parse(await readFile(resolve(client, path, 'package.json'), 'utf8')); }
    catch (error) {
      if (error.code !== 'ENOENT') throw error;
      // npm omits optional platform binaries and optional peers when appropriate.
      if (direct.has(path) || (!expected.optional && !expected.devOptional)) problems.push(`${path}: missing (locked ${expected.version})`);
      continue;
    }
    if (installed.version !== expected.version) problems.push(`${path}: installed ${installed.version}, locked ${expected.version}`);
  }
  if (problems.length) throw Error(`Installed dependencies differ from package-lock.json:\n${problems.join('\n')}\nRun npm ci --ignore-scripts before testing or building. Do not update the lockfile to match stale node_modules.`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try { await assertInstalledDependencies(resolve(dirname(fileURLToPath(import.meta.url)), '..')); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
