import { describe, it, expect } from "vitest";
import fixture from "../../testdata/embedded/v1.json";
import { compileConditions, evaluateExpression, validateConditional } from "../src/embedded-conditions";
import { normalizeValues } from "../src/embedded-values";
import type { ConditionSet, EmbeddedField, EmbeddedModel, JSONValue } from "../src/embedded-contract";

const sharedModel = fixture.form as unknown as EmbeddedModel;
const indexedModel = fixture.indexed.form as unknown as EmbeddedModel;

function outcome(model: EmbeddedModel, raw: unknown, policy: "reject" | "normalize" = "reject") {
  const normalized = normalizeValues(model, raw);
  const result = validateConditional(model, normalized.values, { policy });
  return {
    issues: normalized.issues
      .concat(result.issues)
      .map((issue) => `${issue.code}:${issue.path}`)
      .sort(),
    values: result.projection?.values,
    projection: result.projection,
  };
}

function field(partial: Partial<EmbeddedField> & { name: string; type: EmbeddedField["type"] }): EmbeddedField {
  return { required: false, ...partial };
}

function withRules(base: EmbeddedModel, rules: unknown[]): EmbeddedModel {
  return { ...base, conditions: { version: "formgen.conditions/v1", rules } as unknown as ConditionSet };
}

const baseModel: EmbeddedModel = {
  fields: [
    field({ name: "enabled", type: "boolean" }),
    field({ name: "a", type: "integer" }),
    field({ name: "b", type: "string" }),
    field({ name: "c", type: "boolean" }),
  ],
};

describe("embedded conditions: shared fixture parity", () => {
  for (const testCase of fixture.cases) {
    it(`matches Go outcomes for ${testCase.id}`, () => {
      expect(outcome(sharedModel, testCase.raw).issues).toEqual([...testCase.issues].sort());
    });
  }

  it("keeps typed values and drops stale inactive input under normalize (C04, C07)", () => {
    const c04 = fixture.cases.find((entry) => entry.id === "C04")!;
    expect(outcome(sharedModel, c04.raw).values).toEqual({ enabled: true, level: 4, colors: ["blue"] });
    const c07 = fixture.cases.find((entry) => entry.id === "C07")!;
    const normalized = outcome(sharedModel, c07.raw, "normalize");
    expect(normalized.issues).toEqual([]);
    expect(normalized.values).not.toHaveProperty("level");
    expect(normalized.projection?.applicable).toEqual({ level: false });
  });

  for (const invalid of fixture.invalidConditions) {
    it(`rejects invalid definition ${invalid.id}`, () => {
      const compiled = compileConditions({ ...sharedModel, conditions: invalid.definition as unknown as ConditionSet });
      expect(compiled.program).toBeNull();
      expect(compiled.diagnostics.length).toBeGreaterThan(0);
      expect(JSON.stringify(compiled.diagnostics)).not.toContain("eval(");
    });
  }

  for (const testCase of fixture.indexed.cases) {
    it(`matches indexed Go outcomes for ${testCase.id}`, () => {
      const result = outcome(indexedModel, testCase.raw, testCase.policy as "reject" | "normalize");
      expect(result.issues).toEqual([...testCase.issues].sort());
      expect(result.values).toEqual(testCase.values);
    });
  }
});

describe("embedded conditions: evaluator semantics", () => {
  it("evaluates chains, excludes inactive upstream values, and keeps retained copies independent", () => {
    const model = withRules(baseModel, [
      { path: "b", applicableWhen: { op: "present", path: "a" }, onInapplicable: "retain" },
      { path: "a", applicableWhen: { op: "eq", path: "enabled", value: true } },
      {
        path: "c",
        applicableWhen: {
          op: "or",
          args: [
            { op: "eq", path: "b", value: "yes" },
            { op: "and", args: [{ op: "eq", path: "enabled", value: true }, { op: "eq", path: "a", value: 0 }] },
          ],
        },
      },
    ]);
    const compiled = compileConditions(model);
    expect(compiled.diagnostics).toEqual([]);
    const input: Record<string, JSONValue> = { enabled: false, a: 0, b: "yes", c: true };
    const projection = compiled.program!.evaluate(input);
    expect(Object.keys(projection.values)).toEqual(["enabled"]);
    expect(projection.applicable).toEqual({ a: false, b: false, c: false });
    expect(projection.retained).toEqual({ b: "yes" });
    expect(input).toEqual({ enabled: false, a: 0, b: "yes", c: true });

    // Mutating the authoritative definition after compilation cannot alter the plan.
    (model.conditions as unknown as { rules: Array<{ applicableWhen: { value: unknown } }> }).rules[1].applicableWhen.value = false;
    expect(compiled.program!.evaluate(input).applicable.a).toBe(false);

    const enabled = compiled.program!.evaluate({ ...input, enabled: true });
    expect(enabled.applicable).toEqual({ a: true, b: true, c: true });
  });

  it("requires presence for comparisons and never infers false or zero", () => {
    const table: Array<{ value?: JSONValue; present: boolean; want: boolean }> = [
      { value: null, present: true, want: false },
      { value: false, present: true, want: true },
      { value: 0, present: true, want: true },
      { value: "", present: true, want: true },
      { value: "false", present: true, want: true },
      { present: false, want: false },
    ];
    for (const row of table) {
      const model = withRules(baseModel, [{ path: "a", applicableWhen: { op: "present", path: "enabled" } }]);
      const program = compileConditions(model).program!;
      const input: Record<string, JSONValue> = row.present ? { enabled: row.value as JSONValue } : {};
      expect(program.evaluate(input).applicable.a).toBe(row.want);
    }
    const eqModel = withRules(baseModel, [{ path: "a", applicableWhen: { op: "eq", path: "enabled", value: false } }]);
    const program = compileConditions(eqModel).program!;
    expect(program.evaluate({ enabled: "false" }).applicable.a).toBe(false);
    expect(program.evaluate(null).applicable.a).toBe(false);
    expect(program.evaluate({ enabled: false }).applicable.a).toBe(true);
    expect(evaluateExpression({ op: "eq", path: "n", value: 1 }, { n: 1.0 })).toBe(true);
    expect(evaluateExpression({ op: "eq", path: "n", value: "1" }, { n: 1 })).toBe(false);
  });

  it("gates children through parents and applies fixed indexed rules", () => {
    const model = withRules(
      {
        fields: [
          ...baseModel.fields,
          field({
            name: "rows",
            type: "array",
            items: field({
              name: "row",
              type: "object",
              nested: [field({ name: "enabled", type: "boolean" }), field({ name: "answer", type: "string" })],
            }),
          }),
          field({ name: "profile", type: "object", nested: [field({ name: "answer", type: "string" })] }),
        ],
      },
      [
        { path: "rows[1].answer", applicableWhen: { op: "eq", path: "rows[1].enabled", value: true } },
        { path: "profile.answer", applicableWhen: { op: "present", path: "enabled" } },
        { path: "profile", applicableWhen: { op: "eq", path: "enabled", value: true } },
      ]
    );
    const compiled = compileConditions(model);
    expect(compiled.diagnostics).toEqual([]);
    const input: Record<string, JSONValue> = {
      enabled: false,
      profile: { answer: "secret" },
      rows: [{ answer: "keep" }, { enabled: false, answer: "drop" }],
    };
    const projection = compiled.program!.evaluate(input);
    expect(projection.values).not.toHaveProperty("profile");
    expect(projection.applicable["profile.answer"]).toBe(false);
    expect(projection.values.rows).toEqual([{ answer: "keep" }, { enabled: false }]);
    expect(input.rows).toEqual([{ answer: "keep" }, { enabled: false, answer: "drop" }]);
  });

  it("rejects invalid definitions with diagnostics and no program", () => {
    const cases: unknown[] = [
      { version: "future" },
      { version: "formgen.conditions/v1", rules: [{ path: "missing", applicableWhen: { op: "present", path: "enabled" } }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "eq", path: "missing", value: true } }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "js", path: "enabled", value: true } }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "eq", path: "enabled", value: null } }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "and" } }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "present", path: "a" } }] },
      {
        version: "formgen.conditions/v1",
        rules: [
          { path: "a", applicableWhen: { op: "present", path: "b" } },
          { path: "b", applicableWhen: { op: "present", path: "a" } },
        ],
      },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "present", path: "enabled" }, onInapplicable: "guess" }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a" }] },
      { version: "formgen.conditions/v1", rules: [{ path: "a", applicableWhen: { op: "eq", path: "enabled", value: true } }, { path: "a", requiredWhen: { op: "present", path: "b" } }] },
    ];
    for (const [index, conditions] of cases.entries()) {
      const compiled = compileConditions({ ...baseModel, conditions: conditions as ConditionSet });
      expect(compiled.program, `case ${index}`).toBeNull();
      expect(compiled.diagnostics.length, `case ${index}`).toBeGreaterThan(0);
      for (const diagnostic of compiled.diagnostics) {
        expect(diagnostic.message).not.toContain("true");
      }
    }
    const cycle = compileConditions({
      ...baseModel,
      conditions: cases[7] as ConditionSet,
    });
    expect(cycle.diagnostics.map((diagnostic) => diagnostic.code)).toEqual(["conditionCycle", "conditionCycle"]);
  });

  it("keeps retained copies independent and never restores cleared input on re-entry", () => {
    const model = withRules(
      { fields: [...baseModel.fields, field({ name: "object", type: "object", nested: [field({ name: "answer", type: "string" })] })] },
      [
        { path: "a", applicableWhen: { op: "eq", path: "enabled", value: true } },
        { path: "object", applicableWhen: { op: "eq", path: "enabled", value: true }, onInapplicable: "retain" },
      ]
    );
    const program = compileConditions(model).program!;
    const input: Record<string, JSONValue> = { enabled: false, a: 4, object: { answer: "saved" } };
    const projected = program.evaluate(input);
    (projected.retained.object as Record<string, JSONValue>).answer = "mutated";
    expect((input.object as Record<string, JSONValue>).answer).toBe("saved");
    projected.values.enabled = true;
    const next = program.evaluate(projected.values);
    expect(next.applicable.a).toBe(true);
    expect(next.values).not.toHaveProperty("a");
  });

  it("applies indexed rules only to existing rows", () => {
    const model = withRules(
      {
        fields: [
          field({ name: "enabled", type: "boolean" }),
          field({
            name: "rows",
            type: "array",
            items: field({ name: "row", type: "object", nested: [field({ name: "answer", type: "string", required: true })] }),
          }),
        ],
      },
      [{ path: "rows[2].answer", applicableWhen: { op: "eq", path: "enabled", value: true } }]
    );
    const program = compileConditions(model).program!;
    for (const rows of [undefined, [], [{}]]) {
      const input: Record<string, JSONValue> = { enabled: true };
      if (rows !== undefined) {
        input.rows = rows as JSONValue;
      }
      const projection = program.evaluate(input);
      expect(projection.applicable["rows[2].answer"]).toBe(false);
      expect(projection.required["rows[2].answer"]).toBe(false);
    }
    const existing = program.evaluate({ enabled: true, rows: [{}, {}, {}] });
    expect(existing.applicable["rows[2].answer"]).toBe(true);
    expect(existing.required["rows[2].answer"]).toBe(true);
    const result = validateConditional(model, { enabled: true, rows: [{ answer: "ok" }, { answer: "ok" }, {}] });
    expect(result.issues.map((issue) => `${issue.code}:${issue.path}`)).toEqual(["required:rows[2].answer"]);
    expect(validateConditional(model, { enabled: true, rows: [] }).issues).toEqual([]);
    expect(validateConditional(model, { enabled: false, rows: [{ answer: "ok" }, { answer: "ok" }, {}] }).issues).toEqual([]);
  });

  it("limits scoped conditional validation to the selected subtree", () => {
    const normalized = normalizeValues(sharedModel, { enabled: true, colors: ["blue"] });
    expect(validateConditional(sharedModel, normalized.values, { scope: { paths: ["colors"] } }).issues).toEqual([]);
    expect(
      validateConditional(sharedModel, normalized.values, { scope: { paths: ["level"] } }).issues.map((issue) => issue.code)
    ).toEqual(["required"]);
    const stale = normalizeValues(sharedModel, { enabled: false, level: 4, colors: ["blue"] });
    expect(validateConditional(sharedModel, stale.values, { scope: { paths: ["colors"] } }).issues).toEqual([]);
    expect(validateConditional(sharedModel, stale.values, { scope: { paths: ["level"] } }).issues.map((issue) => issue.code)).toEqual([
      "inactiveInput",
    ]);
  });

  it("reports definition diagnostics as condition issues and still validates", () => {
    const invalid = { ...sharedModel, conditions: fixture.invalidConditions[0].definition as unknown as ConditionSet };
    const result = validateConditional(invalid, { enabled: true, colors: [] });
    expect(result.projection).toBeNull();
    expect(result.issues.map((issue) => `${issue.code}:${issue.path}`)).toEqual(["condition:level", "minItems:colors"]);
  });
});
