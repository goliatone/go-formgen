import { afterEach, describe, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import * as source from "../src/index";

const shipped = new Function(`${readFileSync(resolve("../pkg/runtime/assets/formgen-relationships.min.js"), "utf8")};return FormgenRelationships;`)() as typeof source;
const deferred = <T = void>() => { let resolve!: (value: T) => void; const promise = new Promise<T>(r => resolve = r); return { promise, resolve }; };
const response = (value = "old") => ({ ok: true, status: 200, text: async () => JSON.stringify([{ value, label: value }]) }) as Response;

for (const [entry, runtime] of [["source", source], ["shipped", shipped]] as const) describe(`relationship boundaries ${entry}`, () => {
  const registries: source.ResolverRegistry[] = [];
  afterEach(() => { registries.splice(0).forEach(registry => registry.destroy()); document.body.innerHTML = ""; vi.unstubAllGlobals(); });
  async function init(markup: string, config: source.GlobalConfig = {}) {
    document.body.innerHTML = markup;
    const form = document.querySelector("form")!;
    const registry = await runtime.initFormgenRoot(form, config);
    registries.push(registry);
    return { form, registry, select: form.querySelector("select")! };
  }
  const manual = (name = "code", extra = "", options = '<option value="a" selected>A</option>') => `<select name="${name}" data-endpoint-url="/options" data-endpoint-refresh="manual" ${extra}>${options}</select>`;

  for (const phase of ["headers", "cache", "custom", "beforeFetch", "body", "transform", "afterFetch", "validation"] as const) it(`invalidates delayed ${phase} across destroy and reinit`, async () => {
    const started = deferred(), gate = deferred();
    const wait = async () => { started.resolve(); await gate.promise; };
    const config: source.GlobalConfig = { cache: { strategy: "none" } };
    vi.stubGlobal("fetch", vi.fn(async () => {
      const result = response();
      if (phase === "body") result.text = async () => { await wait(); return JSON.stringify([{ value: "old", label: "old" }]); };
      return result;
    }));
    if (phase === "headers") config.buildHeaders = async () => { await wait(); return {}; };
    if (phase === "beforeFetch") config.beforeFetch = wait;
    if (phase === "transform") config.transformOptions = async (_context, options) => { await wait(); return options; };
    if (phase === "afterFetch") config.afterFetch = wait;
    if (phase === "validation") config.validateSelection = async () => { await wait(); return { valid: false, errors: [{ code: "late", message: "late" }], messages: ["late"] }; };
    const cacheSet = vi.fn();
    if (phase === "cache") config.cache = { strategy: "custom", adapter: { get: async () => { await wait(); return [{ value: "old", label: "old" }]; }, set: cacheSet } };
    const { form, registry, select } = await init(`<form>${manual()}</form>`, config);
    if (phase === "custom") registry.registerResolver("delayed", { name: "delayed", matches: () => true, resolve: async () => { await wait(); return [{ value: "old", label: "old" }]; } });
    const controller = runtime.attachFormController(form, { registry });
    const pending = registry.resolve(select);
    await started.promise;
    controller.destroy();
    const fresh = await runtime.initFormgenRoot(form, { cache: { strategy: "none" } }); registries.push(fresh);
    vi.stubGlobal("fetch", vi.fn(async () => response("new")));
    await fresh.resolve(select);
    const events: string[] = [];
    for (const type of ["success", "error", "validation"]) select.addEventListener(`formgen:relationship:${type}`, () => events.push(type));
    gate.resolve(); await pending;
    expect(select.value).toBe("new"); expect(events).toEqual([]); expect(cacheSet).not.toHaveBeenCalled();
  });

  it("keeps newer refreshes authoritative while an old transform is pending", async () => {
    const started = deferred(), gate = deferred(); let transforms = 0;
    vi.stubGlobal("fetch", vi.fn(async () => response(++transforms === 1 ? "old" : "new")));
    const { registry, select } = await init(`<form>${manual()}</form>`, { cache: { strategy: "none" }, transformOptions: async (_context, options) => {
      if (options[0].value === "old") { started.resolve(); await gate.promise; } return options;
    } });
    const pending = registry.resolve(select); await started.promise;
    await registry.resolve(select); gate.resolve(); await pending;
    expect(select.value).toBe("new");
  });

  it("keeps explicit hydration authoritative over a pending transform", async () => {
    const started = deferred(), gate = deferred();
    vi.stubGlobal("fetch", vi.fn(async () => response("old")));
    const { form, registry, select } = await init(`<form>${manual()}</form>`, {
      cache: { strategy: "none" }, transformOptions: async (_context, options) => {
        started.resolve(); await gate.promise; return options;
      },
    });
    const controller = runtime.attachFormController(form, { registry });
    const pending = registry.resolve(select); await started.promise;
    controller.setValues({ code: "hydrated" }); gate.resolve(); await pending;
    expect(select.value).toBe("hydrated"); controller.destroy();
  });

  it("preserves empty JSON choices and external form ownership", async () => {
    const { form } = await init(`<form id="external"></form><div>${manual("code", 'form="external" data-endpoint-submit-as="json"', '<option value="" selected>Empty</option>')}</div>`);
    // Initialize the external select separately, as it is outside the root.
    const select = document.querySelector("select")!;
    const registry = await runtime.initFormgenRoot(select); registries.push(registry);
    expect([...new FormData(form)]).toEqual([["code", '""']]);
    select.disabled = true; select.dispatchEvent(new Event("change", { bubbles: true }));
    expect([...new FormData(form)]).toEqual([]);
    registry.destroy(); expect(select.name).toBe("code");
    select.disabled = false; select.dispatchEvent(new Event("change", { bubbles: true }));
    expect(document.querySelectorAll("[data-relationship-hidden]")).toHaveLength(0);
    expect([...new FormData(form)]).toEqual([["code", ""]]);
  });

  it("invalidates pending manual validation and creation on teardown", async () => {
    const validation = deferred(), creation = deferred(), enteredValidation = deferred(), enteredCreation = deferred();
    const { registry, select } = await init(`<form>${manual()}</form>`, {
      validateSelection: async () => { enteredValidation.resolve(); await validation.promise; return { valid: false, errors: [], messages: ["stale"] }; },
      createOption: async () => { enteredCreation.resolve(); await creation.promise; return { value: "stale", label: "stale" }; },
    });
    const validating = registry.validate(select); await enteredValidation.promise;
    const creating = registry.create(select, "new"); await enteredCreation.promise;
    registry.destroy(); validation.resolve(); creation.resolve();
    expect(await creating).toBeUndefined(); await validating;
    expect(select.getAttribute("data-validation-state")).not.toBe("invalid");
  });

  it("isolates sibling JSON and array mirrors and restores names on teardown", async () => {
    const { form, registry } = await init(`<form>${manual("first", 'data-endpoint-submit-as="json"')}${manual("second", 'data-endpoint-submit-as="json"', '<option value="b" selected>B</option>')}${manual("codes[]", "multiple")}</form>`);
    const expected = [["first", '"a"'], ["second", '"b"'], ["codes[]", "a"]];
    expect([...new FormData(form)]).toEqual(expected);
    expect(form.querySelectorAll("[data-relationship-hidden]")).toHaveLength(3);
    registry.destroy();
    expect(form.querySelectorAll("[data-relationship-hidden]")).toHaveLength(0);
    expect(Array.from(form.querySelectorAll("select")).map(select => select.name)).toEqual(["first", "second", "codes[]"]);
    const again = await runtime.initFormgenRoot(form); registries.push(again);
    expect([...new FormData(form)]).toEqual(expected);
  });

  for (const name of ["codes", "codes[]"]) it(`submits one array channel for ${name}`, async () => {
    const { form, select } = await init(`<form>${manual(name, "multiple")}</form>`);
    expect([...new FormData(form)]).toEqual([["codes[]", "a"]]);
    select.disabled = true; select.dispatchEvent(new Event("change", { bubbles: true }));
    expect([...new FormData(form)]).toEqual([]);
    select.disabled = false; await Promise.resolve();
    expect([...new FormData(form)]).toEqual([["codes[]", "a"]]);
  });

  for (const mode of ["array", "json"]) it(`honors native locks and option eligibility in ${mode} mode`, async () => {
    const extra = mode === "json" ? 'multiple data-endpoint-submit-as="json"' : "multiple";
    const { form, select } = await init(`<form><fieldset>${manual("codes[]", extra, '<option value="a" selected>A</option><option value="blocked" selected disabled>Blocked</option><optgroup disabled><option value="group" selected>Group</option></optgroup>')}</fieldset></form>`);
    const expected = mode === "json" ? [["codes", '["a"]']] : [["codes[]", "a"]];
    expect([...new FormData(form)]).toEqual(expected);
    select.setAttribute("aria-readonly", "true"); expect([...new FormData(form)]).toEqual(expected);
    form.querySelector("fieldset")!.disabled = true; await Promise.resolve(); expect([...new FormData(form)]).toEqual([]);
    form.querySelector("fieldset")!.disabled = false; select.disabled = true; await Promise.resolve(); expect([...new FormData(form)]).toEqual([]);
  });

  it("keeps first-legend exemptions and repeated field renames", async () => {
    const { form, select } = await init(`<form><fieldset disabled><legend>${manual("rows[0].codes[]", "multiple")}</legend>${manual("blocked[]", "multiple")}</fieldset></form>`);
    expect([...new FormData(form)]).toEqual([["rows[0].codes[]", "a"]]);
    select.name = "rows[2].codes[]"; await Promise.resolve();
    expect([...new FormData(form)]).toEqual([["rows[2].codes[]", "a"]]);
  });

  it("recomputes same-turn successful values through the live formdata channel", async () => {
    vi.stubGlobal("FormDataEvent", class {});
    const { form, select } = await init(`<form><input name="ordinary" value="kept">${manual("codes[]", "multiple")}${manual("code", 'data-endpoint-submit-as="json"')}</form>`);
    select.disabled = true;
    const data = new FormData(form), event = new Event("formdata", { bubbles: true });
    Object.defineProperty(event, "formData", { value: data }); form.dispatchEvent(event);
    expect([...data]).toEqual([["ordinary", "kept"], ["code", '"a"']]);
  });
});
