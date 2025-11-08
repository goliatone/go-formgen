import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import Link from "@tiptap/extension-link";
import Underline from "@tiptap/extension-underline";
import type { WysiwygClassMap } from "../theme/classes.js";

export interface WysiwygConfig {
  toolbar?: any[];
  placeholder?: string;
  maxLength?: number;
}

export interface WysiwygStore {
  textarea: HTMLTextAreaElement;
  editor: HTMLElement;
  tiptap: Editor;
  config: WysiwygConfig;
  theme: WysiwygClassMap;
}

const WYSIWYG_ATTR = "data-fg-component";
const WYSIWYG_WRAPPER_ATTR = "data-fg-wysiwyg-wrapper";

export function renderWysiwyg(
  textarea: HTMLTextAreaElement,
  theme: WysiwygClassMap,
  config?: WysiwygConfig
): WysiwygStore {
  // Validate textarea exists
  if (!textarea || textarea.tagName !== "TEXTAREA") {
    throw new Error("WYSIWYG renderer requires a textarea element");
  }

  // Get wrapper
  const wrapper = textarea.closest(`[${WYSIWYG_WRAPPER_ATTR}]`) as HTMLElement;
  if (!wrapper) {
    throw new Error("WYSIWYG textarea must be inside a wrapper with data-fg-wysiwyg-wrapper attribute");
  }

  // Get or create editor container
  const editorId = `${textarea.id}-editor`;
  let editorContainer = document.getElementById(editorId);

  if (!editorContainer) {
    editorContainer = document.createElement("div");
    editorContainer.id = editorId;
    wrapper.appendChild(editorContainer);
  }

  // Always set theme classes (in case template created the div without proper classes)
  editorContainer.className = theme.editor.join(" ");

  // Parse config from data attribute if not provided
  const dataConfig = textarea.getAttribute("data-component-config");
  const mergedConfig: WysiwygConfig = {
    ...config,
    ...(dataConfig ? JSON.parse(dataConfig) : {}),
  };

  // Get placeholder
  const placeholder =
    mergedConfig.placeholder ||
    textarea.getAttribute("data-placeholder") ||
    textarea.placeholder ||
    "Start typing...";

  // Get maxLength
  const maxLength =
    mergedConfig.maxLength ||
    (textarea.hasAttribute("data-maxlength")
      ? parseInt(textarea.getAttribute("data-maxlength") || "0", 10)
      : undefined);

  // Hide original textarea (keep for form submission)
  textarea.classList.add("sr-only");
  textarea.setAttribute("aria-hidden", "true");

  // Create toolbar
  const toolbar = createToolbar(theme);
  editorContainer.insertBefore(toolbar, editorContainer.firstChild);

  // Initialize Tiptap
  const tiptap = new Editor({
    element: editorContainer,
    extensions: [
      StarterKit,
      Placeholder.configure({
        placeholder,
      }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: {
          class: "text-blue-600 underline hover:text-blue-700 dark:text-blue-400",
        },
      }),
      Underline,
    ],
    content: textarea.value || "",
    editorProps: {
      attributes: {
        class: theme.content.join(" "),
      },
    },
    onUpdate: ({ editor }) => {
      // Sync Tiptap → Textarea (for form submission)
      textarea.value = editor.getHTML();

      // Trigger change event for validation libraries
      textarea.dispatchEvent(new Event("change", { bubbles: true }));
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
    },
  });

  // Wire up toolbar buttons
  wireToolbar(toolbar, tiptap);

  // Handle form reset
  textarea.form?.addEventListener("reset", () => {
    setTimeout(() => {
      tiptap.commands.setContent(textarea.value || "");
    }, 0);
  });

  // Handle disabled state
  if (textarea.disabled) {
    tiptap.setEditable(false);
    editorContainer.classList.add("opacity-50", "pointer-events-none");
  }

  // Handle readonly state
  if (textarea.readOnly) {
    tiptap.setEditable(false);
  }

  const store: WysiwygStore = {
    textarea,
    editor: editorContainer,
    tiptap,
    config: mergedConfig,
    theme,
  };

  return store;
}

function createToolbar(theme: WysiwygClassMap): HTMLElement {
  const toolbar = document.createElement("div");
  toolbar.className = theme.toolbar.join(" ");

  toolbar.innerHTML = `
    <button type="button" data-action="bold" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Bold">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 12h9a4 4 0 0 1 0 8H7a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1h7a4 4 0 0 1 0 8"></path></svg>
    </button>
    <button type="button" data-action="italic" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Italic">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" x2="10" y1="4" y2="4"></line><line x1="14" x2="5" y1="20" y2="20"></line><line x1="15" x2="9" y1="4" y2="20"></line></svg>
    </button>
    <button type="button" data-action="underline" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Underline">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 4v6a6 6 0 0 0 12 0V4"></path><line x1="4" x2="20" y1="20" y2="20"></line></svg>
    </button>
    <button type="button" data-action="strike" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Strikethrough">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 4H9a3 3 0 0 0-2.83 4"></path><path d="M14 12a4 4 0 0 1 0 8H6"></path><line x1="4" x2="20" y1="12" y2="12"></line></svg>
    </button>
    <button type="button" data-action="link" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Link">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>
    </button>
    <button type="button" data-action="bulletList" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Bullet List">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="8" x2="21" y1="6" y2="6"></line><line x1="8" x2="21" y1="12" y2="12"></line><line x1="8" x2="21" y1="18" y2="18"></line><line x1="3" x2="3.01" y1="6" y2="6"></line><line x1="3" x2="3.01" y1="12" y2="12"></line><line x1="3" x2="3.01" y1="18" y2="18"></line></svg>
    </button>
    <button type="button" data-action="orderedList" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Numbered List">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="10" x2="21" y1="6" y2="6"></line><line x1="10" x2="21" y1="12" y2="12"></line><line x1="10" x2="21" y1="18" y2="18"></line><path d="M4 6h1v4"></path><path d="M4 10h2"></path><path d="M6 18H4c0-1 2-2 2-3s-1-1.5-2-1"></path></svg>
    </button>
    <button type="button" data-action="blockquote" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Blockquote">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 6H3"></path><path d="M21 12H8"></path><path d="M21 18H8"></path><path d="M3 12v6"></path></svg>
    </button>
    <button type="button" data-action="codeBlock" class="p-2 inline-flex items-center gap-x-1 text-sm font-medium rounded-lg border border-transparent text-gray-800 hover:bg-gray-100 disabled:opacity-50 disabled:pointer-events-none dark:text-white dark:hover:bg-neutral-700" title="Code Block">
      <svg class="shrink-0 size-4" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"></polyline><polyline points="8 6 2 12 8 18"></polyline></svg>
    </button>
  `;

  return toolbar;
}

function wireToolbar(toolbar: HTMLElement, editor: Editor): void {
  // Bold
  toolbar.querySelector('[data-action="bold"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleBold().run();
  });

  // Italic
  toolbar.querySelector('[data-action="italic"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleItalic().run();
  });

  // Underline
  toolbar.querySelector('[data-action="underline"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleUnderline().run();
  });

  // Strike
  toolbar.querySelector('[data-action="strike"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleStrike().run();
  });

  // Link
  toolbar.querySelector('[data-action="link"]')?.addEventListener('click', () => {
    const url = window.prompt('Enter URL:');
    if (url) {
      editor.chain().focus().setLink({ href: url }).run();
    }
  });

  // Bullet List
  toolbar.querySelector('[data-action="bulletList"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleBulletList().run();
  });

  // Ordered List
  toolbar.querySelector('[data-action="orderedList"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleOrderedList().run();
  });

  // Blockquote
  toolbar.querySelector('[data-action="blockquote"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleBlockquote().run();
  });

  // Code Block
  toolbar.querySelector('[data-action="codeBlock"]')?.addEventListener('click', () => {
    editor.chain().focus().toggleCodeBlock().run();
  });

  // Update button states when selection changes
  editor.on('selectionUpdate', () => {
    updateToolbarState(toolbar, editor);
  });
  editor.on('update', () => {
    updateToolbarState(toolbar, editor);
  });
}

function updateToolbarState(toolbar: HTMLElement, editor: Editor): void {
  const buttons = {
    bold: toolbar.querySelector('[data-action="bold"]'),
    italic: toolbar.querySelector('[data-action="italic"]'),
    underline: toolbar.querySelector('[data-action="underline"]'),
    strike: toolbar.querySelector('[data-action="strike"]'),
    bulletList: toolbar.querySelector('[data-action="bulletList"]'),
    orderedList: toolbar.querySelector('[data-action="orderedList"]'),
    blockquote: toolbar.querySelector('[data-action="blockquote"]'),
    codeBlock: toolbar.querySelector('[data-action="codeBlock"]'),
  };

  // Update active states
  Object.entries(buttons).forEach(([name, button]) => {
    if (button && editor.isActive(name)) {
      button.classList.add('bg-gray-100', 'dark:bg-neutral-700');
    } else if (button) {
      button.classList.remove('bg-gray-100', 'dark:bg-neutral-700');
    }
  });
}

// Auto-initialize on DOMContentLoaded
export function autoInitWysiwyg(theme?: WysiwygClassMap): void {
  const defaultTheme: WysiwygClassMap = theme || {
    wrapper: ["wysiwyg-wrapper", "space-y-2"],
    editor: [
      "wysiwyg-editor",
      "border",
      "border-gray-200",
      "rounded-lg",
      "dark:border-neutral-700",
    ],
    content: [
      "prose",
      "prose-sm",
      "max-w-none",
      "p-4",
      "min-h-[200px]",
      "focus:outline-none",
      "dark:prose-invert",
    ],
    toolbar: [
      "wysiwyg-toolbar",
      "border-b",
      "border-gray-200",
      "dark:border-neutral-700",
    ],
  };

  document.querySelectorAll<HTMLTextAreaElement>(`[${WYSIWYG_ATTR}="wysiwyg"]`).forEach((textarea) => {
    // Skip if already initialized
    if (textarea.hasAttribute("data-wysiwyg-initialized")) return;

    textarea.setAttribute("data-wysiwyg-initialized", "true");
    try {
      renderWysiwyg(textarea, defaultTheme);
    } catch (error) {
      console.error("[formgen:wysiwyg] Failed to initialize editor:", error);
      // Fallback: show textarea if initialization fails
      textarea.classList.remove("sr-only");
      textarea.removeAttribute("aria-hidden");
    }
  });
}
