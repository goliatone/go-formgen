/** Optional model-backed contracts. These types do not enable controller behavior. */
export type CanonicalPath = string;
export type JSONValue = null | boolean | number | string | JSONValue[] | { [key: string]: JSONValue };
export type Expression =
  | { op: "present"; path: CanonicalPath }
  | { op: "eq"; path: CanonicalPath; value: string | number | boolean }
  | { op: "and" | "or"; args: Expression[] };
export interface ConditionRule {
  /** Fixed indexed paths apply only when their array slots exist; they never create rows. */
  path: CanonicalPath;
  applicableWhen?: Expression;
  requiredWhen?: Expression;
  onInapplicable?: "clear" | "retain";
}
export interface ConditionSet {
  version: "formgen.conditions/v1";
  rules: ConditionRule[];
}
export interface EmbeddedField {
  name: string;
  type: "string" | "integer" | "number" | "boolean" | "object" | "array";
  required: boolean;
  /** Optional display label used in issue messages; falls back to the field name. */
  label?: string;
  default?: JSONValue;
  enum?: JSONValue[];
  options?: { value: JSONValue; label?: string; disabled?: boolean }[];
  validations?: { kind: string; params?: Record<string, string> }[];
  nested?: EmbeddedField[];
  items?: EmbeddedField;
}
export interface EmbeddedModel {
  fields: EmbeddedField[];
  conditions?: ConditionSet;
}
/** Compatible additive payload within the existing descriptor/v1 envelope. */
export interface EmbeddedDescriptor {
  version: "formgen.descriptor/v1";
  form: EmbeddedModel;
}
export interface EmbeddedIssue {
  code: string;
  path?: CanonicalPath;
  rendererPath?: string;
  message: string;
}
export interface NormalizedValues {
  values: Record<string, JSONValue>;
  issues: EmbeddedIssue[];
}
export interface HydrationOptions {
  mode?: "patch" | "replace";
  notify?: boolean;
}
export interface ChangeMetadata {
  origin: "ui" | "hydrate" | "program" | "reset";
  paths: CanonicalPath[];
  /** Settled applicability per condition rule path when the model declares conditions. */
  applicable?: Record<CanonicalPath, boolean>;
}
/** Definition diagnostic: locations and causes only, never answer values. */
export interface DefinitionDiagnostic {
  code: string;
  path: string;
  message: string;
}
/**
 * Explicit presence of a leaf path in a typed snapshot: `present` carries a
 * value (false, zero, "" and [] included), `null` was explicitly cleared, and
 * `absent` was never answered or was removed by replace/clear policies.
 */
export type PresenceState = "present" | "null" | "absent";
/** Opt-in model-backed snapshot; independent copy, never aliasing control state. */
export interface TypedSnapshot {
  version: "formgen.snapshot/v1";
  /** Effective typed values (conditions applied); locked controls keep their captured values. */
  values: Record<string, JSONValue>;
  /** Presence per canonical leaf path known to the controller. */
  presence: Record<CanonicalPath, PresenceState>;
  /** Inactive values kept aside by `retain` rules; never part of `values`. */
  retained: Record<string, JSONValue>;
  /** Normalization, unsafe-integer, detached-row, and unknown-path issues. */
  issues: EmbeddedIssue[];
  /** Canonical paths of disabled (locked) controls whose values came from the presence store. */
  locked: CanonicalPath[];
}
export interface ValidationScope { paths?: CanonicalPath[] }
export interface ConditionProjection {
  /** Fixed coordinates remain stable: cleared indexed members are null structural holes.
   * Interpret those slots with applicable; exclude inactive members from logical
   * required/count/enum/uniqueness checks. Do not compact and retarget references.
   */
  values: Record<string, JSONValue>;
  retained: Record<string, JSONValue>;
  applicable: Record<CanonicalPath, boolean>;
  required: Record<CanonicalPath, boolean>;
}
