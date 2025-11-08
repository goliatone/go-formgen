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
    editorContainer.className = theme.editor.join(" ");
    wrapper.appendChild(editorContainer);
  }

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
