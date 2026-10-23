#!/usr/bin/env node
import { execFileSync, spawnSync } from 'node:child_process';
import { readFile, writeFile, mkdir, mkdtemp, rm, copyFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, dirname } from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';
import { validVersion, versionInfo, checkManifest, manifestPath, sha256, json } from '../client/scripts/artifacts.mjs';

export function run(root, command, args, capture = false) {
  const result = spawnSync(command, args, { cwd: root, stdio: capture ? 'pipe' : 'inherit', encoding: 'utf8', env: { ...process.env, GOWORK: 'off', GOFLAGS: '-mod=readonly' } });
  if (result.error || result.status !== 0) throw Error(`${command} ${args.join(' ')} failed (${result.status}): ${result.error?.message ?? result.stderr ?? ''}`);
  return capture ? result.stdout.trim() : '';
}
export async function prepare(root, version, execute = run) {
  if (!validVersion(version)) throw Error('Supply a valid shared version');
  if (execute(root, 'git', ['tag', '--list', `v${version}`], true)) throw Error('Version already tagged; choose an unused version');
  const pkg = await json(resolve(root, 'client/package.json'));
  const lock = await json(resolve(root, 'client/package-lock.json'));
  pkg.version = lock.version = lock.packages[''].version = version;
  await writeFile(resolve(root, '.version'), version + '\n');
  await writeFile(resolve(root, 'client/package.json'), JSON.stringify(pkg, null, 2) + '\n');
  await writeFile(resolve(root, 'client/package-lock.json'), JSON.stringify(lock, null, 2) + '\n');
  execute(resolve(root, 'client'), 'npm', ['ci']);
  execute(resolve(root, 'client'), 'npm', ['run', 'build']);
  await checkManifest(root);
  console.log('Candidate prepared. Finalize release notes/changelog, review and commit the intended files, then run release:verify.');
}
export async function identityCheck(root) {
  const manifest = await checkManifest(root);
  // Execute the actual shipped IIFEs in fresh contexts; the Preact hook is a separate entry point.
  const { runInNewContext } = await import('node:vm');
  for (const [path, name] of [
    ['client/dist/browser/formgen-relationships.min.js', 'FormgenRelationships'],
    ['client/dist/browser/formgen-behaviors.min.js', 'FormgenBehaviors'],
    ['client/dist/browser/frameworks/preact.min.js', 'FormgenRelationshipsPreact'],
  ]) {
    const messages = [];
    const context = { console: { info: m => messages.push(m), warn: () => {} } };
    runInNewContext(await readFile(resolve(root, path), 'utf8'), context);
    const entry = context[name];
    if (entry?.RUNTIME_VERSION !== manifest.version || entry?.RUNTIME_BUILD !== manifest.fingerprint ||
        !messages.some(m => m.includes(`version ${manifest.version} build ${manifest.fingerprint.slice(0,12)}`))) throw Error(`Incorrect runtime identity: ${path}`);
  }
  return manifest;
}
export function validateEvidence(evidence, commit, manifestHash, ci = false) {
  if (evidence.format !== 'formgen.release-evidence/v1' || evidence.sourceCommit !== commit ||
      evidence.manifestHash !== manifestHash || evidence.status !== 'passed' ||
      (ci && (evidence.provider !== 'github-actions' || !evidence.runUrl))) throw Error('Evidence does not verify this exact candidate');
}
export async function verify(root, ref = 'HEAD', execute = run) {
  // Never accidentally validate a dirty working tree as though it were the commit.
  if (execute(root, 'git', ['status', '--porcelain', '--untracked-files=no'], true)) throw Error('Commit intended changes before exact-candidate verification');
  const commit = execute(root, 'git', ['rev-parse', `${ref}^{commit}`], true);
  if (commit !== execute(root, 'git', ['rev-parse', 'HEAD'], true)) throw Error('Verify the checked-out candidate commit');
  const temp = await mkdtemp(resolve(tmpdir(), 'formgen-verify-'));
  try {
    const archive = resolve(temp, 'candidate.tar');
    execute(root, 'git', ['archive', '--format=tar', `--output=${archive}`, commit]);
    const source = resolve(temp, 'source');
    await mkdir(source);
    execute(source, 'tar', ['-xf', archive]);
    const expected = await readFile(resolve(source, manifestPath), 'utf8');
    const client = resolve(source, 'client');
    execute(client, 'npm', ['ci']);
    execute(client, 'npm', ['run', 'build']);
    if (await readFile(resolve(source, manifestPath), 'utf8') !== expected) throw Error('Candidate build is not reproducible');
    const manifest = await identityCheck(source);
    // Compare all committed embedded copies to regenerated ones, not just the manifest.
    for (const [path, hash] of Object.entries(manifest.artifacts)) {
      if (!path.startsWith('client/dist/')) {
        // git output is binary-safe via execFileSync rather than the trimmed runner.
        if (sha256(execFileSync('git', ['show', `${commit}:${path}`], { cwd: root })) !== hash) throw Error(`Committed asset is stale: ${path}`);
      }
    }
    execute(client, 'npm', ['test', '--', '--threads=false']);
    execute(source, 'node', ['--test', 'scripts/release.test.mjs']);
    execute(source, 'go', ['test', './...']);
    execute(source, 'go', ['test', '-tags', 'example', './examples/...']);
    // Verify the actual npm tarball's public entry points in a fresh install tree.
    const pack = JSON.parse(execute(client, 'npm', ['pack', '--json', '--pack-destination', temp], true))[0];
    execute(source, 'node', ['scripts/check-package.mjs', resolve(temp, pack.filename), manifest.version, manifest.fingerprint]);
    const fixtures = {};
    for (const path of ['testdata/embedded/v1.json', 'testdata/embedded/definition-shapes.json', 'testdata/embedded/portable-definitions.json', 'testdata/embedded/authoring.json']) fixtures[path] = sha256(await readFile(resolve(source,path)));
    const provider = process.env.GITHUB_ACTIONS === 'true' ? 'github-actions' : 'local';
    const evidence = { format: 'formgen.release-evidence/v1', sourceCommit: commit, version: manifest.version,
      fingerprint: manifest.fingerprint, manifestHash: sha256(expected), fixtureRevision: { commit, hashes: fixtures },
      status: 'passed', provider, runUrl: provider === 'github-actions' ? `${process.env.GITHUB_SERVER_URL}/${process.env.GITHUB_REPOSITORY}/actions/runs/${process.env.GITHUB_RUN_ID}` : null };
    await rm(resolve(root, '.tmp/release'), { recursive: true, force: true });
    await mkdir(resolve(root, '.tmp/release'), { recursive: true });
    await writeFile(resolve(root, '.tmp/release/evidence.json'), JSON.stringify(evidence, null, 2)+'\n');
    await writeFile(resolve(root, '.tmp/release/runtime-manifest.json'), expected);
    await copyFile(resolve(temp, pack.filename),resolve(root,'.tmp/release',pack.filename));
    for(const path of Object.keys(manifest.artifacts)) {
      const target=resolve(root,'.tmp/release/artifacts',path);
      await mkdir(dirname(target),{recursive:true});await copyFile(resolve(source,path),target);
    }
    console.log(JSON.stringify(evidence, null, 2));
    return evidence;
  } finally { await rm(temp, { recursive: true, force: true }); }
}
export async function publish(root, runID, execute = run, verifyCandidate = verify) {
  if (!/^\d+$/.test(runID ?? '')) throw Error('Supply the successful GitHub Actions run ID');
  if (execute(root, 'git', ['status', '--porcelain', '--untracked-files=no'], true)) throw Error('Candidate must be clean');
  const commit = execute(root, 'git', ['rev-parse', 'HEAD'], true);
  const { version } = await versionInfo(root);
  if (execute(root, 'git', ['tag', '--list', `v${version}`], true)) throw Error('Tag already exists');
  const remote = execute(root, 'git', ['ls-remote', '--tags', 'origin', `refs/tags/v${version}`], true);
  if (remote) throw Error('Remote tag already exists');
  const runInfo = JSON.parse(execute(root, 'gh', ['run', 'view', runID, '--json', 'headSha,conclusion,url'], true));
  if (runInfo.headSha !== commit || runInfo.conclusion !== 'success') throw Error('CI did not pass for this commit');
  const dir = await mkdtemp(resolve(tmpdir(), 'formgen-evidence-'));
  try {
    execute(root, 'gh', ['run', 'download', runID, '--name', 'release-evidence', '--dir', dir]);
    const evidence = await json(resolve(dir, 'evidence.json'));
    validateEvidence(evidence, commit, sha256(await readFile(resolve(root, manifestPath))), true);
    if (evidence.runUrl !== runInfo.url) throw Error('CI run identity differs');
    // No publication operation is reachable before local exact-candidate verification.
    await verifyCandidate(root, 'HEAD', execute);
    execute(root, 'git', ['tag', '-a', `v${version}`, '-m', `Release v${version}`]);
    execute(root, 'git', ['push', 'origin', `refs/tags/v${version}`]);
  } finally { await rm(dir, { recursive: true, force: true }); }
}
export async function main(root, command, args) {
  if (command === 'prepare') return prepare(root, args[0]);
  if (command === 'check') return identityCheck(root);
  if (command === 'verify') return verify(root, args[0]);
  if (command === 'publish') return publish(root, args[0]);
  throw Error('Usage: release:prepare VERSION | release:check | release:verify [HEAD] | release:publish CI_RUN_ID');
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main(resolve(dirname(fileURLToPath(import.meta.url)), '..'), process.argv[2], process.argv.slice(3)).catch(error => { console.error(error.message); process.exitCode = 1; });
}
