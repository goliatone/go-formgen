/**
 * JSON GUI Editor
 *
 * Provides a visual key-value editor for JSON objects and arrays with support for:
 * - Primitive types: string, number, boolean, null
 * - Nested types: object, array (rendered inline recursively)
 * - Top-level arrays and objects
 * - Add, delete, reorder fields/items
 * - Mode toggle between GUI and raw textarea (hybrid mode)
 * - Readonly/disabled states
 * - Type coercion with validation
 */

// ============================================================================
// Types
// ============================================================================

type JSONValue = string | number | boolean | null | JSONObject | JSONArray;
type JSONObject = { [key: string]: JSONValue };
type JSONArray = JSONValue[];

type JSONType = "string" | "number" | "boolean" | "null" | "object" | "array";

interface FieldRow {
  id: string;
  key: string;
  value: JSONValue;
  type: JSONType;
  element: HTMLElement;
  depth: number;
  lastValidNumber?: number; // Track last valid number for validation
  hasError?: boolean;
  numberError?: string;
}

interface EditorState {
  root: HTMLElement;
  textarea: HTMLTextAreaElement | null;
  preview: HTMLPreElement | null;
  guiContainer: HTMLElement | null;
  rowsContainer: HTMLElement | null;
  addButton: HTMLButtonElement | null;
  rows: FieldRow[];
  mode: "raw" | "gui" | "hybrid";
  activeView: "raw" | "gui";
  readonly: boolean;
  disabled: boolean;
  rootType: "object" | "array";
  parseError: string | null;
}

// ============================================================================
// Constants
// ============================================================================

const ROOT_SELECTOR = '[data-json-editor="true"]';
const INIT_ATTR = "data-json-editor-init";

const TYPE_OPTIONS: { value: JSONType; label: string }[] = [
  { value: "string", label: "String" },
  { value: "number", label: "Number" },
  { value: "boolean", label: "Boolean" },
  { value: "null", label: "Null" },
  { value: "object", label: "Object" },
  { value: "array", label: "Array" },
];

// ============================================================================
// Utilities
// ============================================================================

let rowIdCounter = 0;
function generateRowId(): string {
  return `json-row-${++rowIdCounter}`;
}

function parseJSON(value: string): JSONValue | undefined {
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function stringifyJSON(value: JSONValue): string {
  return JSON.stringify(value, null, 2);
}

function detectType(value: JSONValue): JSONType {
  if (value === null) return "null";
  if (Array.isArray(value)) return "array";
  if (typeof value === "object") return "object";
  if (typeof value === "number") return "number";
  if (typeof value === "boolean") return "boolean";
  return "string";
}

function detectRootType(value: JSONValue): "object" | "array" {
  return Array.isArray(value) ? "array" : "object";
}

function getDefaultValue(type: JSONType): JSONValue {
  switch (type) {
    case "string":
      return "";
    case "number":
      return 0;
    case "boolean":
      return false;
    case "null":
      return null;
    case "object":
      return {};
    case "array":
      return [];
  }
}

/**
 * Validates and parses a number string.
 * Returns { valid: true, value: number } if valid.
 * Returns { valid: false, error: string } if invalid.
 */
function validateNumber(input: string): { valid: true; value: number } | { valid: false; error: string } {
  const trimmed = input.trim();

  // Empty input is invalid for numbers
  if (trimmed === "") {
    return { valid: false, error: "Number required" };
  }

  const num = Number(trimmed);

  // Check for NaN
  if (Number.isNaN(num)) {
    return { valid: false, error: "Invalid number" };
  }

  // Check for Infinity
  if (!Number.isFinite(num)) {
    return { valid: false, error: "Infinity not allowed" };
  }

  return { valid: true, value: num };
}

/**
 * Coerce a value when switching types.
 * String values preserve exact input.
 * Number values only accept valid numbers.
 */
function coerceToType(currentValue: JSONValue, newType: JSONType): JSONValue {
  switch (newType) {
    case "string":
      // Convert to string, preserving value
      if (currentValue === null) return "null";
      if (typeof currentValue === "object") return JSON.stringify(currentValue);
      return String(currentValue);
    case "number": {
      // Try to parse, default to 0 on failure
      if (typeof currentValue === "number") return currentValue;
      if (typeof currentValue === "string") {
        const result = validateNumber(currentValue);
        return result.valid ? result.value : 0;
      }
      return 0;
    }
    case "boolean":
      return Boolean(currentValue);
    case "null":
      return null;
    case "object":
      // Initialize fresh object on type switch
      return {};
    case "array":
      // Initialize fresh array on type switch
      return [];
  }
}

// ============================================================================
// Row Rendering
// ============================================================================

function createRowElement(
  state: EditorState,
  key: string,
  value: JSONValue,
  type: JSONType,
  depth: number,
  onUpdate: () => void,
  isArrayItem: boolean = false
): FieldRow {
  const id = generateRowId();
  const row: FieldRow = {
    id,
    key,
    value,
    type,
    element: null as any,
    depth,
    lastValidNumber: typeof value === "number" ? value : 0,
    hasError: false
  };
  const isEditable = !state.readonly && !state.disabled;

  const el = document.createElement("div");
  el.className = `flex items-start gap-2 ${depth > 0 ? "ml-4 pl-2 border-l-2 border-gray-200 dark:border-gray-700" : ""}`;
  el.setAttribute("data-json-row-id", id);

  // Key/index input
  const keyInput = document.createElement("input");
  keyInput.type = "text";
  keyInput.value = key;

  if (isArrayItem) {
    // Array items show index as readonly
    keyInput.placeholder = "idx";
    keyInput.disabled = true;
    keyInput.readOnly = true;
    keyInput.className =
      "flex-shrink-0 w-16 px-2 py-1.5 text-sm text-center border border-gray-200 rounded-md bg-gray-100 dark:bg-slate-700 dark:border-gray-600 dark:text-gray-300 opacity-60 cursor-not-allowed";
  } else {
    keyInput.placeholder = "key";
    keyInput.disabled = !isEditable;
    keyInput.readOnly = state.readonly;
    keyInput.className =
      "flex-shrink-0 w-32 px-2 py-1.5 text-sm border border-gray-200 rounded-md bg-white dark:bg-slate-800 dark:border-gray-600 dark:text-gray-200 focus:border-blue-500 focus:ring-1 focus:ring-blue-500" +
      (!isEditable ? " opacity-60 cursor-not-allowed" : "");
    if (isEditable) {
      keyInput.addEventListener("input", () => {
        row.key = keyInput.value;
        onUpdate();
      });
    }
  }

  // Value input container (varies by type)
  const valueContainer = document.createElement("div");
  valueContainer.className = "flex-1 min-w-0";
  renderValueInput(valueContainer, row, state, onUpdate);

  // Type dropdown
  const typeSelect = document.createElement("select");
  typeSelect.disabled = !isEditable;
  typeSelect.className =
    "flex-shrink-0 w-24 px-2 py-1.5 text-sm border border-gray-200 rounded-md bg-white dark:bg-slate-800 dark:border-gray-600 dark:text-gray-200 focus:border-blue-500 focus:ring-1 focus:ring-blue-500" +
    (!isEditable ? " opacity-60 cursor-not-allowed" : "");
  for (const opt of TYPE_OPTIONS) {
    const option = document.createElement("option");
    option.value = opt.value;
    option.textContent = opt.label;
    option.selected = opt.value === type;
    typeSelect.appendChild(option);
  }
  if (isEditable) {
    typeSelect.addEventListener("change", () => {
      const newType = typeSelect.value as JSONType;
      const prevValue = row.value;
      row.type = newType;
      row.hasError = false;
      row.numberError = undefined;

      if (newType === "number") {
        if (typeof prevValue === "number") {
          row.value = prevValue;
          row.lastValidNumber = prevValue;
        } else if (typeof prevValue === "string") {
          const result = validateNumber(prevValue);
          if (result.valid) {
            row.value = result.value;
            row.lastValidNumber = result.value;
          } else {
            row.value = row.lastValidNumber ?? 0;
            row.hasError = true;
            row.numberError = result.error;
          }
        } else {
          row.value = row.lastValidNumber ?? 0;
        }
      } else {
        row.value = coerceToType(prevValue, newType);
      }
      valueContainer.innerHTML = "";
      renderValueInput(valueContainer, row, state, onUpdate);
      onUpdate();
    });
  }

  // Action buttons (only shown when editable)
  const actions = document.createElement("div");
  actions.className = "flex items-center gap-1 flex-shrink-0";

  if (isEditable) {
    const moveUpBtn = createActionButton("↑", "Move up", () => {
      moveRow(state, row, -1);
      onUpdate();
    });
    const moveDownBtn = createActionButton("↓", "Move down", () => {
      moveRow(state, row, 1);
      onUpdate();
    });
    const deleteBtn = createActionButton("×", "Delete", () => {
      deleteRow(state, row);
      onUpdate();
    });
    deleteBtn.classList.add("text-red-500", "hover:text-red-700");

    actions.appendChild(moveUpBtn);
    actions.appendChild(moveDownBtn);
    actions.appendChild(deleteBtn);
  }

  el.appendChild(keyInput);
  el.appendChild(valueContainer);
  el.appendChild(typeSelect);
  el.appendChild(actions);

  row.element = el;
  return row;
}

function renderValueInput(
  container: HTMLElement,
  row: FieldRow,
  state: EditorState,
  onUpdate: () => void
): void {
  const isEditable = !state.readonly && !state.disabled;

  switch (row.type) {
    case "boolean": {
      const wrapper = document.createElement("div");
      wrapper.className = "flex items-center gap-2 py-1.5";

      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = row.value === true;
      checkbox.disabled = !isEditable;
      checkbox.className =
        "w-5 h-5 text-blue-600 border-gray-300 rounded focus:ring-blue-500" +
        (!isEditable ? " opacity-60 cursor-not-allowed" : "");
      if (isEditable) {
        checkbox.addEventListener("change", () => {
          row.value = checkbox.checked;
          onUpdate();
        });
      }

      const label = document.createElement("span");
      label.textContent = row.value ? "true" : "false";
      label.className = "text-sm text-gray-600 dark:text-gray-400";

      if (isEditable) {
        checkbox.addEventListener("change", () => {
          label.textContent = checkbox.checked ? "true" : "false";
        });
      }

      wrapper.appendChild(checkbox);
      wrapper.appendChild(label);
      container.appendChild(wrapper);
      break;
    }
    case "null": {
      const nullLabel = document.createElement("span");
      nullLabel.textContent = "null";
      nullLabel.className = "text-sm text-gray-400 italic py-1.5 block";
      container.appendChild(nullLabel);
      break;
    }
    case "number": {
      const wrapper = document.createElement("div");
      wrapper.className = "relative";

      const input = document.createElement("input");
      input.type = "text"; // Use text to have full control over validation
      input.inputMode = "decimal"; // Mobile keyboard hint
      input.value = String(row.value ?? 0);
      input.disabled = !isEditable;
      input.readOnly = state.readonly;
      input.className =
        "w-full px-2 py-1.5 text-sm border rounded-md bg-white dark:bg-slate-800 dark:text-gray-200 focus:ring-1" +
        (row.hasError
          ? " border-red-500 focus:border-red-500 focus:ring-red-500"
          : " border-gray-200 dark:border-gray-600 focus:border-blue-500 focus:ring-blue-500") +
        (!isEditable ? " opacity-60 cursor-not-allowed" : "");

      const errorEl = document.createElement("span");
      errorEl.className = "absolute right-2 top-1/2 -translate-y-1/2 text-xs text-red-500 hidden";
      input.dataset.lastValidNumber = String(row.lastValidNumber ?? 0);

      if (row.hasError) {
        errorEl.textContent = row.numberError ?? "Invalid number";
        errorEl.classList.remove("hidden");
      }

      if (isEditable) {
        input.addEventListener("input", () => {
          const result = validateNumber(input.value);
          if (result.valid) {
            row.value = result.value;
            row.lastValidNumber = result.value;
            row.hasError = false;
            row.numberError = undefined;
            input.dataset.lastValidNumber = String(result.value);
            input.classList.remove("border-red-500", "focus:border-red-500", "focus:ring-red-500");
            input.classList.add("border-gray-200", "dark:border-gray-600", "focus:border-blue-500", "focus:ring-blue-500");
            errorEl.classList.add("hidden");
            onUpdate();
          } else {
            // Keep last valid value, show error
            row.value = row.lastValidNumber ?? 0;
            row.hasError = true;
            row.numberError = result.error;
            input.classList.remove("border-gray-200", "dark:border-gray-600", "focus:border-blue-500", "focus:ring-blue-500");
            input.classList.add("border-red-500", "focus:border-red-500", "focus:ring-red-500");
            errorEl.textContent = result.error;
            errorEl.classList.remove("hidden");
            // Don't call onUpdate() - keep last valid value in JSON
          }
        });

        // On blur, restore to last valid value if error
        input.addEventListener("blur", () => {
          if (row.hasError) {
            input.value = String(row.lastValidNumber ?? 0);
            row.hasError = false;
            row.numberError = undefined;
            input.dataset.lastValidNumber = String(row.lastValidNumber ?? 0);
            input.classList.remove("border-red-500", "focus:border-red-500", "focus:ring-red-500");
            input.classList.add("border-gray-200", "dark:border-gray-600", "focus:border-blue-500", "focus:ring-blue-500");
            errorEl.classList.add("hidden");
          }
        });
      }

      wrapper.appendChild(input);
      wrapper.appendChild(errorEl);
      container.appendChild(wrapper);
      break;
    }
    case "object":
    case "array": {
      // Inline nested editor
      const nestedContainer = document.createElement("div");
      nestedContainer.className = "space-y-2 py-1";

      const nestedLabel = document.createElement("span");
      nestedLabel.textContent = row.type === "object" ? "{ Object }" : "[ Array ]";
      nestedLabel.className =
        "text-xs text-gray-500 dark:text-gray-400 font-medium block mb-1";
      nestedContainer.appendChild(nestedLabel);

      const nestedRows = document.createElement("div");
      nestedRows.className = "space-y-2";
      if (row.type === "array") {
        nestedRows.setAttribute("data-json-array", "true");
      }

      // Render nested rows
      if (row.type === "object" && typeof row.value === "object" && row.value !== null && !Array.isArray(row.value)) {
        for (const [k, v] of Object.entries(row.value)) {
          const nestedRow = createRowElement(
            state,
            k,
            v,
            detectType(v),
            row.depth + 1,
            () => {
              // Rebuild object from nested rows
              const obj: JSONObject = {};
              nestedRows.querySelectorAll(":scope > [data-json-row-id]").forEach((el) => {
                const keyInput = el.querySelector('input[type="text"]') as HTMLInputElement;
                if (keyInput && !keyInput.disabled) {
                  obj[keyInput.value] = getNestedRowValue(el);
                } else if (keyInput) {
                  // For disabled key inputs (shouldn't happen in objects, but safety)
                  obj[keyInput.value] = getNestedRowValue(el);
                }
              });
              row.value = obj;
              onUpdate();
            },
            false // Not an array item
          );
          nestedRows.appendChild(nestedRow.element);
        }
      } else if (row.type === "array" && Array.isArray(row.value)) {
        row.value.forEach((item, idx) => {
          const nestedRow = createRowElement(
            state,
            String(idx),
            item,
            detectType(item),
            row.depth + 1,
            () => {
              // Rebuild array from nested rows
              const arr: JSONArray = [];
              nestedRows.querySelectorAll(":scope > [data-json-row-id]").forEach((el) => {
                arr.push(getNestedRowValue(el));
              });
              row.value = arr;
              onUpdate();
            },
            true // Is an array item
          );
          nestedRows.appendChild(nestedRow.element);
        });
      }

      nestedContainer.appendChild(nestedRows);

      // Add button for nested (only when editable)
      if (isEditable) {
        const addNestedBtn = document.createElement("button");
        addNestedBtn.type = "button";
        addNestedBtn.textContent = row.type === "array" ? "+ Add Item" : "+ Add Field";
        addNestedBtn.className =
          "mt-1 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400";
        addNestedBtn.addEventListener("click", () => {
          const isNestedArray = row.type === "array";
          const newKey = isNestedArray ? String(nestedRows.children.length) : "";
          const nestedRow = createRowElement(
            state,
            newKey,
            "",
            "string",
            row.depth + 1,
            () => {
              if (row.type === "object") {
                const obj: JSONObject = {};
                nestedRows.querySelectorAll(":scope > [data-json-row-id]").forEach((el) => {
                  const keyInput = el.querySelector('input[type="text"]') as HTMLInputElement;
                  if (keyInput) obj[keyInput.value] = getNestedRowValue(el);
                });
                row.value = obj;
              } else {
                const arr: JSONArray = [];
                nestedRows.querySelectorAll(":scope > [data-json-row-id]").forEach((el) => {
                  arr.push(getNestedRowValue(el));
                });
                row.value = arr;
              }
              if (row.type === "array") {
                syncArrayRowKeys(nestedRows);
              }
              onUpdate();
            },
            isNestedArray
          );
          nestedRows.appendChild(nestedRow.element);

          // Trigger update
          if (row.type === "object") {
            const obj: JSONObject = {};
            nestedRows.querySelectorAll(":scope > [data-json-row-id]").forEach((el) => {
              const keyInput = el.querySelector('input[type="text"]') as HTMLInputElement;
              if (keyInput) obj[keyInput.value] = getNestedRowValue(el);
            });
            row.value = obj;
          } else {
            const arr: JSONArray = [];
            nestedRows.querySelectorAll(":scope > [data-json-row-id]").forEach((el) => {
              arr.push(getNestedRowValue(el));
            });
            row.value = arr;
          }
          if (row.type === "array") {
            syncArrayRowKeys(nestedRows);
          }
          onUpdate();
        });
        nestedContainer.appendChild(addNestedBtn);
      }

      container.appendChild(nestedContainer);
      break;
    }
    default: {
      // String - preserves exact input
      const input = document.createElement("input");
      input.type = "text";
      input.value = String(row.value ?? "");
      input.disabled = !isEditable;
      input.readOnly = state.readonly;
      input.className =
        "w-full px-2 py-1.5 text-sm border border-gray-200 rounded-md bg-white dark:bg-slate-800 dark:border-gray-600 dark:text-gray-200 focus:border-blue-500 focus:ring-1 focus:ring-blue-500" +
        (!isEditable ? " opacity-60 cursor-not-allowed" : "");
      if (isEditable) {
        input.addEventListener("input", () => {
          // Preserve exact string input - no coercion
          row.value = input.value;
          onUpdate();
        });
      }
      container.appendChild(input);
    }
  }
}

function getNestedRowValue(rowEl: Element): JSONValue {
  // Get value from a row element
  const typeSelect = rowEl.querySelector("select") as HTMLSelectElement;
  const type = (typeSelect?.value || "string") as JSONType;

  switch (type) {
    case "boolean": {
      const checkbox = rowEl.querySelector('input[type="checkbox"]') as HTMLInputElement;
      return checkbox?.checked ?? false;
    }
    case "null":
      return null;
    case "number": {
      // Get from text input (we use text type for validation)
      const input = rowEl.querySelector('input[type="text"][inputmode="decimal"]') as HTMLInputElement;
      if (input) {
        const result = validateNumber(input.value);
        if (result.valid) {
          return result.value;
        }
        const lastValid = input.dataset.lastValidNumber;
        if (lastValid !== undefined && lastValid !== "") {
          return Number(lastValid);
        }
        return 0;
      }
      // Fallback to number input
      const numInput = rowEl.querySelector('input[type="number"]') as HTMLInputElement;
      return parseFloat(numInput?.value ?? "0") || 0;
    }
    case "object":
    case "array": {
      // Recursively collect nested values
      const nestedRows = rowEl.querySelectorAll(":scope > div > div > div > [data-json-row-id]");
      if (type === "object") {
        const obj: JSONObject = {};
        nestedRows.forEach((el) => {
          const keyInput = el.querySelector('input[type="text"]') as HTMLInputElement;
          if (keyInput) obj[keyInput.value] = getNestedRowValue(el);
        });
        return obj;
      } else {
        const arr: JSONArray = [];
        nestedRows.forEach((el) => arr.push(getNestedRowValue(el)));
        return arr;
      }
    }
    default: {
      // String - find the value input (not the key input)
      const inputs = rowEl.querySelectorAll('input[type="text"]');
      // The value input is the one that's not disabled (for objects) or the second one (for arrays)
      for (let i = inputs.length - 1; i >= 0; i--) {
        const input = inputs[i] as HTMLInputElement;
        // Skip key inputs (first input, or disabled inputs for arrays)
        if (i > 0 || (!input.disabled && input.placeholder !== "key" && input.placeholder !== "idx")) {
          return input.value ?? "";
        }
      }
      // Fallback: second input is value
      if (inputs.length > 1) {
        return (inputs[1] as HTMLInputElement).value ?? "";
      }
      return "";
    }
  }
}

function syncArrayRowKeys(container: Element | null): void {
  if (!container) {
    return;
  }
  const rows = Array.from(container.querySelectorAll<HTMLElement>(":scope > [data-json-row-id]"));
  rows.forEach((rowEl, index) => {
    const keyInput = rowEl.querySelector('input[type="text"]') as HTMLInputElement | null;
    if (keyInput) {
      keyInput.value = String(index);
    }
  });
}

function createActionButton(
  label: string,
  title: string,
  onClick: () => void
): HTMLButtonElement {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.textContent = label;
  btn.title = title;
  btn.className =
    "w-6 h-6 flex items-center justify-center text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded hover:bg-gray-100 dark:hover:bg-gray-700";
  btn.addEventListener("click", (e) => {
    e.preventDefault();
    onClick();
  });
  return btn;
}

// ============================================================================
// State Management
// ============================================================================

function moveRow(state: EditorState, row: FieldRow, direction: -1 | 1): void {
  const idx = state.rows.indexOf(row);
  if (idx !== -1) {
    // Root-level row
    const newIdx = idx + direction;
    if (newIdx < 0 || newIdx >= state.rows.length) return;

    // Swap in array
    [state.rows[idx], state.rows[newIdx]] = [state.rows[newIdx], state.rows[idx]];

    // Swap in DOM
    if (state.rowsContainer) {
      const elements = Array.from(state.rowsContainer.children);
      if (direction === -1 && idx > 0) {
        state.rowsContainer.insertBefore(elements[idx], elements[idx - 1]);
      } else if (direction === 1 && idx < elements.length - 1) {
        state.rowsContainer.insertBefore(elements[idx + 1], elements[idx]);
      }
      if (state.rootType === "array") {
        syncArrayRowKeys(state.rowsContainer);
      }
    }
    return;
  }

  // Nested row
  const container = row.element.parentElement;
  if (!container) {
    return;
  }
  const elements = Array.from(
    container.querySelectorAll<HTMLElement>(":scope > [data-json-row-id]")
  );
  const currentIdx = elements.indexOf(row.element);
  if (currentIdx === -1) {
    return;
  }
  const newIdx = currentIdx + direction;
  if (newIdx < 0 || newIdx >= elements.length) {
    return;
  }
  if (direction === -1) {
    container.insertBefore(elements[currentIdx], elements[newIdx]);
  } else {
    container.insertBefore(elements[newIdx], elements[currentIdx]);
  }
  if (container.getAttribute("data-json-array") === "true") {
    syncArrayRowKeys(container);
  }
}

function deleteRow(state: EditorState, row: FieldRow): void {
  const idx = state.rows.indexOf(row);
  if (idx !== -1) {
    // Root-level row
    state.rows.splice(idx, 1);
    row.element.remove();
    if (state.rootType === "array") {
      syncArrayRowKeys(state.rowsContainer);
    }
    return;
  }

  // Nested row
  const container = row.element.parentElement;
  row.element.remove();
  if (container && container.getAttribute("data-json-array") === "true") {
    syncArrayRowKeys(container);
  }
}

function addRow(state: EditorState, onUpdate: () => void): void {
  if (state.readonly || state.disabled) return;

  const isArray = state.rootType === "array";
  const key = isArray ? String(state.rows.length) : "";
  const row = createRowElement(state, key, "", "string", 0, onUpdate, isArray);
  state.rows.push(row);
  state.rowsContainer?.appendChild(row.element);

  if (isArray) {
    syncArrayRowKeys(state.rowsContainer);
  }
  onUpdate();
}

function buildJSONFromRows(state: EditorState): JSONValue {
  if (state.rootType === "array") {
    return state.rows.map((row) => row.value);
  }
  const result: JSONObject = {};
  for (const row of state.rows) {
    if (row.key.trim()) {
      result[row.key] = row.value;
    }
  }
  return result;
}

function syncToTextarea(state: EditorState): void {
  const json = buildJSONFromRows(state);
  const str = stringifyJSON(json);

  // Sync to textarea if present
  if (state.textarea) {
    state.textarea.value = str;
  }

  // Update preview if present
  if (state.preview) {
    state.preview.textContent = str;
  }

  // Clear parse error and update state
  state.parseError = null;
  state.root.setAttribute("data-json-editor-state", "valid");

  // Update add button text
  updateAddButtonText(state);
}

function updateAddButtonText(state: EditorState): void {
  if (!state.addButton) return;

  // Find the text node or span inside the button
  const textSpan = state.addButton.querySelector("span") || state.addButton.lastChild;
  const newText = state.rootType === "array" ? "Add Item" : "Add Field";

  if (textSpan && textSpan.nodeType === Node.TEXT_NODE) {
    textSpan.textContent = newText;
  } else if (textSpan && textSpan instanceof HTMLElement) {
    textSpan.textContent = newText;
  } else {
    // Button has mixed content, find text node
    for (const child of state.addButton.childNodes) {
      if (child.nodeType === Node.TEXT_NODE && child.textContent?.trim()) {
        child.textContent = ` ${newText}`;
        break;
      }
    }
  }
}

function populateRowsFromJSON(state: EditorState, json: JSONValue): void {
  state.rows = [];
  if (state.rowsContainer) {
    state.rowsContainer.innerHTML = "";
  }

  const onUpdate = () => syncToTextarea(state);

  if (!state.rowsContainer) {
    return;
  }

  // Detect root type from JSON
  if (Array.isArray(json)) {
    state.rootType = "array";
    state.rowsContainer.setAttribute("data-json-array", "true");
    json.forEach((value, index) => {
      const type = detectType(value);
      const row = createRowElement(state, String(index), value, type, 0, onUpdate, true);
      state.rows.push(row);
      state.rowsContainer?.appendChild(row.element);
    });
    syncArrayRowKeys(state.rowsContainer);
  } else if (json && typeof json === "object") {
    state.rootType = "object";
    state.rowsContainer.removeAttribute("data-json-array");
    for (const [key, value] of Object.entries(json)) {
      const type = detectType(value);
      const row = createRowElement(state, key, value, type, 0, onUpdate, false);
      state.rows.push(row);
      state.rowsContainer?.appendChild(row.element);
    }
  } else {
    // Primitive at root - wrap in object (shouldn't happen normally)
    state.rootType = "object";
    state.rowsContainer.removeAttribute("data-json-array");
  }

  // Update button text
  updateAddButtonText(state);
}

function showParseError(state: EditorState, error: string): void {
  state.parseError = error;
  state.root.setAttribute("data-json-editor-state", "invalid");

  // Could add a visible error message element here if needed
  if (state.preview) {
    state.preview.setAttribute("data-state", "invalid");
  }
}

// ============================================================================
// Mode Toggle
// ============================================================================

function setActiveView(state: EditorState, view: "raw" | "gui"): void {
  state.activeView = view;
  state.root.setAttribute("data-json-editor-active", view);

  // Toggle visibility
  if (state.guiContainer) {
    state.guiContainer.classList.toggle("hidden", view !== "gui");
  }
  if (state.textarea) {
    state.textarea.classList.toggle("hidden", view !== "raw");
  }
  if (state.preview) {
    state.preview.classList.add("hidden");
  }

  // Update toggle buttons
  const toggleBtns = state.root.querySelectorAll("[data-json-editor-mode-btn]");
  toggleBtns.forEach((btn) => {
    const mode = btn.getAttribute("data-json-editor-mode-btn");
    const isActive = mode === view;
    btn.classList.toggle("bg-blue-600", isActive);
    btn.classList.toggle("text-white", isActive);
    btn.classList.toggle("border-blue-600", isActive);
    btn.classList.toggle("bg-white", !isActive);
    btn.classList.toggle("text-gray-700", !isActive);
    btn.classList.toggle("border-gray-200", !isActive);
  });

  // Sync data when switching views
  if (view === "gui" && state.textarea) {
    // Sync from textarea to GUI
    const parsed = parseJSON(state.textarea.value || "{}");
    if (parsed !== undefined) {
      if (typeof parsed === "object" || Array.isArray(parsed)) {
        populateRowsFromJSON(state, parsed);
        state.parseError = null;
      } else {
        showParseError(state, "Root must be an object or array");
      }
    } else {
      // Parse error - keep GUI state, show error
      showParseError(state, "Invalid JSON in raw editor");
    }
  } else if (view === "raw") {
    // Sync from GUI to textarea
    syncToTextarea(state);
  }
}

// ============================================================================
// Initialization
// ============================================================================

function initEditor(root: HTMLElement): void {
  if (root.getAttribute(INIT_ATTR) === "true") return;
  root.setAttribute(INIT_ATTR, "true");

  const textarea = root.querySelector("[data-json-editor-input]") as HTMLTextAreaElement | null;
  const preview = root.querySelector("[data-json-editor-preview]") as HTMLPreElement | null;
  const guiContainer = root.querySelector("[data-json-editor-gui]") as HTMLElement | null;
  const rowsContainer = root.querySelector("[data-json-editor-rows]") as HTMLElement | null;
  const addFieldBtn = root.querySelector("[data-json-editor-add-field]") as HTMLButtonElement | null;
  const modeToggle = root.querySelector("[data-json-editor-mode-toggle]") as HTMLElement | null;
  const formatBtn = root.querySelector("[data-json-editor-format]") as HTMLButtonElement | null;
  const collapseToggle = root.querySelector("[data-json-editor-toggle]") as HTMLButtonElement | null;

  const mode = (root.getAttribute("data-json-editor-mode") || "raw") as "raw" | "gui" | "hybrid";
  const activeView = (root.getAttribute("data-json-editor-active") || "raw") as "raw" | "gui";

  // Read readonly/disabled from data attributes
  const isReadonly = root.getAttribute("data-json-editor-readonly") === "true";
  const isDisabled = root.getAttribute("data-json-editor-disabled") === "true";

  const state: EditorState = {
    root,
    textarea,
    preview,
    guiContainer,
    rowsContainer,
    addButton: addFieldBtn,
    rows: [],
    mode,
    activeView,
    readonly: isReadonly,
    disabled: isDisabled,
    rootType: "object",
    parseError: null,
  };

  let initialValue = "{}";
  if (textarea) {
    initialValue = textarea.value || "{}";
  }

  const onUpdate = () => syncToTextarea(state);

  // Parse initial value and populate GUI
  if (guiContainer && rowsContainer) {
    const parsed = parseJSON(initialValue);
    if (parsed !== undefined) {
      if (typeof parsed === "object" || Array.isArray(parsed)) {
        // Determine root type from initial value
        state.rootType = detectRootType(parsed);
        populateRowsFromJSON(state, parsed);
      } else {
        state.rootType = "object";
        showParseError(state, "Root must be an object or array");
        updateAddButtonText(state);
      }
    } else {
      // Invalid initial JSON - start with empty object
      state.rootType = "object";
      showParseError(state, "Invalid initial JSON");
      updateAddButtonText(state);
    }
  }

  // Mode toggle handler
  if (modeToggle && !isDisabled) {
    modeToggle.querySelectorAll("[data-json-editor-mode-btn]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        const targetMode = btn.getAttribute("data-json-editor-mode-btn") as "raw" | "gui";
        setActiveView(state, targetMode);
      });
    });
  }

  if (!isReadonly && !isDisabled) {
    // Add field/item button
    if (addFieldBtn) {
      addFieldBtn.addEventListener("click", (e) => {
        e.preventDefault();
        addRow(state, onUpdate);
      });
    }

    // Raw editor: sync from textarea to preview/state
    if (textarea) {
      textarea.addEventListener("input", () => {
        const parsed = parseJSON(textarea.value);
        const valid = parsed !== undefined;
        state.root.setAttribute("data-json-editor-state", valid ? "valid" : "invalid");

        if (valid) {
          state.parseError = null;
          // Update rootType if parsed successfully
          if (typeof parsed === "object" || Array.isArray(parsed)) {
            state.rootType = detectRootType(parsed);
          }
        } else {
          state.parseError = "Invalid JSON";
        }

        if (preview) {
          preview.textContent = valid ? stringifyJSON(parsed) : textarea.value;
          preview.setAttribute("data-state", valid ? "valid" : "invalid");
        }
      });
    }

    // Format button
    if (formatBtn && textarea) {
      formatBtn.addEventListener("click", (e) => {
        e.preventDefault();
        const parsed = parseJSON(textarea.value);
        if (parsed !== undefined) {
          textarea.value = stringifyJSON(parsed);
        }
      });
    }
  }

  // Collapse toggle works even in readonly mode (it's just for viewing)
  if (collapseToggle && textarea && preview) {
    collapseToggle.addEventListener("click", (e) => {
      e.preventDefault();
      const isCollapsed = root.classList.contains("json-editor--collapsed");
      root.classList.toggle("json-editor--collapsed", !isCollapsed);
      textarea.classList.toggle("hidden", !isCollapsed);
      preview.classList.toggle("hidden", isCollapsed);
      collapseToggle.textContent = isCollapsed ? "Collapse" : "Expand";
      collapseToggle.setAttribute("aria-expanded", isCollapsed ? "true" : "false");
    });
  }
}

export function initJSONEditors(): void {
  document.querySelectorAll<HTMLElement>(ROOT_SELECTOR).forEach(initEditor);
}

// Auto-init on DOMContentLoaded
if (typeof document !== "undefined") {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initJSONEditors);
  } else {
    initJSONEditors();
  }
}
