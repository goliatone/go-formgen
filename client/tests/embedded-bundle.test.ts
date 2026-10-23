import { afterEach, describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import shapes from "../../testdata/embedded/definition-shapes.json";
import type { EmbeddedModel } from "../src/embedded-contract";

// Exercise the actual Go-embedded artifact, so a source-only fix cannot pass.
const bundle = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), "../../pkg/runtime/assets/formgen-relationships.min.js"), "utf8");
const runtime = new Function(`${bundle}; return FormgenRelationships;`)() as typeof import("../src/index");

const model: EmbeddedModel = {
  fields: [
    { name: "enabled", type: "boolean", required: true },
    { name: "profile", type: "object", required: false, nested: [{ name: "answer", type: "string", required: true }] },
    { name: "rows", type: "array", required: false, items: { name: "row", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] } },
  ],
  conditions: { version: "formgen.conditions/v1", rules: [
    { path: "profile", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" },
    { path: "rows[0].answer", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" },
  ] },
};

function render() {
  document.body.innerHTML = `<form><div><input type="checkbox" name="enabled"></div>
    <fieldset id="fg-profile"><input name="profile.answer" required></fieldset>
    <div><input name="rows[0].answer"></div></form>`;
  return document.querySelector("form")!;
}

describe("embedded browser artifact regressions", () => {
  afterEach(() => { document.body.innerHTML = ""; });

  it("ships identical runtime and renderer bundles and source maps", () => {
    const asset = (folder: string, file: string) => readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), `../../pkg/${folder}/assets/${file}`), "utf8"
    );
    expect(asset("renderers/vanilla", "formgen-relationships.min.js")).toBe(bundle);
    expect(asset("renderers/vanilla", "formgen-relationships.min.js.map")).toBe(
      asset("runtime", "formgen-relationships.min.js.map")
    );
  });

  it("publishes complete state and restores nested retained answers and explicit null", () => {
    const form = render();
    const controller = runtime.attachFormController(form, { model });
    const observed: ReturnType<typeof controller.snapshot>[] = [];
    controller.onChange(() => observed.push(controller.snapshot()));
    controller.setValues({ enabled: true, profile: { answer: "saved" }, rows: [{ answer: "indexed" }] }, {});
    expect(observed.at(-1)).toEqual(controller.snapshot());
    controller.setValues({ enabled: false }, {});
    const saved = controller.snapshot();
    expect(saved.retained).toEqual({ profile: { answer: "saved" }, "rows[0].answer": "indexed" });
    controller.reset({});
    expect(observed.at(-1)).toEqual(controller.snapshot());
    controller.restore(saved);
    expect(observed.at(-1)).toEqual(saved);
    expect(controller.snapshot()).toEqual(saved);
    controller.setValues({ enabled: null }, { notify: false });
    expect(controller.snapshot().values.enabled).toBeNull();
    expect(controller.validate().some((issue) => issue.path === "enabled" && issue.code === "required")).toBe(true);
    controller.destroy();
  });

  it("gates descendant requiredness and preserves answers under a disabled fieldset", () => {
    const form = render();
    const controller = runtime.attachFormController(form, { model });
    const answer = form.querySelector<HTMLInputElement>('[name="profile.answer"]')!;
    const fieldset = form.querySelector("fieldset")!;
    expect(answer.required).toBe(false);
    expect(form.checkValidity()).toBe(true);
    controller.setValues({ enabled: true, profile: { answer: "locked" } }, { notify: false });
    expect(answer.required).toBe(true);
    fieldset.disabled = true;
    answer.value = "unobserved DOM change";
    expect(controller.getValues()).not.toHaveProperty("profile");
    expect(controller.snapshot().locked).toContain("profile.answer");
    expect(controller.snapshot().values.profile).toEqual({ answer: "locked" });
    controller.destroy();
  });

  it("reports malformed rule collections through controller validation", () => {
    const form = render();
    const malformed = { ...model, conditions: { version: "formgen.conditions/v1", rules: {} } } as unknown as EmbeddedModel;
    const controller = runtime.attachFormController(form, { model: malformed });
    expect(controller.conditionDiagnostics()[0].path).toBe("conditions.rules");
    expect(controller.validate().some((issue) => issue.code === "condition")).toBe(true);
    controller.destroy();
  });
});

// Keep contract boundaries covered in the delivered IIFE, not only TS source.
describe("embedded shipped contract boundaries", () => {
  afterEach(() => { document.body.innerHTML = ""; });

  it("round trips object leaves, normalizes locked payloads and publishes presence-only mutations", () => {
    document.body.innerHTML = '<form><input name="answer" type="checkbox"><textarea name="config">{"zero":0}</textarea><fieldset disabled><input name="level" value="3"></fieldset></form>';
    const m: EmbeddedModel = { fields: [{ name: "answer", type: "boolean", required: true }, { name: "config", type: "object", required: false }, { name: "level", type: "integer", required: true }] };
    const controller = runtime.attachFormController(document.querySelector("form")!, { model: m });
    const saved = controller.snapshot();
    const seen: ReturnType<typeof controller.snapshot>[] = [];
    controller.onChange(() => seen.push(controller.snapshot()));
    controller.setValues({ answer: false }, {});
    expect(seen).toHaveLength(1);
    expect(seen[0].values.answer).toBe(false);
    controller.setValues({ level: "4", config: { zero: 1 } }, { notify: false });
    expect(controller.snapshot().values).toEqual({ answer: false, config: { zero: 1 }, level: 4 });
    expect(controller.restore(saved, { notify: false })).toEqual([]);
    expect(controller.snapshot()).toEqual(saved);
    controller.destroy();
  });

  it("rejects missing empty rows atomically and expands explicit container clears", () => {
    const form = render();
    const controller = runtime.attachFormController(form, { model });
    controller.setValues({ enabled: true, profile: { answer: "saved" }, rows: [{ answer: "row" }] }, { notify: false });
    const saved = controller.snapshot();
    expect(() => controller.setValues({ enabled: false, rows: [{ answer: "changed" }, {}] }, { mode: "replace" })).toThrow(/rows\[1\]/);
    expect(controller.snapshot()).toEqual(saved);
    controller.setValues({ profile: null }, { notify: false });
    expect(controller.snapshot().values.profile).toEqual({ answer: null });
    controller.destroy();
  });

  it("keeps locked choice members while enabled members change", () => {
    document.body.innerHTML = '<form><input name="colors[]" type="checkbox" value="red" checked><input name="colors[]" type="checkbox" value="blue" checked></form>';
    const m: EmbeddedModel = { fields: [{ name: "colors", type: "array", required: true, items: { name: "color", type: "string", required: false } }] };
    const form = document.querySelector("form")!;
    const controller = runtime.attachFormController(form, { model: m });
    const [red, blue] = Array.from(form.querySelectorAll("input"));
    red.disabled = true; red.checked = false;
    blue.checked = false; blue.dispatchEvent(new Event("change", { bubbles: true }));
    expect(controller.snapshot().values.colors).toEqual(["red"]);
    expect(controller.snapshot().locked).toEqual(["colors"]);
    expect(controller.validate()).toEqual([]);
    controller.destroy();
  });

  it("rejects huge unknown retained indexes before allocating their containers", () => {
    const form = render();
    const controller = runtime.attachFormController(form, { model });
    const saved = controller.snapshot();
    saved.retained = { "ghost[9007199254740991].answer": "ignored", "rows[9007199254740991]": { answer: "ignored" } };
    const push = Array.prototype.push;
    let nullPushes = 0;
    Array.prototype.push = function(...args: unknown[]) {
      if (args.length === 1 && args[0] === null && ++nullPushes > 8) throw new Error("unexpected retained index allocation");
      return push.apply(this, args);
    };
    try {
      expect(controller.restore(saved).map((issue) => issue.code)).toEqual(["unknownField", "unknownField"]);
      expect(nullPushes).toBeLessThan(8);
    } finally { Array.prototype.push = push; controller.destroy(); }
  });

  it("keeps condition scopes inside the selected subtree", () => {
    const m: EmbeddedModel = { fields: [{ name: "gate", type: "boolean", required: false }, { name: "profile", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] }], conditions: { version: "formgen.conditions/v1", rules: [{ path: "profile", requiredWhen: { op: "eq", path: "gate", value: true } }] } };
    expect(runtime.validateConditional(m, { gate: true }, { scope: { paths: ["profile.answer"] } }).issues).toEqual([]);
  });

  it("matches public Go builder definition-shape fixtures", () => {
    for (const testCase of shapes.cases) {
      const m = { fields: [{ name: "gate", type: "boolean", required: false }, { name: "answer", type: "string", required: false }], conditions: testCase.conditions } as unknown as EmbeddedModel;
      expect(runtime.compileConditions(m).program === null, testCase.name).toBe(testCase.invalid);
    }
  });
});

describe("shipped review-fix contracts (IR-2026-10-22-06)", () => {
  afterEach(() => { document.body.innerHTML = ""; });
  for (const parentPolicy of ["retain", "clear"] as const) {
    it(`keeps retained history independent of a ${parentPolicy} parent's native clears`, () => {
      const m: EmbeddedModel = { fields: [
        { name: "gate", type: "boolean", required: false },
        { name: "profile", type: "object", required: false, nested: [{ name: "answer", type: "string", required: false }] },
      ], conditions: { version: "formgen.conditions/v1", rules: [
        { path: "profile", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: parentPolicy },
        { path: "profile.answer", applicableWhen: { op: "eq", path: "gate", value: true }, onInapplicable: parentPolicy === "clear" ? "retain" : "clear" },
      ] } };
      document.body.innerHTML = '<form><div><input name="gate" type="checkbox" checked></div><fieldset id="fg-profile"><div><input name="profile.answer" value="saved"></div></fieldset></form>';
      const c = runtime.attachFormController(document.querySelector("form")!, { model: m });
      c.setValues({ gate: false }, { notify: false });
      const saved = c.snapshot();
      expect(saved.retained).toEqual(parentPolicy === "retain" ? { profile: { answer: "saved" } } : { "profile.answer": "saved" });
      expect(c.settle().retained).toEqual(saved.retained);
      c.setValues({ gate: true, profile: { answer: "edited" } }, { notify: false });
      expect(c.restore(saved, { notify: false })).toEqual([]);
      expect(c.snapshot()).toEqual(saved);
      c.setValues({ gate: true }, { notify: false });
      expect((c.snapshot().values.profile as Record<string, unknown>)?.answer).toBeUndefined();
      c.destroy();
    });
  }
  it("round trips literal reserved-prefix options and captures lossy hydration/restore issues", () => {
    const literal = runtime.encodeEnumValue(false);
    document.body.innerHTML = `<form><input name="code" type="radio" value="${runtime.encodeEnumValue(literal)}" checked><input name="code" type="radio" value="other"><input name="n" type="number" value="3"><fieldset disabled><input name="bad" value="bad"></fieldset></form>`;
    const c = runtime.attachFormController(document.querySelector("form")!, { model: { fields: [
      { name: "code", type: "string", required: false, enum: [literal, "other"] },
      { name: "n", type: "integer", required: false }, { name: "bad", type: "integer", required: false },
    ] } });
    const saved = c.snapshot();
    c.setValues({ code: "other", n: "bad", bad: 7 }, { notify: false });
    expect(c.snapshot().issues.map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:n"]);
    expect(c.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:n"]);
    c.restore(saved, { notify: false });
    expect(c.snapshot()).toEqual(saved);
    expect(c.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["type:bad"]);
    c.setValues({ bad: 0 }, { notify: false });
    expect(c.validate()).toEqual([]);
    c.destroy();
  });
  it("excludes soft-deleted rows and notifies even without conditions", () => {
    document.body.innerHTML = '<form><div><div data-formgen-array-items data-formgen-array-name="rows" data-formgen-array-next-index="1" data-formgen-array-prototype-path="rows[1]"><div data-formgen-array-item data-formgen-array-existing="true" data-formgen-array-path="rows[0]"><input name="rows[0].answer" value="saved"><input name="rows[0]._delete" type="hidden" value="false"><button type="button" data-formgen-array-action="remove">Remove</button></div><template data-formgen-array-prototype><div data-formgen-array-item><input name="rows[1].answer"></div></template></div><button type="button" data-formgen-array-action="add">Add</button></div></form>';
    const form = document.querySelector("form")!;
    runtime.initArrayRepeaters(form);
    const c = runtime.attachFormController(form, { model: { fields: [{ name: "rows", type: "array", required: false, validations: [{ kind: "minItems", params: { value: "1" } }], items: { name: "row", type: "object", required: true, nested: [{ name: "answer", type: "string", required: true }] } }] } });
    const seen: ReturnType<typeof c.snapshot>[] = [];
    c.onChange(() => seen.push(c.snapshot()));
    form.querySelector<HTMLButtonElement>('[data-formgen-array-action="remove"]')!.click();
    expect(seen).toHaveLength(1);
    expect(seen[0].values).toEqual({ rows: [] });
    expect(c.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["minItems:rows"]);
    expect(c.effective().applicable["rows[0]"]).toBe(false);
    form.querySelector<HTMLButtonElement>('[data-formgen-array-action="add"]')!.click();
    expect(seen).toHaveLength(2);
    expect(c.validate().map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:rows[1].answer"]);
    c.destroy();
  });
  it("keeps raw JSON member keys as own data through snapshot restoration", () => {
    const raw = JSON.parse('{"constructor":"builder","prototype":{"a":1},"__proto__":{"flag":true}}');
    document.body.innerHTML = '<form><textarea name="config"></textarea></form>';
    const c = runtime.attachFormController(document.querySelector("form")!, { model: { fields: [{ name: "config", type: "object", required: false }] } });
    c.setValues({ config: raw }, { notify: false });
    const saved = c.snapshot();
    expect(JSON.stringify(saved.values.config)).toBe(JSON.stringify(raw));
    c.setValues({ config: { edited: true } }, { notify: false });
    c.restore(saved, { notify: false });
    expect(c.snapshot()).toEqual(saved);
    expect(({} as Record<string, unknown>).flag).toBeUndefined();
    c.destroy();
  });
});
