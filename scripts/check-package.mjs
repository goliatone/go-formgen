import { mkdtemp, rm, readFile, mkdir, writeFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { execFileSync } from 'node:child_process';
const [tarball, version, fingerprint] = process.argv.slice(2);
const temp = await mkdtemp(resolve(tmpdir(), 'formgen-package-'));
try {
  execFileSync('tar', ['-xf', tarball, '-C', temp]);
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
