import {
  RELATIONSHIP_UPDATE_EVENT,
  ensureRelationshipSelectionBridge,
  type RelationshipUpdateDetail,
} from "./relationship-events";
import { relationshipOptionValue } from "./relationship-current";

const FIELD_SELECTOR = '[data-endpoint-url], [data-endpoint-renderer="chips"]';
const HIDDEN_CONTAINER_ATTR = "data-relationship-hidden";
const HIDDEN_INITIALISED_ATTR = "data-relationship-hidden-initialised";
const JSON_INITIALISED_ATTR = "data-relationship-json-initialised";
const JSON_INPUT_ATTR = "data-relationship-json";
const SUBMIT_MODE_ATTR = "data-relationship-submit-mode";
export const RELATIONSHIP_ORIGINAL_NAME_ATTR = "data-relationship-original-name";

export function locateRelationshipFields(
  root: Document | HTMLElement = document
): HTMLElement[] {
  const scope = root instanceof Document ? root : root;
  const candidates = Array.from(scope.querySelectorAll<HTMLElement>(FIELD_SELECTOR));

  if (root instanceof HTMLElement && root.matches(FIELD_SELECTOR)) {
    candidates.unshift(root);
  }

  return Array.from(new Set(candidates));
}

export function readDataset(element: HTMLElement): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, value] of Object.entries(element.dataset)) {
    if (typeof value === "string") {
      result[key] = value;
    }
  }
  return result;
}

export function isMultiSelect(element: Element): element is HTMLSelectElement {
  return element instanceof HTMLSelectElement && element.multiple;
}

type MirrorMode = "array" | "json";
interface MirrorState {
  select: HTMLSelectElement;
  mode: MirrorMode;
  container: HTMLDivElement;
  form: HTMLFormElement | null;
  observer: MutationObserver | null;
  liveFormData: boolean;
  signature: string;
  sync: () => void;
  onFormData: EventListener;
  onSubmit: EventListener;
  onReset: EventListener;
  onSelection: EventListener;
}
const mirrors = new WeakMap<HTMLSelectElement, MirrorState>();

export function attachHiddenInputSync(select: HTMLSelectElement): void {
  if (select.getAttribute(SUBMIT_MODE_ATTR) !== "json") syncHiddenInputs(select);
}

export function attachJsonInputSync(select: HTMLSelectElement): void {
  syncJsonInput(select);
}

export function syncHiddenInputs(select: HTMLSelectElement): void {
  ensureMirror(select, select.getAttribute(SUBMIT_MODE_ATTR) === "json" ? "json" : "array").sync();
}

export function syncJsonInput(select: HTMLSelectElement): void {
  ensureMirror(select, "json").sync();
}

/** Mirrors own their native name, DOM, listeners and observers for one select. */
function ensureMirror(select: HTMLSelectElement, mode: MirrorMode): MirrorState {
  const existing = mirrors.get(select);
  if (existing?.mode === mode) return existing;
  if (existing) destroyRelationshipMirrors(select);
  const container = select.ownerDocument.createElement("div");
  container.setAttribute(HIDDEN_CONTAINER_ATTR, "true");
  container.hidden = true;
  const doc = select.ownerDocument;
  const liveFormData = typeof (doc.defaultView as unknown as { FormDataEvent?: unknown })?.FormDataEvent === "function";
  const seen = new WeakSet<Event>();
  const state: MirrorState = {
    select, mode, container, form: null, observer: null, liveFormData, signature: "",
    sync: () => syncMirror(state),
    onFormData: event => {
      if (event.target !== select.form || seen.has(event)) return;
      seen.add(event);
      state.sync();
      // These mirrors are disabled in engines with formdata support. Append
      // current successful values, so even same-turn programmatic locks work.
      if (liveFormData && !select.matches(":disabled")) {
        const data = (event as FormDataEvent).formData;
        for (const [name, value] of mirrorEntries(state)) data.append(name, value);
      }
    },
    onSubmit: event => { if (event.target === select.form) state.sync(); },
    onReset: () => queueMicrotask(() => { if (mirrors.get(select) === state) state.sync(); }),
    onSelection: () => state.sync(),
  };
  mirrors.set(select, state);
  ensureRelationshipSelectionBridge(select);
  select.setAttribute(mode === "json" ? JSON_INITIALISED_ATTR : HIDDEN_INITIALISED_ATTR, "true");
  select.setAttribute(SUBMIT_MODE_ATTR, mode === "json" ? "json" : "hidden-array");
  select.addEventListener(RELATIONSHIP_UPDATE_EVENT, state.onSelection);
  select.addEventListener("change", state.onSelection);
  select.addEventListener("blur", state.onSelection);
  doc.addEventListener("formdata", state.onFormData, true);
  doc.addEventListener("submit", state.onSubmit, true);
  if (typeof MutationObserver !== "undefined") {
    state.observer = new MutationObserver(state.sync);
    state.observer.observe(select, { attributes: true, childList: true, subtree: true,
      attributeFilter: ["disabled", "selected", "value", "name", "form", RELATIONSHIP_ORIGINAL_NAME_ATTR] });
    for (let parent = select.parentElement; parent; parent = parent.parentElement) {
      if (parent instanceof HTMLFieldSetElement) state.observer.observe(parent, { attributes: true, attributeFilter: ["disabled"] });
    }
  }
  return state;
}

function mirrorEntries(state: MirrorState): [string, string][] {
  const { select, mode } = state;
  const original = select.getAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR) ?? "";
  if (!original) return [];
  const values = Array.from(select.selectedOptions)
    .filter(option => !option.disabled && !(option.parentElement instanceof HTMLOptGroupElement && option.parentElement.disabled))
    .map(option => option.value);
  if (mode === "json") {
    const name = original.endsWith("[]") ? original.slice(0, -2) : original;
    return [[name, select.multiple ? JSON.stringify(values) : values.length ? JSON.stringify(values[0]) : "null"]];
  }
  const name = original.endsWith("[]") ? original : `${original}[]`;
  return values.map(value => [name, value]);
}

function syncMirror(state: MirrorState): void {
  const { select, container } = state;
  if (mirrors.get(select) !== state) return;
  const name = select.getAttribute("name");
  if (name !== null) {
    select.setAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR, name);
    select.removeAttribute("name");
  } else if (!select.hasAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR) && select.id) {
    select.setAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR, select.id);
  }
  // Identity belongs to the select, never its parent or its current field name.
  if (select.parentNode && (container.parentNode !== select.parentNode || select.nextSibling !== container)) {
    select.parentNode.insertBefore(container, select.nextSibling);
  }
  if (state.form !== select.form) {
    state.form?.removeEventListener("formdata", state.onFormData);
    state.form?.removeEventListener("reset", state.onReset);
    state.form = select.form;
    state.form?.addEventListener("formdata", state.onFormData);
    state.form?.addEventListener("reset", state.onReset);
  }
  const entries = mirrorEntries(state);
  const disabled = state.liveFormData || select.matches(":disabled");
  const form = select.getAttribute("form");
  const signature = JSON.stringify([entries, disabled, form]);
  if (signature === state.signature) return;
  state.signature = signature;
  container.replaceChildren(...entries.map(([field, value]) => {
    const input = select.ownerDocument.createElement("input");
    input.type = "hidden"; input.name = field; input.value = value; input.disabled = disabled;
    if (form !== null) input.setAttribute("form", form);
    if (state.mode === "json") input.setAttribute(JSON_INPUT_ATTR, "true");
    return input;
  }));
}

export function destroyRelationshipMirrors(select: HTMLSelectElement): void {
  const state = mirrors.get(select);
  if (!state) return;
  mirrors.delete(select);
  state.observer?.disconnect();
  select.removeEventListener(RELATIONSHIP_UPDATE_EVENT, state.onSelection);
  select.removeEventListener("change", state.onSelection);
  select.removeEventListener("blur", state.onSelection);
  state.form?.removeEventListener("formdata", state.onFormData);
  state.form?.removeEventListener("reset", state.onReset);
  select.ownerDocument.removeEventListener("formdata", state.onFormData, true);
  select.ownerDocument.removeEventListener("submit", state.onSubmit, true);
  state.container.remove();
  const name = select.getAttribute(RELATIONSHIP_ORIGINAL_NAME_ATTR);
  if (!select.hasAttribute("name") && name !== null) select.setAttribute("name", name);
  for (const attr of [RELATIONSHIP_ORIGINAL_NAME_ATTR, HIDDEN_INITIALISED_ATTR, JSON_INITIALISED_ATTR, SUBMIT_MODE_ATTR]) select.removeAttribute(attr);
}

export function readElementValue(element: HTMLElement | null): string | string[] | null {
  if (!element) {
    return null;
  }

  if (element instanceof HTMLInputElement) {
    if (element.type === "checkbox" || element.type === "radio") {
      if (!element.checked) {
        return null;
      }
      return element.value;
    }
    return element.value;
  }

  if (element instanceof HTMLSelectElement) {
    if (element.multiple) {
      return Array.from(element.selectedOptions).map(relationshipOptionValue);
    }
    const option = element.selectedOptions[0];
    return option ? relationshipOptionValue(option) : null;
  }

  if (element instanceof HTMLTextAreaElement) {
    return element.value;
  }

  return element.textContent;
}
