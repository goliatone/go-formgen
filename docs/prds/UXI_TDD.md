# UX/UI Improvement Plan: Chips & Typeahead Components

This document captures the complete improvement plan for the relationship select components (chips for multi-select, typeahead for single-select) based on analysis of Preline's HSSelect component.

## Background

### Current Issues

1. **Search input location**: Currently mixed with chips in the toggle area, making the UI cluttered
2. **Dropdown positioning**: Menu overlaps/covers the input box instead of appearing below it
3. **Keyboard navigation**: No visual highlight when arrowing through options (only focus)
4. **Limited keyboard support**: Missing Home/End keys, Tab behavior, Space key
5. **No animation locking**: Rapid clicks can cause state mismatches
6. **Simple option rendering**: Text-only options, no support for icons/avatars/descriptions

### Reference Implementation

Preline's HSSelect component (`https://preline.co/docs/advanced-select.html`) provides a superior UX with:
- Search input inside dropdown (not in toggle area)
- Dropdown positioned below toggle with gap
- Rich option templates (icon + title + subtitle + checkmark)
- Full keyboard navigation (Arrow keys, Home/End, Tab, Space, Escape, Enter)
- Animation locking to prevent race conditions
- Visual highlight class on keyboard navigation

## Target Layout

### Current Structure
```
┌─────────────────────────────────┐
│ [chip] [chip] [search___] [▼]   │  ← search mixed with chips
├─────────────────────────────────┤
│ Option 1                        │  ← dropdown covers toggle
│ Option 2                        │
└─────────────────────────────────┘
```

### Target Structure (Pinned Header + Footer)
```
┌─────────────────────────────────┐
│ [chip] [chip] |            [▼]  │  ← clean toggle, cursor for inline typing
└─────────────────────────────────┘
┌─────────────────────────────────┐
│ [Search...___________________]  │  ← search input at top of dropdown
├─────────────────────────────────┤
│ [icon] Title                 ✓  │  ← rich option with checkmark
│        subtitle                 │
│ [icon] Title                    │
│        subtitle                 │
├─────────────────────────────────┤
│ Create "New item"               │  ← pinned footer action (optional)
└─────────────────────────────────┘
```

## Implementation Phases

### Phase 1: Layout Fixes (High Impact, Medium Effort)

#### 1.1 Move Search Input into Dropdown (Pinned Header + Footer + Listbox Split)

**Files to modify:**
- `client/src/renderers/chips.ts`
- `client/src/theme/classes.ts`

**Current code location:** `ensureStore()` function, lines 211-314

**Changes needed:**
1. Remove search input creation from the chips/toggle area
2. Split menu into a wrapper with a **pinned search header**, **scrollable options list**, and **pinned footer**
3. Only the options list gets `role="listbox"` and the keyboard/ARIA behavior
4. Footer hosts non-option actions (e.g. create action) and is focusable separately
5. Update `propagateSearch()` to work with the new location
6. Update focus management to open the menu first, then focus the search input

**DOM structure change:**
```ts
// Current: searchInput is child of chips element
chips.appendChild(searchContainer);

// Target: menu wrapper with pinned search + options list + footer
const menuWrapper = document.createElement("div");
setElementClasses(menuWrapper, theme.menu);

const searchWrapper = document.createElement("div");
setElementClasses(searchWrapper, theme.menuSearch);
searchWrapper.appendChild(searchInput);

const optionsList = document.createElement("div");
setElementClasses(optionsList, theme.menuList);
optionsList.setAttribute("role", "listbox");
// Prevent input blur only on options list, not the search input.
optionsList.addEventListener("mousedown", (event) => event.preventDefault());

const footerWrapper = document.createElement("div");
setElementClasses(footerWrapper, theme.menuFooter);

menuWrapper.append(searchWrapper, optionsList, footerWrapper);
container.append(inner, menuWrapper);
```

**Theme classes to add:**
```ts
// In classes.ts, add to ChipsClassMap:
menuSearch: ClassToken[];
menuSearchInput: ClassToken[];
menuDivider: ClassToken[];
menuList: ClassToken[];
menuFooter: ClassToken[];
menuFooterAction: ClassToken[];
menuFooterActionFocused: ClassToken[];
```

#### 1.2 Fix Dropdown Positioning

**Files to modify:**
- `client/src/theme/classes.ts`

**Current CSS (via classes):**
```ts
menu: [
  "absolute",
  "z-50",
  // ... positions menu overlapping toggle
]
```

**Target CSS:**
```ts
menu: [
  "absolute",
  "z-50",
  "top-full",      // Position below toggle
  "mt-1",          // 4px gap
  "left-0",
  "right-0",
  // ... rest of styling
]
```

#### 1.3 Add Visual Highlight for Keyboard Navigation

**Files to modify:**
- `client/src/renderers/chips.ts`
- `client/src/theme/classes.ts`

**Current behavior:** `focusMenuOption()` only calls `options[target]?.focus()`

**Target behavior:** Add highlight class before focusing, remove from previous

```ts
function focusMenuOption(store: ChipStore, index: number): void {
  const options = getMenuOptions(store);
  if (options.length === 0) {
    store.highlightedIndex = -1;
    return;
  }

  // Remove highlight from all options
  options.forEach(opt => removeElementClasses(opt, store.theme.menuItemHighlighted));

  const target = Math.max(0, Math.min(index, options.length - 1));
  store.highlightedIndex = target;

  // Add highlight class and focus
  const targetOption = options[target];
  if (targetOption) {
    addElementClasses(targetOption, store.theme.menuItemHighlighted);
    targetOption.focus();
    targetOption.scrollIntoView({ block: "nearest" });
  }
}
```

**Theme class to add:**
```ts
menuItemHighlighted: ["bg-blue-50", "dark:bg-slate-700"];
```

### Phase 2: Rich Options Support (Medium Impact, Medium Effort)

#### 2.1 Support Icon/Avatar in Options (Safe Icon Strategy)

**Files to modify:**
- `client/src/renderers/chips.ts`
- `client/src/config.ts` (Option type)

**Extend Option type:**
```ts
// In config.ts
interface Option {
  value: string;
  label: string;
  icon?: string;        // Icon name for registry lookup (no raw HTML)
  avatar?: string;      // URL to avatar image
  description?: string; // Subtitle text
}
```

**Update `renderMenu()` to support rich options:**
```ts
for (const option of available) {
  const button = document.createElement("button");
  // ... existing setup

  // Add icon/avatar if present
  if (option.avatar || option.icon) {
    const iconWrapper = document.createElement("span");
    setElementClasses(iconWrapper, theme.menuItemIcon);
    if (option.avatar) {
      const img = document.createElement("img");
      img.src = option.avatar;
      img.alt = "";
      setElementClasses(img, theme.menuItemAvatar);
      iconWrapper.appendChild(img);
    } else if (option.icon) {
      iconWrapper.appendChild(createIconElementFromRegistry(option.icon));
    }
    button.appendChild(iconWrapper);
  }

  // Add text container
  const textContainer = document.createElement("div");
  setElementClasses(textContainer, theme.menuItemText);

  // Title with highlight
  const titleSpan = document.createElement("span");
  setElementClasses(titleSpan, theme.menuItemTitle);
  titleSpan.appendChild(buildHighlightedFragment(label, query, classesToString(theme.searchHighlight)));
  textContainer.appendChild(titleSpan);

  // Description/subtitle if present
  if (option.description) {
    const descSpan = document.createElement("span");
    setElementClasses(descSpan, theme.menuItemDescription);
    descSpan.textContent = option.description;
    textContainer.appendChild(descSpan);
  }

  button.appendChild(textContainer);

  // Chips UI hides selected options; no checkmark needed here.

  menu.appendChild(button);
}
```

#### 2.2 Support Icon/Avatar in Chips (Lookup by Value)

**Update `renderChips()` to include icons:**
```ts
for (const option of selectedOptions) {
  const chip = document.createElement("span");
  setElementClasses(chip, theme.chip);

  // Add avatar if available (look up from store.options by value)
  const optionData = store.options.find((item) => item.value === option.value);
  if (optionData?.avatar) {
    const avatar = document.createElement("img");
    avatar.src = optionData.avatar;
    avatar.alt = "";
    setElementClasses(avatar, theme.chipAvatar);
    chip.appendChild(avatar);
  }

  // ... rest of chip rendering
}
```

### Phase 3: UX Polish (Medium Impact, Low Effort)

#### 3.1 Add Home/End Key Support

**Location:** `chips.ts`, keyboard event handlers

```ts
// In searchInput keydown handler and menu keydown handler:
if (event.key === "Home") {
  event.preventDefault();
  focusMenuOption(store, 0);
  return;
}

if (event.key === "End") {
  event.preventDefault();
  const options = getMenuOptions(store);
  focusMenuOption(store, options.length - 1);
  return;
}
```

#### 3.2 Tab Key Closes Dropdown

```ts
// In searchInput event listeners:
searchInput.addEventListener("keydown", (event) => {
  if (event.key === "Tab") {
    toggleMenu(store, false);
    // Don't prevent default - let focus move naturally
    return;
  }
  // ... rest of handler
});
```

#### 3.3 Animation Lock

**Add to ChipStore interface:**
```ts
interface ChipStore {
  // ... existing properties
  animationInProcess: boolean;
}
```

**Update toggleMenu:**
```ts
function toggleMenu(store: ChipStore, open: boolean): void {
  if (store.animationInProcess) {
    return;
  }

  if (store.isOpen === open) {
    return;
  }

  store.animationInProcess = true;

  // ... existing toggle logic

  // Release lock after transition (or immediately if no transition)
  requestAnimationFrame(() => {
    store.animationInProcess = false;
  });
}
```

#### 3.4 Focus Restoration on Close

**Update toggleMenu:**
```ts
function toggleMenu(store: ChipStore, open: boolean, restoreFocus = true): void {
  // ... existing logic

  if (!open && restoreFocus) {
    // Return focus to toggle button or chips area
    store.toggle.focus();
  }

  store.isOpen = open;
}
```

### Phase 4: Advanced Features (Optional)

#### 4.1 FloatingUI Integration

For smart positioning that handles viewport edges and `overflow: hidden` parents.

**Dependencies to add:**
```json
{
  "@floating-ui/dom": "^1.x"
}
```

**Implementation sketch:**
```ts
import { computePosition, flip, shift, offset } from '@floating-ui/dom';

async function positionDropdown(store: ChipStore): Promise<void> {
  const { x, y } = await computePosition(store.toggle, store.menu, {
    placement: 'bottom-start',
    middleware: [
      offset(4),
      flip(),
      shift({ padding: 8 }),
    ],
  });

  Object.assign(store.menu.style, {
    left: `${x}px`,
    top: `${y}px`,
  });
}
```

#### 4.2 Dynamic Input Width (Tags Mode)

Create a hidden span that mirrors the input's font, measure text width, resize input.

```ts
function updateInputWidth(input: HTMLInputElement, store: ChipStore): void {
  if (!store.widthMeasurer) {
    store.widthMeasurer = document.createElement("span");
    store.widthMeasurer.style.cssText = `
      position: absolute;
      visibility: hidden;
      white-space: pre;
      font: inherit;
    `;
    input.parentElement?.appendChild(store.widthMeasurer);
  }

  store.widthMeasurer.textContent = input.value || input.placeholder;
  const width = Math.max(50, store.widthMeasurer.offsetWidth + 4);
  input.style.width = `${width}px`;
}
```

## Test Updates Required

### Tests to Update

The following tests query for `input[type="text"]` in the chips container. After moving search to dropdown, these selectors need updating:

1. `"propagates search input to resolver requests"` - line 220
2. `"filters chip menus during search and resets after clearing"` - line 264
3. `"opens chips menu when typing without requiring extra click"` - line 324
4. `"shows create option in chips menu when typing non-matching query"` - line 387
5. `"allows creating new tags in chips search mode"` - line 428
6. `"creates on Enter from chips input when query is not a literal match"` - line 496
7. `"selects existing option on Enter from chips input when query is a literal match"` - line 554

**Selector change:**
```ts
// Current:
const searchInput = container.querySelector<HTMLInputElement>('input[type="text"]')!;

// After change (search is inside menu):
const menu = container.querySelector<HTMLElement>('[data-fg-chip-menu="true"]')!;
const searchInput = menu.querySelector<HTMLInputElement>('input[type="text"]')!;
```

### New Tests to Add

```ts
it("positions dropdown below toggle without overlapping", async () => {
  // Setup chips component
  // Assert menu.getBoundingClientRect().top >= toggle.getBoundingClientRect().bottom
});

it("highlights option visually during keyboard navigation", async () => {
  // Setup and open dropdown
  // Press ArrowDown
  // Assert first option has highlight class
  // Press ArrowDown again
  // Assert second option has highlight class, first does not
});

it("navigates to first/last option with Home/End keys", async () => {
  // Setup with multiple options
  // Focus middle option
  // Press Home - assert first option highlighted
  // Press End - assert last option highlighted
});

it("closes dropdown on Tab and moves focus naturally", async () => {
  // Setup and open dropdown
  // Press Tab
  // Assert dropdown closed
  // Assert focus moved to next focusable element
});

it("prevents rapid open/close with animation lock", async () => {
  // Setup component
  // Rapidly call toggleMenu multiple times
  // Assert final state is consistent
});
```

## Theme Classes Summary

### New Classes to Add to `ChipsClassMap`

```ts
interface ChipsClassMap {
  // ... existing classes

  // Search in dropdown
  menuSearch: ClassToken[];
  menuSearchInput: ClassToken[];
  menuDivider: ClassToken[];
  menuList: ClassToken[];
  menuFooter: ClassToken[];
  menuFooterAction: ClassToken[];
  menuFooterActionFocused: ClassToken[];

  // Keyboard highlight
  menuItemHighlighted: ClassToken[];

  // Rich options
  menuItemIcon: ClassToken[];
  menuItemAvatar: ClassToken[];
  menuItemText: ClassToken[];
  menuItemTitle: ClassToken[];
  menuItemDescription: ClassToken[];

  // Chip avatar
  chipAvatar: ClassToken[];
}
```

### Suggested Class Values

```ts
menuSearch: ["p-2", "border-b", "border-gray-200", "dark:border-gray-700"],
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
menuDivider: ["border-t", "border-gray-200", "dark:border-gray-700", "my-1"],
menuList: ["max-h-72", "overflow-y-auto", "space-y-0.5", "p-1"],
menuFooter: ["border-t", "border-gray-200", "dark:border-gray-700", "p-2"],
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
],
menuFooterActionFocused: ["bg-blue-50", "ring-2", "ring-blue-500", "ring-inset"],
menuItemHighlighted: ["bg-blue-50", "dark:bg-slate-700"],
menuItemIcon: ["shrink-0", "size-8", "rounded-full", "overflow-hidden", "mr-3"],
menuItemAvatar: ["w-full", "h-full", "object-cover"],
menuItemText: ["flex", "flex-col", "flex-1", "min-w-0"],
menuItemTitle: ["font-medium", "text-gray-900", "dark:text-white", "truncate"],
menuItemDescription: ["text-sm", "text-gray-500", "dark:text-gray-400", "truncate"],
chipAvatar: ["size-5", "rounded-full", "object-cover", "mr-1.5"],
```

## File Change Summary

| File | Changes |
|------|---------|
| `client/src/renderers/chips.ts` | Split menu wrapper + options list, move search to dropdown, add highlight class, add keyboard handlers, add animation lock |
| `client/src/theme/classes.ts` | Add new theme classes for search, highlight, rich options |
| `client/src/config.ts` | Extend Option type with icon, avatar, description |
| `client/tests/runtime.test.ts` | Update selectors, add new tests |

## Priority Order

1. **Phase 1.2**: Fix dropdown positioning (quick CSS fix, immediate visual improvement)
2. **Phase 1.1**: Move search to dropdown (structural change, big UX win)
3. **Phase 1.3**: Add visual highlight (small change, better keyboard UX)
4. **Phase 3.1-3.2**: Home/End and Tab keys (easy wins)
5. **Phase 3.3**: Animation lock (prevents edge case bugs)
6. **Phase 2**: Rich options (nice-to-have, more effort)
7. **Phase 4**: Advanced features (optional polish)

## Acceptance Criteria

- [ ] Dropdown appears below toggle with visible gap (doesn't cover toggle)
- [ ] Search input is inside dropdown as first element
- [ ] Arrow key navigation shows visual highlight on focused option
- [ ] Home/End keys jump to first/last option
- [ ] Tab key closes dropdown
- [ ] Rapid clicks don't cause state mismatch
- [ ] All existing tests pass (with updated selectors)
- [ ] New tests for highlight, Home/End, Tab, animation lock pass

## Keyboard Focus Model Decision

**Recommended (Hybrid):**
- Options use `aria-activedescendant` while focus stays in the search input.
- The footer action is a real focusable button outside the listbox.
- ArrowDown from the last option moves focus to the footer action.
- ArrowUp from the footer action restores focus to the input and re-highlights the last option.

This keeps typing uninterrupted while still allowing a true action control.

## Updated Structural Notes

- **Menu wrapper:** no `role`.
- **Search header:** pinned to top; always visible.
- **Options list:** `role="listbox"`, scrollable, receives mousedown `preventDefault`.
- **Footer:** pinned area for actions (e.g., create action button).
- **Selection visibility:** chips menu hides selected options; drop checkmark UI for chips.
- **Inline create vs create action:** inline create remains query-conditional in the list; create action is always in the footer.
- **Typeahead alignment:** create action renders as the last row and follows the same hybrid focus model.
#### 3.5 Keyboard Handling for Non-Search Mode

When search mode is disabled, attach keyboard handlers to the menu wrapper or toggle button. The focus model should:
- Move focus through options directly (current behavior).
- Allow the footer action to receive focus after the last option.
- Return focus to the toggle or chips area on Escape.

This keeps default mode usable without a search input anchor.
