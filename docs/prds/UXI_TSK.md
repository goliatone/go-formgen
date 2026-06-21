# UXI_TSK – Implementation Plan (Chips & Typeahead UX Improvements)

Roadmap aligned with `UXI_TDD.md`, following the phased structure used in other modules.

## Phase 0. Planning & Scaffolding
**Why**: Align the chips/typeahead UX refactor scope and confirm the new DOM structure.

**Tasks**
- [x] Task 0.1 – Cross-reference `UXI_TDD.md` with `client/src/renderers/chips.ts` and `client/src/theme/classes.ts`.
- [x] Task 0.2 – Confirm menu structure (wrapper + pinned search header + listbox + footer) and hybrid focus model.
- [x] Task 0.3 – Note any test selector changes and new test coverage needed.
- [x] Task 0.4 – Coordinate with `HAS_TSK.md` for footer action placement and hybrid focus transitions to avoid duplicated logic.

**Acceptance Criteria**
- DOM structure and keyboard model decisions recorded and agreed.

## Phase 1. Layout & Structure (Chips)
**Why**: Fix the dropdown layout and move search into a pinned header without breaking render cycles.

**Tasks**
- [x] Task 1.1 – Split chips menu into wrapper + search header + options list (listbox) + footer.
- [x] Task 1.2 – Move search input into the menu header and keep it persistent across renders.
- [x] Task 1.3 – Add a pinned footer container for actions (e.g., create action).
- [x] Task 1.4 – Update open/focus flow to open menu first, then focus search input.
- [x] Task 1.5 – Adjust menu positioning (`top-full`, `mt-1`, `left-0`, `right-0`).

**Acceptance Criteria**
- Search input is pinned in the dropdown header; footer is pinned; menu opens below the toggle with a gap.

## Phase 2. Keyboard & Focus Model
**Why**: Improve keyboard navigation and ensure consistent focus behavior.

**Tasks**
- [x] Task 2.1 – Implement visual highlight for keyboard navigation in chips options.
- [x] Task 2.2 – Switch chips keyboard navigation to the hybrid model: keep focus in the input for options via `aria-activedescendant`, move focus to footer action on ArrowDown after last option, and restore focus on ArrowUp.
- [x] Task 2.3 – Add Home/End key support for chips options.
- [x] Task 2.4 – Close chips dropdown on Tab without blocking native focus traversal.
- [x] Task 2.5 – Ensure Escape closes the dropdown and restores focus appropriately.
- [x] Task 2.6 – Add non-search mode keyboard handling (menu wrapper or toggle anchor) that supports options + footer focus.

**Acceptance Criteria**
- Chips keyboard navigation shows highlight state, supports footer focus, and behaves consistently with the hybrid model.

## Phase 3. Menu Interaction Polish
**Why**: Prevent edge-case UI state mismatches during rapid interactions.

**Tasks**
- [x] Task 3.1 – Add animation lock to `toggleMenu` to prevent rapid open/close races.
- [x] Task 3.2 – Restore focus to the toggle on close (configurable flag).

**Acceptance Criteria**
- Rapid toggles do not desync menu state; focus returns predictably on close.

## Phase 4. Rich Options Support
**Why**: Support icons/avatars and descriptions in chips menu and chips UI.

**Tasks**
- [x] Task 4.1 – Extend `Option` type with `icon`, `avatar`, and `description`.
- [x] Task 4.2 – Add theme classes for rich menu layout and chip avatars.
- [x] Task 4.3 – Render icons/avatars/descriptions in the chips menu using a safe icon registry.
- [x] Task 4.4 – Render avatar thumbnails inside chips using `store.options` lookup.

**Acceptance Criteria**
- Rich options render in the dropdown and chips without using unsafe HTML.

## Phase 5. Theme Classes & Styling
**Why**: Provide class hooks for the new structure and states.

**Tasks**
- [x] Task 5.1 – Add `menuSearch`, `menuSearchInput`, `menuDivider`, `menuList`, `menuFooter`, `menuFooterAction`, `menuFooterActionFocused`, and `menuItemHighlighted` to `ChipsClassMap`.
- [x] Task 5.2 – Add menu item layout classes for icons, text, descriptions, and avatars.
- [x] Task 5.3 – Update default class values to match the pinned header + footer structure.

**Acceptance Criteria**
- Theme classes cover all new elements; defaults render correctly.

## Phase 6. Tests
**Why**: Prevent regressions and validate UX behavior.

**Tasks**
- [x] Task 6.1 – Update chips tests to query the search input inside the menu wrapper.
- [x] Task 6.2 – Add tests for dropdown positioning below the toggle.
- [x] Task 6.3 – Add tests for keyboard highlight behavior and Home/End navigation.
- [x] Task 6.4 – Add tests for Tab closing and animation lock behavior.
- [x] Task 6.5 – Add tests for hybrid focus transitions (last option → footer action, footer action → input).

**Acceptance Criteria**
- All existing tests pass with updated selectors; new tests validate UX changes.

## Phase 7. Optional Enhancements
**Why**: Enable advanced positioning and tag-input sizing when needed.

**Tasks**
- [x] Task 7.1 – Integrate Floating UI for viewport-aware dropdown positioning.
- [x] Task 7.2 – Implement dynamic input width for tags mode.

**Acceptance Criteria**
- Optional features are gated and do not affect default behavior.

## Phase 8. Docs & QA
**Why**: Document the changes and confirm behavior across modes.

**Tasks**
- [x] Task 8.1 – Update docs or README if new customization hooks/classes are exposed.
- [x] Task 8.2 – Manual QA for chips search mode and non-search mode.
- [x] Task 8.3 – Verify keyboard behavior in both desktop and mobile contexts.

**Acceptance Criteria**
- Docs updated; manual QA confirms expected UX behavior.

---

## Cross-Reference: HAS_TSK (Create Action Feature)

The footer structure and hybrid keyboard model implemented in this refactor are leveraged by the relationship create-action feature documented in `HAS_TDD.md` and `HAS_TSK.md`.

**Shared Infrastructure:**
- `menuFooter` element (hidden by default, shown when create-action is enabled)
- `menuFooterAction` / `menuFooterActionFocused` theme classes
- Hybrid focus model (ArrowDown from last option → footer, ArrowUp → options)
- Footer keyboard handlers (ArrowUp/ArrowDown/Escape)

**No divergence expected:** Create-action adds content to the existing footer structure without modifying UXI implementation.
