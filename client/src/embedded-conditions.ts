import type {
  ConditionProjection,
  ConditionRule,
  DefinitionDiagnostic,
  EmbeddedField,
  EmbeddedIssue,
  EmbeddedModel,
  JSONValue,
  ValidationScope,
} from "./embedded-contract";
import {
  cloneJSON,
  lookupField,
  parseCanonicalPath,
  pathWithin,
  rendererPath,
  resolveEmbeddedModel,
  validateValues,
  type EmbeddedModelInput,
  type PathSegment,
} from "./embedded-values";

/**
 * Pure evaluator for the `formgen.conditions/v1` contract. It mirrors the Go
 * `conditions` package: the same diagnostics, the same topological gating, the
 * same clear/retain projection with fixed indexes preserved as null holes, and
 * the same server composition (`validateConditional`). Nothing here touches the
 * DOM or legacy visibility.
 */

export const CONDITIONS_VERSION = "formgen.conditions/v1";
const MAX_EXPRESSION_DEPTH = 64;

export type InactiveInputPolicy = "reject" | "normalize";

interface RawExpression {
  op?: unknown;
  path?: unknown;
  value?: unknown;
  args?: unknown;
}

interface RawRule {
  path?: unknown;
  applicableWhen?: unknown;
  requiredWhen?: unknown;
  onInapplicable?: unknown;
}

export interface CompiledRule {
  rule: ConditionRule;
  /** Static requiredness of the target field, captured at compile time. */
  required: boolean;
}

export interface ConditionProgram {
  readonly rules: readonly CompiledRule[];
  /** Project already normalized typed values. The input is never mutated. */
  evaluate(values: Record<string, JSONValue> | null | undefined, inactivePaths?: readonly string[]): ConditionProjection;
}

export interface CompileResult {
  program: ConditionProgram | null;
  diagnostics: DefinitionDiagnostic[];
}

export interface ConditionalValidation {
  /** Condition, inactive-input, and validation issues (parse issues are the caller's). */
  issues: EmbeddedIssue[];
  /** Effective projection, or null when the definition did not compile. */
  projection: ConditionProjection | null;
  diagnostics: DefinitionDiagnostic[];
}

export interface ValidateConditionalOptions {
  scope?: ValidationScope | null;
  /** Supplied inactive inputs: report `inactiveInput` (default) or drop silently. */
  policy?: InactiveInputPolicy;
  /** Reuse a previous compilation of the same model. */
  compiled?: CompileResult;
  /** Trusted structural exclusions (for example deleted rows), never authorization from answers. */
  inactivePaths?: readonly string[];
}

// ---------------------------------------------------------------------------
// Compilation
// ---------------------------------------------------------------------------

export function compileConditions(modelInput: EmbeddedModelInput): CompileResult {
  const model = resolveEmbeddedModel(modelInput);
  const conditions = model.conditions as { version?: unknown; rules?: unknown } | undefined;
  if (conditions == null) {
    return { program: new Program([]), diagnostics: [] };
  }
  const compiler = new Compiler(model);
  if (!compiler.checkProperties(conditions, ["version", "rules"], "conditions")) {
    return { program: null, diagnostics: compiler.diagnostics };
  }
  if (conditions.version !== CONDITIONS_VERSION) {
    compiler.add("conditionVersion", "conditions.version", "unsupported condition contract version");
    return { program: null, diagnostics: compiler.diagnostics };
  }
  if (conditions.rules != null && !Array.isArray(conditions.rules)) {
    compiler.add("conditionRules", "conditions.rules", "condition rules must be an array");
    return { program: null, diagnostics: compiler.diagnostics };
  }
  const rules = (conditions.rules ?? []) as RawRule[];
  for (const rule of rules) {
    compiler.collectRule(rule);
  }
  if (compiler.diagnostics.length > 0) {
    return { program: null, diagnostics: compiler.diagnostics };
  }
  const ordered = compiler.orderedRules();
  if (compiler.diagnostics.length > 0) {
    return { program: null, diagnostics: compiler.diagnostics };
  }
  return { program: new Program(ordered), diagnostics: [] };
}

class Compiler {
  readonly diagnostics: DefinitionDiagnostic[] = [];
  private readonly rules = new Map<string, ConditionRule>();
  private readonly refs = new Map<string, string[]>();

  constructor(private readonly model: EmbeddedModel) {}

  add(code: string, path: string, message: string): void {
    this.diagnostics.push({ code, path, message });
  }

  /** Validate the serialized AST shape before semantic checks or copying. */
  checkProperties(raw: unknown, allowed: string[], path: string): boolean {
    if (!isPlainObject(raw)) {
      this.add("conditionShape", path, "condition node must be an object");
      return false;
    }
    for (const key of Object.keys(raw)) {
      if (!allowed.includes(key)) this.add("conditionShape", path, "unexpected condition property");
    }
    return true;
  }

  collectRule(raw: RawRule): void {
    const before = this.diagnostics.length;
    if (!this.checkProperties(raw, ["path", "applicableWhen", "requiredWhen", "onInapplicable"], "conditions.rules")) return;
    const path = typeof raw?.path === "string" ? raw.path : "";
    if (!parseCanonicalPath(path) || !lookupField(this.model, path)) {
      this.add("conditionPath", path, "condition target must be a known canonical field path");
    }
    if (this.rules.has(path)) {
      this.add("conditionDuplicate", path, "only one condition rule is allowed per path");
    }
    const policy = raw?.onInapplicable;
    if (policy != null && policy !== "" && policy !== "clear" && policy !== "retain") {
      this.add("conditionPolicy", path, "onInapplicable must be clear or retain");
    }
    if (raw?.applicableWhen == null && raw?.requiredWhen == null) {
      this.add("conditionExpression", path, "condition rule requires an expression");
    }
    for (const expression of [raw?.applicableWhen, raw?.requiredWhen]) {
      if (expression != null) {
        this.checkExpression(expression, path, 0);
      }
    }
    if (this.diagnostics.length !== before) {
      // Keep target identity for duplicate diagnostics, but never copy invalid
      // or excessively deep trees (including cyclic non-JSON host objects).
      this.rules.set(path, { path });
      return;
    }
    const rule: ConditionRule = {
      path,
      applicableWhen: raw?.applicableWhen == null ? undefined : (cloneJSON(raw.applicableWhen) as unknown as ConditionRule["applicableWhen"]),
      requiredWhen: raw?.requiredWhen == null ? undefined : (cloneJSON(raw.requiredWhen) as unknown as ConditionRule["requiredWhen"]),
      onInapplicable: policy === "retain" ? "retain" : "clear",
    };
    this.rules.set(path, rule);
  }

  private checkExpression(expression: unknown, target: string, depth: number): void {
    if (depth > MAX_EXPRESSION_DEPTH) {
      this.add("conditionExpression", target, "condition nesting exceeds 64 levels");
      return;
    }
    if (!isPlainObject(expression)) {
      this.add("conditionExpression", target, "condition expression must be an object");
      return;
    }
    this.checkProperties(expression, ["op", "path", "value", "args"], target);
    const raw = expression as RawExpression;
    if (raw.args != null && !Array.isArray(raw.args)) {
      this.add("conditionShape", target, "condition args must be an array");
      return;
    }
    if (raw.path != null && typeof raw.path !== "string") {
      this.add("conditionShape", target, "condition path must be a string");
      return;
    }
    switch (raw.op) {
      case "present":
      case "eq":
        this.checkLeaf(raw, target);
        return;
      case "and":
      case "or": {
        const args = Array.isArray(raw.args) ? raw.args : [];
        if (args.length === 0 || (raw.path != null && raw.path !== "") || (raw.value !== undefined && raw.value !== null)) {
          this.add("conditionExpression", target, "and/or require nonempty args and no path or value");
        }
        for (const arg of args) {
          this.checkExpression(arg, target, depth + 1);
        }
        return;
      }
      default:
        this.add("conditionOperator", target, "unsupported condition operator");
    }
  }

  private checkLeaf(raw: RawExpression, target: string): void {
    const path = typeof raw.path === "string" ? raw.path : "";
    const field = parseCanonicalPath(path) ? lookupField(this.model, path) : null;
    if (!field) {
      this.add("conditionReference", target, "condition reference must be a known canonical field path");
    }
    const refs = this.refs.get(target) ?? [];
    refs.push(path);
    this.refs.set(target, refs);
    if (Array.isArray(raw.args) && raw.args.length > 0) {
      this.add("conditionExpression", target, "leaf expressions cannot contain args");
    }
    if (raw.op === "present" && raw.value !== undefined && raw.value !== null) {
      this.add("conditionExpression", target, "present cannot contain a value");
    }
    if (raw.op !== "eq") {
      return;
    }
    if (!isScalarLiteral(raw.value)) {
      this.add("conditionExpression", target, "eq requires a non-null scalar literal");
    }
    if (field && (field.type === "array" || field.type === "object")) {
      this.add("conditionExpression", target, "eq references must be scalar fields");
    }
  }

  private dependencies(paths: string[]): Map<string, Set<string>> {
    const deps = new Map<string, Set<string>>();
    for (const path of paths) {
      const gates = new Set<string>();
      for (const gate of paths) {
        if (gate !== path && pathWithin(path, gate)) {
          gates.add(gate);
        }
        for (const ref of this.refs.get(path) ?? []) {
          if (pathWithin(ref, gate) || pathWithin(gate, ref)) {
            gates.add(gate);
          }
        }
      }
      deps.set(path, gates);
    }
    return deps;
  }

  orderedRules(): CompiledRule[] {
    const paths = Array.from(this.rules.keys()).sort();
    const deps = this.dependencies(paths);
    const visited = new Set<string>();
    const out: CompiledRule[] = [];
    while (visited.size < paths.length) {
      let progressed = false;
      for (const path of paths) {
        if (visited.has(path)) {
          continue;
        }
        const gates = deps.get(path) ?? new Set<string>();
        if (!Array.from(gates).every((gate) => visited.has(gate))) {
          continue;
        }
        const field = lookupField(this.model, path);
        out.push({ rule: this.rules.get(path)!, required: field?.required === true });
        visited.add(path);
        progressed = true;
      }
      if (!progressed) {
        for (const path of paths) {
          if (!visited.has(path)) {
            this.add("conditionCycle", path, "cyclic condition or parent dependency");
          }
        }
        break;
      }
    }
    return out;
  }
}

function isScalarLiteral(value: unknown): boolean {
  return typeof value === "string" || typeof value === "boolean" || (typeof value === "number" && Number.isFinite(value));
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

class Program implements ConditionProgram {
  constructor(readonly rules: readonly CompiledRule[]) {}

  evaluate(values: Record<string, JSONValue> | null | undefined, inactivePaths: readonly string[] = []): ConditionProjection {
    const out: ConditionProjection = {
      values: isPlainObject(values) ? (cloneJSON(values) as Record<string, JSONValue>) : {},
      retained: {},
      applicable: {},
      required: {},
    };
    for (const path of inactivePaths) {
      if (!parseCanonicalPath(path)) continue;
      out.applicable[path] = false;
      removeValueAt(out.values, path);
    }
    // Retaining a container must not bring a structurally deleted descendant
    // back from the original input after exclusion.
    const source: Record<string, unknown> = inactivePaths.length > 0
      ? cloneJSON(out.values) as Record<string, JSONValue>
      : isPlainObject(values) ? values : {};
    for (const entry of this.rules) {
      const rule = entry.rule;
      let active = indexesExist(out.values, rule.path);
      for (const [parent, parentActive] of Object.entries(out.applicable)) {
        if (!parentActive && pathWithin(rule.path, parent)) {
          active = false;
          break;
        }
      }
      active = active && (!rule.applicableWhen || evaluateExpression(rule.applicableWhen, out.values));
      out.applicable[rule.path] = active;
      out.required[rule.path] =
        active && (entry.required || (!!rule.requiredWhen && evaluateExpression(rule.requiredWhen, out.values)));
      if (!active) {
        if (rule.onInapplicable === "retain" && !inactivePaths.some((path) => pathWithin(rule.path, path))) {
          const found = getValueAt(source, rule.path);
          if (found.exists) {
            out.retained[rule.path] = cloneJSON(found.value);
          }
        }
        removeValueAt(out.values, rule.path);
      }
    }
    return out;
  }
}

/** Evaluate an expression over typed values. Absent and null never satisfy `present` or `eq`. */
export function evaluateExpression(expression: unknown, values: Record<string, unknown>): boolean {
  if (!isPlainObject(expression)) {
    return false;
  }
  const raw = expression as RawExpression;
  switch (raw.op) {
    case "present": {
      const found = getValueAt(values, typeof raw.path === "string" ? raw.path : "");
      return found.exists && found.value !== null && found.value !== undefined;
    }
    case "eq": {
      const found = getValueAt(values, typeof raw.path === "string" ? raw.path : "");
      return found.exists && found.value !== null && found.value !== undefined && valuesEqual(found.value, raw.value);
    }
    case "and":
      return Array.isArray(raw.args) && raw.args.every((arg) => evaluateExpression(arg, values));
    case "or":
      return Array.isArray(raw.args) && raw.args.some((arg) => evaluateExpression(arg, values));
    default:
      return false;
  }
}

function valuesEqual(a: unknown, b: unknown): boolean {
  if (typeof a === "number" || typeof b === "number") {
    return typeof a === "number" && typeof b === "number" && a === b;
  }
  if (Array.isArray(a) || Array.isArray(b)) {
    return Array.isArray(a) && Array.isArray(b) && a.length === b.length && a.every((item, i) => valuesEqual(item, b[i]));
  }
  if (isPlainObject(a) || isPlainObject(b)) {
    if (!isPlainObject(a) || !isPlainObject(b)) {
      return false;
    }
    const keys = Object.keys(a);
    return keys.length === Object.keys(b).length && keys.every((key) => hasOwn(b, key) && valuesEqual(a[key], b[key]));
  }
  return a === b;
}

// ---------------------------------------------------------------------------
// Server-equivalent composition
// ---------------------------------------------------------------------------

/**
 * Mirror of Go `submission.ValidateConditional`: project the typed values,
 * report supplied inactive inputs by policy, validate the effective values
 * with conditional requiredness, and add conditionally required answers that
 * are missing from existing containers. Invalid definitions yield `condition`
 * issues plus plain validation of the unprojected values.
 */
export function validateConditional(
  modelInput: EmbeddedModelInput,
  values: Record<string, JSONValue> | null | undefined,
  options: ValidateConditionalOptions = {}
): ConditionalValidation {
  const model = resolveEmbeddedModel(modelInput);
  const compiled = options.compiled ?? compileConditions(model);
  const scope = options.scope ?? undefined;
  const policy = options.policy ?? "reject";
  const issues: EmbeddedIssue[] = [];
  const source: Record<string, JSONValue> = isPlainObject(values) ? (values as Record<string, JSONValue>) : {};

  if (!compiled.program) {
    for (const diagnostic of compiled.diagnostics) {
      issues.push(issue("condition", diagnostic.path, diagnostic.message));
    }
    issues.push(...validateValues(model, source, scope));
    return { issues, projection: null, diagnostics: compiled.diagnostics };
  }

  const projection = compiled.program.evaluate(source, options.inactivePaths);
  const inactive = Object.keys(projection.applicable)
    .filter((path) => !projection.applicable[path])
    .sort();
  if (policy === "reject") {
    for (const path of inactive) {
      if (!scopeOverlaps(path, scope)) {
        continue;
      }
      if (suppliedInactive(source, path)) {
        issues.push(issue("inactiveInput", path, "input is not applicable"));
      }
    }
  }
  const validateOptions = { required: projection.required, inactive };
  issues.push(...validateValues(model, projection.values, scope, validateOptions));
  issues.push(...missingConditionalIssues(model, projection, issues, scope, validateOptions));
  return { issues, projection, diagnostics: [] };
}

function suppliedInactive(values: Record<string, unknown>, path: string): boolean {
  const found = getValueAt(values, path);
  // Indexed nulls preserve fixed coordinates after clear; they carry no answer.
  return found.exists && (found.value !== null || !path.endsWith("]"));
}

function missingConditionalIssues(
  model: EmbeddedModel,
  projection: ConditionProjection,
  existing: EmbeddedIssue[],
  scope: ValidationScope | undefined,
  options: { required: Record<string, boolean>; inactive: string[] }
): EmbeddedIssue[] {
  const out: EmbeddedIssue[] = [];
  const required = Object.keys(projection.required)
    .filter((path) => projection.required[path])
    .sort();
  for (const path of required) {
    if (scope?.paths !== undefined && !scope.paths.some((selected) => pathWithin(path, selected))) {
      continue;
    }
    if (getValueAt(projection.values, path).exists) {
      continue;
    }
    if (existing.some((item) => item.path === path && item.code === "required")) {
      continue;
    }
    out.push(...validateValues(model, projection.values, { paths: [path] }, options));
  }
  return out;
}

// Inactive inputs overlap a selected subtree; required issues belong only to it.
function scopeOverlaps(path: string, scope: ValidationScope | undefined): boolean {
  if (!scope || scope.paths === undefined) {
    return true;
  }
  return scope.paths.some((selected) => pathWithin(path, selected) || pathWithin(selected, path));
}

// ---------------------------------------------------------------------------
// Typed value paths
// ---------------------------------------------------------------------------

export function getValueAt(values: unknown, path: string): { value: unknown; exists: boolean } {
  const segments = parseCanonicalPath(path);
  if (!segments) {
    return { value: undefined, exists: false };
  }
  let current: unknown = values;
  for (const segment of segments) {
    const next = descend(current, segment);
    if (!next.exists) {
      return { value: undefined, exists: false };
    }
    current = next.value;
  }
  return { value: current, exists: true };
}

/**
 * Structural array membership for every indexed segment of the path. Missing
 * non-indexed containers may still own conditionally required descendants; an
 * item rule never creates an array member.
 */
export function indexesExist(values: unknown, path: string): boolean {
  const segments = parseCanonicalPath(path);
  if (!segments) {
    return false;
  }
  let lastIndex = -1;
  segments.forEach((segment, position) => {
    if (segment.isIndex) {
      lastIndex = position;
    }
  });
  let current: unknown = values;
  for (const segment of segments.slice(0, lastIndex + 1)) {
    const next = descend(current, segment);
    if (!next.exists) {
      return false;
    }
    current = next.value;
  }
  return true;
}

/** Set a typed value at a canonical path, creating object and array containers. */
export function setValueAt(values: Record<string, unknown>, path: string, value: unknown): boolean {
  const segments = parseCanonicalPath(path);
  if (!segments || segments.length === 0) {
    return false;
  }
  let parent: unknown = values;
  for (let position = 0; position < segments.length - 1; position += 1) {
    const segment = segments[position];
    const next = segments[position + 1];
    const container = segment.isIndex ? parent : parent;
    if (segment.isIndex) {
      if (!Array.isArray(container)) {
        return false;
      }
      while (container.length <= segment.index) {
        container.push(null);
      }
      if (container[segment.index] === null || container[segment.index] === undefined) {
        container[segment.index] = next.isIndex ? [] : {};
      }
      parent = container[segment.index];
    } else {
      if (!isPlainObject(container)) {
        return false;
      }
      if (container[segment.name] === null || container[segment.name] === undefined) {
        container[segment.name] = next.isIndex ? [] : {};
      }
      parent = container[segment.name];
    }
  }
  const last = segments[segments.length - 1];
  if (last.isIndex) {
    if (!Array.isArray(parent)) {
      return false;
    }
    while (parent.length <= last.index) {
      parent.push(null);
    }
    parent[last.index] = value;
    return true;
  }
  if (!isPlainObject(parent)) {
    return false;
  }
  parent[last.name] = value;
  return true;
}

/** Remove an effective value; an inactive array element becomes a null hole. */
export function removeValueAt(values: Record<string, unknown>, path: string): void {
  const segments = parseCanonicalPath(path);
  if (!segments || segments.length === 0) {
    return;
  }
  let parent: unknown = values;
  for (const segment of segments.slice(0, -1)) {
    const next = descend(parent, segment);
    if (!next.exists) {
      return;
    }
    parent = next.value;
  }
  const last = segments[segments.length - 1];
  if (last.isIndex) {
    if (Array.isArray(parent) && last.index < parent.length) {
      parent[last.index] = null;
    }
    return;
  }
  if (isPlainObject(parent)) {
    delete parent[last.name];
  }
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

function issue(code: string, path: string, message: string): EmbeddedIssue {
  return { code, path, rendererPath: rendererPath(path), message };
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasOwn(target: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(target, key);
}

export type { EmbeddedField };
