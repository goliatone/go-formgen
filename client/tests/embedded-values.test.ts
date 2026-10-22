import { describe, it, expect } from "vitest";
import fixture from "../../testdata/embedded/v1.json";
import {
  normalizeValues,
  validateValues,
  parseCanonicalPath,
  formatCanonicalPath,
  rendererPath,
  pathWithin,
  encodeEnumValue,
  decodeEnumValue,
  resolveEmbeddedModel,
} from "../src/embedded-values";
import type { EmbeddedField, EmbeddedModel, JSONValue } from "../src/embedded-contract";

const sharedModel = fixture.form as unknown as EmbeddedModel;

// Cases whose expectations do not depend on condition evaluation (T09/server).
const UNCONDITIONAL_CASES = [
  "C01",
  "C02",
  "C04",
  "C05",
  "C06",
  "C06-empty",
  "C06-duplicates",
  "C05-min-bound",
  "C05-max-bound",
  "C06-count",
];

function outcome(model: EmbeddedModel, raw: unknown, scope?: { paths?: string[] }): string[] {
  const normalized = normalizeValues(model, raw);
  return normalized.issues
    .concat(validateValues(model, normalized.values, scope))
    .map((issue) => `${issue.code}:${issue.path}`);
}

function field(partial: Partial<EmbeddedField> & { name: string; type: EmbeddedField["type"] }): EmbeddedField {
  return { required: false, ...partial };
}

describe("embedded values: shared fixture parity", () => {
  for (const id of UNCONDITIONAL_CASES) {
    it(`matches Go outcomes for ${id}`, () => {
      const testCase = fixture.cases.find((entry) => entry.id === id)!;
      expect(outcome(sharedModel, testCase.raw)).toEqual(testCase.issues);
    });
  }

  it("leaves condition-dependent expectations (C03, C07) to applicability evaluation", () => {
    const c03 = fixture.cases.find((entry) => entry.id === "C03")!;
    const c07 = fixture.cases.find((entry) => entry.id === "C07")!;
    expect(outcome(sharedModel, c03.raw)).toEqual([]);
    expect(outcome(sharedModel, c07.raw)).toEqual([]);
  });
});

describe("embedded values: normalization", () => {
  const model: EmbeddedModel = {
    fields: [
      ...sharedModel.fields,
      field({ name: "note", type: "string", validations: [{ kind: "maxLength", params: { value: "80" } }] }),
    ],
  };

  it("normalizes raw DOM strings into typed values and keeps zero and false (C10)", () => {
    const scaleModel: EmbeddedModel = {
      fields: [
        ...model.fields,
        field({
          name: "scale",
          type: "integer",
          validations: [
            { kind: "min", params: { value: "0" } },
            { kind: "max", params: { value: "10" } },
          ],
        }),
      ],
    };
    const normalized = normalizeValues(scaleModel, { enabled: "false", scale: "0", colors: ["blue"], note: "" });
    expect(normalized.issues).toEqual([]);
    expect(normalized.values).toEqual({ enabled: false, scale: 0, colors: ["blue"], note: "" });
    expect(validateValues(scaleModel, normalized.values)).toEqual([]);
    // Zero is a value, not a missing answer; the 1–10 level still enforces its own bound.
    expect(outcome(scaleModel, { enabled: true, level: 0, colors: ["blue"] })).toEqual(["min:level"]);
    expect(outcome(scaleModel, { enabled: false, scale: 0, colors: ["blue"] })).toEqual([]);
  });

  it("distinguishes missing, null, false, zero, and string false", () => {
    expect(outcome(model, {})).toEqual(["required:enabled", "required:colors"]);
    expect(outcome(model, { enabled: null, colors: ["blue"] })).toEqual(["required:enabled"]);
    expect(outcome(model, { enabled: false, colors: ["blue"] })).toEqual([]);
    const codeModel: EmbeddedModel = { fields: [field({ name: "code", type: "string", enum: ["false", "true"] })] };
    expect(normalizeValues(codeModel, { code: "false" }).values).toEqual({ code: "false" });
    expect(normalizeValues(model, { enabled: "false", level: "", colors: ["blue"] }).values).toEqual({
      enabled: false,
      level: null,
      colors: ["blue"],
    });
  });

  it("applies the empty-string policy", () => {
    expect(normalizeValues(model, { level: "" }).values).toEqual({ level: null });
    const preserved = normalizeValues(model, { level: "", note: "" }, { emptyStrings: "preserve" });
    expect(preserved.values).toEqual({ level: "", note: "" });
    // Whitespace-only optional values skip constraints without reporting type issues.
    expect(validateValues(model, preserved.values)).toEqual(expect.arrayContaining([]));
    expect(validateValues(model, preserved.values).map((issue) => issue.code)).not.toContain("type");
  });

  it("decodes typed enum control values and keeps plain strings plain", () => {
    expect(decodeEnumValue(encodeEnumValue(false))).toEqual({ value: false, encoded: true });
    expect(decodeEnumValue(encodeEnumValue(4))).toEqual({ value: 4, encoded: true });
    expect(decodeEnumValue(encodeEnumValue("x"))).toEqual({ value: "x", encoded: true });
    expect(decodeEnumValue(encodeEnumValue(null))).toEqual({ value: null, encoded: true });
    expect(decodeEnumValue("__fg_enum_v1:not-base64!!")).toEqual({ value: "__fg_enum_v1:not-base64!!", encoded: false });
    expect(decodeEnumValue("false")).toEqual({ value: "false", encoded: false });

    const flagModel: EmbeddedModel = { fields: [field({ name: "flag", type: "boolean", required: true })] };
    expect(normalizeValues(flagModel, { flag: encodeEnumValue(false) }).values).toEqual({ flag: false });
    const prefixModel: EmbeddedModel = {
      fields: [field({ name: "code", type: "string", enum: [encodeEnumValue(false), "false"] })],
    };
    for (const value of [encodeEnumValue(false), "false"]) {
      // The Go control encoding leaves plain strings untouched; a string that collides
      // with the prefix is re-encoded, so decoding restores the original string value.
      const control = value.startsWith("__fg_enum_v1:") ? encodeEnumValue(value) : value;
      const normalized = normalizeValues(prefixModel, { code: control });
      expect(normalized.issues).toEqual([]);
      expect(normalized.values.code).toBe(value);
      expect(validateValues(prefixModel, normalized.values)).toEqual([]);
    }
  });

  it("reports unsafe or non-integral integers instead of rounding", () => {
    const bigModel: EmbeddedModel = { fields: [field({ name: "big", type: "integer" })] };
    const fromString = normalizeValues(bigModel, { big: "9007199254740993" });
    expect(fromString.values).toEqual({ big: null });
    expect(fromString.issues.map((issue) => issue.code)).toEqual(["type"]);
    const fromNumber = normalizeValues(bigModel, { big: 9007199254740993 });
    expect(fromNumber.values).toEqual({ big: null });
    expect(fromNumber.issues).toHaveLength(1);
    expect(normalizeValues(bigModel, { big: 4.5 }).issues.map((issue) => issue.code)).toEqual(["type"]);
    expect(normalizeValues(bigModel, { big: " 42 " }).values).toEqual({ big: 42 });
    expect(normalizeValues(bigModel, { big: "abc" }).issues[0]).toMatchObject({
      code: "type",
      path: "big",
      rendererPath: "big",
    });
    expect(normalizeValues(bigModel, { big: "abc" }).issues[0]).not.toHaveProperty("value");
  });

  it("handles unknown keys by policy and drops repeater transport keys", () => {
    const raw = { enabled: true, colors: ["blue"], extra: 1, _row_key: "new-1" };
    expect(normalizeValues(model, raw).issues.map((issue) => `${issue.code}:${issue.path}`)).toEqual([
      "unknownField:extra",
    ]);
    const ignored = normalizeValues(model, raw, { unknownFields: "ignore" });
    expect(ignored.issues).toEqual([]);
    expect(ignored.values).toEqual({ enabled: true, colors: ["blue"] });
    const preserved = normalizeValues(model, raw, { unknownFields: "preserve" });
    expect(preserved.values.extra).toBe(1);
    expect(preserved.values).not.toHaveProperty("_row_key");
  });

  it("treats numeric-keyed objects as arrays and wraps scalars", () => {
    expect(normalizeValues(model, { colors: { 0: "blue", 1: "green" } }).values).toEqual({
      colors: ["blue", "green"],
    });
    expect(normalizeValues(model, { colors: "blue" }).values).toEqual({ colors: ["blue"] });
    expect(normalizeValues(model, { colors: { 1: "green" } }).values).toEqual({ colors: [null, "green"] });
  });

  it("accepts raw object fields as objects or JSON strings", () => {
    const rawModel: EmbeddedModel = { fields: [field({ name: "meta", type: "object" })] };
    const source = { a: 1 };
    const normalized = normalizeValues(rawModel, { meta: source });
    expect(normalized.values).toEqual({ meta: { a: 1 } });
    expect(normalized.values.meta).not.toBe(source);
    expect(normalizeValues(rawModel, { meta: '{"a":1}' }).values).toEqual({ meta: { a: 1 } });
    expect(normalizeValues(rawModel, { meta: "not json" }).issues[0].code).toBe("invalidJSON");
    expect(normalizeValues(rawModel, { meta: 5 }).issues[0].code).toBe("object");
    expect(normalizeValues(rawModel, { meta: "[1]" }).issues[0].code).toBe("object");
  });

  it("never mutates prototypes through unsafe keys", () => {
    const unsafe = JSON.parse('{"__proto__":{"polluted":true},"enabled":true,"colors":["blue"]}') as Record<string, unknown>;
    const normalized = normalizeValues(model, unsafe);
    expect(normalized.values).toEqual({ enabled: true, colors: ["blue"] });
    expect((Object.prototype as Record<string, unknown>).polluted).toBeUndefined();
    expect(normalized.issues).toEqual([]);
  });

  it("resolves descriptor envelopes and rejects incomplete models", () => {
    expect(resolveEmbeddedModel({ version: "formgen.descriptor/v1", form: sharedModel })).toBe(sharedModel);
    expect(() => resolveEmbeddedModel({} as EmbeddedModel)).toThrow(/fields/);
    expect(() => resolveEmbeddedModel(null)).toThrow(/fields/);
  });
});

describe("embedded values: validation", () => {
  const rowItem = field({
    name: "row",
    type: "object",
    nested: [
      field({ name: "name", type: "string", required: true }),
      field({ name: "future", type: "string", required: true }),
    ],
  });
  const rowsModel: EmbeddedModel = {
    fields: [
      field({ name: "rows", type: "array", items: rowItem }),
      field({ name: "future", type: "string", required: true }),
    ],
  };

  it("validates only the selected scope and ignores unrelated required fields (C09)", () => {
    const normalized = normalizeValues(sharedModel, { colors: ["blue"] });
    expect(validateValues(sharedModel, normalized.values, { paths: ["colors"] })).toEqual([]);
    expect(validateValues(sharedModel, normalized.values).map((issue) => `${issue.code}:${issue.path}`)).toEqual([
      "required:enabled",
    ]);
    expect(validateValues(sharedModel, normalized.values, { paths: [] })).toEqual([]);
  });

  it("reports invalid and unknown scope paths", () => {
    for (const path of ["rows[-1].name", "rows[01].name", "rows..name", "__proto__.x", "rows[].name", "rows[0]name"]) {
      const issues = validateValues(rowsModel, {}, { paths: [path] });
      expect(issues.map((issue) => issue.code)).toEqual(["invalidPath"]);
      expect(issues[0].path).toBe(path);
    }
    expect(validateValues(rowsModel, {}, { paths: ["unknown"] }).map((issue) => issue.code)).toEqual(["unknownField"]);
    expect(validateValues(rowsModel, {}, { paths: ["rows[0].missing"] }).map((issue) => issue.code)).toEqual([
      "unknownField",
    ]);
  });

  it("validates rendered repeated rows with precise paths and renderer grouping", () => {
    const normalized = normalizeValues(rowsModel, {
      rows: { 0: { name: "ok", _present: "true" }, 1: { _present: "true" } },
    });
    expect(normalized.issues).toEqual([]);
    expect(normalized.values).toEqual({ rows: [{ name: "ok" }, {}] });

    const scoped = validateValues(rowsModel, normalized.values, { paths: ["rows[1].name"] });
    expect(scoped).toEqual([
      { code: "required", path: "rows[1].name", rendererPath: "rows.name", message: "name is required" },
    ]);
    expect(validateValues(rowsModel, normalized.values, { paths: ["rows[0].name"] })).toEqual([]);
    expect(validateValues(rowsModel, normalized.values, { paths: ["rows[2].name"] })).toHaveLength(1);
    const overlapping = validateValues(rowsModel, normalized.values, { paths: ["rows[1]", "rows[1].name", "rows[1]"] });
    expect(overlapping.map((issue) => issue.path)).toEqual(["rows[1].name", "rows[1].future"]);
  });

  it("counts Unicode code points and applies portable patterns", () => {
    const textModel: EmbeddedModel = {
      fields: [
        field({ name: "text", type: "string", validations: [{ kind: "maxLength", params: { value: "2" } }] }),
        field({ name: "slug", type: "string", validations: [{ kind: "pattern", params: { pattern: "^[a-z]+$" } }] }),
        field({ name: "loose", type: "string", validations: [{ kind: "pattern", params: { pattern: "(" } }] }),
      ],
    };
    expect(validateValues(textModel, { text: "é😀" })).toEqual([]);
    expect(validateValues(textModel, { text: "é😀a" }).map((issue) => issue.code)).toEqual(["maxLength"]);
    expect(validateValues(textModel, { text: "   " })).toEqual([]);
    expect(validateValues(textModel, { slug: "abc" })).toEqual([]);
    expect(validateValues(textModel, { slug: "ab1" }).map((issue) => issue.code)).toEqual(["pattern"]);
    expect(validateValues(textModel, { loose: "anything" })).toEqual([]);
  });

  it("keeps the parent per-element enum and item constraints for indexed scopes", () => {
    const model: EmbeddedModel = {
      fields: [
        field({
          name: "colors",
          type: "array",
          enum: ["blue", "green"],
          items: field({ name: "color", type: "string", enum: ["blue", "red"] }),
        }),
      ],
    };
    const table: Array<[string, number]> = [
      ["red", 1],
      ["green", 1],
      ["yellow", 2],
      ["blue", 0],
    ];
    for (const [value, expected] of table) {
      const values: Record<string, JSONValue> = { colors: [value, "blue"] };
      const full = validateValues(model, values);
      const scoped = validateValues(model, values, { paths: ["colors[0]"] });
      expect(scoped).toHaveLength(expected);
      expect(scoped).toEqual(full);
    }
  });

  it("does not validate siblings or array counts when a child is selected", () => {
    const model: EmbeddedModel = {
      fields: [
        field({
          name: "matrix",
          type: "array",
          items: field({
            name: "row",
            type: "array",
            validations: [{ kind: "minItems", params: { value: "3" } }],
            items: field({ name: "cell", type: "string" }),
          }),
        }),
      ],
    };
    expect(validateValues(model, { matrix: [["red", "blue"]] }, { paths: ["matrix[0][0]"] })).toEqual([]);
    expect(validateValues(model, { matrix: [["red", "blue"]] }, { paths: ["matrix[0]"] }).map((issue) => issue.code)).toEqual([
      "minItems",
    ]);
  });

  it("uses typed JSON equality for distinct items", () => {
    const model: EmbeddedModel = {
      fields: [field({ name: "choices", type: "array", validations: [{ kind: "uniqueItems" }] })],
    };
    for (const items of [[false, "false"], [0, "0"], [{ x: false }, { x: "false" }]]) {
      expect(validateValues(model, { choices: items as JSONValue[] })).toEqual([]);
    }
    for (const items of [[1, 1.0], [{ x: 1 }, { x: 1 }], [["a"], ["a"]]]) {
      expect(validateValues(model, { choices: items as JSONValue[] }).map((issue) => issue.code)).toEqual(["uniqueItems"]);
    }
  });

  it("reports empty required arrays through minItems when declared, otherwise required", () => {
    expect(outcome(sharedModel, { enabled: true, level: 4, colors: [] })).toEqual(["minItems:colors"]);
    const plain: EmbeddedModel = { fields: [field({ name: "choices", type: "array", required: true })] };
    expect(validateValues(plain, { choices: [] }).map((issue) => issue.code)).toEqual(["required"]);
    expect(validateValues(plain, {}).map((issue) => issue.code)).toEqual(["required"]);
  });

  it("applies exclusive bounds and typed enum comparisons", () => {
    const model: EmbeddedModel = {
      fields: [
        field({
          name: "score",
          type: "number",
          validations: [{ kind: "min", params: { value: "0", exclusive: "true" } }],
          enum: [0.5, 1, 2],
        }),
      ],
    };
    expect(validateValues(model, { score: 0 }).map((issue) => issue.code).sort()).toEqual(["enum", "min"]);
    expect(validateValues(model, { score: 1 })).toEqual([]);
    expect(validateValues(model, { score: "1" as unknown as JSONValue }).map((issue) => issue.code)).toEqual(["type"]);
  });

  it("uses labels in messages when present", () => {
    const model: EmbeddedModel = { fields: [field({ name: "enabled", type: "boolean", required: true, label: "Enable feature" })] };
    expect(validateValues(model, {})[0].message).toBe("Enable feature is required");
  });
});

describe("embedded values: canonical paths", () => {
  it("parses, formats, and strips indexes", () => {
    const segments = parseCanonicalPath("rows[2].name")!;
    expect(segments).toEqual([
      { name: "rows", index: -1, isIndex: false },
      { name: "", index: 2, isIndex: true },
      { name: "name", index: -1, isIndex: false },
    ]);
    expect(formatCanonicalPath(segments)).toBe("rows[2].name");
    expect(rendererPath("rows[2].name")).toBe("rows.name");
    expect(rendererPath("matrix[0][1]")).toBe("matrix");
    expect(parseCanonicalPath("rows[2]name")).toBeNull();
    expect(parseCanonicalPath("constructor.x")).toBeNull();
    expect(parseCanonicalPath("")).toBeNull();
  });

  it("checks subtree membership", () => {
    expect(pathWithin("rows[1].name", "rows[1]")).toBe(true);
    expect(pathWithin("rows[1].name", "rows")).toBe(true);
    expect(pathWithin("rows[1]", "rows[1]")).toBe(true);
    expect(pathWithin("rowsx", "rows")).toBe(false);
    expect(pathWithin("rows[10]", "rows[1]")).toBe(false);
  });
});
