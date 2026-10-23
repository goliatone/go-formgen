import { mkdtemp, rm, readFile, writeFile, access, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { sha256 } from '../client/scripts/artifacts.mjs';
import { execFileSync } from 'node:child_process';
/** Validate every distributed file, including inventory, against the candidate manifest. */
export async function verifyPackageContents(packageRoot, manifest) {
  const actual = {};
  async function walk(dir) {
    for (const entry of await readdir(resolve(packageRoot, dir), { withFileTypes: true })) {
      const path = `${dir}/${entry.name}`;
      if (entry.isDirectory()) await walk(path);
      else if (entry.isFile()) actual[`client/${path}`] = sha256(await readFile(resolve(packageRoot, path)));
      else throw Error(`Unsupported packed artifact: ${path}`);
    }
  }
  await walk('dist');
  const expected = Object.fromEntries(Object.entries(manifest.artifacts).filter(([path]) => path.startsWith('client/dist/')));
  const sorted = object => JSON.stringify(Object.entries(object).sort(([a],[b]) => a.localeCompare(b)));
  if (sorted(actual) !== sorted(expected)) throw Error('Packed artifacts differ from the candidate manifest');
}
export async function checkPackage(tarball, version, fingerprint, expectedManifest) {
  const temp = await mkdtemp(resolve(tmpdir(), 'formgen-package-'));
  try {
    execFileSync('tar', ['-xf', tarball, '-C', temp]);
    const manifestBytes = await readFile(resolve(temp, 'package/runtime-manifest.json'), 'utf8');
    if (expectedManifest !== undefined && manifestBytes !== expectedManifest) throw Error('Packed manifest differs from the candidate');
    const manifest = JSON.parse(manifestBytes);
    if (manifest.version !== version || manifest.fingerprint !== fingerprint) throw Error('Packed manifest identity differs');
    await verifyPackageContents(resolve(temp, 'package'), manifest);
    const pkg = JSON.parse(await readFile(resolve(temp, 'package/package.json')));
    if (pkg.version !== version) throw Error('Packed package version differs');
    // Node ESM execution uses an explicit loader because esbuild targets browser bundlers.
    await writeFile(resolve(temp, 'loader.mjs'), `import { existsSync } from 'node:fs';\nexport async function resolve(s,c,n){if(s.startsWith('.')&&!/\\.[a-z]+$/i.test(s)){let u=new URL(s+'.js',c.parentURL);if(existsSync(u))return n(u.href,c);}return n(s,c);}`);
    const entries=['.','./behaviors','./frameworks/preact'];
    for(const entry of entries)for(const path of Object.values(pkg.exports[entry]))await access(resolve(temp,'package',path));
    for(const name of ['module','browser','types'])await access(resolve(temp,'package',pkg[name]));
    const paths=entries.map(key=>'./package/'+pkg.exports[key].import.replace(/^\.\//,''));
    const probe = `for(const p of ${JSON.stringify(paths)}){const m=await import(p);if(m.RUNTIME_VERSION!==${JSON.stringify(version)}||m.RUNTIME_BUILD!==${JSON.stringify(fingerprint)})throw Error('Packed identity mismatch: '+p);}`;
    await writeFile(resolve(temp, 'probe.mjs'), probe);
    execFileSync(process.execPath, ['--loader', resolve(temp,'loader.mjs'), resolve(temp,'probe.mjs')], { stdio: 'inherit' });
  } finally { await rm(temp, { recursive: true, force: true }); }

}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const [tarball, version, fingerprint, manifestFile] = process.argv.slice(2);
  try { await checkPackage(tarball, version, fingerprint, manifestFile ? await readFile(manifestFile, 'utf8') : undefined); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
