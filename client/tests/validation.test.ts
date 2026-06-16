import { describe, it, expect } from "vitest";
import { validateFieldValue, mergeValidationResults } from "../src/validation";
import type { FieldConfig, ValidationResult } from "../src/config";

describe("validation helpers", () => {
  it("flags required fields without values", () => {
    const field: FieldConfig = {
      name: "title",
      label: "Title",
      required: true,
    };

    const result = validateFieldValue(field, null);
    expect(result.valid).toBe(false);
    expect(result.errors[0]?.code).toBe("required");
    expect(result.messages[0]).toContain("Title is required");
  });

  it("validates minLength rules", () => {
    const field: FieldConfig = {
      name: "slug",
      label: "Slug",
      validations: [{ kind: "minLength", params: { value: "3" } }],
    };

    const short = validateFieldValue(field, "ab");
    expect(short.valid).toBe(false);
    expect(short.errors[0]?.code).toBe("minLength");

    const ok = validateFieldValue(field, "abc");
    expect(ok.valid).toBe(true);
  });

  it("validates array cardinality rules", () => {
    const field: FieldConfig = {
      name: "columns",
      label: "Columns",
      validations: [
        { kind: "minItems", params: { value: "2" } },
        { kind: "maxItems", params: { value: "3" } },
      ],
    };

    const tooFew = validateFieldValue(field, ["left"]);
    expect(tooFew.valid).toBe(false);
    expect(tooFew.errors[0]?.code).toBe("minItems");

    const empty = validateFieldValue(field, []);
    expect(empty.valid).toBe(false);
    expect(empty.errors[0]?.code).toBe("minItems");

    const requiredEmpty = validateFieldValue({ ...field, required: true }, []);
    expect(requiredEmpty.valid).toBe(false);
    expect(requiredEmpty.errors[0]?.code).toBe("minItems");

    const tooMany = validateFieldValue(field, ["a", "b", "c", "d"]);
    expect(tooMany.valid).toBe(false);
    expect(tooMany.errors[0]?.code).toBe("maxItems");

    const ok = validateFieldValue(field, ["a", "b"]);
    expect(ok.valid).toBe(true);
  });

  it("merges validation results", () => {
    const valid: ValidationResult = { valid: true, messages: [], errors: [] };
    const invalid = validateFieldValue(
      { name: "status", label: "Status", required: true },
      null
    );
    const merged = mergeValidationResults(valid, invalid);
    expect(merged.valid).toBe(false);
    expect(merged.errors.length).toBe(invalid.errors.length);
  });
});
