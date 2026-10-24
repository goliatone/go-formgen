import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { access, mkdtemp, mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { sha256 } from '../client/scripts/artifacts.mjs';

/** Install the actual tarball offline, without repository dependency resolution. */
export async function checkPackageConsumer(tarball, pkg, version, fingerprint) {
  const require = createRequire(new URL('../client/package.json', import.meta.url));
  const temp = await mkdtemp(resolve(tmpdir(), 'formgen-consumer-'));
  const run = (cwd, args) => execFileSync('npm', args, { cwd, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
  try {
    // Repack only the already installed, reviewed peer: no registry lookups or upgrades.
    const preactRoot = dirname(require.resolve('preact/package.json'));
    const [peer] = JSON.parse(run(preactRoot, ['pack', '--ignore-scripts', '--offline', '--json', '--pack-destination', temp]));
    // A consumer's older direct dependency must not displace the reviewed nested
    // Markdown dependency. This local fixture contains no executable payload.
    const older = resolve(temp, 'older-markdown');
    await mkdir(older);
    await writeFile(resolve(older, 'package.json'), JSON.stringify({ name: 'markdown-it', version: '14.1.0' }));
    const consumer = { type: 'module', private: true, dependencies: {
      [pkg.name]: `file:${resolve(tarball)}`, 'markdown-it': `file:${older}`,
    } };
    await writeFile(resolve(temp, 'package.json'), JSON.stringify(consumer));
    run(temp, ['install', '--offline', '--ignore-scripts', '--no-audit', '--no-fund']);
    await assert.rejects(access(resolve(temp, 'node_modules/preact')), 'Vanilla consumers must not install the optional Preact peer');
    consumer.dependencies.preact = `file:${resolve(temp, peer.filename)}`;
    await writeFile(resolve(temp, 'package.json'), JSON.stringify(consumer));
    run(temp, ['install', '--offline', '--ignore-scripts', '--no-audit', '--no-fund']);
    const installed = resolve(temp, 'node_modules', pkg.name);
    const markdown = createRequire(resolve(installed, 'node_modules/prosemirror-markdown/package.json'));
    assert.equal(markdown('markdown-it/package.json').version, '14.3.3', 'Consumer must retain the reviewed nested Markdown version');
    assert.equal(createRequire(resolve(temp, 'package.json'))('markdown-it/package.json').version, '14.1.0');
    const inventory = JSON.parse(await readFile(resolve(installed, 'dist/dependencies.json'), 'utf8'));
    // npm must retain the reviewed nested tree, even under a fresh consumer root.
    // npm may add a hidden lockfile during installation; verify the archive separately
    // and compare each distributed file after installation without counting that metadata.
    for (const [path, hash] of Object.entries(inventory.files)) {
      if (sha256(await readFile(resolve(installed, path))) !== hash) throw Error(`Consumer dependency differs: ${path}`);
    }
    await writeFile(resolve(temp, 'consumer.ts'), `
      import { initRelationships, type WysiwygStore } from ${JSON.stringify(pkg.name)};
      import { useRelationshipOptions } from ${JSON.stringify(pkg.name + '/frameworks/preact')};
      type EditorState = WysiwygStore['tiptap']['state'];
      const connect = (field: HTMLElement | null) => useRelationshipOptions(field);
      void initRelationships; void connect;
    `);
    execFileSync(process.execPath, [require.resolve('typescript/bin/tsc'), '--noEmit', '--target', 'ES2020',
      '--module', 'ESNext', '--moduleResolution', 'Bundler', '--lib', 'DOM,ES2020', resolve(temp, 'consumer.ts')],
      { cwd: temp, stdio: 'inherit' });
    const jsdom = pathToFileURL(require.resolve('jsdom')).href;
    const probe = `
      import assert from 'node:assert/strict';
      import { readFile } from 'node:fs/promises';
      import { JSDOM } from ${JSON.stringify(jsdom)};
      import { h, render } from 'preact';
      import { useState } from 'preact/hooks';
      const dom = new JSDOM('<select data-state="ready"><option value="one">One</option></select><main></main>', { runScripts: 'outside-only', pretendToBeVisual: true, url: 'http://localhost/' });
      for (const key of ['window','document','Element','Node','HTMLElement','HTMLSelectElement','HTMLInputElement','HTMLTextAreaElement','HTMLFormElement','HTMLButtonElement','HTMLFieldSetElement','Event','CustomEvent','MutationObserver']) globalThis[key] = dom.window[key];
      const adapter = await import(${JSON.stringify(pkg.name + '/frameworks/preact')});
      assert.equal(adapter.RUNTIME_VERSION, ${JSON.stringify(version)});
      assert.equal(adapter.RUNTIME_BUILD, ${JSON.stringify(fingerprint)});
      const root = document.querySelector('main');
      const field = document.querySelector('select');
      let increment;
      function Component() {
        const state = adapter.useRelationshipOptions(field);
        const [count, setCount] = useState(0);
        increment = () => setCount(n => n + 1);
        return h('span', null, state.options.map(o => o.label).join(',') + ':' + count);
      }
      render(h(Component), root);
      await new Promise(resolve => setTimeout(resolve, 150));
      assert.equal(root.textContent, 'One:0');
      increment();
      await new Promise(resolve => setTimeout(resolve, 10));
      assert.equal(root.textContent, 'One:1');
      render(null, root);
      // A separate browser realm loads the real host UMD renderer/hooks, followed
      // by the installed IIFE adapter. Mount, update, teardown and mount again.
      dom.window.eval(await readFile(${JSON.stringify(resolve(temp, 'node_modules/preact/dist/preact.umd.js'))}, 'utf8'));
      dom.window.eval(await readFile(${JSON.stringify(resolve(temp, 'node_modules/preact/hooks/dist/hooks.umd.js'))}, 'utf8'));
      dom.window.eval(await readFile(${JSON.stringify(resolve(installed, 'dist/browser/frameworks/preact.min.js'))}, 'utf8') + ';globalThis.adapter = FormgenRelationshipsPreact;');
      dom.window.eval(\`globalThis.mount = () => {
        function Component() {
          const state = adapter.useRelationshipOptions(document.querySelector('select'));
          const [count, setCount] = preactHooks.useState(0);
          globalThis.increment = () => setCount(n => n + 1);
          return preact.h('span', null, state.options.map(o => o.label).join(',') + ':' + count);
        }
        preact.render(preact.h(Component), document.querySelector('main'));
      };\`);
      for (let i = 0; i < 2; i++) {
        dom.window.mount();
        await new Promise(resolve => setTimeout(resolve, 150));
        assert.equal(root.textContent, 'One:0');
        dom.window.increment();
        await new Promise(resolve => setTimeout(resolve, 10));
        assert.equal(root.textContent, 'One:1');
        dom.window.preact.render(null, root);
      }
      dom.window.close();
      console.log('Packed consumer: ESM and browser hooks mount, update and remount with host Preact');
    `;
    await writeFile(resolve(temp, 'probe.mjs'), probe);
    execFileSync(process.execPath, [resolve(temp, 'probe.mjs')], { cwd: temp, stdio: 'inherit' });
  } finally { await rm(temp, { recursive: true, force: true }); }
}
