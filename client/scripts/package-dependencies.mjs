import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFile, writeFile, readdir, rm } from 'node:fs/promises';
import { resolve, dirname } from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';

const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const ordered = entries => Object.fromEntries(Object.entries(entries).sort(([a], [b]) => a.localeCompare(b)));
export const dependencyInventoryPath = 'dist/dependencies.json';

// npm's pack list accounts for transitive dependencies, peers and package file rules.
// Hash exactly those bytes, rather than guessing the installed production closure.
export async function packageDependencyInventory(client) {
  // npm 11's hidden lock cache omits package.files metadata. Its fast path can
  // therefore include files that a full manifest read excludes. Discard this
  // regenerable optimization so clean and warm installs pack the same bytes.
  await rm(resolve(client, 'node_modules/.package-lock.json'), { force: true });
  const pkg = JSON.parse(await readFile(resolve(client, 'package.json'), 'utf8'));
  const bundled = new Set(pkg.bundleDependencies ?? []);
  if (Object.keys(pkg.dependencies ?? {}).some(name => !bundled.has(name))) throw Error('Every production dependency must be bundled');
  const [pack] = JSON.parse(execFileSync('npm', ['pack', '--dry-run', '--ignore-scripts', '--offline', '--json'], {
    cwd: client, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024,
  }));
  const files = {};
  for (const { path } of pack.files) {
    if (path.startsWith('node_modules/')) files[path] = digest(await readFile(resolve(client, path)));
  }
  for (const name of bundled) {
    if (!files[`node_modules/${name}/package.json`]) throw Error(`Missing bundled dependency: ${name}`);
  }
  return { format: 'formgen.dependencies/v1', files: ordered(files) };
}

export async function verifyBundledDependencies(client, packed = false) {
  const expected = JSON.parse(await readFile(resolve(client, dependencyInventoryPath), 'utf8'));
  let actual;
  if (packed) {
    const files = {};
    async function walk(dir) {
      for (const entry of await readdir(resolve(client, dir), { withFileTypes: true })) {
        const path = `${dir}/${entry.name}`;
        if (entry.isDirectory()) await walk(path);
        else if (entry.isFile()) files[path] = digest(await readFile(resolve(client, path)));
        else throw Error(`Unsupported bundled dependency file: ${path}`);
      }
    }
    await walk('node_modules');
    actual = { format: 'formgen.dependencies/v1', files: ordered(files) };
  } else actual = await packageDependencyInventory(client);
  if (JSON.stringify(actual) !== JSON.stringify(expected)) throw Error('Bundled dependencies differ from the candidate inventory');
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const client = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  const inventory = await packageDependencyInventory(client);
  if (!Object.keys(inventory.files).length) throw Error('Missing bundled production dependencies');
  await writeFile(resolve(client, dependencyInventoryPath), JSON.stringify(inventory, null, 2) + '\n');
}
