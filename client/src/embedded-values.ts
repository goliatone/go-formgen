import type {
  EmbeddedDescriptor,
  EmbeddedField,
  EmbeddedIssue,
  EmbeddedModel,
  JSONValue,
  NormalizedValues,
  ValidationScope,
} from "./embedded-contract";

/**
 * Pure, model-aware value normalization and validation for embedded forms.
 *
 * The functions mirror the Go `submission` package for the declared shared
 * subset: typed enum decoding, field-aware coercion, required/type/enum/
 * min/max/minLength/maxLength/pattern/minItems/maxItems/uniqueItems rules,
 * canonical scoped validation, and precise issue paths. They never touch the
 * DOM, start resolver requests, or submit anything. Applicability (conditions)
 * is a separate layer; nothing here evaluates condition rules.
 */

/** Accepts an embedded model, a descriptor envelope, or a serialized FormModel. */
export type EmbeddedModelInput = EmbeddedModel | EmbeddedDescriptor | { form: EmbeddedModel };

export interface NormalizeOptions {
  /** Unknown keys: report an issue (default, like Go), drop silently, or keep untouched. */
  unknownFields?: "issue" | "ignore" | "preserve";
  /** Empty strings: `default` turns optional non-string scalars into null; `preserve` keeps "". */
  emptyStrings?: "default" | "preserve";
  /** Decode native enum tokens (default), or normalize already typed JSON values. */
  enumValues?: "control" | "literal";
}

export interface ValidateOptions {
  /** Report unknown nested object keys (default, like Go) or ignore them. */
  unknownFields?: "issue" | "ignore";
  /** Conditional requiredness overrides keyed by canonical path (Go `opts.required`). */
  required?: Record<string, boolean>;
  /** Inactive subtrees skipped entirely; inactive array members leave effective counts (Go `opts.inactive`). */
  inactive?: string[];
}

export interface PathSegment {
  name: string;
  index: number;
  isIndex: boolean;
}

const CANONICAL_PATH_PATTERN =
  /^[A-Za-z_][A-Za-z0-9_-]*(\[[0-9]+\])*(\.[A-Za-z_][A-Za-z0-9_-]*(\[[0-9]+\])*)*$/;
const UNSAFE_SEGMENTS = new Set(["__proto__", "prototype", "constructor"]);
/** Repeater transport keys rendered next to row controls; they are never answers. */
const STRUCTURAL_KEYS = new Set(["_present", "_row_state", "_row_key", "_delete"]);
const ENUM_PREFIX = "__fg_enum_v1:";

type ResolvedNormalizeOptions = Required<NormalizeOptions>;
interface ResolvedValidateOptions {
  unknownFields: "issue" | "ignore";
  required: Record<string, boolean>;
  inactive: string[];
}

// ---------------------------------------------------------------------------
// Canonical paths
// ---------------------------------------------------------------------------

/**
 * Parse a strict canonical path such as `rows[2].name`. Returns null for any
 * other spelling, non-canonical indexes (`01`, `-1`, `[]`), or unsafe names.
 */
export function parseCanonicalPath(path: string): PathSegment[] | null {
  if (typeof path !== "string" || !CANONICAL_PATH_PATTERN.test(path)) {
    return null;
  }
  const segments: PathSegment[] = [];
  for (const part of path.split(".")) {
    const bracket = part.indexOf("[");
    const name = bracket < 0 ? part : part.slice(0, bracket);
    if (UNSAFE_SEGMENTS.has(name)) {
      return null;
    }
    segments.push({ name, index: -1, isIndex: false });
    let tail = bracket < 0 ? "" : part.slice(bracket);
    while (tail) {
      const end = tail.indexOf("]");
      const raw = tail.slice(1, end);
      const index = Number(raw);
      if (!Number.isSafeInteger(index) || index < 0 || String(index) !== raw) {
        return null;
      }
      segments.push({ name: "", index, isIndex: true });
      tail = tail.slice(end + 1);
    }
  }
  return segments;
}

export function formatCanonicalPath(segments: PathSegment[]): string {
  return segments.reduce((path, segment) => {
    if (segment.isIndex) {
      return `${path}[${segment.index}]`;
    }
    return path ? `${path}.${segment.name}` : segment.name;
  }, "");
}

/** Legacy renderer grouping path: indexes removed, names joined with dots. */
export function rendererPath(path: string): string {
  const segments = parseCanonicalPath(path);
  if (segments) {
    return segments
      .filter((segment) => !segment.isIndex)
      .map((segment) => segment.name)
      .join(".");
  }
  return path
    .replace(/\[[^\]]*\]/g, "")
    .split(".")
    .filter(Boolean)
    .join(".");
}

/** True when `path` equals `parent` or lies inside its subtree. */
export function pathWithin(path: string, parent: string): boolean {
  return path === parent || path.startsWith(`${parent}.`) || path.startsWith(`${parent}[`);
}

// ---------------------------------------------------------------------------
// Model access
// ---------------------------------------------------------------------------

export function resolveEmbeddedModel(input: EmbeddedModelInput | null | undefined): EmbeddedModel {
  if (input && typeof input === "object") {
    const direct = input as EmbeddedModel;
    if (Array.isArray(direct.fields)) {
      return direct;
    }
    const form = (input as { form?: EmbeddedModel }).form;
    if (form && typeof form === "object" && Array.isArray(form.fields)) {
      return form;
    }
  }
  throw new Error("formgen: embedded model requires a complete form with fields");
}

interface FieldResolution {
  field: EmbeddedField;
  /** Immediate array container of an indexed element; its per-element enum is a separate layer. */
  array: EmbeddedField | null;
}

function resolveField(model: EmbeddedModel, segments: PathSegment[]): FieldResolution | null {
  let current: EmbeddedField = { name: "", type: "object", required: false, nested: model.fields };
  let array: EmbeddedField | null = null;
  for (const segment of segments) {
    array = null;
    if (segment.isIndex) {
      if (current.type !== "array") {
        return null;
      }
      array = current;
      current = arrayItemField(current);
      continue;
    }
    if (current.type !== "object") {
      return null;
    }
    const next = (current.nested ?? []).find((field) => field.name === segment.name);
    if (!next) {
      return null;
    }
    current = next;
  }
  return { field: current, array };
}

/** Resolve the field a canonical path addresses, or null for unknown paths. */
export function lookupField(modelInput: EmbeddedModelInput, path: string): EmbeddedField | null {
  const segments = parseCanonicalPath(path);
  if (!segments) {
    return null;
  }
  return resolveField(resolveEmbeddedModel(modelInput), segments)?.field ?? null;
}

function arrayItemField(parent: EmbeddedField): EmbeddedField {
  if (parent.items) {
    return parent.items;
  }
  return { name: parent.name, type: inferEnumType(parent.enum), required: false, enum: parent.enum };
}

function inferEnumType(values: JSONValue[] | undefined): EmbeddedField["type"] {
  for (const value of values ?? []) {
    if (typeof value === "boolean") {
      return "boolean";
    }
    if (typeof value === "number") {
      return Number.isInteger(value) ? "integer" : "number";
    }
  }
  return "string";
}

function isRawObjectField(field: EmbeddedField): boolean {
  return field.type === "object" && (field.nested ?? []).length === 0;
}

// ---------------------------------------------------------------------------
// Typed enum control values
// ---------------------------------------------------------------------------

/** Encode a typed option value the way Go `submission.EncodeEnumValue` does. */
export function encodeEnumValue(value: JSONValue): string {
  let type = "string";
  if (typeof value === "boolean") {
    type = "bool";
  } else if (typeof value === "number") {
    type = Number.isInteger(value) ? "int" : "number";
  } else if (value === null) {
    type = "null";
  } else if (typeof value !== "string") {
    value = JSON.stringify(value);
  }
  return ENUM_PREFIX + base64UrlEncode(JSON.stringify({ t: type, v: value }));
}

/**
 * Decode a typed option value emitted by Go `EncodeEnumValue`. Plain strings
 * and undecodable payloads are returned unchanged with `encoded: false`.
 */
export function decodeEnumValue(raw: string): { value: JSONValue; encoded: boolean } {
  const unchanged = { value: raw, encoded: false };
  if (typeof raw !== "string" || !raw.startsWith(ENUM_PREFIX)) {
    return unchanged;
  }
  let payload: unknown;
  try {
    payload = JSON.parse(base64UrlDecode(raw.slice(ENUM_PREFIX.length)));
  } catch (_err) {
    return unchanged;
  }
  if (!isPlainObject(payload) || typeof payload.t !== "string") {
    return unchanged;
  }
  const value = payload.v;
  switch (payload.t) {
    case "string":
      return typeof value === "string" ? { value, encoded: true } : unchanged;
    case "bool":
      return typeof value === "boolean" ? { value, encoded: true } : unchanged;
    case "int":
    case "uint":
      // Unsafe integers are left encoded so coercion reports them instead of rounding.
      return typeof value === "number" && Number.isSafeInteger(value) ? { value, encoded: true } : unchanged;
    case "number":
      return typeof value === "number" && Number.isFinite(value) ? { value, encoded: true } : unchanged;
    case "null":
      return { value: null, encoded: true };
    default:
      return unchanged;
  }
}

// Self-contained base64url/UTF-8 codec: no dependency on btoa/atob or TextEncoder
// so the same bytes decode in browsers, WKWebView, jsdom, and Node.
const BASE64URL_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

function utf8Encode(text: string): number[] {
  const bytes: number[] = [];
  for (const char of text) {
    const point = char.codePointAt(0) ?? 0;
    if (point < 0x80) {
      bytes.push(point);
    } else if (point < 0x800) {
      bytes.push(0xc0 | (point >> 6), 0x80 | (point & 0x3f));
    } else if (point < 0x10000) {
      bytes.push(0xe0 | (point >> 12), 0x80 | ((point >> 6) & 0x3f), 0x80 | (point & 0x3f));
    } else {
      bytes.push(
        0xf0 | (point >> 18),
        0x80 | ((point >> 12) & 0x3f),
        0x80 | ((point >> 6) & 0x3f),
        0x80 | (point & 0x3f)
      );
    }
  }
  return bytes;
}

function utf8Decode(bytes: number[]): string {
  let out = "";
  for (let position = 0; position < bytes.length; ) {
    const lead = bytes[position];
    let point: number;
    let width: number;
    if (lead < 0x80) {
      point = lead;
      width = 1;
    } else if ((lead & 0xe0) === 0xc0) {
      point = lead & 0x1f;
      width = 2;
    } else if ((lead & 0xf0) === 0xe0) {
      point = lead & 0x0f;
      width = 3;
    } else if ((lead & 0xf8) === 0xf0) {
      point = lead & 0x07;
      width = 4;
    } else {
      throw new Error("invalid utf-8 sequence");
    }
    for (let offset = 1; offset < width; offset += 1) {
      const continuation = bytes[position + offset];
      if (continuation === undefined || (continuation & 0xc0) !== 0x80) {
        throw new Error("invalid utf-8 sequence");
      }
      point = (point << 6) | (continuation & 0x3f);
    }
    out += String.fromCodePoint(point);
    position += width;
  }
  return out;
}

function base64UrlEncode(text: string): string {
  const bytes = utf8Encode(text);
  let out = "";
  for (let position = 0; position < bytes.length; position += 3) {
    const first = bytes[position];
    const second = bytes[position + 1];
    const third = bytes[position + 2];
    const triple = (first << 16) | ((second ?? 0) << 8) | (third ?? 0);
    out += BASE64URL_ALPHABET[(triple >> 18) & 63] + BASE64URL_ALPHABET[(triple >> 12) & 63];
    if (second !== undefined) {
      out += BASE64URL_ALPHABET[(triple >> 6) & 63];
    }
    if (third !== undefined) {
      out += BASE64URL_ALPHABET[triple & 63];
    }
  }
  return out;
}

function base64UrlDecode(encoded: string): string {
  const bytes: number[] = [];
  let buffer = 0;
  let bits = 0;
  for (const char of encoded) {
    if (char === "=") {
      continue;
    }
    let value = BASE64URL_ALPHABET.indexOf(char);
    if (value < 0) {
      value = char === "+" ? 62 : char === "/" ? 63 : -1;
    }
    if (value < 0) {
      throw new Error("invalid base64url input");
    }
    buffer = ((buffer << 6) | value) & 0xffffff;
    bits += 6;
    if (bits >= 8) {
      bits -= 8;
      bytes.push((buffer >> bits) & 0xff);
    }
  }
  return utf8Decode(bytes);
}

// ---------------------------------------------------------------------------
// Normalization (mirrors Go CoerceValue / ParseMap)
// ---------------------------------------------------------------------------

/**
 * Normalize nested raw values (such as `controller.getValues()`) into typed
 * JSON values and parse issues. Absent keys stay absent; present keys keep an
 * explicit null when they coerce to nothing. Values are independent copies.
 */
export function normalizeValues(
  modelInput: EmbeddedModelInput,
  raw: unknown,
  options: NormalizeOptions = {}
): NormalizedValues {
  const model = resolveEmbeddedModel(modelInput);
  const opts: ResolvedNormalizeOptions = {
    unknownFields: options.unknownFields ?? "issue",
    emptyStrings: options.emptyStrings ?? "default",
    enumValues: options.enumValues ?? "control",
  };
  const issues: EmbeddedIssue[] = [];
  const values: Record<string, JSONValue> = {};
  const source = isPlainObject(raw) ? raw : {};
  const fields = new Map(model.fields.map((field) => [field.name, field] as const));

  for (const key of Object.keys(source).sort()) {
    if (UNSAFE_SEGMENTS.has(key)) {
      continue;
    }
    const field = fields.get(key);
    if (!field) {
      if (STRUCTURAL_KEYS.has(key)) {
        continue;
      }
      if (opts.unknownFields === "issue") {
        issues.push(makeIssue("unknownField", key, `unknown field "${key}"`));
      } else if (opts.unknownFields === "preserve") {
        values[key] = cloneJSON(source[key]);
      }
      continue;
    }
    values[key] = coerceValue(field, source[key], opts, key, issues);
  }
  return { values, issues };
}

/** Normalize one resolved leaf before writing a potentially lossy native control. */
export function normalizeFieldValue(
  field: EmbeddedField,
  raw: unknown,
  path: string,
  options: NormalizeOptions = {}
): { value: JSONValue; issues: EmbeddedIssue[] } {
  const issues: EmbeddedIssue[] = [];
  const value = coerceValue(field, raw, {
    unknownFields: options.unknownFields ?? "issue",
    emptyStrings: options.emptyStrings ?? "default",
    enumValues: options.enumValues ?? "control",
  }, path, issues);
  return { value, issues };
}

function coerceValue(
  field: EmbeddedField,
  input: unknown,
  opts: ResolvedNormalizeOptions,
  path: string,
  issues: EmbeddedIssue[]
): JSONValue {
  let value: unknown = input === undefined ? null : input;
  if (typeof value === "string" && opts.enumValues === "control" && !declaredStringValue(field, value)) {
    const decoded = decodeEnumValue(value);
    if (decoded.encoded) {
      value = decoded.value;
    }
  }
  if (value === "") {
    if (opts.emptyStrings === "preserve") {
      return "";
    }
    return field.type === "string" ? "" : null;
  }
  switch (field.type) {
    case "string":
      return coerceString(value);
    case "integer":
      return coerceInteger(value, path, issues);
    case "number":
      return coerceNumber(value, path, issues);
    case "boolean":
      return coerceBoolean(value, path, issues);
    case "array":
      return coerceArray(field, value, opts, path, issues);
    case "object":
      return coerceObject(field, value, opts, path, issues);
    default:
      return cloneJSON(value);
  }
}

function declaredStringValue(field: EmbeddedField, value: string): boolean {
  return (field.enum ?? []).some((candidate) => candidate === value) ||
    (field.options ?? []).some((option) => option.value === value) ||
    (field.type === "array" && !!field.items && declaredStringValue(field.items, value));
}

function coerceString(value: unknown): JSONValue {
  if (value === null) {
    return null;
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value) ?? null;
}

function coerceInteger(value: unknown, path: string, issues: EmbeddedIssue[]): JSONValue {
  if (value === null) {
    return null;
  }
  if (typeof value === "number") {
    if (!Number.isInteger(value)) {
      issues.push(makeIssue("type", path, "expected integer"));
      return null;
    }
    if (!Number.isSafeInteger(value)) {
      issues.push(makeIssue("type", path, "integer exceeds the safe JavaScript integer range"));
      return null;
    }
    return value;
  }
  if (typeof value === "string") {
    const text = value.trim();
    if (/^[+-]?\d+$/.test(text)) {
      const parsed = Number(text);
      if (Number.isSafeInteger(parsed)) {
        return parsed;
      }
      issues.push(makeIssue("type", path, "integer exceeds the safe JavaScript integer range"));
      return null;
    }
  }
  issues.push(makeIssue("type", path, "expected integer"));
  return null;
}

function coerceNumber(value: unknown, path: string, issues: EmbeddedIssue[]): JSONValue {
  if (value === null) {
    return null;
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const text = value.trim();
    const parsed = Number(text);
    if (text !== "" && Number.isFinite(parsed)) {
      return parsed;
    }
  }
  issues.push(makeIssue("type", path, "expected number"));
  return null;
}

function coerceBoolean(value: unknown, path: string, issues: EmbeddedIssue[]): JSONValue {
  if (value === null) {
    return null;
  }
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    switch (value.trim().toLowerCase()) {
      case "true":
      case "1":
      case "on":
      case "yes":
        return true;
      case "false":
      case "0":
      case "off":
      case "no":
        return false;
      default:
        break;
    }
  }
  issues.push(makeIssue("type", path, "expected boolean"));
  return null;
}

function coerceArray(
  field: EmbeddedField,
  value: unknown,
  opts: ResolvedNormalizeOptions,
  path: string,
  issues: EmbeddedIssue[]
): JSONValue {
  if (value === null) {
    return null;
  }
  let items: unknown[];
  if (Array.isArray(value)) {
    items = value;
  } else if (isPlainObject(value) && isIndexedObject(value)) {
    // Raw DOM collection represents rows as objects keyed by index.
    items = indexedObjectToArray(value);
  } else {
    items = [value];
  }
  const itemField = arrayItemField(field);
  return items.map((item, position) => coerceValue(itemField, item, opts, `${path}[${position}]`, issues));
}

function coerceObject(
  field: EmbeddedField,
  value: unknown,
  opts: ResolvedNormalizeOptions,
  path: string,
  issues: EmbeddedIssue[]
): JSONValue {
  if (value === null) {
    return null;
  }
  if (isRawObjectField(field)) {
    if (isPlainObject(value)) {
      return cloneJSON(value);
    }
    if (typeof value === "string") {
      let decoded: unknown;
      try {
        decoded = JSON.parse(value);
      } catch (_err) {
        issues.push(makeIssue("invalidJSON", path, "invalid JSON object"));
        return null;
      }
      if (!isPlainObject(decoded)) {
        issues.push(makeIssue("object", path, "expected object"));
        return null;
      }
      return cloneJSON(decoded);
    }
    issues.push(makeIssue("object", path, "expected object"));
    return null;
  }
  if (!isPlainObject(value)) {
    issues.push(makeIssue("object", path, "expected object"));
    return null;
  }
  const out: Record<string, JSONValue> = {};
  const nested = new Map((field.nested ?? []).map((child) => [child.name, child] as const));
  for (const key of Object.keys(value).sort()) {
    if (UNSAFE_SEGMENTS.has(key) || STRUCTURAL_KEYS.has(key)) {
      continue;
    }
    const child = nested.get(key);
    const childPath = `${path}.${key}`;
    if (!child) {
      if (opts.unknownFields === "issue") {
        issues.push(makeIssue("unknownField", childPath, `unknown field "${childPath}"`));
      } else if (opts.unknownFields === "preserve") {
        out[key] = cloneJSON(value[key]);
      }
      continue;
    }
    out[key] = coerceValue(child, value[key], opts, childPath, issues);
  }
  return out;
}

// ---------------------------------------------------------------------------
// Validation (mirrors Go Validate / ValidatePaths)
// ---------------------------------------------------------------------------

/**
 * Validate normalized values. Without a scope the whole form is validated.
 * `scope.paths` selects canonical subtrees: an empty list validates nothing,
 * a container includes its subtree, and a child never validates siblings or
 * parent cardinality. Invalid or unknown scope paths report issues.
 */
export function validateValues(
  modelInput: EmbeddedModelInput,
  values: Record<string, JSONValue> | null | undefined,
  scope?: ValidationScope | null,
  options: ValidateOptions = {}
): EmbeddedIssue[] {
  const model = resolveEmbeddedModel(modelInput);
  const opts: ResolvedValidateOptions = {
    unknownFields: options.unknownFields ?? "issue",
    required: options.required ?? {},
    inactive: options.inactive ?? [],
  };
  const issues: EmbeddedIssue[] = [];
  const source: Record<string, unknown> = isPlainObject(values) ? values : {};

  if (!scope || scope.paths === undefined) {
    for (const field of model.fields) {
      validateField(field, source[field.name], hasOwn(source, field.name), field.name, opts, issues);
    }
    return issues;
  }

  const selected: string[] = [];
  for (const path of scope.paths) {
    const segments = parseCanonicalPath(path);
    if (!segments) {
      issues.push(makeIssue("invalidPath", String(path), "invalid canonical field path"));
      continue;
    }
    if (!resolveField(model, segments)) {
      issues.push(makeIssue("unknownField", path, "unknown validation field"));
      continue;
    }
    selected.push(path);
  }

  selected.forEach((path, position) => {
    const covered = selected.some(
      (other, otherPosition) =>
        position !== otherPosition && pathWithin(path, other) && (path !== other || otherPosition < position)
    );
    if (covered) {
      return;
    }
    const segments = parseCanonicalPath(path)!;
    const resolved = resolveField(model, segments)!;
    let current: unknown = source;
    let exists = true;
    for (const segment of segments) {
      const next = descend(current, segment);
      if (!next.exists) {
        exists = false;
        current = undefined;
        break;
      }
      current = next.value;
    }
    // An indexed item keeps both the array's per-element enum and its own constraints.
    if (resolved.array && resolved.array.items) {
      validateEnum(resolved.array, exists ? current : null, path, issues);
    }
    validateField(resolved.field, current, exists, path, opts, issues);
  });
  return issues;
}

function descend(value: unknown, segment: PathSegment): { value: unknown; exists: boolean } {
  if (segment.isIndex) {
    if (!Array.isArray(value) || segment.index >= value.length) {
      return { value: undefined, exists: false };
    }
    return { value: value[segment.index], exists: true };
  }
  if (!isPlainObject(value)) {
    return { value: undefined, exists: false };
  }
  return { value: value[segment.name], exists: hasOwn(value, segment.name) };
}

function inactivePath(path: string, opts: ResolvedValidateOptions): boolean {
  return opts.inactive.some((inactive) => pathWithin(path, inactive));
}

function validateField(
  field: EmbeddedField,
  value: unknown,
  exists: boolean,
  path: string,
  opts: ResolvedValidateOptions,
  issues: EmbeddedIssue[]
): void {
  if (inactivePath(path, opts)) {
    return;
  }
  if (opts.required[path]) {
    field = { ...field, required: true };
  }
  if (!exists || value === null || value === undefined) {
    if (field.required) {
      issues.push(makeIssue("required", path, message(field, path, "is required")));
    }
    return;
  }
  if (isEmpty(value) && field.type !== "array") {
    if (field.required) {
      issues.push(makeIssue("required", path, message(field, path, "is required")));
    }
    return;
  }
  switch (field.type) {
    case "string":
      validateStringField(field, value, path, issues);
      return;
    case "integer":
      validateIntegerField(field, value, path, issues);
      return;
    case "number":
      validateNumberField(field, value, path, issues);
      return;
    case "boolean":
      validateBooleanField(field, value, path, issues);
      return;
    case "array":
      validateArrayField(field, value, path, opts, issues);
      return;
    case "object":
      validateObjectField(field, value, path, opts, issues);
      return;
    default:
      return;
  }
}

function validateStringField(field: EmbeddedField, value: unknown, path: string, issues: EmbeddedIssue[]): void {
  if (typeof value !== "string") {
    issues.push(makeIssue("type", path, message(field, path, "must be a string")));
    return;
  }
  validateEnum(field, value, path, issues);
  for (const rule of field.validations ?? []) {
    switch (rule.kind) {
      case "minLength": {
        const limit = ruleInt(rule.params);
        if (limit !== null && codePoints(value) < limit) {
          issues.push(makeIssue("minLength", path, message(field, path, `must be at least ${limit} characters`)));
        }
        break;
      }
      case "maxLength": {
        const limit = ruleInt(rule.params);
        if (limit !== null && codePoints(value) > limit) {
          issues.push(makeIssue("maxLength", path, message(field, path, `must be at most ${limit} characters`)));
        }
        break;
      }
      case "pattern": {
        const pattern = (rule.params?.pattern ?? "").trim();
        if (!pattern) {
          break;
        }
        let matcher: RegExp | null = null;
        try {
          matcher = new RegExp(pattern);
        } catch (_err) {
          matcher = null;
        }
        if (matcher && !matcher.test(value)) {
          issues.push(makeIssue("pattern", path, message(field, path, "does not match the required pattern")));
        }
        break;
      }
      default:
        break;
    }
  }
}

function validateIntegerField(field: EmbeddedField, value: unknown, path: string, issues: EmbeddedIssue[]): void {
  if (typeof value !== "number" || !Number.isInteger(value)) {
    issues.push(makeIssue("type", path, message(field, path, "must be an integer")));
    return;
  }
  validateEnum(field, value, path, issues);
  validateNumberRules(field, value, path, issues);
}

function validateNumberField(field: EmbeddedField, value: unknown, path: string, issues: EmbeddedIssue[]): void {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    issues.push(makeIssue("type", path, message(field, path, "must be a number")));
    return;
  }
  validateEnum(field, value, path, issues);
  validateNumberRules(field, value, path, issues);
}

function validateBooleanField(field: EmbeddedField, value: unknown, path: string, issues: EmbeddedIssue[]): void {
  if (typeof value !== "boolean") {
    issues.push(makeIssue("type", path, message(field, path, "must be a boolean")));
    return;
  }
  validateEnum(field, value, path, issues);
}

function validateArrayField(
  field: EmbeddedField,
  value: unknown,
  path: string,
  opts: ResolvedValidateOptions,
  issues: EmbeddedIssue[]
): void {
  if (!Array.isArray(value)) {
    issues.push(makeIssue("type", path, message(field, path, "must be an array")));
    return;
  }
  // Effective constraints count applicable members; item issues keep fixed indexes.
  const effective =
    opts.inactive.length === 0 ? value : value.filter((_item, position) => !inactivePath(`${path}[${position}]`, opts));
  const before = issues.length;
  validateArrayRules(field, effective, path, issues);
  if (effective.length === 0 && field.required && issues.length === before) {
    issues.push(makeIssue("required", path, message(field, path, "is required")));
  }
  if ((field.enum ?? []).length > 0) {
    value.forEach((item, position) => {
      const itemPath = `${path}[${position}]`;
      if (!inactivePath(itemPath, opts)) {
        validateEnum(field, item, itemPath, issues);
      }
    });
  }
  if (field.items) {
    value.forEach((item, position) => validateField(field.items!, item, true, `${path}[${position}]`, opts, issues));
  }
}

function validateObjectField(
  field: EmbeddedField,
  value: unknown,
  path: string,
  opts: ResolvedValidateOptions,
  issues: EmbeddedIssue[]
): void {
  if (!isPlainObject(value)) {
    issues.push(makeIssue("object", path, message(field, path, "must be an object")));
    return;
  }
  if (isRawObjectField(field)) {
    return;
  }
  const nested = field.nested ?? [];
  for (const child of nested) {
    validateField(child, value[child.name], hasOwn(value, child.name), `${path}.${child.name}`, opts, issues);
  }
  if (opts.unknownFields === "issue") {
    const known = new Set(nested.map((child) => child.name));
    for (const key of Object.keys(value)) {
      if (known.has(key) || STRUCTURAL_KEYS.has(key)) {
        continue;
      }
      const unknownPath = `${path}.${key}`;
      issues.push(makeIssue("unknownField", unknownPath, `unknown field "${unknownPath}"`));
    }
  }
}

function validateEnum(field: EmbeddedField, value: unknown, path: string, issues: EmbeddedIssue[]): void {
  const candidates = field.enum ?? [];
  if (candidates.length === 0 || value === null || value === undefined) {
    return;
  }
  if (candidates.some((candidate) => enumValueEqual(candidate, value))) {
    return;
  }
  issues.push(makeIssue("enum", path, message(field, path, "must be one of the allowed values")));
}

function validateNumberRules(field: EmbeddedField, value: number, path: string, issues: EmbeddedIssue[]): void {
  for (const rule of field.validations ?? []) {
    if (rule.kind !== "min" && rule.kind !== "max") {
      continue;
    }
    const limit = ruleFloat(rule.params);
    if (limit === null) {
      continue;
    }
    const exclusive = (rule.params?.exclusive ?? "").toLowerCase() === "true";
    if (rule.kind === "min" && ((!exclusive && value < limit) || (exclusive && value <= limit))) {
      issues.push(makeIssue("min", path, message(field, path, `must be at least ${limit}`)));
    }
    if (rule.kind === "max" && ((!exclusive && value > limit) || (exclusive && value >= limit))) {
      issues.push(makeIssue("max", path, message(field, path, `must be at most ${limit}`)));
    }
  }
}

function validateArrayRules(field: EmbeddedField, value: unknown[], path: string, issues: EmbeddedIssue[]): void {
  for (const rule of field.validations ?? []) {
    switch (rule.kind) {
      case "uniqueItems": {
        const duplicate = value.some((item, position) =>
          value.slice(0, position).some((earlier) => jsonValueEqual(item, earlier))
        );
        if (duplicate) {
          issues.push(makeIssue("uniqueItems", path, message(field, path, "must contain distinct items")));
        }
        break;
      }
      case "minItems":
      case "minLength": {
        const limit = ruleInt(rule.params);
        if (limit !== null && value.length < limit) {
          issues.push(makeIssue("minItems", path, message(field, path, `must contain at least ${limit} items`)));
        }
        break;
      }
      case "maxItems":
      case "maxLength": {
        const limit = ruleInt(rule.params);
        if (limit !== null && value.length > limit) {
          issues.push(makeIssue("maxItems", path, message(field, path, `must contain at most ${limit} items`)));
        }
        break;
      }
      default:
        break;
    }
  }
}

// ---------------------------------------------------------------------------
// Equality, helpers
// ---------------------------------------------------------------------------

function enumValueEqual(candidate: unknown, value: unknown): boolean {
  if (typeof candidate === "number" && typeof value === "number") {
    return candidate === value;
  }
  return kindOf(candidate) === kindOf(value) && String(candidate) === String(value);
}

export function jsonValueEqual(a: unknown, b: unknown): boolean {
  if (typeof a === "number" && typeof b === "number") {
    return a === b;
  }
  if (Array.isArray(a)) {
    return Array.isArray(b) && a.length === b.length && a.every((item, position) => jsonValueEqual(item, b[position]));
  }
  if (isPlainObject(a)) {
    if (!isPlainObject(b)) {
      return false;
    }
    const keys = Object.keys(a);
    if (keys.length !== Object.keys(b).length) {
      return false;
    }
    return keys.every((key) => hasOwn(b, key) && jsonValueEqual(a[key], b[key]));
  }
  return a === b;
}

function kindOf(value: unknown): string {
  return value === null ? "null" : typeof value;
}

function isEmpty(value: unknown): boolean {
  if (value === null || value === undefined) {
    return true;
  }
  if (typeof value === "string") {
    return value.trim() === "";
  }
  if (Array.isArray(value)) {
    return value.length === 0;
  }
  return false;
}

function codePoints(text: string): number {
  return Array.from(text).length;
}

function ruleInt(params: Record<string, string> | undefined): number | null {
  const raw = params?.value;
  if (raw === undefined) {
    return null;
  }
  const parsed = Number(raw.trim());
  return Number.isInteger(parsed) && /^[+-]?\d+$/.test(raw.trim()) ? parsed : null;
}

function ruleFloat(params: Record<string, string> | undefined): number | null {
  const raw = params?.value;
  if (raw === undefined || raw.trim() === "") {
    return null;
  }
  const parsed = Number(raw.trim());
  return Number.isFinite(parsed) ? parsed : null;
}

function message(field: EmbeddedField, path: string, detail: string): string {
  const label = (field.label ?? "").trim() || (field.name ?? "").trim() || path;
  return detail ? `${label} ${detail}` : label;
}

function makeIssue(code: string, path: string, text: string): EmbeddedIssue {
  return { code, path, rendererPath: rendererPath(path), message: text };
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasOwn(target: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(target, key);
}

function isIndexedObject(value: Record<string, unknown>): boolean {
  const keys = Object.keys(value);
  return keys.length > 0 && keys.every((key) => /^\d+$/.test(key) && String(Number(key)) === key);
}

function indexedObjectToArray(value: Record<string, unknown>): unknown[] {
  const out: unknown[] = [];
  for (const key of Object.keys(value)) {
    out[Number(key)] = value[key];
  }
  for (let position = 0; position < out.length; position += 1) {
    if (!(position in out)) {
      out[position] = null;
    }
  }
  return out;
}

/** Deep-copy JSON-compatible data; non-JSON values become null. */
export function cloneJSON(value: unknown): JSONValue {
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof value === "string" || typeof value === "boolean") {
    return value;
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : null;
  }
  if (Array.isArray(value)) {
    return value.map((item) => cloneJSON(item));
  }
  if (isPlainObject(value)) {
    const out: Record<string, JSONValue> = {};
    for (const key of Object.keys(value)) {
      // Opaque JSON members are data, including names that are forbidden in
      // address paths. Define own properties so __proto__ cannot invoke a setter.
      Object.defineProperty(out, key, { value: cloneJSON(value[key]), enumerable: true, configurable: true, writable: true });
    }
    return out;
  }
  return null;
}
