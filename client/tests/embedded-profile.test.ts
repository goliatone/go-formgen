import { describe, it, expect } from "vitest";
import { attachFormController } from "../src/index";
import type { EmbeddedModel } from "../src/embedded-contract";

/**
 * Profiling harness for the embedded controller. It is skipped unless
 * FORMGEN_PROFILE=1 so the regular suite stays fast:
 *
 *   FORMGEN_PROFILE=1 npm test -- --threads=false tests/embedded-profile.test.ts
 *
 * Measurements come from jsdom on the current machine; they are observations
 * for the maintainer trial, not budgets or browser-engine claims.
 */
describe.skipIf(!process.env.FORMGEN_PROFILE)("embedded controller profile", () => {
  function build(fieldCount: number): { root: HTMLFormElement; model: EmbeddedModel; payload: Record<string, unknown> } {
    const fields: EmbeddedModel["fields"] = [{ name: "enabled", type: "boolean", required: true }];
    const rules: unknown[] = [];
    const payload: Record<string, unknown> = { enabled: true };
    let markup = `<div data-component="input"><input type="checkbox" name="enabled" value="true"></div>`;
    for (let index = 0; index < fieldCount; index += 1) {
      const name = `f${index}`;
      if (index % 3 === 0) {
        fields.push({ name, type: "integer", required: false, validations: [{ kind: "min", params: { value: "0" } }] });
        markup += `<div data-component="input"><input name="${name}" type="text"></div>`;
        payload[name] = index;
      } else if (index % 3 === 1) {
        fields.push({ name, type: "string", required: false, validations: [{ kind: "maxLength", params: { value: "80" } }] });
        markup += `<div data-component="input"><input name="${name}" type="text"></div>`;
        payload[name] = `value ${index}`;
      } else {
        fields.push({ name, type: "boolean", required: false });
        markup += `<div data-component="input"><input type="checkbox" name="${name}" value="true"></div>`;
        payload[name] = index % 2 === 0;
      }
      if (index % 10 === 0) {
        rules.push({ path: name, applicableWhen: { op: "eq", path: "enabled", value: true } });
      }
    }
    document.body.innerHTML = `<form id="profile">${markup}</form>`;
    const root = document.getElementById("profile") as HTMLFormElement;
    const model: EmbeddedModel = {
      fields,
      conditions: { version: "formgen.conditions/v1", rules } as EmbeddedModel["conditions"],
    };
    return { root, model, payload };
  }

  function timed<T>(fn: () => T): [T, number] {
    const start = performance.now();
    const result = fn();
    return [result, performance.now() - start];
  }

  for (const size of [6, 300]) {
    it(`profiles a ${size}-field form`, () => {
      const { root, model, payload } = build(size);
      let listeners = 0;
      const original = root.addEventListener.bind(root);
      root.addEventListener = ((...args: Parameters<typeof original>) => {
        listeners += 1;
        return original(...args);
      }) as typeof root.addEventListener;

      const [controller, attachMs] = timed(() => attachFormController(root, { model }));
      let callbacks = 0;
      controller.onChange(() => {
        callbacks += 1;
      });
      const [, hydrateMs] = timed(() => controller.setValues(payload, { notify: false }));
      const silentCallbacks = callbacks;
      const first = root.querySelector<HTMLInputElement>('input[name="f1"]')!;
      first.value = "edited";
      const [, editMs] = timed(() => first.dispatchEvent(new Event("input", { bubbles: true })));
      const editCallbacks = callbacks - silentCallbacks;
      const [, toggleMs] = timed(() => {
        const enabled = root.querySelector<HTMLInputElement>('input[name="enabled"]')!;
        enabled.checked = false;
        enabled.dispatchEvent(new Event("change", { bubbles: true }));
      });
      const toggleCallbacks = callbacks - silentCallbacks - editCallbacks;
      const [issues, validateMs] = timed(() => controller.validate());
      const [snapshot, snapshotMs] = timed(() => controller.snapshot());
      const [, destroyMs] = timed(() => controller.destroy());

      const row = {
        fields: size,
        rules: model.conditions!.rules.length,
        rootListeners: listeners,
        attachMs: attachMs.toFixed(2),
        hydrateSilentMs: hydrateMs.toFixed(2),
        hydrateCallbacks: silentCallbacks,
        editMs: editMs.toFixed(2),
        editCallbacks,
        toggleMs: toggleMs.toFixed(2),
        toggleCallbacks,
        validateMs: validateMs.toFixed(2),
        issues: issues.length,
        snapshotMs: snapshotMs.toFixed(2),
        snapshotPaths: Object.keys(snapshot.presence).length,
        destroyMs: destroyMs.toFixed(2),
      };
      console.log(JSON.stringify(row));
      expect(silentCallbacks).toBe(0);
      expect(editCallbacks).toBe(1);
      expect(toggleCallbacks).toBe(1);
      expect(listeners).toBe(3);
    });
  }
});
