import { describe, it, expect } from "vitest";
import fixture from "../../testdata/embedded/v1.json";
import type { ConditionSet, Expression, EmbeddedDescriptor } from "../src/embedded-contract";

// Compile-time examples exercise the published import seam without enabling runtime behavior.
const condition: ConditionSet = { version: "formgen.conditions/v1", rules: [
  { path: "rows[2].answer", applicableWhen: { op: "eq", path: "rows[2].enabled", value: false } },
] };
const expressions: Expression[] = [
  { op: "eq", path: "zero", value: 0 }, { op: "eq", path: "code", value: "false" },
  { op: "present", path: "missing" },
];
const descriptor: EmbeddedDescriptor = { version: "formgen.descriptor/v1", form: { fields: [], conditions: condition } };
describe("embedded/v1 shared contract", () => {
  it("loads versioned synthetic expectations and typed examples", () => {
    expect(fixture.version).toBe("formgen.embedded/v1");
    expect(fixture.form.conditions.version).toBe(condition.version);
    expect(fixture.cases.find(c => c.id === "C02")?.raw.enabled).toBe(false);
    expect(expressions).toHaveLength(3);
    expect(fixture.invalidConditions).toHaveLength(3);
    expect(fixture.indexed.cases).toHaveLength(8);
    expect(descriptor.form.conditions?.rules[0].path).toBe("rows[2].answer");
  });
});
