export type ClassToken = string;
export type ClassValue = string | string[];

export interface ChipsClassMap {
  [key: string]: ClassToken[];
  container: ClassToken[];
  containerReady: ClassToken[];
  containerOpen: ClassToken[];
  inner: ClassToken[];
  chips: ClassToken[];
  chipsContent: ClassToken[];
  placeholder: ClassToken[];
  search: ClassToken[];
  searchInput: ClassToken[];
  searchHighlight: ClassToken[];
  chip: ClassToken[];
  chipLabel: ClassToken[];
  chipRemove: ClassToken[];
  actions: ClassToken[];
  action: ClassToken[];
  actionClear: ClassToken[];
  actionToggle: ClassToken[];
  menu: ClassToken[];
  menuItem: ClassToken[];
  menuEmpty: ClassToken[];
  nativeSelect: ClassToken[];
}

export interface TypeaheadClassMap {
  [key: string]: ClassToken[];
  container: ClassToken[];
  containerReady: ClassToken[];
  containerOpen: ClassToken[];
  control: ClassToken[];
  input: ClassToken[];
  clear: ClassToken[];
  dropdown: ClassToken[];
  option: ClassToken[];
  optionActive: ClassToken[];
  highlight: ClassToken[];
  empty: ClassToken[];
  nativeSelect: ClassToken[];
}

export interface ThemeClassMap {
  chips: ChipsClassMap;
  typeahead: TypeaheadClassMap;
}

export type PartialThemeClassMap = {
  chips?: Partial<{ [K in keyof ChipsClassMap]: ClassValue }>;
  typeahead?: Partial<{ [K in keyof TypeaheadClassMap]: ClassValue }>;
};

const DEFAULT_THEME_CLASSES: ThemeClassMap = {
  chips: {
    container: ["relative", "w-full", "text-sm"],
    containerReady: ["flex"],
    containerOpen: ["ring-2", "ring-blue-500", "ring-offset-2", "ring-offset-white"],
    inner: ["flex", "w-full", "items-stretch", "gap-3"],
    chips: ["flex", "flex-wrap", "gap-2", "grow", "items-center"],
    chipsContent: ["flex", "flex-wrap", "items-center", "gap-2"],
    placeholder: ["text-slate-500"],
    search: ["flex", "items-center", "gap-2", "grow"],
    searchInput: [
      "w-full",
      "min-w-[6rem]",
      "rounded-md",
      "border",
      "border-slate-300",
      "bg-transparent",
      "px-2.5",
      "py-1.5",
      "text-sm",
      "placeholder:text-slate-400",
      "focus:outline-none",
      "focus:border-blue-500",
      "focus:ring-2",
      "focus:ring-blue-500",
      "focus:ring-offset-2",
    ],
    searchHighlight: ["bg-amber-100", "font-semibold"],
    chip: [
      "inline-flex",
      "items-center",
      "gap-2",
      "max-w-full",
      "rounded-md",
      "border",
      "border-slate-200",
      "bg-slate-100",
      "px-3",
      "py-1.5",
      "text-sm",
      "font-medium",
      "text-slate-700",
      "shadow-sm",
    ],
    chipLabel: ["truncate"],
    chipRemove: [
      "flex",
      "h-7",
      "w-7",
      "items-center",
      "justify-center",
      "rounded-full",
      "text-lg",
      "text-slate-400",
      "transition",
      "hover:text-slate-600",
      "focus-visible:outline-none",
      "focus-visible:ring-2",
      "focus-visible:ring-blue-500",
      "focus-visible:ring-offset-2",
    ],
    actions: ["flex", "items-center", "gap-1", "ml-auto"],
    action: [
      "inline-flex",
      "h-9",
      "w-9",
      "items-center",
      "justify-center",
      "rounded-md",
      "border",
      "border-slate-200",
      "bg-white",
      "text-slate-500",
      "shadow-sm",
      "transition-colors",
      "focus-visible:outline-none",
      "focus-visible:ring-2",
      "focus-visible:ring-blue-500",
      "focus-visible:ring-offset-2",
    ],
    actionClear: ["hover:text-rose-500"],
    actionToggle: ["text-base", "text-slate-500"],
    menu: [
      "absolute",
      "left-0",
      "right-0",
      "z-20",
      "mt-2",
      "flex",
      "flex-col",
      "gap-1",
      "rounded-lg",
      "border",
      "border-slate-200",
      "bg-white",
      "p-2",
      "shadow-xl",
      "max-h-48",
      "overflow-y-auto",
    ],
    menuItem: [
      "flex",
      "items-center",
      "gap-2",
      "rounded-md",
      "px-3",
      "py-2",
      "text-left",
      "text-sm",
      "text-slate-700",
      "transition",
      "hover:bg-slate-100",
      "focus-visible:outline-none",
      "focus-visible:ring-2",
      "focus-visible:ring-blue-500",
    ],
    menuEmpty: ["px-3", "py-2", "text-sm", "text-slate-500"],
    nativeSelect: ["hidden"],
  },
  typeahead: {
    container: ["relative", "w-full", "text-sm"],
    containerReady: ["block"],
    containerOpen: ["ring-2", "ring-blue-500", "ring-offset-2", "ring-offset-white"],
    control: [
      "flex",
      "items-center",
      "gap-2",
      "rounded-lg",
      "border",
      "border-slate-300",
      "bg-white",
      "px-3",
      "py-2",
      "shadow-sm",
      "transition",
      "focus-within:border-blue-500",
      "focus-within:ring-2",
      "focus-within:ring-blue-500",
      "focus-within:ring-offset-2",
    ],
    input: [
      "w-full",
      "min-w-0",
      "border-none",
      "bg-transparent",
      "px-0",
      "py-0",
      "text-sm",
      "text-slate-900",
      "placeholder:text-slate-400",
      "focus:outline-none",
    ],
    clear: [
      "inline-flex",
      "h-8",
      "w-8",
      "items-center",
      "justify-center",
      "rounded-md",
      "text-slate-400",
      "transition",
      "hover:bg-slate-100",
      "hover:text-slate-600",
      "focus-visible:outline-none",
      "focus-visible:ring-2",
      "focus-visible:ring-blue-500",
      "focus-visible:ring-offset-2",
      "disabled:cursor-default",
      "disabled:opacity-40",
    ],
    dropdown: [
      "absolute",
      "left-0",
      "right-0",
      "z-20",
      "mt-2",
      "flex",
      "flex-col",
      "gap-1",
      "rounded-lg",
      "border",
      "border-slate-200",
      "bg-white",
      "p-2",
      "shadow-xl",
      "max-h-56",
      "overflow-y-auto",
    ],
    option: [
      "flex",
      "items-center",
      "gap-2",
      "rounded-md",
      "px-3",
      "py-2",
      "text-left",
      "text-sm",
      "text-slate-700",
      "transition",
      "hover:bg-slate-100",
      "focus-visible:outline-none",
      "focus-visible:ring-2",
      "focus-visible:ring-blue-500",
    ],
    optionActive: ["bg-slate-100"],
    highlight: ["bg-amber-100", "font-semibold"],
    empty: ["px-3", "py-2", "text-sm", "text-slate-500"],
    nativeSelect: ["hidden"],
  },
};

let activeThemeClasses: ThemeClassMap = cloneThemeClasses(DEFAULT_THEME_CLASSES);

export function getThemeClasses(): ThemeClassMap {
  return cloneThemeClasses(activeThemeClasses);
}

export function registerThemeClasses(overrides: PartialThemeClassMap = {}): ThemeClassMap {
  const next: ThemeClassMap = {
    chips: mergeSection<ChipsClassMap>(activeThemeClasses.chips, overrides.chips),
    typeahead: mergeSection<TypeaheadClassMap>(activeThemeClasses.typeahead, overrides.typeahead),
  };
  activeThemeClasses = next;
  return getThemeClasses();
}

export function __resetThemeClassesForTests(): void {
  activeThemeClasses = cloneThemeClasses(DEFAULT_THEME_CLASSES);
}

export function setElementClasses(element: Element, classes: ClassToken[] | undefined): void {
  if (!classes || classes.length === 0) {
    element.className = "";
    return;
  }
  element.className = classes.join(" ");
}

export function addElementClasses(element: Element, classes: ClassToken[] | undefined): void {
  if (!classes || classes.length === 0) {
    return;
  }
  element.classList.add(...classes);
}

export function removeElementClasses(element: Element, classes: ClassToken[] | undefined): void {
  if (!classes || classes.length === 0) {
    return;
  }
  element.classList.remove(...classes);
}

export function classesToString(classes: ClassToken[] | undefined): string {
  if (!classes || classes.length === 0) {
    return "";
  }
  return classes.join(" ");
}

export function combineClasses(...lists: Array<ClassToken[] | undefined>): ClassToken[] {
  const result: ClassToken[] = [];
  const seen = new Set<string>();
  for (const list of lists) {
    if (!list) continue;
    for (const token of list) {
      if (!seen.has(token)) {
        seen.add(token);
        result.push(token);
      }
    }
  }
  return result;
}

function cloneThemeClasses(map: ThemeClassMap): ThemeClassMap {
  return {
    chips: cloneSection<ChipsClassMap>(map.chips),
    typeahead: cloneSection<TypeaheadClassMap>(map.typeahead),
  };
}

function cloneSection<T extends Record<string, ClassToken[]>>(section: T): T {
  const clone = {} as T;
  (Object.keys(section) as Array<keyof T>).forEach((key) => {
    clone[key] = [...section[key]] as T[keyof T];
  });
  return clone;
}

function mergeSection<T extends Record<string, ClassToken[]>>(
  base: T,
  overrides?: Partial<{ [K in keyof T]: ClassValue }>
): T {
  if (!overrides) {
    return cloneSection(base);
  }
  const next = cloneSection(base);
  (Object.keys(overrides) as Array<keyof T>).forEach((key) => {
    const value = overrides[key];
    if (value === undefined) {
      return;
    }
    next[key] = normalizeClassValue(value) as T[keyof T];
  });
  return next;
}

function normalizeClassValue(value: ClassValue): ClassToken[] {
  const tokens = Array.isArray(value) ? value : [value];
  return tokens
    .flatMap((token) => token.split(/\s+/))
    .map((token) => token.trim())
    .filter(Boolean);
}
