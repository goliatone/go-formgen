import type {
  FieldConfig,
  FieldValidationRule,
  ValidationError,
  ValidationResult,
} from "./config";

export type ValidationValue = string | string[] | null;

interface ValidationContext {
  field: FieldConfig;
  value: ValidationValue;
}

export function validateFieldValue(field: FieldConfig, value: ValidationValue): ValidationResult {
  const errors: ValidationError[] = [];
  const label = resolveFieldLabel(field);
  const context: ValidationContext = { field, value };

  if (requiresValue(field) && isEmptyValue(value)) {
    errors.push({
      code: "required",
      message: `${label} is required.`,
      value,
    });
    return buildResult(errors);
  }

  const normalized = normalizeValues(value);
  if (field.cardinality === "one" && normalized.length > 1) {
    errors.push({
      code: "cardinality",
      message: `Select only one ${label.toLowerCase()}.`,
      value,
    });
  }

  const rules = field.validations ?? [];
  for (const rule of rules) {
    const error = evaluateRule(rule, context, label);
    if (error) {
      errors.push(error);
    }
  }

  return buildResult(errors);
}

export function mergeValidationResults(
  ...results: Array<ValidationResult | undefined | null>
): ValidationResult {
  const errors: ValidationError[] = [];
  for (const result of results) {
    if (!result || result.valid) {
      continue;
    }
    errors.push(...(result.errors ?? []));
  }
  return buildResult(errors);
}

function evaluateRule(
  rule: FieldValidationRule,
  context: ValidationContext,
  label: string
): ValidationError | null {
  const value = context.value;
  if (isEmptyValue(value)) {
    return null;
  }

  switch (rule.kind) {
    case "min": {
      const threshold = parseNumber(rule.params?.value);
      const numeric = toNumber(value);
      if (threshold == null || numeric == null) {
        return null;
      }
      const exclusive = rule.params?.exclusive === "true";
      if (exclusive ? numeric <= threshold : numeric < threshold) {
        const comparator = exclusive ? "greater than" : "at least";
        return {
          code: "min",
          message: `${label} must be ${comparator} ${threshold}.`,
          rule,
          value,
        };
      }
      break;
    }
    case "max": {
      const threshold = parseNumber(rule.params?.value);
      const numeric = toNumber(value);
      if (threshold == null || numeric == null) {
        return null;
      }
      const exclusive = rule.params?.exclusive === "true";
      if (exclusive ? numeric >= threshold : numeric > threshold) {
        const comparator = exclusive ? "less than" : "no more than";
        return {
          code: "max",
          message: `${label} must be ${comparator} ${threshold}.`,
          rule,
          value,
        };
      }
      break;
    }
    case "minLength": {
      const target = parseNumber(rule.params?.value);
      const text = toStringValue(value);
      if (target == null || text == null) {
        return null;
      }
      if (text.length < target) {
        return {
          code: "minLength",
          message: `${label} must be at least ${target} characters.`,
          rule,
          value,
        };
      }
      break;
    }
    case "maxLength": {
      const target = parseNumber(rule.params?.value);
      const text = toStringValue(value);
      if (target == null || text == null) {
        return null;
      }
      if (text.length > target) {
        return {
          code: "maxLength",
          message: `${label} must be at most ${target} characters.`,
          rule,
          value,
        };
      }
      break;
    }
    case "pattern": {
      const pattern = rule.params?.pattern;
      const text = toStringValue(value);
      if (!pattern || text == null) {
        return null;
      }
      try {
        const regex = new RegExp(pattern);
        if (!regex.test(text)) {
          return {
            code: "pattern",
            message: `Enter a valid ${label.toLowerCase()}.`,
            rule,
            value,
          };
        }
      } catch (_err) {
        return null;
      }
      break;
    }
    default:
      return null;
  }

  return null;
}

function buildResult(errors: ValidationError[]): ValidationResult {
  if (errors.length === 0) {
    return { valid: true, messages: [], errors: [] };
  }
  return {
    valid: false,
    errors,
    messages: errors.map((error) => error.message),
  };
}

function resolveFieldLabel(field: FieldConfig): string {
  if (field.label && field.label.trim() !== "") {
    return field.label.trim();
  }
  if (field.name && field.name.trim() !== "") {
    return field.name.trim();
  }
  return "This field";
}

function requiresValue(field: FieldConfig): boolean {
  return field.required === true;
}

function isEmptyValue(value: ValidationValue): boolean {
  if (value == null) {
    return true;
  }
  if (Array.isArray(value)) {
    return value.length === 0 || value.every((item) => item == null || item === "");
  }
  return String(value).trim() === "";
}

function normalizeValues(value: ValidationValue): string[] {
  if (!value) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.filter((item) => item != null && item !== "");
  }
  return String(value) === "" ? [] : [String(value)];
}

function toNumber(value: ValidationValue): number | null {
  const raw = toStringValue(value);
  if (raw == null || raw.trim() === "") {
    return null;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : null;
}

function toStringValue(value: ValidationValue): string | null {
  if (value == null) {
    return null;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return null;
    }
    return value[0] ?? null;
  }
  return String(value);
}

function parseNumber(input: string | undefined): number | null {
  if (input == null) {
    return null;
  }
  const parsed = Number(input);
  return Number.isFinite(parsed) ? parsed : null;
}
