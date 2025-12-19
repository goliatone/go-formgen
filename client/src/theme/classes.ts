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
  icon: ClassToken[];
  placeholder: ClassToken[];
  /** @deprecated Search input is now inside the menu. Kept for backwards compatibility. */
  search: ClassToken[];
  /** @deprecated Search input is now inside the menu. Kept for backwards compatibility. */
  searchInput: ClassToken[];
  searchHighlight: ClassToken[];
  chip: ClassToken[];
  chipLabel: ClassToken[];
  chipRemove: ClassToken[];
  /** Avatar image inside a chip. */
  chipAvatar: ClassToken[];
  actions: ClassToken[];
  action: ClassToken[];
  actionClear: ClassToken[];
  actionToggle: ClassToken[];
  menu: ClassToken[];
  menuSearch: ClassToken[];
  menuSearchInput: ClassToken[];
  /** Divider line between menu sections (e.g., between options list and footer). */
  menuDivider: ClassToken[];
  menuList: ClassToken[];
  menuFooter: ClassToken[];
  menuFooterAction: ClassToken[];
  menuFooterActionFocused: ClassToken[];
  menuItem: ClassToken[];
  menuItemHighlighted: ClassToken[];
  /** Wrapper for icon or avatar in a menu item. */
  menuItemIcon: ClassToken[];
  /** Avatar image inside a menu item icon wrapper. */
  menuItemAvatar: ClassToken[];
  /** Container for title and description text in a menu item. */
  menuItemText: ClassToken[];
  /** Title/label span in a rich menu item. */
  menuItemTitle: ClassToken[];
  /** Description/subtitle span in a rich menu item. */
  menuItemDescription: ClassToken[];
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
  inputWithIcon: ClassToken[];
  clear: ClassToken[];
  dropdown: ClassToken[];
  dropdownList: ClassToken[];
  option: ClassToken[];
  optionActive: ClassToken[];
  highlight: ClassToken[];
  empty: ClassToken[];
  icon: ClassToken[];
  iconSvg: ClassToken[];
  nativeSelect: ClassToken[];
  createAction: ClassToken[];
  createActionFocused: ClassToken[];
}

export interface SwitchClassMap {
  [key: string]: ClassToken[];
  container: ClassToken[];
  input: ClassToken[];
  track: ClassToken[];
  toggle: ClassToken[];
}

export interface WysiwygClassMap {
  [key: string]: ClassToken[];
  wrapper: ClassToken[];
  editor: ClassToken[];
  content: ClassToken[];
  toolbar: ClassToken[];
}

export interface FileUploaderClassMap {
  [key: string]: ClassToken[];
  wrapper: ClassToken[];
  dropzone: ClassToken[];
  button: ClassToken[];
  preview: ClassToken[];
  fileList: ClassToken[];
  fileItem: ClassToken[];
  fileMeta: ClassToken[];
  fileName: ClassToken[];
  fileSize: ClassToken[];
  fileActions: ClassToken[];
  progress: ClassToken[];
  status: ClassToken[];
  error: ClassToken[];
}

export interface ThemeClassMap {
  chips: ChipsClassMap;
  typeahead: TypeaheadClassMap;
  switch: SwitchClassMap;
  wysiwyg: WysiwygClassMap;
  fileUploader: FileUploaderClassMap;
}

export type PartialThemeClassMap = {
  chips?: Partial<{ [K in keyof ChipsClassMap]: ClassValue }>;
  typeahead?: Partial<{ [K in keyof TypeaheadClassMap]: ClassValue }>;
  switch?: Partial<{ [K in keyof SwitchClassMap]: ClassValue }>;
  wysiwyg?: Partial<{ [K in keyof WysiwygClassMap]: ClassValue }>;
  fileUploader?: Partial<{ [K in keyof FileUploaderClassMap]: ClassValue }>;
};

const DEFAULT_THEME_CLASSES: ThemeClassMap = {
  chips: {
    container: ["relative", "w-full", "text-sm"],
    containerReady: ["flex"],
    containerOpen: [],
    inner: [
      "flex",
      "w-full",
      "items-stretch",
      "gap-3",
      "rounded-lg",
      "border",
      "border-gray-200",
      "bg-white",
      "py-3",
      "ps-4",
      "pe-9",
      "transition",
      "focus-within:border-blue-500",
      "focus-within:ring-blue-500",
    ],
    chips: ["flex", "flex-wrap", "gap-2", "grow", "items-center"],
    chipsContent: ["flex", "flex-wrap", "items-center", "gap-2"],
    icon: [
      "inline-flex",
      "items-center",
      "justify-center",
      "text-slate-400",
      "dark:text-slate-500",
    ],
    placeholder: ["text-slate-500"],
    // Deprecated: search input is now inside the menu header
    search: ["flex", "items-center", "gap-2", "grow"],
    searchInput: [
      "w-full",
      "min-w-[6rem]",
      "border-none",
      "bg-transparent",
      "px-0",
      "py-0",
      "text-sm",
      "placeholder:text-slate-400",
      "focus:outline-none",
      "focus:ring-0",
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
    chipAvatar: [
      "size-5",
      "rounded-full",
      "object-cover",
      "mr-1.5",
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
      "top-full",
      "left-0",
      "right-0",
      "z-20",
      "mt-1",
      "flex",
      "flex-col",
      "rounded-lg",
      "border",
      "border-gray-200",
      "bg-white",
      "shadow-xl",
      "overflow-hidden",
    ],
    menuSearch: [
      "p-2",
      "border-b",
      "border-gray-200",
      "dark:border-gray-700",
    ],
    menuSearchInput: [
      "w-full",
      "px-3",
      "py-2",
      "text-sm",
      "border",
      "border-gray-300",
      "rounded-md",
      "focus:outline-none",
      "focus:ring-2",
      "focus:ring-blue-500",
      "dark:bg-slate-800",
      "dark:border-gray-600",
    ],
    menuDivider: [
      "border-t",
      "border-gray-200",
      "dark:border-gray-700",
      "my-1",
    ],
    menuList: [
      "max-h-72",
      "overflow-y-auto",
      "space-y-0.5",
      "p-1",
    ],
    menuFooter: [
      "border-t",
      "border-gray-200",
      "dark:border-gray-700",
      "p-2",
    ],
    menuFooterAction: [
      "flex",
      "items-center",
      "gap-2",
      "w-full",
      "px-3",
      "py-2",
      "text-sm",
      "text-blue-600",
      "rounded-md",
      "hover:bg-blue-50",
      "focus:outline-none",
      "focus:bg-blue-50",
    ],
    menuFooterActionFocused: [
      "bg-blue-50",
      "ring-2",
      "ring-blue-500",
      "ring-inset",
    ],
    menuItem: [
      "flex",
      "justify-between",
      "items-center",
      "gap-2",
      "rounded-lg",
      "px-4",
      "py-2",
      "w-full",
      "text-left",
      "text-sm",
      "text-gray-800",
      "cursor-pointer",
      "transition",
      "hover:bg-gray-100",
      "focus:outline-none",
      "focus:bg-gray-100",
    ],
    menuItemHighlighted: [
      "bg-blue-50",
      "dark:bg-slate-700",
    ],
    menuItemIcon: [
      "shrink-0",
      "size-8",
      "rounded-full",
      "overflow-hidden",
      "mr-3",
      "flex",
      "items-center",
      "justify-center",
    ],
    menuItemAvatar: [
      "w-full",
      "h-full",
      "object-cover",
    ],
    menuItemText: [
      "flex",
      "flex-col",
      "flex-1",
      "min-w-0",
    ],
    menuItemTitle: [
      "font-medium",
      "text-gray-900",
      "dark:text-white",
      "truncate",
    ],
    menuItemDescription: [
      "text-sm",
      "text-gray-500",
      "dark:text-gray-400",
      "truncate",
    ],
    menuEmpty: ["px-3", "py-2", "text-sm", "text-slate-500"],
    nativeSelect: ["hidden"],
  },
  typeahead: {
    container: ["relative", "w-full", "text-sm"],
    containerReady: ["block"],
    containerOpen: [],
    control: [
      "flex",
      "items-center",
      "gap-x-2",
      "relative",
      "rounded-lg",
      "border",
      "border-gray-200",
      "bg-white",
      "py-3",
      "ps-4",
      "pe-9",
      "transition",
      "focus-within:border-blue-500",
      "focus-within:ring-blue-500",
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
      "focus:ring-0",
    ],
    inputWithIcon: ["ps-10"],
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
      "rounded-lg",
      "border",
      "border-gray-200",
      "bg-white",
      "p-1",
      "shadow-xl",
      "overflow-hidden",
      "flex",
      "flex-col",
    ],
    dropdownList: [
      "flex",
      "flex-col",
      "space-y-0.5",
      "min-h-0",
      "max-h-72",
      "overflow-y-auto",
    ],
    option: [
      "flex",
      "justify-between",
      "items-center",
      "gap-2",
      "rounded-lg",
      "px-4",
      "py-2",
      "w-full",
      "text-left",
      "text-sm",
      "text-gray-800",
      "cursor-pointer",
      "transition",
      "hover:bg-gray-100",
      "focus:outline-none",
      "focus:bg-gray-100",
    ],
    optionActive: ["bg-slate-100"],
    highlight: ["bg-amber-100", "font-semibold"],
    empty: ["px-3", "py-2", "text-sm", "text-slate-500"],
    icon: [
      "absolute",
      "top-1/2",
      "start-3",
      "-translate-y-1/2",
      "pointer-events-none",
      "text-slate-400",
      "dark:text-slate-500",
      "flex",
      "items-center",
      "justify-center",
    ],
    iconSvg: ["size-5", "text-current"],
    nativeSelect: ["hidden"],
    createAction: [
      "flex",
      "items-center",
      "gap-2",
      "w-full",
      "px-4",
      "py-2",
      "text-sm",
      "text-blue-600",
      "font-medium",
      "rounded-lg",
      "cursor-pointer",
      "transition",
      "border-t",
      "border-gray-100",
      "mt-1",
      "hover:bg-blue-50",
      "focus:outline-none",
      "focus:bg-blue-50",
    ],
    createActionFocused: [
      "bg-blue-50",
      "ring-2",
      "ring-blue-500",
      "ring-inset",
    ],
  },
  switch: {
    container: [
      "relative",
      "inline-block",
      "w-11",
      "h-6",
      "cursor-pointer",
    ],
    input: [
      "peer",
      "sr-only",
    ],
    track: [
      "absolute",
      "inset-0",
      "bg-gray-200",
      "rounded-full",
      "transition-colors",
      "duration-200",
      "ease-in-out",
      "peer-checked:bg-blue-600",
      "dark:bg-neutral-700",
      "dark:peer-checked:bg-blue-500",
      "peer-disabled:opacity-50",
      "peer-disabled:pointer-events-none",
    ],
    toggle: [
      "absolute",
      "top-1/2",
      "start-0.5",
      "-translate-y-1/2",
      "size-5",
      "bg-white",
      "rounded-full",
      "shadow-xs",
      "transition-transform",
      "duration-200",
      "ease-in-out",
      "peer-checked:translate-x-full",
      "dark:bg-neutral-400",
      "dark:peer-checked:bg-white",
    ],
  },
  wysiwyg: {
    wrapper: ["wysiwyg-wrapper"],
    editor: [
      "wysiwyg-editor",
      "border",
      "border-gray-200",
      "rounded-lg",
      "overflow-hidden",
      "dark:border-neutral-700",
      "focus-within:border-blue-500",
      "focus-within:ring-1",
      "focus-within:ring-blue-500",
    ],
    content: [
      "max-w-none",
      "p-4",
      "min-h-[200px]",
      "focus:outline-none",
      "text-sm",
      "leading-relaxed",
      "text-gray-800",
      "dark:text-neutral-200",
    ],
    toolbar: [
      "wysiwyg-toolbar",
      "flex",
      "gap-1",
      "p-2",
      "border-b",
      "border-gray-200",
      "bg-gray-50",
      "dark:border-neutral-700",
      "dark:bg-neutral-800",
    ],
  },
  fileUploader: {
    wrapper: ["space-y-4"],
    dropzone: [
      "border-2",
      "border-dashed",
      "border-gray-300",
      "rounded-xl",
      "px-6",
      "py-10",
      "text-center",
      "text-sm",
      "text-gray-600",
      "cursor-pointer",
      "hover:border-blue-500",
      "hover:text-blue-600",
      "transition",
    ],
    button: [
      "inline-flex",
      "items-center",
      "justify-center",
      "gap-2",
      "rounded-lg",
      "border",
      "border-gray-300",
      "px-4",
      "py-2.5",
      "text-sm",
      "font-medium",
      "text-gray-700",
      "hover:bg-gray-50",
      "focus-visible:outline-none",
      "focus-visible:ring-2",
      "focus-visible:ring-blue-500",
      "dark:text-gray-200",
      "dark:border-gray-600",
    ],
    preview: [
      "w-full",
      "h-48",
      "object-cover",
      "rounded-lg",
      "border",
      "border-gray-200",
      "bg-gray-50",
      "dark:border-gray-700",
    ],
    fileList: ["space-y-3"],
    fileItem: [
      "border",
      "border-gray-200",
      "rounded-lg",
      "p-3",
      "flex",
      "flex-col",
      "gap-2",
      "bg-white",
      "dark:border-gray-700",
      "dark:bg-gray-900",
    ],
    fileMeta: ["flex", "items-center", "justify-between", "text-sm", "text-gray-700", "dark:text-gray-300"],
    fileName: ["font-medium", "truncate"],
    fileSize: ["text-xs", "text-gray-500"],
    fileActions: ["flex", "items-center", "justify-between", "text-xs", "text-gray-500"],
    progress: ["w-full", "h-2", "bg-gray-100", "rounded-full", "overflow-hidden"],
    status: ["text-sm", "text-gray-600"],
    error: ["text-sm", "text-red-600"],
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
    switch: mergeSection<SwitchClassMap>(activeThemeClasses.switch, overrides.switch),
    wysiwyg: mergeSection<WysiwygClassMap>(activeThemeClasses.wysiwyg, overrides.wysiwyg),
    fileUploader: mergeSection<FileUploaderClassMap>(activeThemeClasses.fileUploader, overrides.fileUploader),
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
    switch: cloneSection<SwitchClassMap>(map.switch),
    wysiwyg: cloneSection<WysiwygClassMap>(map.wysiwyg),
    fileUploader: cloneSection<FileUploaderClassMap>(map.fileUploader),
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
