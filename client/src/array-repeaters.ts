export interface ArrayRepeaterInitOptions {
  onItemAdded?: (item: HTMLElement) => void | Promise<void>;
}

const ARRAY_ITEMS_ATTR = "data-formgen-array-items";
const ARRAY_PROTOTYPE_ATTR = "data-formgen-array-prototype";
const ARRAY_ACTION_SELECTOR = '[data-formgen-array-action="add"]';
const ARRAY_INITIALIZED_ATTR = "data-formgen-array-initialized";

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
    items.setAttribute(ARRAY_INITIALIZED_ATTR, "true");
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
    if (child instanceof HTMLButtonElement && child.matches(ARRAY_ACTION_SELECTOR)) {
      return child;
    }
  }
  return null;
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
    element.removeAttribute("disabled");
    element.removeAttribute("data-relationship-current");
    element.removeAttribute("data-relationship-current-applied");

    if (element instanceof HTMLInputElement) {
      element.disabled = false;
      if (element.type === "checkbox" || element.type === "radio") {
        element.checked = false;
      } else {
        element.value = "";
      }
      continue;
    }
    if (element instanceof HTMLSelectElement) {
      element.disabled = false;
      Array.from(element.options).forEach((option, index) => {
        option.selected = !element.multiple && index === 0;
      });
      continue;
    }
    if (element instanceof HTMLTextAreaElement) {
      element.disabled = false;
      element.value = "";
      continue;
    }
    if (
      element instanceof HTMLButtonElement ||
      element instanceof HTMLFieldSetElement ||
      element instanceof HTMLOptGroupElement
    ) {
      element.disabled = false;
    }
  }
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
