import { afterEach, describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import * as source from '../src/index';
import type { EmbeddedModel } from '../src/embedded-contract';
import fixture from '../../testdata/embedded/v1.json';

const shipped = new Function(`${readFileSync(resolve('../pkg/runtime/assets/formgen-relationships.min.js'), 'utf8')}; return FormgenRelationships;`)() as typeof source;

// The Go fixture test parses precisely these FormData entries. It cannot pass
// with a hand-built payload that accidentally hides duplicate or lost fields.
for (const [name, runtime] of [['source', source], ['shipped', shipped]] as const) {
  describe(`native submission contract (${name})`, () => {
    afterEach(() => { document.body.innerHTML = ''; vi.unstubAllGlobals(); });
    for (const test of fixture.nativeSubmissionCases) {
      it(test.id, async () => {
        document.body.innerHTML = test.markup;
        const form = document.querySelector('form')!;
        vi.stubGlobal('fetch', vi.fn(async (url: string) => ({ ok: true, status: 200,
          text: async () => JSON.stringify([{ value: url.includes('other') ? 'b' : 'a', label: 'Option' }]),
        })));
        const registry = 'initialize' in test && test.initialize ? await runtime.initFormgenRoot(form) : undefined;
        const controller = runtime.attachFormController(form, { registry, model: test.form as EmbeddedModel });
        try {
          const entries = Array.from(new FormData(form).entries());
          const byName = (a: readonly unknown[], b: readonly unknown[]) => String(a[0]).localeCompare(String(b[0]));
          expect(entries.sort(byName)).toEqual([...test.nativeEntries].sort(byName));
          expect(controller.normalize()).toEqual({ values: test.values, issues: [] });
          expect(controller.validate()).toEqual([]);
        } finally { controller.destroy(); }
      });
    }
  });
}
