export interface ArrayRepeaterInitOptions {
  onItemAdded?: (item: HTMLElement) => void | Promise<void>;
}

const ARRAY_ITEMS_ATTR = "data-formgen-array-items";
const ARRAY_PROTOTYPE_ATTR = "data-formgen-array-prototype";
const ARRAY_ADD_ACTION_SELECTOR = '[data-formgen-array-action="add"]';
const ARRAY_REMOVE_ACTION_SELECTOR = '[data-formgen-array-action="remove"]';
const ARRAY_INITIALIZED_ATTR = "data-formgen-array-initialized";
const ARRAY_ITEM_ATTR = "data-formgen-array-item";
const ARRAY_EXISTING_ATTR = "data-formgen-array-existing";
const PROTOTYPE_DISABLED_ATTR = "data-formgen-prototype-disabled";

interface ArrayRepeaterInstance {
  button: HTMLButtonElement;
  handleAdd: () => void;
}

const arrayRepeaterInstances = new WeakMap<HTMLElement, ArrayRepeaterInstance>();

let generatedRowKeyCounter = 0;

interface ReindexContext {
  prototypePath: string;
  targetPath: string;
  prototypeIDPrefix: string;
  targetIDPrefix: string;
}

export function initArrayRepeaters(
  root: Document | HTMLElement = document,
  options: ArrayRepeaterInitOptions = {}
): void {
  for (const items of collectArrayItemContainers(root)) {
    if (items.getAttribute(ARRAY_INITIALIZED_ATTR) === "true") {
      continue;
    }
    const template = findPrototypeTemplate(items);
    const button = findAddButton(items);
    if (!template || !button) {
      continue;
    }

    const handleAdd = () => {
      const added = addArrayItem(items);
      for (const item of added) {
        void options.onItemAdded?.(item);
      }
    };

    button.addEventListener("click", handleAdd);
    items.addEventListener("click", handleRemove);
    items.setAttribute(ARRAY_INITIALIZED_ATTR, "true");
    arrayRepeaterInstances.set(items, { button, handleAdd });
  }
}

export function destroyArrayRepeaters(root: Document | HTMLElement = document): void {
  for (const items of collectArrayItemContainers(root)) {
    const instance = arrayRepeaterInstances.get(items);
    if (instance) {
      instance.button.removeEventListener("click", instance.handleAdd);
      items.removeEventListener("click", handleRemove);
      arrayRepeaterInstances.delete(items);
    }
    items.removeAttribute(ARRAY_INITIALIZED_ATTR);
  }
}

export function addArrayItem(items: HTMLElement): HTMLElement[] {
  const template = findPrototypeTemplate(items);
  if (!template) {
    return [];
  }

  const nextIndex = readNextIndex(items);
  const arrayName = items.dataset.formgenArrayName ?? "";
  const prototypePath = items.dataset.formgenArrayPrototypePath ?? (arrayName ? `${arrayName}[${nextIndex}]` : "");
  const targetPath = arrayName ? `${arrayName}[${nextIndex}]` : prototypePath;
  const prototypeIDPrefix =
    items.dataset.formgenArrayPrototypeIdPrefix ?? (prototypePath ? controlIDFromPath(prototypePath) : "");
  const targetIDPrefix = targetPath ? controlIDFromPath(targetPath) : prototypeIDPrefix;

  const fragment = template.content.cloneNode(true) as DocumentFragment;
  rewritePrototypeFragment(fragment, {
    prototypePath,
    targetPath,
    prototypeIDPrefix,
    targetIDPrefix,
  });
  resetPrototypeFragment(fragment);

  const added = Array.from(fragment.children).filter(
    (node): node is HTMLElement => node instanceof HTMLElement
  );
  items.insertBefore(fragment, template);
  items.dataset.formgenArrayNextIndex = String(nextIndex + 1);
  return added;
}

function collectArrayItemContainers(root: Document | HTMLElement): HTMLElement[] {
  const scope = root instanceof Document ? root : root;
  const selector = `[${ARRAY_ITEMS_ATTR}]`;
  const nodes = Array.from(scope.querySelectorAll<HTMLElement>(selector));
  if (root instanceof HTMLElement && root.hasAttribute(ARRAY_ITEMS_ATTR)) {
    nodes.unshift(root);
  }
  return Array.from(new Set(nodes));
}

function findPrototypeTemplate(items: HTMLElement): HTMLTemplateElement | null {
  for (const child of Array.from(items.children)) {
    if (child instanceof HTMLTemplateElement && child.hasAttribute(ARRAY_PROTOTYPE_ATTR)) {
      return child;
    }
  }
  return null;
}

function findAddButton(items: HTMLElement): HTMLButtonElement | null {
  const parent = items.parentElement;
  if (!parent) {
    return null;
  }
  for (const child of Array.from(parent.children)) {
    if (child instanceof HTMLButtonElement && child.matches(ARRAY_ADD_ACTION_SELECTOR)) {
      return child;
    }
  }
  return null;
}

function handleRemove(event: Event): void {
  const target = event.target;
  if (!(target instanceof Element)) {
    return;
  }
  const button = target.closest<HTMLButtonElement>(ARRAY_REMOVE_ACTION_SELECTOR);
  if (!button) {
    return;
  }
  event.preventDefault();
  removeArrayItem(button);
}

function removeArrayItem(button: HTMLButtonElement): void {
  const item = button.closest<HTMLElement>(`[${ARRAY_ITEM_ATTR}]`);
  if (!item) {
    return;
  }
  if (isExistingArrayItem(item) && markArrayItemDeleted(item)) {
    item.hidden = true;
    item.setAttribute("aria-hidden", "true");
    return;
  }
  item.remove();
}

function isExistingArrayItem(item: HTMLElement): boolean {
  return item.getAttribute(ARRAY_EXISTING_ATTR) === "true";
}

function markArrayItemDeleted(item: HTMLElement): boolean {
  const deleteControl = findDeleteControl(item);
  if (!deleteControl) {
    return false;
  }
  setDeleteControlValue(deleteControl);
  disableArrayItemControls(item, deleteControl);
  return true;
}

function findDeleteControl(item: HTMLElement): HTMLInputElement | null {
  const controls = Array.from(item.querySelectorAll<HTMLInputElement>("input"));
  return controls.find((control) => control.closest(`[${ARRAY_ITEM_ATTR}]`) === item && isDeleteIntentControl(control)) ?? null;
}

function isDeleteIntentControl(control: HTMLInputElement): boolean {
  const name = control.getAttribute("name") ?? "";
  const fieldPath = control.getAttribute("data-field-path") ?? control.dataset.fieldName ?? "";
  return name.endsWith("._delete") ||
    name.endsWith("[_delete]") ||
    fieldPath.endsWith("._delete") ||
    fieldPath.endsWith("[_delete]");
}

function setDeleteControlValue(control: HTMLInputElement): void {
  control.disabled = false;
  if (control.type === "checkbox" || control.type === "radio") {
    control.checked = true;
    control.value = "true";
    return;
  }
  control.value = "true";
}

function disableArrayItemControls(item: HTMLElement, deleteControl: HTMLInputElement): void {
  item
    .querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | HTMLButtonElement>(
      "input, select, textarea, button"
    )
    .forEach((control) => {
      if (control === deleteControl || isArrayIntentControl(control)) {
        return;
      }
      control.disabled = true;
    });
}

function isArrayIntentControl(control: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | HTMLButtonElement): boolean {
  if (!(control instanceof HTMLInputElement)) {
    return false;
  }
  return isDeleteIntentControl(control) || isRowIntentControl(control);
}

function isRowIntentControl(control: HTMLInputElement): boolean {
  const name = control.getAttribute("name") ?? "";
  const fieldPath = control.getAttribute("data-field-path") ?? control.dataset.fieldName ?? "";
  return rowIntentName(name) || rowIntentName(fieldPath);
}

function rowIntentName(value: string): boolean {
  return value.endsWith("._present") ||
    value.endsWith("[_present]") ||
    value.endsWith("._row_state") ||
    value.endsWith("[_row_state]") ||
    value.endsWith("._row_key") ||
    value.endsWith("[_row_key]");
}

function readNextIndex(items: HTMLElement): number {
  const raw = Number.parseInt(items.dataset.formgenArrayNextIndex ?? "", 10);
  if (Number.isFinite(raw) && raw >= 0) {
    return raw;
  }
  return countExistingItems(items);
}

function countExistingItems(items: HTMLElement): number {
  return Array.from(items.children).filter(
    (child) => !(child instanceof HTMLTemplateElement)
  ).length;
}

function rewritePrototypeFragment(fragment: DocumentFragment, context: ReindexContext): void {
  for (const element of fragmentElements(fragment)) {
    rewriteAttributes(element, context);
  }
}

function rewriteAttributes(element: Element, context: ReindexContext): void {
  for (const attr of Array.from(element.attributes)) {
    const next = rewriteValue(attr.value, context);
    if (next !== attr.value) {
      element.setAttribute(attr.name, next);
    }
  }
}

function rewriteValue(value: string, context: ReindexContext): string {
  let next = value;
  if (context.prototypePath && context.targetPath) {
    next = next.split(context.prototypePath).join(context.targetPath);
  }
  if (context.prototypeIDPrefix && context.targetIDPrefix) {
    next = next.split(context.prototypeIDPrefix).join(context.targetIDPrefix);
  }
  return next;
}

function resetPrototypeFragment(fragment: DocumentFragment): void {
  for (const element of fragmentElements(fragment)) {
    const shouldEnable = element.getAttribute(PROTOTYPE_DISABLED_ATTR) === "true";
    element.removeAttribute(PROTOTYPE_DISABLED_ATTR);
    element.removeAttribute("data-relationship-current");
    element.removeAttribute("data-relationship-current-applied");
    if (element.hasAttribute(ARRAY_ITEM_ATTR)) {
      element.setAttribute(ARRAY_EXISTING_ATTR, "false");
    }

    if (element instanceof HTMLInputElement) {
      if (shouldEnable) {
        element.disabled = false;
      }
      if (resetArrayIntentInput(element)) {
        continue;
      }
      if (element.type === "checkbox" || element.type === "radio") {
        element.checked = false;
      } else {
        element.value = "";
      }
      continue;
    }
    if (element instanceof HTMLSelectElement) {
      if (shouldEnable) {
        element.disabled = false;
      }
      Array.from(element.options).forEach((option, index) => {
        option.selected = !element.multiple && index === 0;
      });
      continue;
    }
    if (element instanceof HTMLTextAreaElement) {
      if (shouldEnable) {
        element.disabled = false;
      }
      element.value = "";
      continue;
    }
    if (
      element instanceof HTMLButtonElement ||
      element instanceof HTMLFieldSetElement ||
      element instanceof HTMLOptGroupElement
    ) {
      if (shouldEnable) {
        element.disabled = false;
      }
    }
  }
}

function resetArrayIntentInput(input: HTMLInputElement): boolean {
  const name = input.getAttribute("name") ?? "";
  if (name.endsWith("._present") || name.endsWith("[_present]")) {
    input.value = "true";
    return true;
  }
  if (name.endsWith("._row_state") || name.endsWith("[_row_state]")) {
    input.value = "new";
    return true;
  }
  if (name.endsWith("._row_key") || name.endsWith("[_row_key]")) {
    input.value = nextGeneratedRowKey();
    return true;
  }
  if (isDeleteIntentControl(input)) {
    input.value = "false";
    if (input.type === "checkbox" || input.type === "radio") {
      input.checked = false;
    }
    return true;
  }
  return false;
}

function nextGeneratedRowKey(): string {
  generatedRowKeyCounter += 1;
  return `new-${Date.now().toString(36)}-${generatedRowKeyCounter}`;
}

function fragmentElements(fragment: DocumentFragment): Element[] {
  const elements: Element[] = [];
  for (const child of Array.from(fragment.children)) {
    elements.push(child);
    elements.push(...Array.from(child.querySelectorAll("*")));
  }
  return elements;
}

function controlIDFromPath(path: string): string {
  return `fg-${sanitizeID(path.split("[]").join(".item"))}`;
}

function sanitizeID(value: string): string {
  let out = "";
  let lastDash = false;
  for (const char of value.trim()) {
    if (/^[A-Za-z0-9_-]$/.test(char)) {
      out += char;
      lastDash = false;
      continue;
    }
    if (!lastDash) {
      out += "-";
      lastDash = true;
    }
  }
  return out.replace(/^-+|-+$/g, "");
}
