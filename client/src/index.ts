export { captureSnapshotEnvelope, restoreSnapshotEnvelope, fixedRowFieldPath } from "./embedded-host";
export type { SnapshotContext, SnapshotEnvelope } from "./embedded-host";
export { isPortablePattern, preflightEmbeddedDefinition } from "./embedded-definition";
// Optional model contracts; exporting types does not attach embedded behavior.
export type {
  CanonicalPath, JSONValue, Expression, ConditionRule, ConditionSet,
  EmbeddedField, EmbeddedModel, EmbeddedDescriptor, EmbeddedIssue,
  NormalizedValues, HydrationOptions, ChangeMetadata, ValidationScope,
  ConditionProjection,
} from "./embedded-contract";

import "./version";
import { ResolverRegistry } from "./registry";
import type {
  GlobalConfig,
  FieldConfig,
} from "./config";
import { setGlobalConfig } from "./config";
import {
  locateRelationshipFields,
  readDataset,
  RELATIONSHIP_ORIGINAL_NAME_ATTR,
  syncHiddenInputs,
  syncJsonInput,
} from "./dom";
import { datasetToEndpoint, datasetToFieldConfig } from "./relationship-config";
import { createDebouncedInvoker, createThrottledInvoker } from "./timers";
import { registerChipRenderer, bootstrapChips } from "./renderers/chips";
import { registerTypeaheadRenderer, bootstrapTypeahead } from "./renderers/typeahead";
import { destroyComponents, initComponents } from "./components/registry";
import { ARRAY_CHANGE_EVENT, arrayItemPath, destroyArrayRepeaters, initArrayRepeaters, type ArrayChangeDetail } from "./array-repeaters";
import { destroyRendererWidgets } from "./renderers/relationship-cleanup";
import {
  clearFieldError,
  currentFormErrors,
  findFormErrorContainer,
  renderFieldError,
  renderFormErrors,
  syncInvalidGroups,
} from "./errors";
import { emitRelationshipUpdate } from "./relationship-events";
import type {
  ChangeMetadata,
  ConditionProjection,
  DefinitionDiagnostic,
  EmbeddedIssue,
  EmbeddedModel,
  HydrationOptions,
  JSONValue,
  NormalizedValues,
  PresenceState,
  TypedSnapshot,
  ValidationScope,
} from "./embedded-contract";
import {
  compileConditions,
  getValueAt,
  removeValueAt,
  setValueAt,
  validateConditional,
  type CompileResult,
  type InactiveInputPolicy,
} from "./embedded-conditions";
import {
  cloneJSON,
  formatCanonicalPath,
  jsonValueEqual,
  lookupField,
  decodeEnumValue,
  normalizeValues,
  normalizeFieldValue,
  parseCanonicalPath,
  pathWithin,
  rendererPath,
  resolveEmbeddedModel,
  validateValues,
  type EmbeddedModelInput,
  type NormalizeOptions,
} from "./embedded-values";
import {
  normalizeRelationshipCurrentOptions,
  relationshipCurrentValuesNeedingResolution,
} from "./relationship-current";

/**
 * initRelationships bootstraps the runtime resolver registry. The initial phase
 * provides a no-op implementation that resolves immediately while the full
 * runtime logic is developed across subsequent tasks.
 */
let activeRegistry: ResolverRegistry | null = null;

interface RuntimeRootLifecycle {
  cleanups: Set<() => void>;
  disposed: boolean;
}

const runtimeRootLifecycles = new WeakMap<
  ResolverRegistry,
  WeakMap<HTMLElement, RuntimeRootLifecycle>
>();

export async function initRelationships(
  config: GlobalConfig = {}
): Promise<ResolverRegistry> {
  const hasOverrides = Object.keys(config ?? {}).length > 0;
  const reuseExisting = activeRegistry && !hasOverrides ? activeRegistry : null;
  const globalConfig = hasOverrides ? setGlobalConfig(config) : undefined;
  const registry = reuseExisting ?? new ResolverRegistry(globalConfig);

  if (!reuseExisting) {
    activeRegistry = registry;
    (globalThis as Record<string, unknown>).formgenRelationships = registry;
  }

  registerChipRenderer(registry);
  registerTypeaheadRenderer(registry);

  const roots = Array.from(
    document.querySelectorAll<HTMLElement>("[data-formgen-auto-init]")
  );

  const promises: Promise<void>[] = [];

  for (const root of roots) {
    promises.push(...initializeRuntimeRoot(root, registry, config));
  }

  if (promises.length > 0) {
    await Promise.all(promises);
  }

  return registry;
}

// initFormgenRoot initializes option resolvers and widgets only within one
// dynamically inserted form root. Hosts can pair the returned registry with a
// Formgen controller so controller teardown also aborts root-scoped resolvers.
export async function initFormgenRoot(
  root: HTMLElement,
  config: GlobalConfig = {}
): Promise<ResolverRegistry> {
  const hasOverrides = Object.keys(config ?? {}).length > 0;
  const registry = activeRegistry && !hasOverrides
    ? activeRegistry
    : new ResolverRegistry(hasOverrides ? config : undefined);
  registerChipRenderer(registry);
  registerTypeaheadRenderer(registry);
  const pending = initializeRuntimeRoot(root, registry, config);
  if (pending.length > 0) {
    await Promise.all(pending);
  }
  return registry;
}

/**
 * Reset the global registry. Intended for testing only.
 * @internal
 */
export function resetGlobalRegistry(): void {
  activeRegistry = null;
  delete (globalThis as Record<string, unknown>).formgenRelationships;
}

export type {
  GlobalConfig,
  EndpointConfig,
  EndpointOverride,
  EndpointMapping,
  EndpointAuth,
  FieldConfig,
  FieldValidationRule,
  Option,
  ValidationError,
  ValidationResult,
  CreateActionDetail,
  EditActionDetail,
} from "./config";
export { ResolverRegistry } from "./registry";
export {
  Resolver,
  RESOLVER_SUPERSEDED_ABORT_REASON,
  type ResolverEventDetail,
  type ResolverEventName,
  type Renderer,
  type CustomResolver,
} from "./resolver";
export { RUNTIME_VERSION, RUNTIME_BUILD, RUNTIME_INFO } from "./version";
export {
  normalizeValues,
  validateValues,
  parseCanonicalPath,
  formatCanonicalPath,
  rendererPath,
  pathWithin,
  encodeEnumValue,
  decodeEnumValue,
  resolveEmbeddedModel,
  cloneJSON,
  type EmbeddedModelInput,
  type NormalizeOptions,
  type ValidateOptions,
  type PathSegment,
} from "./embedded-values";
export {
  registerComponent,
  initComponents,
  destroyComponents,
  preflightComponents,
  registeredComponentNames,
  type RuntimeComponentPreflight,
  __resetComponentRegistryForTests as resetComponentRegistryForTests,
} from "./components/registry";
export {
  initArrayRepeaters,
  destroyArrayRepeaters,
  addArrayItem,
  ARRAY_CHANGE_EVENT,
  type ArrayChangeDetail,
} from "./array-repeaters";
export {
  compileConditions,
  validateConditional,
  evaluateExpression,
  getValueAt,
  indexesExist,
  removeValueAt,
  CONDITIONS_VERSION,
  type CompileResult,
  type CompiledRule,
  type ConditionProgram,
  type ConditionalValidation,
  type InactiveInputPolicy,
  type ValidateConditionalOptions,
} from "./embedded-conditions";
export {
  registerErrorRenderer,
  registerFormErrorRenderer,
  renderFormErrors,
  syncInvalidGroups,
  FORM_ERRORS_ATTR,
  GROUP_INVALID_ATTR,
  type FormErrorRenderContext,
  type FormErrorRenderer,
} from "./errors";
export {
  registerThemeClasses,
  getThemeClasses,
  type ThemeClassMap,
  type ChipsClassMap,
  type TypeaheadClassMap,
  type SwitchClassMap,
  type WysiwygClassMap,
} from "./theme/classes";
export { renderSwitch, type SwitchStore } from "./renderers/switch";
export { renderWysiwyg, autoInitWysiwyg, type WysiwygStore, type WysiwygConfig } from "./renderers/wysiwyg";
export {
  RELATIONSHIP_UPDATE_EVENT,
  RELATIONSHIP_CREATE_ACTION_EVENT,
  RELATIONSHIP_EDIT_ACTION_EVENT,
  emitRelationshipUpdate,
  emitRelationshipCreateAction,
  emitRelationshipEditAction,
  type RelationshipUpdateDetail,
  type RelationshipUpdateOrigin,
  type RelationshipCreateActionDetail,
  type RelationshipEditActionDetail,
} from "./relationship-events";


function applyInitialSelection(element: HTMLElement, field: FieldConfig): void {
  if (!field || field.current == null) {
    return;
  }
  if (element.dataset.relationshipCurrentApplied === "true") {
    return;
  }
  if (element instanceof HTMLSelectElement) {
    const options = normalizeRelationshipCurrentOptions(field.current, element.multiple);
    const changed = applySelectValues(element, options);
    if (changed) {
      syncRelationshipMirrors(element, field.submitAs);
      element.dataset.relationshipCurrentApplied = "true";
    }
    return;
  }
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    const options = normalizeRelationshipCurrentOptions(field.current, false);
    if (options.length > 0) {
      element.value = options[0]?.value ?? "";
      element.dataset.relationshipCurrentApplied = "true";
    }
  }
}

function initializeRuntimeRoot(
  root: HTMLElement,
  registry: ResolverRegistry,
  config: GlobalConfig
): Promise<void>[] {
  const lifecycle = runtimeRootLifecycle(root, registry);
  initComponents(root);
  initArrayRepeaters(root, {
    onItemAdded: (itemRoot) => {
      const nestedPromises = initializeRuntimeRoot(itemRoot, registry, config);
      if (nestedPromises.length > 0) {
        void Promise.all(nestedPromises);
      }
    },
  });

  const promises: Promise<void>[] = [];
  const fields = locateRelationshipFields(root);
  for (const element of fields) {
    const dataset = readDataset(element);
    const endpoint = datasetToEndpoint(dataset);
    const field = datasetToFieldConfig(element, dataset);
    applyInitialSelection(element, field);

    if (!registry.get(element)) {
      registry.register(element, { field, endpoint });
    }

    if (
      field.renderer === "chips" &&
      element instanceof HTMLSelectElement &&
      element.multiple
    ) {
      bootstrapChips(element, registry);
    }
    if (
      field.renderer === "typeahead" &&
      element instanceof HTMLSelectElement &&
      !element.multiple
    ) {
      bootstrapTypeahead(element, registry);
    }

    setupDependentRefresh(element, field, root, registry, lifecycle);
    setupManualRefresh(element, field, root, registry, lifecycle);
    setupSearchMode(element, field, registry, lifecycle);

    if (shouldAutoResolve(field, endpoint)) {
      promises.push(registry.resolve(element));
    }
  }
  return promises;
}

function runtimeRootLifecycle(root: HTMLElement, registry: ResolverRegistry): RuntimeRootLifecycle {
  let roots = runtimeRootLifecycles.get(registry);
  if (!roots) {
    roots = new WeakMap();
    runtimeRootLifecycles.set(registry, roots);
  }
  const existing = roots.get(root);
  if (existing && !existing.disposed) {
    return existing;
  }

  const lifecycle: RuntimeRootLifecycle = {
    cleanups: new Set(),
    disposed: false,
  };
  roots.set(root, lifecycle);
  registry.registerRootCleanup(root, () => {
    if (lifecycle.disposed) {
      return;
    }
    lifecycle.disposed = true;
    for (const cleanup of lifecycle.cleanups) {
      cleanup();
    }
    lifecycle.cleanups.clear();
    destroyRendererWidgets(root);
    destroyArrayRepeaters(root);
    destroyComponents(root);
    roots?.delete(root);
  });
  return lifecycle;
}

function applySelectValues(
  select: HTMLSelectElement,
  options: Array<{ value: string; label: string }>,
  preserveLabels = false
): boolean {
  const optionByValue = new Map<string, { value: string; label: string }>();
  for (const option of options) {
    if (!option.value || optionByValue.has(option.value)) {
      continue;
    }
    optionByValue.set(option.value, option);
    if (!select.multiple) {
      break;
    }
  }
  const targetValues = new Set(optionByValue.keys());
  let changed = false;

  Array.from(select.options).forEach((option) => {
    const shouldSelect = targetValues.has(option.value);
    if (option.selected !== shouldSelect) {
      option.selected = shouldSelect;
      changed = true;
    }
    if (shouldSelect) {
      const seed = optionByValue.get(option.value);
      if (!preserveLabels && seed && option.textContent !== seed.label) {
        option.textContent = seed.label;
        changed = true;
      }
      targetValues.delete(option.value);
    }
  });

  targetValues.forEach((value) => {
    if (!value) {
      return;
    }
    const option = document.createElement("option");
    option.value = value;
    option.textContent = optionByValue.get(value)?.label ?? value;
    option.selected = true;
    select.appendChild(option);
    changed = true;
  });

  if (!select.multiple && optionByValue.size === 0) {
    if (select.value !== "") {
      select.value = "";
      changed = true;
    }
  }

  return changed;
}

function syncRelationshipMirrors(select: HTMLSelectElement, submitAs?: FieldConfig["submitAs"]): void {
  if (submitAs === "json") {
    syncJsonInput(select);
    return;
  }
  if (select.multiple) {
    syncHiddenInputs(select);
  }
}

function shouldAutoResolve(field: FieldConfig, endpoint: ReturnType<typeof datasetToEndpoint>): boolean {
  if (!endpoint.url) {
    return false;
  }
  if (field.refreshMode === "manual") {
    return false;
  }
  if (field.mode === "search") {
    return currentNeedsResolution(field.current);
  }
  return true;
}

function currentNeedsResolution(current: FieldConfig["current"]): boolean {
  return relationshipCurrentValuesNeedingResolution(current).length > 0;
}

export interface HydrationPayload {
  values?: Record<string, unknown>;
  errors?: Record<string, string | string[]>;
}

export interface FormControllerOptions {
  includeHidden?: boolean;
  registry?: ResolverRegistry;
  /**
   * Complete embedded model (or descriptor envelope) enabling the model-aware
   * `normalize()` and `validate()` operations. A rendered subset is not enough.
   */
  model?: EmbeddedModelInput;
  /** Normalization policy for model-aware operations. */
  normalization?: NormalizeOptions;
  /** Reactive applicability behavior when the model declares conditions. */
  conditions?: ConditionBehaviorOptions;
}

export interface ConditionBehaviorOptions {
  /**
   * Hide inapplicable controls (their component wrapper, or a matching group
   * container) with the `hidden` attribute. Applicability markers are always set.
   */
  hideInapplicable?: boolean;
  /** How `validate()` treats supplied inactive inputs; defaults to `reject`. */
  policy?: InactiveInputPolicy;
}

/** Origin attached to every controller change notification. */
export type ChangeOrigin = ChangeMetadata["origin"];

/**
 * Change subscribers receive the raw controller values, the triggering event,
 * and metadata with the mutation origin and canonical changed paths. Callbacks
 * written against the legacy two-argument shape keep working.
 */
export type FormChangeCallback = (
  values: Record<string, unknown>,
  event: Event,
  meta: ChangeMetadata
) => void;

export interface FormController {
  root: Document | HTMLElement;
  getValues(options?: FormControllerOptions): Record<string, unknown>;
  /**
   * Hydrate controls from a values object.
   *
   * Without options the legacy contract applies: patch semantics and one
   * subscriber callback per dispatched native input/change event, including
   * duplicates. Passing an options object opts into the embedded/v1 contract:
   * the mutation settles once with `origin: "hydrate"`, emits no callback when
   * no control changed, and `notify: false` suppresses this controller's
   * subscribers only. Widget, mirror, and dependency synchronization still runs
   * and native listeners on changed controls still fire. `mode: "replace"`
   * clears omitted managed values, treats `null` as an explicit clear, replaces
   * arrays, and throws before mutating when a payload row has no rendered row.
   */
  setValues(values: Record<string, unknown>, options?: HydrationOptions): void;
  /**
   * Restore rendered defaults and clear field errors. Without options the legacy
   * per-event notification contract applies. With options the reset settles once
   * with `origin: "reset"`, `notify: false` suppresses subscribers, and `mode` is
   * ignored because defaults always cover every managed control.
   */
  reset(options?: HydrationOptions): void;
  setErrors(errors: Record<string, string | string[]>): void;
  clearErrors(names?: string[]): void;
  onChange(callback: FormChangeCallback): () => void;
  focus(name: string): boolean;
  destroy(): void;
  /**
   * Model-aware typed snapshot of the current raw values plus parse issues.
   * Requires `model`; `getValues()` stays raw. Pure and synchronous.
   */
  normalize(options?: FormControllerOptions): NormalizedValues;
  /**
   * Model-aware validation of selected canonical paths (`scope.paths`) or the
   * full form. Normalization issues are limited to the selected subtrees.
   * Requires `model`; never fetches options, submits, or navigates.
   */
  validate(scope?: ValidationScope): EmbeddedIssue[];
  /**
   * Present issues (browser or server) without touching values or submitting.
   * Issues whose precise path maps to a rendered control display inline on that
   * exact control; form-level and unmapped issues go to the summary. Existing
   * issue display is replaced unless `append` is set.
   */
  setIssues(issues: EmbeddedIssue[], options?: SetIssuesOptions): IssuePresentation;
  /** Clear every displayed issue (inline and summary) while preserving values. */
  clearIssues(): void;
  /**
   * Focus the first enabled, visible control for an issue path; fall back to
   * the summary container when no such control exists.
   */
  focusIssue(target: EmbeddedIssue | string): IssueFocusResult;
  /**
   * Effective projection of the current typed values under the model's
   * conditions: effective values, retained inactive values, applicability, and
   * requiredness. Without conditions the projection is the normalized values.
   */
  effective(): ConditionProjection;
  /**
   * Re-evaluate conditions against the current controls and settle applicability
   * chrome, requiredness, clear-policy values, and obsolete issues. Hosts call
   * this after DOM changes the runtime cannot observe. No subscriber callback.
   */
  settle(): ConditionProjection;
  /** Definition diagnostics from compiling the model's conditions (empty when valid). */
  conditionDiagnostics(): DefinitionDiagnostic[];
  /**
   * Opt-in typed snapshot with explicit presence. The controller tracks, per
   * leaf path, whether a value is present (including false, zero, "" and []),
   * explicitly null, or absent, using hydration payloads and user actions
   * rather than guessing from control strings. Disabled controls keep their
   * last known values (locked), retained inactive values stay separate, and
   * normalization, unsafe-integer, detached-row and unknown-path problems are
   * reported as issues. The result is an independent copy. Requires `model`.
   */
  snapshot(): TypedSnapshot;
  /**
   * Restore a typed snapshot: present values hydrate, explicit nulls clear,
   * absent paths are cleared (replace mode by default), retained values return
   * to their controls, and presence states are restored. Unknown or unsafe
   * snapshot paths are reported and skipped. Malformed envelopes, presence
   * states or present entries without values return invalidSnapshot without
   * mutating any control. Requires `model`.
   */
  restore(snapshot: TypedSnapshot, options?: HydrationOptions): EmbeddedIssue[];
}

interface PresenceEntry {
  state: PresenceState;
  value?: JSONValue;
  /** Parse issues survive locking, just like the typed value. */
  issues?: EmbeddedIssue[];
  /** Native controls could not represent this observed logical value. */
  override?: boolean;
}

export interface SetIssuesOptions {
  /** Keep previously displayed issues instead of replacing them. */
  append?: boolean;
  /** Render form-level and unmapped issues into the summary (default true). */
  summary?: boolean;
}

export interface IssuePresentation {
  /** Issues displayed inline on a rendered control. */
  field: EmbeddedIssue[];
  /** Form-level issues and issues without a rendered control. */
  summary: EmbeddedIssue[];
}

export interface IssueFocusResult {
  focused: boolean;
  target: "control" | "summary" | null;
  path: string;
}

interface MutationBoundary {
  origin: ChangeOrigin;
  settled: boolean;
}

interface HydrationRun {
  mode: "patch" | "replace";
  /** Settled runs dispatch native events only for changed controls and record their paths. */
  settled: boolean;
  changed: Set<string>;
  /** Exact rendered paths resolved from this hydration payload, including ancestor nulls. */
  supplied?: Map<string, unknown>;
  model?: EmbeddedModel;
  normalization?: NormalizeOptions;
  enumValues?: NormalizeOptions["enumValues"];
  expected?: Map<string, PresenceEntry>;
}

interface ChangeSubscription {
  callback: FormChangeCallback;
  active: boolean;
}

/** Event type carried by settled notifications. The event is created, not dispatched. */
export const CONTROLLER_CHANGE_EVENT = "formgen:controller:change" as const;

export function attachFormController(
  root: Document | HTMLElement | string = document,
  options: FormControllerOptions = {}
): FormController {
  const resolvedRoot = resolveControllerRoot(root);
  const controllerRegistry = options.registry ?? activeRegistry;
  const embeddedModel = options.model === undefined ? null : resolveEmbeddedModel(options.model);
  const compiled: CompileResult =
    embeddedModel
      ? compileConditions(embeddedModel)
      : { program: null, diagnostics: [] };
  const reactive = compiled.program !== null && compiled.program.rules.length > 0;
  const requireModel = (operation: string) => {
    if (!embeddedModel) {
      throw new Error(
        `formgen: controller.${operation}() requires the complete embedded model; attach with { model }`
      );
    }
    return embeddedModel;
  };
  const subscriptions: ChangeSubscription[] = [];
  const boundaries: MutationBoundary[] = [];
  let destroyed = false;
  let listening = false;
  let lastProjection: ConditionProjection | null = null;
  /** Explicit presence per canonical leaf path; only maintained with a model. */
  const presence = new Map<string, PresenceEntry>();
  // A clear rule can erase a native leaf that another rule retains. Keep that
  // history apart from effective answers; it must never drive reentry rules.
  const retainedLeaves = new Map<string, PresenceEntry>();
  const inactiveRows = new Set<string>();
  const rememberDeletedRows = (): void => {
    resolvedRoot.querySelectorAll<HTMLElement>('[data-formgen-array-item]').forEach((row) => {
      const deleted = row.dataset.formgenArrayDeleted === "true" || (row.hidden && Array.from(row.querySelectorAll<HTMLInputElement>("input"))
        .some((input) => /(?:\._delete|\[_delete\])$/.test(input.name) &&
          (input.type === "checkbox" ? input.checked : ["true", "1"].includes(input.value))));
      const raw = deleted ? arrayItemPath(row) : undefined;
      const path = raw ? toCanonicalPath(toDotKey(raw)) : "";
      if (path && lookupField(embeddedModel!, path)) inactiveRows.add(path);
    });
  };

  /** Controls grouped by canonical path, built once per operation (O(n), not O(n²)). */
  const indexLeafControls = (): Map<string, HTMLElement[]> => {
    const index = new Map<string, HTMLElement[]>();
    controllerFields(resolvedRoot).forEach((control) => {
      const path = canonicalPathForElement(control);
      if (!path || isArrayIntentName(fieldName(control))) {
        return;
      }
      const list = index.get(path);
      if (list) {
        list.push(control);
      } else {
        index.set(path, [control]);
      }
    });
    return index;
  };

  const isLeafControlPath = (path: string, index?: Map<string, HTMLElement[]>): HTMLElement[] =>
    (index ?? indexLeafControls()).get(path) ?? [];

  // Store individual group members so one disabled choice does not freeze the
  // group or reread locked DOM. Entries refresh only through observed edits,
  // explicit hydration or rendered-default reset.
  let choices = new WeakMap<HTMLInputElement, { checked: boolean; value: string }>();
  const rememberChoices = (paths: string[], includeLocked: boolean): void => {
    const index = indexLeafControls();
    for (const path of paths) {
      const controls = index.get(path) ?? [];
      const selectedRadio = !includeLocked && controls.some((control) =>
        isChoiceMember(control) && control.type === "radio" && control.checked && !isDisabledControl(control));
      for (const control of controls) {
        if (!isChoiceMember(control)) continue;
        if (includeLocked || !isDisabledControl(control)) {
          choices.set(control, { checked: control.checked, value: control.value });
        } else if (selectedRadio && control.type === "radio") {
          // Selecting an enabled alternative legitimately deselects the locked
          // member; keep its captured token without reading disabled DOM.
          const saved = choices.get(control);
          if (saved) choices.set(control, { ...saved, checked: false });
        }
      }
    }
  };

  const normalizedControlsNow = (includeDisabled = false, logicalChoices = false): NormalizedValues => {
    const selectedRadios = new Set<string>();
    if (logicalChoices) {
      controllerFields(resolvedRoot).forEach((control) => {
        if (control instanceof HTMLInputElement && control.type === "radio" && control.checked && !isDisabledControl(control)) {
          selectedRadios.add(canonicalPathForElement(control));
        }
      });
    }
    return normalizeValues(embeddedModel!, collectControllerValues(resolvedRoot, {
      includeHidden: options.includeHidden ?? true,
      includeDisabled: includeDisabled || logicalChoices,
      readValue: (control) => {
        if (isArrayIntentName(fieldName(control))) return undefined;
        if (Array.from(inactiveRows).some((row) => pathWithin(canonicalPathForElement(control), row))) return undefined;
        if (!logicalChoices || !isDisabledControl(control)) return readTypedControlValue(control);
        if (!isChoiceMember(control)) return undefined;
        const saved = choices.get(control);
        if (!saved?.checked || (control.type === "radio" && selectedRadios.has(canonicalPathForElement(control)))) return undefined;
        return decodeControlSelection(control.type === "checkbox" ? [saved.value] : saved.value);
      },
    }), { ...options.normalization, enumValues: "literal" });
  };

  const entryForValue = (path: string, normalized: NormalizedValues): PresenceEntry => {
    const found = getValueAt(normalized.values, path);
    const issues = normalized.issues.filter((issue) => issue.path !== undefined && pathWithin(issue.path, path));
    return {
      ...(found.exists ? (found.value === null ? { state: "null" as const } : { state: "present" as const, value: cloneJSON(found.value) }) : { state: "absent" as const }),
      ...(issues.length > 0 ? { issues: issues.map((issue) => ({ ...issue })) } : {}),
    };
  };

  /** Refresh observed leaf presence; fully locked paths retain stored values. */
  const refreshPresence = (paths: string[], cause: "attach" | "edit" | "reset" | "clear"): void => {
    if (!embeddedModel || destroyed) return;
    rememberChoices(paths, cause !== "edit");
    const normalized = normalizedControlsNow(cause === "attach" || cause === "reset", cause === "edit");
    const index = indexLeafControls();
    for (const path of paths) {
      const controls = isLeafControlPath(path, index);
      if (controls.length === 0) continue;
      if (cause !== "clear") retainedLeaves.delete(path);
      if (cause === "clear") {
        presence.set(path, { state: "absent" });
        continue;
      }
      if (controls.every((control) => isDisabledControl(control)) && cause === "edit") continue;
      // Native defaults cannot distinguish an untouched empty control from an
      // explicit answer. Attach/reset start those leaves absent.
      if ((cause === "attach" || cause === "reset") && !controls.some((control) => controlHasValue(control))) {
        presence.set(path, { state: "absent" });
        continue;
      }
      const entry = entryForValue(path, normalized);
      if (entry.state !== "absent") {
        presence.set(path, entry);
      } else if (cause === "edit" && controls.some((control) => isChoiceMember(control) && control.type === "checkbox")) {
        presence.set(path, { state: "present", value: [] });
      } else {
        presence.set(path, { state: "absent" });
      }
    }
  };

  const allLeafPaths = (): string[] => Array.from(indexLeafControls().keys());

  /** Use the exact leaf entries the hydrator resolved, including explicit locks. */
  const recordHydrationPresence = (run: HydrationRun): void => {
    if (!embeddedModel || destroyed) return;
    rememberChoices(Array.from(run.changed), false);
    rememberChoices(Array.from(run.supplied?.keys() ?? []), true);
    const normalized = normalizedControlsNow(true);
    const logical = normalizedControlsNow(false, true);
    const index = indexLeafControls();
    const explicit = new Set<string>();
    run.supplied?.forEach((value, path) => {
      if (!index.has(path)) return;
      explicit.add(path);
      retainedLeaves.delete(path);
      const actual = entryForValue(path, normalized);
      const expected = run.expected?.get(path);
      const entry: PresenceEntry = expected ? { ...expected } : value == null ? { state: "null" } : actual;
      const emptyGroup = expected?.state === "present" && Array.isArray(expected.value) && expected.value.length === 0 && actual.state === "absent";
      if (expected?.state === "present" && !emptyGroup && (actual.state !== "present" || !jsonValueEqual(expected.value, actual.value))) {
        entry.override = true;
        entry.issues = [...(entry.issues ?? []), snapshotIssue("unrepresentable", path, "hydrated value cannot be represented by the rendered control")];
      }
      presence.set(path, entry.state === "absent" && Array.isArray(value) && value.length === 0
        ? { state: "present", value: [] } : entry);
    });
    const recordOmitted = (path: string): void => {
      retainedLeaves.delete(path);
      const controls = index.get(path) ?? [];
      if (controls.some((control) => isChoiceMember(control) && isDisabledControl(control))) {
        presence.set(path, entryForValue(path, logical));
      } else {
        presence.set(path, { state: "absent" });
      }
    };
    run.changed.forEach((path) => {
      if (!explicit.has(path) && index.has(path)) recordOmitted(path);
    });
    if (run.mode === "replace") {
      index.forEach((controls, path) => {
        if (!explicit.has(path) && !controls.every((control) => isDisabledControl(control))) {
          recordOmitted(path);
        }
      });
    }
  };

  /** New leaf paths (added repeater rows) start absent or from their rendered state. */
  const seedNewPresence = (): void => {
    if (!embeddedModel || destroyed) {
      return;
    }
    const fresh = allLeafPaths().filter((path) => !presence.has(path));
    if (fresh.length > 0) {
      refreshPresence(fresh, "attach");
    }
  };

  const notify = (event: Event, meta: ChangeMetadata): void => {
    if (destroyed || subscriptions.length === 0) {
      return;
    }
    for (const subscription of subscriptions.slice()) {
      if (destroyed) {
        return;
      }
      if (!subscription.active) {
        continue;
      }
      subscription.callback(controller.getValues(), event, meta);
    }
  };

  /** Overlay values that native controls cannot encode: explicit null, locked
   * answers, and empty groups. Only snapshots remove unanswered defaults;
   * effective()/validate() keep their existing raw-default semantics.
   */
  const readTypedState = (forSnapshot = false) => {
    rememberDeletedRows();
    const normalized = normalizedControlsNow(false, true);
    const issues: EmbeddedIssue[] = normalized.issues.map((issue) => ({ ...issue }));
    // Overlay the presence store before evaluating conditions: raw collection
    // omits disabled (locked) controls and cannot express absent, explicit
    // null, or an explicit empty array.
    const base = cloneJSON(normalized.values) as Record<string, JSONValue>;
    const presenceOut: Record<string, PresenceState> = {};
    const locked: string[] = [];
    const index = indexLeafControls();
    seedRenderedRowValues(base, resolvedRoot, embeddedModel!, index.keys(), inactiveRows);
    for (const [path, entry] of Array.from(presence.entries())) {
      const controls = isLeafControlPath(path, index).filter(() => !Array.from(inactiveRows).some((row) => pathWithin(path, row)));
      if (controls.length === 0) {
        if (forSnapshot) {
          presence.delete(path);
          retainedLeaves.delete(path);
          issues.push(snapshotIssue("detached", path, "control is no longer rendered; stored value dropped"));
        }
        removeValueAt(base, path);
        continue;
      }
      presenceOut[path] = entry.state;
      const live = getValueAt(base, path);
      if (controls.some((control) => isDisabledControl(control))) locked.push(path);
      for (const issue of entry.issues ?? []) {
        if (!issues.some((existing) => existing.code === issue.code && existing.path === issue.path && existing.message === issue.message)) {
          issues.push({ ...issue });
        }
      }
      if (entry.override || controls.every((control) => isDisabledControl(control))) {
        if (entry.state === "present" && entry.value !== undefined) {
          setValueAt(base, path, cloneJSON(entry.value));
        } else if (entry.state === "null") {
          setValueAt(base, path, null);
        } else {
          removeValueAt(base, path);
        }
        continue;
      }
      if (entry.state === "absent" && forSnapshot) {
        removeValueAt(base, path);
      } else if (entry.state === "null") {
        setValueAt(base, path, null);
      } else if (entry.state === "present" && !live.exists && Array.isArray(entry.value) && entry.value.length === 0) {
        setValueAt(base, path, []);
      }
    }
    inactiveRows.forEach((path) => {
      if (Array.from(inactiveRows).some((ancestor) => ancestor !== path && pathWithin(path, ancestor))) return;
      // Deleting the last answer still represents an answered empty array, not
      // an omitted optional field. Preserve existing sibling coordinates only.
      const arrayPath = path.slice(0, path.lastIndexOf("["));
      if (!getValueAt(base, arrayPath).exists) setValueAt(base, arrayPath, []);
      removeValueAt(base, path);
    });
    return { values: base, issues, presence: presenceOut, locked, inactivePaths: Array.from(inactiveRows) };
  };

  const withApplicability = (meta: ChangeMetadata): ChangeMetadata => {
    if (!reactive || !lastProjection) {
      return meta;
    }
    return { ...meta, applicable: { ...lastProjection.applicable } };
  };

  /** Restore only the retained projection, never its effective input values. */
  const overlayRetained = (
    projection: ConditionProjection,
    state?: { presence: Record<string, PresenceState>; issues: EmbeddedIssue[] }
  ): ConditionProjection => {
    const targets = compiled.program?.rules.filter(({ rule }) =>
      rule.onInapplicable === "retain" && projection.applicable[rule.path] === false
    ) ?? [];
    for (const [leaf, saved] of retainedLeaves) {
      if (Array.from(inactiveRows).some((row) => pathWithin(leaf, row))) continue;
      for (const { rule } of targets) {
        if (!pathWithin(leaf, rule.path)) continue;
        const relative = `root${leaf.slice(rule.path.length)}`;
        const tree: Record<string, unknown> = {};
        if (Object.prototype.hasOwnProperty.call(projection.retained, rule.path)) {
          tree.root = cloneJSON(projection.retained[rule.path]);
        }
        if (state && saved.state === "absent") {
          removeValueAt(tree, relative);
        } else if (saved.value !== undefined || saved.state === "null") {
          setValueAt(tree, relative, cloneJSON(saved.state === "null" ? null : saved.value));
        }
        if (Object.prototype.hasOwnProperty.call(tree, "root")) {
          projection.retained[rule.path] = tree.root as JSONValue;
        } else {
          delete projection.retained[rule.path];
        }
        if (state) {
          state.presence[leaf] = saved.state;
          for (const issue of saved.issues ?? []) {
            if (!state.issues.some((existing) => jsonValueEqual(existing, issue))) state.issues.push({ ...issue });
          }
        }
      }
    }
    return projection;
  };

  /**
   * Evaluate the model's conditions against the current controls and settle
   * applicability chrome, requiredness, clear-policy values, and obsolete
   * issues. Cleared paths are recorded on the hydration run. Runs inside a
   * mutation boundary so the native events it dispatches never re-enter.
   */
  const settleConditions = (run: HydrationRun): ConditionProjection | null => {
    const program = compiled.program;
    if (!program || !reactive || destroyed || !embeddedModel) {
      return null;
    }
    const state = readTypedState();
    const projection = program.evaluate(state.values, state.inactivePaths);
    const hide = options.conditions?.hideInapplicable !== false;
    const clearValues: Record<string, unknown> = {};
    const clearPresence = new Set<string>();
    const clearErrorsPayload: Record<string, string[]> = {};
    const index = indexLeafControls();
    const within = (path: string): HTMLElement[] => {
      const out: HTMLElement[] = [];
      index.forEach((controls, controlPath) => {
        if (pathWithin(controlPath, path) && !state.inactivePaths.some((row) => pathWithin(controlPath, row))) {
          out.push(...controls);
        }
      });
      return out;
    };
    for (const entry of program.rules) {
      const path = entry.rule.path;
      const active = projection.applicable[path] === true;
      const controls = within(path);
      applyApplicabilityChrome(resolvedRoot, path, controls, active, hide);
      for (const control of controls) {
        const controlPath = canonicalPathForElement(control);
        if (!active) {
          if (entry.rule.onInapplicable !== "retain") {
            clearPresence.add(controlPath);
            if (controlHasValue(control)) {
              clearValues[controlPath] = null;
            }
          }
          clearErrorsPayload[toDotKey(stripArraySuffix(fieldName(control)))] = [];
        }
      }
    }
    // A container's applicability gates every descendant constraint. Restore
    // each leaf's own static/conditional requiredness, never the parent's.
    index.forEach((controls, path) => {
      if (!program.rules.some((entry) => pathWithin(path, entry.rule.path))) {
        return;
      }
      const active = !Object.entries(projection.applicable).some(
        ([ancestor, applicable]) => !applicable && pathWithin(path, ancestor)
      );
      const required = projection.required[path] ?? lookupField(embeddedModel, path)?.required ?? false;
      controls.forEach((control) => setControlRequired(control, active && required));
    });
    const retainedPaths = program.rules.filter(({ rule }) =>
      rule.onInapplicable === "retain" && projection.applicable[rule.path] === false
    ).map(({ rule }) => rule.path);
    for (const leaf of retainedLeaves.keys()) {
      if (!index.has(leaf) || state.inactivePaths.some((row) => pathWithin(leaf, row)) || !retainedPaths.some((path) => pathWithin(leaf, path))) retainedLeaves.delete(leaf);
    }
    // Capture before clearing, and keep the first capture through later reads
    // and settlement. Explicit hydration/edits/reset invalidate addressed leaves.
    for (const leaf of clearPresence) {
      if (!retainedLeaves.has(leaf) && !state.inactivePaths.some((row) => pathWithin(leaf, row)) && retainedPaths.some((path) => pathWithin(leaf, path))) {
        const saved = entryForValue(leaf, state);
        retainedLeaves.set(leaf, { ...saved, state: presence.get(leaf)?.state ?? saved.state });
      }
    }
    if (Object.keys(clearValues).length > 0) {
      // Clearing is a patch over the inapplicable controls only; reusing a
      // replace-mode run would treat every other managed control as omitted.
      const clearRun: HydrationRun = { mode: "patch", settled: run.settled, changed: run.changed };
      hydrateFormValuesWithRegistry(resolvedRoot, { values: clearValues }, controllerRegistry, clearRun);
    }
    refreshPresence(Array.from(clearPresence), "clear");
    if (Object.keys(clearErrorsPayload).length > 0) {
      hydrateFormValuesWithRegistry(resolvedRoot, { errors: clearErrorsPayload }, controllerRegistry);
      syncInvalidGroups(resolvedRoot);
    }
    relocateHiddenFocus(resolvedRoot);
    lastProjection = overlayRetained(projection);
    return lastProjection;
  };

  const settleInBoundary = (origin: ChangeOrigin): string[] => {
    const run: HydrationRun = { mode: "patch", settled: true, changed: new Set<string>() };
    const boundary: MutationBoundary = { origin, settled: true };
    boundaries.push(boundary);
    try {
      settleConditions(run);
    } finally {
      const position = boundaries.lastIndexOf(boundary);
      if (position >= 0) {
        boundaries.splice(position, 1);
      }
    }
    return Array.from(run.changed);
  };

  const handleNativeEvent = (event: Event): void => {
    if (destroyed) {
      return;
    }
    const boundary = boundaries[boundaries.length - 1];
    if (boundary) {
      if (boundary.settled) {
        return;
      }
      notify(event, withApplicability({ origin: boundary.origin, paths: eventCanonicalPaths(event) }));
      return;
    }
    const origin: ChangeOrigin = event.isTrusted ? "ui" : "program";
    const paths = eventCanonicalPaths(event);
    const cleared: string[] = [];
    refreshPresence(paths, "edit");
    if (reactive) {
      for (const path of settleInBoundary(origin)) {
        cleared.push(path);
        if (!paths.includes(path)) {
          paths.push(path);
        }
      }
    }
    refreshPresence(cleared, "clear");
    notify(event, withApplicability({ origin, paths }));
  };

  const handleArrayChange = (event: Event): void => {
    if (destroyed) {
      return;
    }
    const detail = (event as CustomEvent<ArrayChangeDetail>).detail;
    const rowPath = detail?.path ? toCanonicalPath(toDotKey(detail.path)) : "";
    if (rowPath && embeddedModel && lookupField(embeddedModel, rowPath)) {
      if (detail.kind === "remove") inactiveRows.add(rowPath);
      else inactiveRows.delete(rowPath);
    }
    seedNewPresence();
    const paths: string[] = [];
    const arrayPath = toCanonicalPath(toDotKey(stripArraySuffix(detail?.name ?? "")));
    if (arrayPath) {
      paths.push(arrayPath);
    }
    for (const cleared of settleInBoundary("program")) {
      if (!paths.includes(cleared)) {
        paths.push(cleared);
      }
    }
    seedNewPresence();
    const meta = withApplicability({ origin: "program", paths });
    notify(createControllerChangeEvent(meta), meta);
  };

  const startListening = (): void => {
    if (listening) {
      return;
    }
    listening = true;
    resolvedRoot.addEventListener("input", handleNativeEvent, true);
    resolvedRoot.addEventListener("change", handleNativeEvent, true);
    resolvedRoot.addEventListener(ARRAY_CHANGE_EVENT, handleArrayChange, true);
  };

  const stopListening = (): void => {
    if (!listening) {
      return;
    }
    listening = false;
    resolvedRoot.removeEventListener("input", handleNativeEvent, true);
    resolvedRoot.removeEventListener("change", handleNativeEvent, true);
    resolvedRoot.removeEventListener(ARRAY_CHANGE_EVENT, handleArrayChange, true);
  };

  const runMutation = (
    origin: ChangeOrigin,
    hydration: HydrationOptions | undefined,
    work: (run: HydrationRun) => void
  ): HydrationRun => {
    const settled = hydration !== undefined;
    const before = settled && embeddedModel ? new Map(presence) : null;
    const run: HydrationRun = {
      mode: hydration?.mode === "replace" ? "replace" : "patch",
      settled,
      changed: new Set<string>(),
      model: embeddedModel ?? undefined,
      normalization: options.normalization,
    };
    const boundary: MutationBoundary = { origin, settled };
    boundaries.push(boundary);
    try {
      work(run);
      if (reactive) {
        settleConditions(run);
      }
    } finally {
      const position = boundaries.lastIndexOf(boundary);
      if (position >= 0) {
        boundaries.splice(position, 1);
      }
    }
    if (before) {
      // Native controls cannot represent absent/null/false as distinct values.
      // Compare logical entries at the transaction boundary before publication.
      for (const path of new Set([...before.keys(), ...presence.keys()])) {
        const previous = before.get(path);
        const current = presence.get(path);
        if (previous?.state !== current?.state || !jsonValueEqual(previous?.value, current?.value) || !jsonValueEqual(previous?.issues, current?.issues)) {
          run.changed.add(path);
        }
      }
    }
    if (!settled || hydration?.notify === false || destroyed || run.changed.size === 0) {
      return run;
    }
    const meta = withApplicability({ origin, paths: Array.from(run.changed) });
    notify(createControllerChangeEvent(meta), meta);
    return run;
  };

  const controller: FormController = {
    root: resolvedRoot,
    getValues(readOptions = {}) {
      if (destroyed) {
        return {};
      }
      return collectControllerValues(resolvedRoot, {
        includeHidden: readOptions.includeHidden ?? options.includeHidden ?? true,
      });
    },
    setValues(values, hydration) {
      if (destroyed) {
        return;
      }
      runMutation("hydrate", hydration, (run) => {
        hydrateFormValuesWithRegistry(resolvedRoot, { values }, controllerRegistry, run);
        recordHydrationPresence(run);
      });
    },
    reset(hydration) {
      if (destroyed) {
        return;
      }
      if (hydration === undefined) {
        runMutation("reset", undefined, (run) => {
          run.enumValues = "control";
          resetControllerFields(resolvedRoot);
          const values = collectControllerValues(resolvedRoot, {
            includeHidden: options.includeHidden ?? true,
          });
          hydrateFormValuesWithRegistry(resolvedRoot, { values }, controllerRegistry, run);
          hydrateFormValuesWithRegistry(
            resolvedRoot,
            { errors: buildClearErrorPayload(resolvedRoot) },
            controllerRegistry,
            run
          );
          refreshPresence(allLeafPaths(), "reset");
        });
        return;
      }
      runMutation("reset", { notify: hydration.notify }, (run) => {
        run.enumValues = "control";
        hydrateFormValuesWithRegistry(
          resolvedRoot,
          { values: collectDefaultHydrationValues(resolvedRoot) },
          controllerRegistry,
          run
        );
        hydrateFormValuesWithRegistry(
          resolvedRoot,
          { errors: buildClearErrorPayload(resolvedRoot) },
          controllerRegistry,
          run
        );
        refreshPresence(allLeafPaths(), "reset");
      });
    },
    setErrors(errors) {
      if (!destroyed) {
        hydrateFormValuesWithRegistry(resolvedRoot, { errors }, controllerRegistry);
        syncInvalidGroups(resolvedRoot);
      }
    },
    clearErrors(names) {
      if (destroyed) {
        return;
      }
      const payload = buildClearErrorPayload(resolvedRoot, names);
      hydrateFormValuesWithRegistry(resolvedRoot, { errors: payload }, controllerRegistry);
      syncInvalidGroups(resolvedRoot);
    },
    setIssues(issues, presentOptions = {}) {
      const presentation: IssuePresentation = { field: [], summary: [] };
      if (destroyed) {
        return presentation;
      }
      const fieldMessages: Record<string, string[]> = {};
      for (const issue of Array.isArray(issues) ? issues : []) {
        if (!issue || typeof issue !== "object") {
          continue;
        }
        const path = typeof issue.path === "string" ? issue.path.trim() : "";
        if (path && findControllerField(resolvedRoot, path)) {
          const key = toDotKey(stripArraySuffix(path));
          (fieldMessages[key] ??= []).push(issueMessage(issue));
          presentation.field.push(issue);
          continue;
        }
        presentation.summary.push(issue);
      }
      if (!presentOptions.append) {
        hydrateFormValuesWithRegistry(
          resolvedRoot,
          { errors: buildClearErrorPayload(resolvedRoot) },
          controllerRegistry
        );
      }
      if (Object.keys(fieldMessages).length > 0) {
        hydrateFormValuesWithRegistry(resolvedRoot, { errors: fieldMessages }, controllerRegistry);
      }
      if (presentOptions.summary !== false) {
        const messages = presentation.summary.map(issueMessage);
        renderFormErrors(
          resolvedRoot,
          presentOptions.append ? currentFormErrors(resolvedRoot).concat(messages) : messages
        );
      }
      syncInvalidGroups(resolvedRoot);
      return presentation;
    },
    clearIssues() {
      if (destroyed) {
        return;
      }
      hydrateFormValuesWithRegistry(
        resolvedRoot,
        { errors: buildClearErrorPayload(resolvedRoot) },
        controllerRegistry
      );
      renderFormErrors(resolvedRoot, []);
      syncInvalidGroups(resolvedRoot);
    },
    focusIssue(target) {
      const path = typeof target === "string" ? target.trim() : (target?.path ?? "").trim();
      const result: IssueFocusResult = { focused: false, target: null, path };
      if (destroyed) {
        return result;
      }
      if (path) {
        const control = findFocusableControllerField(resolvedRoot, path);
        if (control) {
          control.focus();
          result.focused = true;
          result.target = "control";
          return result;
        }
      }
      const summary = findFormErrorContainer(resolvedRoot);
      if (summary && !summary.hidden) {
        if (!summary.hasAttribute("tabindex")) {
          summary.tabIndex = -1;
        }
        summary.focus();
        result.focused = true;
        result.target = "summary";
      }
      return result;
    },
    onChange(callback) {
      if (destroyed || typeof callback !== "function") {
        return () => undefined;
      }
      const subscription: ChangeSubscription = { callback, active: true };
      subscriptions.push(subscription);
      startListening();
      return () => {
        if (!subscription.active) {
          return;
        }
        subscription.active = false;
        const position = subscriptions.indexOf(subscription);
        if (position >= 0) {
          subscriptions.splice(position, 1);
        }
        if (subscriptions.length === 0 && !reactive && !embeddedModel) {
          stopListening();
        }
      };
    },
    focus(name) {
      if (destroyed) {
        return false;
      }
      const field = findControllerField(resolvedRoot, name);
      if (!field || typeof field.focus !== "function") {
        return false;
      }
      field.focus();
      return true;
    },
    destroy() {
      if (destroyed) {
        return;
      }
      destroyed = true;
      stopListening();
      subscriptions.forEach((subscription) => {
        subscription.active = false;
      });
      subscriptions.length = 0;
      boundaries.length = 0;
      lastProjection = null;
      presence.clear();
      retainedLeaves.clear();
      inactiveRows.clear();
      choices = new WeakMap();
      controllerRegistry?.destroy(resolvedRoot);
    },
    normalize(readOptions = {}) {
      const model = requireModel("normalize");
      if (destroyed) {
        return { values: {}, issues: [] };
      }
      return normalizeValues(model, collectControllerValues(resolvedRoot, {
        includeHidden: readOptions.includeHidden ?? options.includeHidden ?? true,
        readValue: readTypedControlValue,
      }), { ...options.normalization, enumValues: "literal" });
    },
    validate(scope) {
      const model = requireModel("validate");
      if (destroyed) {
        return [];
      }
      const normalized = readTypedState();
      const selected = scope?.paths;
      const parseIssues =
        selected === undefined
          ? normalized.issues
          : normalized.issues.filter(
              (issue) => issue.path !== undefined && selected.some((path) => pathWithin(issue.path!, path))
            );
      if (model.conditions == null) {
        return parseIssues.concat(validateValues(model, normalized.values, scope, { inactive: normalized.inactivePaths }));
      }
      // The runtime owns inapplicable controls: it clears them (clear policy) or
      // keeps them aside (retain), and raw collection still reports rendered
      // defaults such as an unchecked box or an empty nested object. None of
      // that is a supplied answer, so every inactive path is stripped before the
      // server-equivalent composition; hosts send effective().values upstream.
      let supplied = normalized.values;
      if (compiled.program) {
        const projection = compiled.program.evaluate(normalized.values, normalized.inactivePaths);
        const strip = Object.keys(projection.applicable).filter((path) => !projection.applicable[path]);
        if (strip.length > 0) {
          supplied = cloneJSON(normalized.values) as Record<string, JSONValue>;
          strip.forEach((path) => removeValueAt(supplied, path));
        }
      }
      const result = validateConditional(model, supplied, {
        scope,
        policy: options.conditions?.policy ?? "reject",
        compiled,
        inactivePaths: normalized.inactivePaths,
      });
      return parseIssues.concat(result.issues);
    },
    effective() {
      requireModel("effective");
      const normalized = destroyed ? { values: {}, issues: [], inactivePaths: [] } : readTypedState();
      if (!compiled.program) {
        return { values: normalized.values, retained: {}, applicable: {}, required: {} };
      }
      return overlayRetained(compiled.program.evaluate(normalized.values, normalized.inactivePaths));
    },
    settle() {
      const model = requireModel("settle");
      if (destroyed) {
        return controller.effective();
      }
      seedNewPresence();
      if (!reactive) {
        return controller.effective();
      }
      settleInBoundary("program");
      return lastProjection ?? { values: normalizeValues(model, controller.getValues(), options.normalization).values, retained: {}, applicable: {}, required: {} };
    },
    conditionDiagnostics() {
      return compiled.diagnostics.map((diagnostic) => ({ ...diagnostic }));
    },
    snapshot() {
      requireModel("snapshot");
      const empty: TypedSnapshot = { version: "formgen.snapshot/v1", values: {}, presence: {}, retained: {}, issues: [], locked: [] };
      if (destroyed) {
        return empty;
      }
      const state = readTypedState(true);
      const { values: base, issues, presence: presenceOut, locked } = state;
      const projection = compiled.program ? overlayRetained(compiled.program.evaluate(base, state.inactivePaths), state) : null;
      return {
        version: "formgen.snapshot/v1",
        values: projection ? projection.values : base,
        presence: presenceOut,
        retained: cloneJSON(projection?.retained ?? {}) as Record<string, JSONValue>,
        issues,
        locked,
      };
    },
    restore(snapshot, restoreOptions = {}) {
      requireModel("restore");
      const issues: EmbeddedIssue[] = [];
      if (destroyed) {
        return issues;
      }
      // Reject malformed snapshot instructions atomically. In replace mode,
      // simply skipping an invalid presence entry would still clear its field
      // as an omitted value, turning corrupt state into an intentional clear.
      if (!snapshot || snapshot.version !== "formgen.snapshot/v1") {
        return [snapshotIssue("invalidSnapshot", "", "snapshot version must be formgen.snapshot/v1")];
      }
      for (const key of ["presence", "values", "retained"] as const) {
        const value = snapshot[key];
        if (!value || typeof value !== "object" || Array.isArray(value)) {
          issues.push(snapshotIssue("invalidSnapshot", "", `snapshot ${key} must be an object`));
        }
      }
      if (issues.length > 0) return issues;
      const presenceIn = snapshot.presence;
      for (const [path, state] of Object.entries(presenceIn)) {
        if (state !== "present" && state !== "null" && state !== "absent") {
          issues.push(snapshotIssue("invalidSnapshot", path, "snapshot presence must be present, null or absent"));
        }
      }
      if (issues.length > 0) return issues;
      const payload: Record<string, unknown> = {};
      const states = new Map<string, PresenceEntry>();
      const index = indexLeafControls();
      const validPath = (key: string, container = false): boolean => {
        if (!parseCanonicalPath(key)) {
          issues.push(snapshotIssue("invalidPath", key, "snapshot path is not a canonical field path"));
          return false;
        }
        if (!lookupField(embeddedModel!, key) || !(container
          ? Array.from(index.keys()).some((leaf) => pathWithin(leaf, key))
          : index.has(key))) {
          issues.push(snapshotIssue("unknownField", key, "snapshot path has no rendered control"));
          return false;
        }
        return true;
      };
      const retained = new Map<string, unknown>();
      for (const [path, value] of Object.entries(snapshot.retained ?? {})) {
        if (validPath(path, true)) retained.set(path, value);
      }
      const retainedPaths = Array.from(retained.keys()).sort((a, b) => b.length - a.length);
      const retainedAt = (leaf: string): { exists: boolean; value: unknown } => {
        for (const path of retainedPaths) {
          if (!pathWithin(leaf, path)) continue;
          if (leaf === path) return { exists: true, value: retained.get(path) };
          // Read only rendered leaf coordinates; never materialize retained indexes.
          const found = getValueAt({ root: retained.get(path) }, `root${leaf.slice(path.length)}`);
          if (found.exists) return found;
        }
        return { exists: false, value: undefined };
      };
      for (const key of Object.keys(presenceIn)) {
        const state = presenceIn[key];
        if (!validPath(key)) continue;
        if (state === "present") {
          // Inactive retained paths keep their value in `retained`, not `values`.
          const found = getValueAt(snapshot.values ?? {}, key);
          const stored = retainedAt(key);
          if (!found.exists && !stored.exists) {
            issues.push(snapshotIssue("invalidSnapshot", key, "present snapshot path has no value or retained value"));
            continue;
          }
          const value = found.exists ? cloneJSON(found.value) : stored.exists ? cloneJSON(stored.value) : null;
          payload[key] = value;
          states.set(key, value === null ? { state: "null" } : { state: "present", value });
        } else if (state === "null") {
          payload[key] = null;
          states.set(key, { state: "null" });
        } else {
          // Absence is an explicit restore instruction, including locked group
          // members; omission alone would preserve their stale selections.
          payload[key] = null;
          states.set(key, { state: "absent" });
        }
      }
      for (const key of index.keys()) {
        const stored = retainedAt(key);
        if (stored.exists && !Object.prototype.hasOwnProperty.call(payload, key)) payload[key] = cloneJSON(stored.value);
      }
      if (issues.some((issue) => issue.code === "invalidSnapshot")) return issues;
      // Snapshots carry normalization failures alongside their typed nulls.
      // A null cannot reconstruct the rejected source; retain its diagnostic
      // until an observed edit/reset/hydration replaces that logical entry.
      const savedIssues = new Map<string, EmbeddedIssue[]>();
      for (const issue of Array.isArray(snapshot.issues) ? snapshot.issues : []) {
        if (!issue || typeof issue.path !== "string" || typeof issue.code !== "string" || typeof issue.message !== "string") continue;
        if (!parseCanonicalPath(issue.path)) {
          issues.push(snapshotIssue("invalidPath", issue.path, "snapshot issue path is not a canonical field path"));
          continue;
        }
        const owner = Array.from(states.keys()).filter((leaf) => pathWithin(issue.path!, leaf)).sort((a, b) => b.length - a.length)[0];
        if (!owner) continue;
        const list = savedIssues.get(owner) ?? [];
        list.push(snapshotIssue(issue.code, issue.path, issue.message));
        savedIssues.set(owner, list);
      }
      runMutation("hydrate", { mode: restoreOptions.mode ?? "replace", notify: restoreOptions.notify }, (run) => {
        run.enumValues = "literal";
        hydrateFormValuesWithRegistry(resolvedRoot, { values: payload }, controllerRegistry, run);
        recordHydrationPresence(run);
        states.forEach((entry, key) => {
          // Present values and their issues were normalized by the shared
          // hydration path. Only restore the null/absent distinction here.
          if (entry.state !== "present") presence.set(key, entry);
        });
        savedIssues.forEach((saved, path) => {
          const entry = presence.get(path);
          if (!entry) return;
          const combined = [...(entry.issues ?? [])];
          for (const issue of saved) {
            if (!combined.some((existing) => jsonValueEqual(existing, issue))) combined.push(issue);
          }
          presence.set(path, { ...entry, issues: combined });
        });
      });
      return issues;
    },
  };

  if (embeddedModel) {
    startListening();
    refreshPresence(allLeafPaths(), "attach");
  }
  if (reactive) {
    startListening();
    settleInBoundary("program");
  }

  return controller;
}

export const Formgen = {
  attach: attachFormController,
  hydrate: hydrateFormValues,
  normalize: normalizeValues,
  validate: validateValues,
};

if (typeof globalThis !== "undefined") {
  (globalThis as Record<string, unknown>).Formgen = Formgen;
}

export function hydrateFormValues(
  root: Document | HTMLElement = document,
  payload: HydrationPayload = {}
): void {
  hydrateFormValuesWithRegistry(root, payload, activeRegistry);
}

function hydrateFormValuesWithRegistry(
  root: Document | HTMLElement,
  payload: HydrationPayload,
  registry: ResolverRegistry | null,
  run: HydrationRun = legacyHydrationRun()
): void {
  const scope = root ?? document;
  const fields = locateHydratableFields(scope);
  if (fields.length === 0) {
    return;
  }

  const valueIndex = buildPayloadIndex(payload.values, run.model !== undefined);
  const errorIndex = buildPayloadIndex(payload.errors);
  const replace = run.mode === "replace" && payload.values !== undefined;
  if (replace) {
    assertReplaceStructure(scope, fields, payload.values ?? {});
  }

  fields.forEach((element) => {
    applyHydratedValue(element, valueIndex, registry, run, replace && isReplaceManaged(element));
    applyHydratedErrors(element, errorIndex, registry);
  });
}

function legacyHydrationRun(): HydrationRun {
  return { mode: "patch", settled: false, changed: new Set<string>() };
}

function resolveControllerRoot(root: Document | HTMLElement | string): Document | HTMLElement {
  if (typeof root === "string") {
    const found = document.querySelector<HTMLElement>(root);
    if (!found) {
      throw new Error(`formgen: controller root not found for selector ${root}`);
    }
    return found;
  }
  return root ?? document;
}

interface CollectOptions extends FormControllerOptions {
  /** Include disabled controls (presence seeding only; raw collection stays legacy). */
  includeDisabled?: boolean;
  readValue?: (control: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement) => unknown;
}

function collectControllerValues(
  root: Document | HTMLElement,
  options: CollectOptions
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  controllerFields(root).forEach((field) => {
    if (!shouldCollectField(field, options)) {
      return;
    }
    const name = fieldName(field);
    if (!name) {
      return;
    }
    const value = options.readValue ? options.readValue(field) : readControllerValue(field);
    if (value === undefined) {
      return;
    }
    assignControllerValue(out, toDotKey(stripArraySuffix(name)), value);
  });
  return out;
}

function controllerFields(root: Document | HTMLElement): Array<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement> {
  const fields = new Set<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>();
  if (root instanceof HTMLInputElement || root instanceof HTMLSelectElement || root instanceof HTMLTextAreaElement) {
    fields.add(root);
  }
  root
    .querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>("input, select, textarea")
    .forEach((field) => fields.add(field));
  return Array.from(fields);
}

function resetControllerFields(root: Document | HTMLElement): void {
  controllerFields(root).forEach((field) => {
    if (field instanceof HTMLInputElement) {
      const type = field.type.toLowerCase();
      if (type === "checkbox" || type === "radio") {
        field.checked = field.defaultChecked;
      } else if (type !== "file") {
        field.value = field.defaultValue;
      }
      return;
    }
    if (field instanceof HTMLTextAreaElement) {
      field.value = field.defaultValue;
      return;
    }
    Array.from(field.options).forEach((option) => {
      option.selected = option.defaultSelected;
    });
    if (!field.multiple && field.selectedIndex < 0 && field.options.length > 0) {
      field.selectedIndex = 0;
    }
  });
}

function shouldCollectField(
  field: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement,
  options: CollectOptions
): boolean {
  if (isDisabledControl(field) && !options.includeDisabled) {
    return false;
  }
  if (field instanceof HTMLInputElement) {
    // Relationship renderers create hidden inputs to preserve native form
    // submission semantics. The select remains the authoritative controller
    // field; collecting its mirrors would overwrite arrays with scalar or
    // serialized values when the controller walks the DOM.
    if (field.closest("[data-relationship-hidden]")) {
      return false;
    }
    const type = field.type.toLowerCase();
    if (["button", "submit", "reset", "file"].includes(type)) {
      return false;
    }
    if (type === "hidden" && options.includeHidden === false) {
      return false;
    }
  }
  return Boolean(fieldName(field));
}

function fieldName(field: HTMLElement): string {
  return (
    field.getAttribute("name") ||
    field.getAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR) ||
    field.getAttribute("data-field-path") ||
    field.dataset.fieldName ||
    field.id ||
    ""
  ).trim();
}

function readControllerValue(
  field: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
): unknown {
  if (field instanceof HTMLSelectElement) {
    if (field.multiple) {
      return Array.from(field.selectedOptions).map((option) => option.value);
    }
    return field.value;
  }
  if (field instanceof HTMLInputElement) {
    const type = field.type.toLowerCase();
    if (type === "checkbox") {
      if (field.name.endsWith("[]")) {
        if (!field.checked) {
          return undefined;
        }
        return [field.value];
      }
      return field.checked;
    }
    if (type === "radio") {
      if (!field.checked) {
        return undefined;
      }
      return field.value;
    }
  }
  return field.value;
}

/** Only native option controls carry the enum transport codec. Text/JSON
 * controls contain literal strings, even when their text resembles a token. */
function carriesEnumControlValue(element: HTMLElement): boolean {
  return element instanceof HTMLSelectElement || isChoiceMember(element);
}

function decodeControlSelection(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(decodeControlSelection);
  return typeof value === "string" ? decodeEnumValue(value).value : value;
}

function readTypedControlValue(control: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement): unknown {
  const value = readControllerValue(control);
  return carriesEnumControlValue(control) ? decodeControlSelection(value) : value;
}

function assignControllerValue(target: Record<string, unknown>, path: string, value: unknown): void {
  const parts = safeControllerPathSegments(path);
  if (parts.length === 0) {
    return;
  }
  let cursor: Record<string, unknown> = target;
  parts.forEach((part, index) => {
    if (index === parts.length - 1) {
      const existing = Object.prototype.hasOwnProperty.call(cursor, part)
        ? cursor[part]
        : undefined;
      if (Array.isArray(existing) && Array.isArray(value)) {
        cursor[part] = existing.concat(value);
      } else {
        cursor[part] = value;
      }
      return;
    }
    if (
      !Object.prototype.hasOwnProperty.call(cursor, part) ||
      !cursor[part] ||
      typeof cursor[part] !== "object" ||
      Array.isArray(cursor[part])
    ) {
      cursor[part] = {};
    }
    cursor = cursor[part] as Record<string, unknown>;
  });
}

function buildClearErrorPayload(
  root: Document | HTMLElement,
  names?: string[]
): Record<string, string[]> {
  const payload: Record<string, string[]> = {};
  const targets = names && names.length > 0 ? names : controllerFields(root).map(fieldName);
  targets.forEach((name) => {
    const trimmed = name.trim();
    const dotted = toDotKey(stripArraySuffix(trimmed));
    if (safeControllerPathSegments(dotted).length > 0) {
      payload[dotted] = [];
    }
  });
  return payload;
}

function findControllerField(
  root: Document | HTMLElement,
  name: string
): HTMLElement | null {
  return controllerFieldsForPath(root, name)[0] ?? null;
}

function controllerFieldsForPath(root: Document | HTMLElement, name: string): HTMLElement[] {
  const variants = new Set<string>();
  const trimmed = name.trim();
  if (!trimmed) {
    return [];
  }
  const dotted = toDotKey(stripArraySuffix(trimmed));
  if (safeControllerPathSegments(dotted).length === 0) {
    return [];
  }
  [trimmed, dotted, toBracketKey(dotted), `${toBracketKey(dotted)}[]`].forEach((item) => {
    if (item) {
      variants.add(item);
    }
  });
  return controllerFields(root).filter((field) => collectFieldKeys(field).some((key) => variants.has(key)));
}

function findFocusableControllerField(root: Document | HTMLElement, name: string): HTMLElement | null {
  return controllerFieldsForPath(root, name).find((field) => isFocusableControl(field, root)) ?? null;
}

function isFocusableControl(field: HTMLElement, root: Document | HTMLElement): boolean {
  if (isDisabledControl(field)) {
    return false;
  }
  if (field instanceof HTMLInputElement && field.type.toLowerCase() === "hidden") {
    return false;
  }
  if (typeof field.focus !== "function" || field.closest("[hidden]")) {
    return false;
  }
  let node: HTMLElement | null = field;
  while (node && node !== root) {
    if (node.style && (node.style.display === "none" || node.style.visibility === "hidden")) {
      return false;
    }
    if (typeof getComputedStyle === "function") {
      try {
        const computed = getComputedStyle(node);
        if (computed.display === "none" || computed.visibility === "hidden") {
          return false;
        }
      } catch (_err) {
        // Environments without layout support fall back to attribute checks.
      }
    }
    node = node.parentElement;
  }
  return true;
}

function controlsWithinPath(root: Document | HTMLElement, path: string): HTMLElement[] {
  return controllerFields(root).filter((field) => {
    const controlPath = canonicalPathForElement(field);
    return controlPath !== "" && pathWithin(controlPath, path);
  });
}

/** Row membership is structural, not evidence of an answered leaf. Native
 * radios omit unanswered values and intent controls are transport-only, so
 * rebuild only the coordinates actually rendered, before overlaying presence.
 * Empty object rows remain {}, primitive rows remain unanswered null slots.
 */
function seedRenderedRowValues(
  values: Record<string, JSONValue>,
  root: Document | HTMLElement,
  model: EmbeddedModel,
  leafPaths: Iterable<string>,
  inactiveRows: ReadonlySet<string>
): void {
  const paths = new Set(leafPaths);
  root.querySelectorAll<HTMLElement>("[data-formgen-array-item]").forEach((row) => {
    const path = arrayItemPath(row);
    if (path) paths.add(toCanonicalPath(toDotKey(path)));
  });
  const seeded = new Set<string>();
  for (const path of paths) {
    const segments = parseCanonicalPath(path);
    if (!segments?.some((segment) => segment.isIndex) || !lookupField(model, path)) continue;
    segments.forEach((segment, position) => {
      if (!segment.isIndex) return;
      const rowPath = formatCanonicalPath(segments.slice(0, position + 1));
      if (seeded.has(rowPath) || Array.from(inactiveRows).some((inactive) => pathWithin(rowPath, inactive))) return;
      seeded.add(rowPath);
      const field = lookupField(model, rowPath);
      if (!field) return;
      const found = getValueAt(values, rowPath);
      if (!found.exists || found.value === null) {
        setValueAt(values, rowPath, field.type === "object" ? {} : field.type === "array" ? [] : null);
      }
    });
  }
}

function controlHasValue(control: HTMLElement): boolean {
  if (control instanceof HTMLInputElement) {
    const type = control.type.toLowerCase();
    if (type === "checkbox" || type === "radio") {
      return control.checked;
    }
    return control.value !== "";
  }
  if (control instanceof HTMLSelectElement) {
    return Array.from(control.selectedOptions).some((option) => option.value !== "");
  }
  if (control instanceof HTMLTextAreaElement) {
    return control.value !== "";
  }
  return false;
}

function setControlRequired(control: HTMLElement, required: boolean): void {
  if (
    control instanceof HTMLInputElement ||
    control instanceof HTMLSelectElement ||
    control instanceof HTMLTextAreaElement
  ) {
    // A native or ARIA `required` on a checkbox group member would demand every
    // box; checkbox groups expose requiredness through their legend hint instead.
    const checkboxGroup =
      control instanceof HTMLInputElement && control.type.toLowerCase() === "checkbox" && control.name.endsWith("[]");
    // Radio members keep native `required` (any required member makes the group
    // required) but never `aria-required`: ARIA supports it on the radiogroup
    // fieldset, which is set below, not on `radio`.
    const radioMember = control instanceof HTMLInputElement && control.type.toLowerCase() === "radio";
    if (!checkboxGroup) {
      if (control.required !== required) {
        control.required = required;
      }
      if (!radioMember) {
        if (required) {
          control.setAttribute("aria-required", "true");
        } else {
          control.removeAttribute("aria-required");
        }
      }
    }
    const group = control.closest<HTMLElement>("[data-formgen-choice]");
    if (group) {
      if (required) {
        group.setAttribute("data-formgen-required", "true");
      } else {
        group.removeAttribute("data-formgen-required");
      }
      // aria-required is only valid on radiogroup, not on a plain fieldset group.
      if (group.getAttribute("role") === "radiogroup") {
        if (required) {
          group.setAttribute("aria-required", "true");
        } else {
          group.removeAttribute("aria-required");
        }
      }
      const hint = group.querySelector<HTMLElement>("[data-formgen-required-hint]");
      if (hint) {
        hint.hidden = !required;
      }
    }
  }
}

/**
 * Applicability chrome targets the control's component wrapper (or parent) and,
 * for container rules, the matching group element (`fg-<path>`). Markers are
 * always set; `hidden` follows the hideInapplicable option.
 */
function applyApplicabilityChrome(
  root: Document | HTMLElement,
  path: string,
  controls: HTMLElement[],
  active: boolean,
  hide: boolean
): void {
  const targets = new Set<HTMLElement>();
  for (const control of controls) {
    control.setAttribute("data-formgen-applicable", String(active));
    const wrapper = control.closest<HTMLElement>("[data-component]") ?? control.parentElement;
    if (wrapper && wrapper !== root && !(root instanceof HTMLElement && wrapper.contains(root))) {
      targets.add(wrapper);
    }
  }
  const groupID = controlIDForPath(path);
  if (groupID) {
    const group = root.querySelector<HTMLElement>(`#${safeSelectorValue(groupID)}`);
    if (
      group &&
      group !== root &&
      !(group instanceof HTMLInputElement) &&
      !(group instanceof HTMLSelectElement) &&
      !(group instanceof HTMLTextAreaElement)
    ) {
      targets.add(group);
    }
  }
  targets.forEach((target) => {
    target.setAttribute("data-formgen-applicable", String(active));
    if (!hide) {
      return;
    }
    if (active) {
      if (target.getAttribute("data-formgen-hidden-by") === "applicability") {
        target.removeAttribute("hidden");
        target.removeAttribute("data-formgen-hidden-by");
      }
    } else if (!target.hasAttribute("hidden")) {
      target.setAttribute("hidden", "");
      target.setAttribute("data-formgen-hidden-by", "applicability");
    }
  });
}

function controlIDForPath(path: string): string {
  const sanitized = path
    .split("[]")
    .join(".item")
    .trim()
    .replace(/[^A-Za-z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return sanitized ? `fg-${sanitized}` : "";
}

/** Move focus off a control that applicability just hid; prefer the summary, else blur. */
function relocateHiddenFocus(root: Document | HTMLElement): void {
  if (typeof document === "undefined") {
    return;
  }
  const active = document.activeElement;
  if (!(active instanceof HTMLElement) || !active.closest('[data-formgen-hidden-by="applicability"], [hidden]')) {
    return;
  }
  if (!root.contains(active)) {
    return;
  }
  active.blur();
  const summary = findFormErrorContainer(root);
  if (summary && !summary.hidden) {
    if (!summary.hasAttribute("tabindex")) {
      summary.tabIndex = -1;
    }
    summary.focus();
  }
}

/** Native disabled inheritance, including each fieldset's first legend exemption. */
function isDisabledControl(control: HTMLElement): boolean {
  if (!(control instanceof HTMLInputElement || control instanceof HTMLSelectElement || control instanceof HTMLTextAreaElement)) {
    return false;
  }
  if (control.disabled) {
    return true;
  }
  for (let parent = control.parentElement; parent; parent = parent.parentElement) {
    if (parent instanceof HTMLFieldSetElement && parent.disabled) {
      const legend = Array.from(parent.children).find((child) => child instanceof HTMLLegendElement);
      if (!legend?.contains(control)) {
        return true;
      }
    }
  }
  return false;
}

function isChoiceMember(control: HTMLElement): control is HTMLInputElement {
  return control instanceof HTMLInputElement &&
    (control.type === "radio" || (control.type === "checkbox" && control.name.endsWith("[]")));
}

function snapshotIssue(code: string, path: string, message: string): EmbeddedIssue {
  return { code, path, rendererPath: rendererPath(path), message };
}

function issueMessage(issue: EmbeddedIssue): string {
  const message = typeof issue.message === "string" ? issue.message.trim() : "";
  return message || String(issue.code ?? "invalid");
}

function locateHydratableFields(root: Document | HTMLElement): HTMLElement[] {
  const nodes = new Set<HTMLElement>();
  const scope = root instanceof Document ? root : root;
  if (root instanceof HTMLElement && isHydratableElement(root)) {
    nodes.add(root);
  }

  const selector = "input, select, textarea, [data-field-name], [data-field-path]";
  scope.querySelectorAll<HTMLElement>(selector).forEach((element) => {
    if (isHydratableElement(element)) {
      nodes.add(element);
    }
  });

  locateRelationshipFields(scope).forEach((element) => nodes.add(element));
  return Array.from(nodes);
}

function isHydratableElement(element: Element): element is HTMLElement {
  if (element instanceof HTMLInputElement) {
    if (element.type === "hidden") {
      if (element.hasAttribute("data-relationship-hidden") || element.hasAttribute("data-relationship-json")) {
        return false;
      }
      return Boolean(element.dataset.fieldName || element.dataset.fieldPath);
    }
    return true;
  }
  return element instanceof HTMLSelectElement || element instanceof HTMLTextAreaElement;
}

function buildPayloadIndex(
  source?: Record<string, unknown>,
  includeContainers = false
): Map<string, unknown> {
  const index = new Map<string, unknown>();
  if (!source) {
    return index;
  }
  const flattened = new Map<string, unknown>();
  flattenPayload(source, flattened, "", includeContainers);
  flattened.forEach((value, key) => {
    addKeyVariants(index, key, value);
  });
  return index;
}

function flattenPayload(
  input: Record<string, unknown>,
  target: Map<string, unknown>,
  prefix: string,
  includeContainers = false
): void {
  Object.entries(input).forEach(([key, value]) => {
    const trimmedKey = key.trim();
    if (!trimmedKey) {
      return;
    }
    const nextKey = prefix ? `${prefix}.${trimmedKey}` : trimmedKey;
    if (safeControllerPathSegments(nextKey).length === 0) {
      return;
    }
    if (Array.isArray(value)) {
      target.set(nextKey, value);
      // Arrays of objects also address rendered repeated rows by index while the
      // array itself stays addressable for multi-value controls.
      value.forEach((item, position) => {
        const itemKey = `${nextKey}.${position}`;
        if (includeContainers) {
          // Structural membership exists even when the row has no answer leaves.
          flattenPayload({ [String(position)]: item }, target, nextKey, true);
        } else if (item && typeof item === "object" && !Array.isArray(item)) {
          flattenPayload(item as Record<string, unknown>, target, itemKey);
        }
      });
      return;
    }
    if (value && typeof value === "object") {
      if (includeContainers) target.set(nextKey, value);
      flattenPayload(value as Record<string, unknown>, target, nextKey, includeContainers);
      return;
    }
    target.set(nextKey, value);
  });
}

function addKeyVariants(
  index: Map<string, unknown>,
  key: string,
  value: unknown
): void {
  const variants = new Set<string>();
  const canonical = key.trim();
  if (!canonical) {
    return;
  }
  const stripped = stripArraySuffix(canonical);
  const dotted = toDotKey(stripped);
  const bracket = toBracketKey(dotted);

  [canonical, stripped, dotted, bracket].forEach((entry) => {
    if (entry) {
      variants.add(entry);
    }
  });

  if (Array.isArray(value)) {
    [canonical, stripped, dotted, bracket].forEach((entry) => {
      if (entry) {
        variants.add(`${entry}[]`);
      }
    });
  }

  variants.forEach((variant) => {
    if (variant) {
      index.set(variant, value);
    }
  });
}

function stripArraySuffix(value: string): string {
  return value.endsWith("[]") ? value.slice(0, -2) : value;
}

function toDotKey(value: string): string {
  if (!value.includes("[")) {
    return value.replace(/\[\]/g, "");
  }
  return value
    .replace(/\[\]/g, "")
    .replace(/\[([^\]]+)\]/g, ".$1")
    .replace(/^\./, "");
}

const unsafeControllerPathSegments = new Set(["__proto__", "constructor", "prototype"]);

function safeControllerPathSegments(value: string): string[] {
  const dotted = toDotKey(stripArraySuffix(value));
  const segments = dotted.split(".").map((segment) => segment.trim()).filter(Boolean);
  if (segments.length === 0 || segments.some((segment) => unsafeControllerPathSegments.has(segment))) {
    return [];
  }
  return segments;
}

function toBracketKey(value: string): string {
  if (!value.includes(".")) {
    return value;
  }
  const segments = value.split(".").filter(Boolean);
  if (segments.length === 0) {
    return value;
  }
  const [first, ...rest] = segments;
  return `${first}${rest.map((segment) => `[${segment}]`).join("")}`;
}

function collectFieldKeys(element: HTMLElement): string[] {
  const keys = new Set<string>();
  const candidates = [
    element.getAttribute("name"),
    element.getAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR),
    element.dataset.fieldName,
    element.getAttribute("data-field-path"),
    element.id,
  ];

  candidates.forEach((value) => {
    if (!value) {
      return;
    }
    const dotted = toDotKey(stripArraySuffix(value));
    if (safeControllerPathSegments(dotted).length === 0) {
      return;
    }
    keys.add(value);
    keys.add(stripArraySuffix(value));
    keys.add(dotted);
    keys.add(stripArraySuffix(dotted));
    keys.add(toBracketKey(dotted));
  });

  return Array.from(keys).filter(Boolean);
}

function resolvePayloadEntry(
  index: Map<string, unknown>,
  element: HTMLElement,
  clearAncestors = false
): { found: boolean; value: unknown } {
  const keys = collectFieldKeys(element);
  for (const key of keys) {
    if (index.has(key)) {
      return { found: true, value: index.get(key) };
    }
  }
  if (clearAncestors) {
    const parts = safeControllerPathSegments(fieldName(element));
    for (let length = parts.length - 1; length > 0; length -= 1) {
      const key = parts.slice(0, length).join(".");
      if (index.has(key) && index.get(key) == null) {
        return { found: true, value: null };
      }
    }
  }
  return { found: false, value: undefined };
}

function applyHydratedValue(
  element: HTMLElement,
  index: Map<string, unknown>,
  registry: ResolverRegistry | null,
  run: HydrationRun,
  clearIfOmitted: boolean
): void {
  let entry = resolvePayloadEntry(index, element, true);
  if (entry.found) {
    const path = canonicalPathForElement(element);
    if (path) (run.supplied ??= new Map()).set(path, entry.value);
  }
  if (!entry.found) {
    if (!clearIfOmitted) {
      return;
    }
    entry = { found: true, value: null };
  }
  const field = run.model && lookupField(run.model, canonicalPathForElement(element));
  const rawObject = field?.type === "object" && (field.nested ?? []).length === 0;
  let normalized = rawObject && entry.value !== null && typeof entry.value === "object" && !Array.isArray(entry.value)
    ? JSON.stringify(entry.value)
    : normalizeHydratedSelection(entry.value, element);
  let nativeValue = entry.value;
  if (field && entry.found && run.supplied?.has(canonicalPathForElement(element))) {
    const path = canonicalPathForElement(element);
    // Option-like objects are a legacy hydration convenience for scalar choices;
    // normalize their value tokens, while object items remain whole JSON data.
    const arrayInput = field.type === "array" && field.items?.type !== "object"
      ? (Array.isArray(entry.value) ? entry.value.map((item) => item && typeof item === "object" && !Array.isArray(item) ? coerceHydratedValue(item) : item) : entry.value)
      : entry.value;
    const expected = normalizeFieldValue(field, run.enumValues === "literal" ? entry.value : field.type === "array" ? arrayInput : normalized, path, {
      ...run.normalization,
      enumValues: carriesEnumControlValue(element) ? run.enumValues ?? run.normalization?.enumValues ?? "auto" : "literal",
    });
    (run.expected ??= new Map()).set(path, {
      state: expected.value === null ? "null" : "present",
      ...(expected.value === null ? {} : { value: expected.value }),
      ...(expected.issues.length > 0 ? { issues: expected.issues } : {}),
    });
    // Model-aware choices match the normalized logical value, never an
    // ambiguous mixture of literal strings and native transport tokens.
    nativeValue = expected.value;
  }
  let changed = false;
  if (element instanceof HTMLSelectElement) {
    const values = resolveSelectTokens(element, nativeValue, normalized, !!field);
    changed = applySelectValues(
      element,
      values.map((value) => ({ value, label: value })),
      true
    );
    const submitMode =
      element.getAttribute("data-relationship-submit-mode") === "json" ||
      element.dataset.endpointSubmitAs === "json"
        ? "json"
        : undefined;
    if (changed) {
      syncRelationshipMirrors(element, submitMode as FieldConfig["submitAs"]);
      emitRelationshipUpdate(element, {
        kind: "selection",
        origin: "hydrate",
        selectedValues: Array.from(element.selectedOptions)
          .map((option) => option.value)
          .filter((value) => value !== ""),
      });
      element.dispatchEvent(new Event("change", { bubbles: true }));
    }
    normalized = element.multiple ? values : values[0] ?? null;
  } else if (element instanceof HTMLInputElement) {
    changed = applyHydratedInputValue(element, nativeValue, normalized, run.settled, !!field);
  } else if (element instanceof HTMLTextAreaElement) {
    const before = element.value;
    element.value = hydratedTextValue(normalized);
    changed = element.value !== before;
    if (changed || !run.settled) {
      element.dispatchEvent(new Event("input", { bubbles: true }));
    }
  }

  updateRelationshipCurrentAttribute(element, normalized);

  const resolver = registry?.get(element);
  if (resolver) {
    resolver.setCurrentValue(normalized);
  }

  if (changed) {
    const path = canonicalPathForElement(element);
    if (path) {
      run.changed.add(path);
    }
  }
}

function hydratedTextValue(normalized: string | string[] | null): string {
  if (Array.isArray(normalized)) {
    return normalized[0] ?? "";
  }
  return normalized == null ? "" : normalized;
}

function applyHydratedInputValue(
  element: HTMLInputElement,
  rawValue: unknown,
  normalized: string | string[] | null,
  settled: boolean,
  literalValues = false
): boolean {
  const type = element.type.toLowerCase();
  if (type === "checkbox") {
    const before = element.checked;
    element.checked = checkboxHydratedChecked(element, rawValue, normalized, literalValues);
    const changed = element.checked !== before;
    if (changed || !settled) {
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
    }
    return changed;
  }
  if (type === "radio") {
    const before = element.checked;
    element.checked = radioHydratedChecked(element, rawValue, normalized, literalValues);
    const changed = element.checked !== before;
    if (changed || !settled) {
      element.dispatchEvent(new Event("change", { bubbles: true }));
    }
    return changed;
  }
  if (type === "file") {
    // File inputs only accept programmatic clearing.
    if (hydratedTextValue(normalized) !== "" || element.value === "") {
      return false;
    }
    element.value = "";
    element.dispatchEvent(new Event("input", { bubbles: true }));
    return true;
  }
  const before = element.value;
  element.value = hydratedTextValue(normalized);
  const changed = element.value !== before;
  if (changed || !settled) {
    element.dispatchEvent(new Event("input", { bubbles: true }));
  }
  return changed;
}

function checkboxHydratedChecked(
  element: HTMLInputElement,
  rawValue: unknown,
  normalized: string | string[] | null,
  literalValues = false
): boolean {
  if (Array.isArray(rawValue)) {
    return rawValue.some((item) => matchesControlValue(element.value, item, literalValues));
  }
  if (Array.isArray(normalized)) {
    return normalized.includes(element.value);
  }
  if (isTypedEnumControlValue(element.value) && (typeof rawValue === "boolean" || typeof rawValue === "number")) {
    // Typed choice members compare against their decoded option value.
    return matchesControlValue(element.value, rawValue);
  }
  if (typeof rawValue === "boolean") {
    return rawValue;
  }
  if (typeof rawValue === "number") {
    return rawValue !== 0;
  }
  if (typeof rawValue === "string") {
    const token = rawValue.trim().toLowerCase();
    if (["true", "1", "yes", "on"].includes(token)) {
      return true;
    }
    if (["false", "0", "no", "off", ""].includes(token)) {
      return false;
    }
  }
  return normalized != null && normalized === element.value;
}

function radioHydratedChecked(
  element: HTMLInputElement,
  rawValue: unknown,
  normalized: string | string[] | null,
  literalValues = false
): boolean {
  if (Array.isArray(rawValue)) {
    return rawValue.some((item) => matchesControlValue(element.value, item, literalValues));
  }
  if (Array.isArray(normalized)) {
    return normalized.includes(element.value);
  }
  return matchesControlValue(element.value, rawValue, literalValues);
}

const TYPED_ENUM_PREFIX = "__fg_enum_v1:";

function isTypedEnumControlValue(value: string): boolean {
  return value.startsWith(TYPED_ENUM_PREFIX);
}

/**
 * A control value matches a hydrated item when the strings are identical, when
 * an option-like object resolves to it, or when the control carries a typed
 * enum codec value whose decoded value equals a typed (boolean/number) item.
 * Model-aware calls supply literals: decode the control once before comparing
 * strings. Legacy calls also accept raw transport tokens. String "false" stays
 * distinct from an encoded boolean false in both cases.
 */
function matchesControlValue(controlValue: string, raw: unknown, literalValues = false): boolean {
  if (raw === null || raw === undefined) {
    return false;
  }
  if (typeof raw === "string") {
    if (literalValues) {
      const decoded = decodeEnumValue(controlValue);
      return decoded.encoded ? decoded.value === raw : controlValue === raw;
    }
    if (controlValue === raw) return true;
    // Reserved-prefix literal strings are themselves encoded by Go. Decode
    // exactly once, preserving the distinction between string and boolean false.
    const decoded = decodeEnumValue(controlValue);
    return decoded.encoded && typeof decoded.value === "string" && decoded.value === raw;
  }
  if (typeof raw === "object") {
    const token = coerceHydratedValue(raw);
    return token !== null && controlValue === token;
  }
  if (controlValue === String(raw)) {
    return true;
  }
  if (!isTypedEnumControlValue(controlValue)) {
    return false;
  }
  const decoded = decodeEnumValue(controlValue);
  return decoded.encoded && decoded.value === raw;
}

/**
 * Map hydrated items to existing option values so typed items select encoded
 * options instead of creating stray options; unmatched items keep the legacy
 * token behavior.
 */
function resolveSelectTokens(
  select: HTMLSelectElement,
  rawValue: unknown,
  normalized: string | string[] | null,
  literalValues = false
): string[] {
  const items = Array.isArray(rawValue) ? rawValue : rawValue === null || rawValue === undefined ? [] : [rawValue];
  if (items.length === 0) {
    return [];
  }
  const options = Array.from(select.options);
  const out: string[] = [];
  items.forEach((item, position) => {
    const match = options.find((option) => option.value !== "" && matchesControlValue(option.value, item, literalValues));
    if (match) {
      out.push(match.value);
      return;
    }
    const fallback = Array.isArray(normalized) ? normalized[position] : position === 0 ? normalized : null;
    if (typeof fallback === "string" && fallback !== "") {
      out.push(fallback);
    }
  });
  return select.multiple ? out : out.slice(0, 1);
}

function applyHydratedErrors(
  element: HTMLElement,
  index: Map<string, unknown>,
  registry: ResolverRegistry | null
): void {
  const entry = resolvePayloadEntry(index, element);
  if (!entry.found) {
    return;
  }
  const messages = normalizeErrorMessages(entry.value);
  const resolver = registry?.get(element);

  if (messages.length === 0) {
    clearFieldError(element);
    element.removeAttribute("data-validation-state");
    element.removeAttribute("data-validation-message");
    resolver?.setServerValidation(undefined);
    return;
  }

  const message = messages[0];
  renderFieldError(element, message);
  element.setAttribute("data-validation-state", "invalid");
  element.setAttribute("data-validation-message", messages.join("; "));
  resolver?.setServerValidation({
    valid: false,
    messages,
    errors: messages.map((text) => ({
      code: "server",
      message: text,
    })),
  });
}

function normalizeHydratedSelection(
  value: unknown,
  element: HTMLElement
): string | string[] | null {
  if (value == null) {
    return null;
  }
  if (Array.isArray(value)) {
    const tokens = value
      .map((item) => coerceHydratedValue(item))
      .filter((token): token is string => typeof token === "string" && token !== "");
    if (element instanceof HTMLSelectElement && element.multiple) {
      return tokens;
    }
    return tokens[0] ?? null;
  }
  const token = coerceHydratedValue(value);
  return token ?? null;
}

function coerceHydratedValue(value: unknown): string | null {
  if (value == null) {
    return null;
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    for (const key of ["value", "id", "slug"]) {
      const candidate = record[key];
      if (candidate != null) {
        return String(candidate);
      }
    }
    return null;
  }
  return String(value);
}

function updateRelationshipCurrentAttribute(
  element: HTMLElement,
  value: string | string[] | null
): void {
  if (value == null || (Array.isArray(value) && value.length === 0)) {
    element.removeAttribute("data-relationship-current");
    return;
  }
  const payload = serializeCurrentValue(value);
  if (payload) {
    element.setAttribute("data-relationship-current", payload);
  }
}

function serializeCurrentValue(
  value: string | string[] | null
): string | undefined {
  if (value == null) {
    return undefined;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return undefined;
    }
    try {
      return JSON.stringify(value);
    } catch (_err) {
      return undefined;
    }
  }
  const trimmed = String(value).trim();
  return trimmed || undefined;
}

function normalizeErrorMessages(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value
      .map((item) => (typeof item === "string" ? item.trim() : ""))
      .filter((item) => item.length > 0);
  }
  if (typeof value === "string") {
    const direct = value.trim();
    if (direct.includes(";")) {
      return direct
        .split(";")
        .map((item) => item.trim())
        .filter((item) => item.length > 0);
    }
    return direct ? [direct] : [];
  }
  return [];
}

function toCanonicalPath(dotted: string): string {
  const segments = safeControllerPathSegments(dotted);
  if (segments.length === 0) {
    return "";
  }
  return segments.reduce((path, segment, position) => {
    if (position === 0) {
      return segment;
    }
    return /^\d+$/.test(segment) ? `${path}[${segment}]` : `${path}.${segment}`;
  }, "");
}

function canonicalPathForElement(element: HTMLElement): string {
  return toCanonicalPath(toDotKey(stripArraySuffix(fieldName(element))));
}

function eventCanonicalPaths(event: Event): string[] {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return [];
  }
  const path = canonicalPathForElement(target);
  return path ? [path] : [];
}

function createControllerChangeEvent(meta: ChangeMetadata): Event {
  try {
    return new CustomEvent<ChangeMetadata>(CONTROLLER_CHANGE_EVENT, { detail: meta });
  } catch (_err) {
    return new Event(CONTROLLER_CHANGE_EVENT);
  }
}

const ARRAY_INTENT_SEGMENTS = new Set(["_present", "_row_state", "_row_key", "_delete"]);

function isArrayIntentName(name: string): boolean {
  const segments = safeControllerPathSegments(name);
  const last = segments[segments.length - 1];
  return last !== undefined && (ARRAY_INTENT_SEGMENTS.has(last) || /__(present|complete|clear)$/.test(last));
}

/**
 * Managed controls are the hydratable controls replace mode clears when the
 * payload omits them: enabled, addressable, non-structural controls. Explicit
 * payload values still apply to disabled controls, matching patch hydration.
 */
function isReplaceManaged(element: HTMLElement): boolean {
  if (
    !(element instanceof HTMLInputElement) &&
    !(element instanceof HTMLSelectElement) &&
    !(element instanceof HTMLTextAreaElement)
  ) {
    return false;
  }
  if (isDisabledControl(element)) {
    return false;
  }
  if (element instanceof HTMLInputElement) {
    const type = element.type.toLowerCase();
    if (["button", "submit", "reset", "image"].includes(type)) {
      return false;
    }
  }
  const name = fieldName(element);
  if (!name || isArrayIntentName(name)) {
    return false;
  }
  return safeControllerPathSegments(name).length > 0;
}

/**
 * Rendered defaults for every hydratable control keyed by dotted path. Radio
 * groups without a default selection and multi-value controls without default
 * selections map to explicit clears so reset hydration can settle them.
 */
function collectDefaultHydrationValues(root: Document | HTMLElement): Record<string, unknown> {
  const defaults = new Map<string, unknown>();
  locateHydratableFields(root).forEach((element) => {
    const name = fieldName(element);
    if (!name || safeControllerPathSegments(name).length === 0) {
      return;
    }
    const key = toDotKey(stripArraySuffix(name));
    if (element instanceof HTMLInputElement) {
      const type = element.type.toLowerCase();
      if (type === "checkbox") {
        if (element.name.endsWith("[]")) {
          const current = defaults.get(key);
          const list = Array.isArray(current) ? (current as string[]) : [];
          if (element.defaultChecked) {
            list.push(element.value);
          }
          defaults.set(key, list);
        } else {
          defaults.set(key, element.defaultChecked);
        }
        return;
      }
      if (type === "radio") {
        if (element.defaultChecked) {
          defaults.set(key, element.value);
        } else if (!defaults.has(key)) {
          defaults.set(key, null);
        }
        return;
      }
      defaults.set(key, type === "file" ? null : element.defaultValue);
      return;
    }
    if (element instanceof HTMLTextAreaElement) {
      defaults.set(key, element.defaultValue);
      return;
    }
    if (element instanceof HTMLSelectElement) {
      const selected = Array.from(element.options)
        .filter((option) => option.defaultSelected)
        .map((option) => option.value);
      if (element.multiple) {
        defaults.set(key, selected);
        return;
      }
      defaults.set(key, selected[0] ?? element.options[0]?.value ?? null);
    }
  });
  const payload: Record<string, unknown> = {};
  defaults.forEach((value, key) => {
    payload[key] = value;
  });
  return payload;
}

/**
 * Replace hydration only addresses rendered rows. A payload row whose index has
 * no rendered controls fails before any control is mutated when the form
 * declares that array; unrelated unknown keys stay ignored like patch mode.
 */
function assertReplaceStructure(
  scope: Document | HTMLElement,
  fields: HTMLElement[],
  values: Record<string, unknown>
): void {
  const flattened = new Map<string, unknown>();
  flattenPayload(values, flattened, "", true);
  const controlKeys = new Set<string>();
  fields.forEach((element) => {
    const name = fieldName(element);
    if (name && safeControllerPathSegments(name).length > 0) {
      controlKeys.add(toDotKey(stripArraySuffix(name)));
    }
  });
  const inspected = new Set<string>();
  flattened.forEach((_value, key) => {
    const segments = safeControllerPathSegments(key);
    segments.forEach((segment, position) => {
      if (position === 0 || !/^\d+$/.test(segment)) {
        return;
      }
      const rowPrefix = segments.slice(0, position + 1).join(".");
      if (inspected.has(rowPrefix)) {
        return;
      }
      inspected.add(rowPrefix);
      if (hasControlUnder(controlKeys, rowPrefix)) {
        return;
      }
      const arrayPath = segments.slice(0, position).join(".");
      if (formDeclaresArray(scope, controlKeys, arrayPath)) {
        throw new Error(
          `formgen: replace hydration cannot create row ${toCanonicalPath(rowPrefix)}; only rendered rows are supported`
        );
      }
    });
  });
}

function hasControlUnder(keys: Set<string>, prefix: string): boolean {
  for (const key of keys) {
    if (key === prefix || key.startsWith(`${prefix}.`)) {
      return true;
    }
  }
  return false;
}

function formDeclaresArray(
  scope: Document | HTMLElement,
  controlKeys: Set<string>,
  arrayPath: string
): boolean {
  for (const key of controlKeys) {
    if (!key.startsWith(`${arrayPath}.`)) {
      continue;
    }
    const next = key.slice(arrayPath.length + 1).split(".")[0] ?? "";
    if (/^\d+$/.test(next)) {
      return true;
    }
  }
  const spellings = new Set([arrayPath, toBracketKey(arrayPath), toCanonicalPath(arrayPath)]);
  for (const spelling of spellings) {
    if (!spelling) {
      continue;
    }
    if (scope.querySelector(`[data-formgen-array-name="${safeSelectorValue(spelling)}"]`)) {
      return true;
    }
  }
  return false;
}

function setupDependentRefresh(
  element: HTMLElement,
  field: FieldConfig,
  root: HTMLElement,
  registry: ResolverRegistry,
  lifecycle: RuntimeRootLifecycle
): void {
  if (!field.refreshOn || field.refreshOn.length === 0) {
    return;
  }
  if (field.refreshMode === "manual") {
    return;
  }
  if (element.dataset.relationshipRefreshBound === "true") {
    return;
  }
  element.dataset.relationshipRefreshBound = "true";

  const form = element.closest("form");
  const scope = form ?? root ?? element.ownerDocument ?? document;

  const trigger = () => {
    registry.resolve(element).catch(() => undefined);
  };

  const bindings: Array<{ target: HTMLElement; eventType: string }> = [];

  field.refreshOn.forEach((reference) => {
    const targets = findDependencyTargets(scope, reference);
    targets.forEach((target) => {
      const eventType = target instanceof HTMLInputElement && target.type === "text" ? "input" : "change";
      target.addEventListener(eventType, trigger);
      bindings.push({ target, eventType });
    });
  });
  lifecycle.cleanups.add(() => {
    for (const binding of bindings) {
      binding.target.removeEventListener(binding.eventType, trigger);
    }
    delete element.dataset.relationshipRefreshBound;
  });
}

function setupManualRefresh(
  element: HTMLElement,
  field: FieldConfig,
  root: HTMLElement,
  registry: ResolverRegistry,
  lifecycle: RuntimeRootLifecycle
): void {
  if (field.refreshMode !== "manual") {
    return;
  }
  const name = field.name ?? element.getAttribute("id");
  if (!name) {
    return;
  }
  if (element.dataset.relationshipManualBound === "true") {
    return;
  }
  element.dataset.relationshipManualBound = "true";

  const escaped = safeSelectorValue(name);
  const doc = root.ownerDocument ?? element.ownerDocument ?? document;
  const triggers = new Set<HTMLElement>();

  const selectors = [
    `[data-endpoint-refresh-target="${escaped}"]`,
    `[data-endpoint-refresh-for="${escaped}"]`,
  ];

  selectors.forEach((selector) => {
    doc.querySelectorAll<HTMLElement>(selector).forEach((node) => triggers.add(node));
  });

  const container = element.closest("[data-relationship-type]") ?? element.parentElement;
  container
    ?.querySelectorAll<HTMLElement>("[data-endpoint-refresh-trigger]")
    .forEach((node) => triggers.add(node));

  const bindings: Array<{ trigger: HTMLElement; handler: (event: Event) => void }> = [];
  triggers.forEach((triggerElement) => {
    if (triggerElement.dataset.relationshipRefreshListener === "true") {
      return;
    }
    triggerElement.dataset.relationshipRefreshListener = "true";
    const handler = (event: Event) => {
      event.preventDefault();
      registry.resolve(element).catch(() => undefined);
    };
    triggerElement.addEventListener("click", handler);
    bindings.push({ trigger: triggerElement, handler });
  });
  lifecycle.cleanups.add(() => {
    for (const binding of bindings) {
      binding.trigger.removeEventListener("click", binding.handler);
      delete binding.trigger.dataset.relationshipRefreshListener;
    }
    delete element.dataset.relationshipManualBound;
  });
}

function setupSearchMode(
  element: HTMLElement,
  field: FieldConfig,
  registry: ResolverRegistry,
  lifecycle: RuntimeRootLifecycle
): void {
  if (field.mode !== "search") {
    return;
  }
  if (element.dataset.relationshipSearchBound === "true") {
    return;
  }
  element.dataset.relationshipSearchBound = "true";

  const config = registry.getConfig();
  const throttleMs = field.throttleMs ?? config.searchThrottleMs;
  const debounceMs = field.debounceMs ?? config.searchDebounceMs;

  const invoke = () => {
    registry.resolve(element).catch(() => undefined);
  };

  let trigger = createDebouncedInvoker(invoke, debounceMs);
  trigger = createThrottledInvoker(trigger, throttleMs);

  const updateSearchValue = () => {
    if (
      element instanceof HTMLInputElement ||
      element instanceof HTMLTextAreaElement
    ) {
      const trimmed = element.value.trim();
      element.setAttribute("data-endpoint-search-value", trimmed);
    }
  };

  const handleSearchEvent = () => {
    updateSearchValue();
    trigger();
  };

  element.addEventListener("input", handleSearchEvent);

  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    element.addEventListener("change", handleSearchEvent);
  }
  lifecycle.cleanups.add(() => {
    element.removeEventListener("input", handleSearchEvent);
    if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
      element.removeEventListener("change", handleSearchEvent);
    }
    trigger.cancel();
    delete element.dataset.relationshipSearchBound;
  });
}

function findDependencyTargets(scope: Document | HTMLElement, reference: string): HTMLElement[] {
  const matches: HTMLElement[] = [];
  const seen = new Set<HTMLElement>();

  const form = scope instanceof HTMLFormElement ? scope : scope instanceof HTMLElement ? scope.closest("form") : null;
  const doc = scope instanceof Document ? scope : scope.ownerDocument ?? document;

  const sources: HTMLElement[] = [];
  if (form) {
    const elements = Array.from(form.elements) as Array<Element>;
    elements.forEach((el) => {
      if (el instanceof HTMLElement) {
        sources.push(el);
      }
    });
  } else {
    sources.push(...Array.from(doc.querySelectorAll<HTMLElement>("[name], [data-field-name], [id]")));
  }

  sources.forEach((candidate) => {
    if (seen.has(candidate)) {
      return;
    }
    const datasetName = (candidate.dataset as Record<string, string | undefined>).fieldName;
    if (datasetName && datasetName === reference) {
      matches.push(candidate);
      seen.add(candidate);
      return;
    }
    const nameAttr = (candidate as HTMLInputElement).name;
    if (matchesFieldName(nameAttr, reference)) {
      matches.push(candidate);
      seen.add(candidate);
      return;
    }
    const idAttr = candidate.id;
    if (idAttr && idAttr === reference) {
      matches.push(candidate);
      seen.add(candidate);
    }
  });

  return matches;
}

function matchesFieldName(candidate: string | undefined, reference: string): boolean {
  if (!candidate) {
    return false;
  }
  if (candidate === reference) {
    return true;
  }
  if (candidate === `${reference}[]`) {
    return true;
  }
  if (candidate.endsWith(`[${reference}]`)) {
    return true;
  }
  if (candidate.endsWith(`[${reference}][]`)) {
    return true;
  }
  const sanitizedCandidate = candidate.replace(/[^a-z0-9]/gi, "").toLowerCase();
  const sanitizedReference = reference.replace(/[^a-z0-9]/gi, "").toLowerCase();
  return sanitizedCandidate.endsWith(sanitizedReference);
}

function safeSelectorValue(value: string): string {
  if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
    return CSS.escape(value);
  }
  return value.replace(/(["\\])/g, "\\$1");
}
