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
