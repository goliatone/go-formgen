# TUI Renderer Implementation Plan (TUI_TSK)

Feature scope: ship the interactive CLI renderer described in `TUI_TDD.md` so formgen can render prompt-driven forms, collect values locally, and emit serialized submissions via the orchestrator/CLI (`TUI_TDD.md:1-120`).

---

## Guiding Notes
- Preserve the existing loader→parser→builder→renderer pipeline; TUI is just another renderer (`TUI_TDD.md:25-60`).
- Default prompt driver is `survey` (small dep tree, no CGO); keep the driver pluggable via `PromptDriver` so teams can swap Bubble Tea/promptui without touching form logic (`TUI_TDD.md:17-40`, `TUI_TDD.md:70-105`).
- Renderer must run offline by default; relationship fetches only occur when explicitly enabled with `WithHTTPClient` (`TUI_TDD.md:135-155`).
- Output must be consumable by wrappers: JSON (default), form-urlencoded, or PrettyText; `ContentType()` must reflect the format (`TUI_TDD.md:165-190`).
- Tests rely on a stub driver for deterministic input/validation; no TTY required in CI.
- To execute the go program call the full path to the binary at /Users/goliatone/.g/go/bin/go

---

## Phase 1. Renderer Skeleton & Driver
**Why**: establish the renderer contract, prompt driver interface, and defaults so subsequent work has stable seams (`TUI_TDD.md:70-120`).

**Tasks**
- [x] **Task 1.1 – Scaffold package and options**
  - Create `pkg/renderers/tui/` with `renderer.go`, `options.go`, `errors.go`.
  - Implement `New`, `Name()=="tui"`, `ContentType`, `Render` signature, and options (`WithPromptDriver`, `WithOutputFormat`, `WithHTTPClient`, `WithSubmitTransformer`, `WithTheme`) (`TUI_TDD.md:70-90`, `TUI_TDD.md:90-120`).
- [x] **Task 1.2 – Prompt driver interface + default survey driver**
  - Define `PromptDriver` and config structs (`InputConfig`, `SelectConfig`, etc.) (`TUI_TDD.md:90-120`).
  - Implement the default driver using `github.com/AlecAivazis/survey`, including `ErrAborted` on Ctrl+C, info messaging, and context cancellation.
- [x] **Task 1.3 – State helpers**
  - Add dotted-path helpers for reading/writing values/errors and carrying prefill into session state (`TUI_TDD.md:125-150`).

---

## Phase 2. Field Strategies, Validation, and Submission
**Why**: map form fields to prompts, enforce validation, and emit the submission formats promised in the TDD (`TUI_TDD.md:125-205`).

**Tasks**
- [x] **Task 2.1 – Field → prompt mapping**
  - Implement per-type strategies (string/password/textarea, number/integer, boolean, enum single/multi) respecting UI hints (`cli.widget`, `cli.help`, placeholders) and server errors/prefill (`RenderOptions`) (`TUI_TDD.md:125-175`).
- [x] **Task 2.2 – Arrays and objects**
  - Implement repeat/edit/remove loops for arrays and nested sessions for objects with breadcrumb/path handling (`TUI_TDD.md:205-225`).
- [x] **Task 2.3 – Validation engine**
  - Enforce required, min/max, minLength/maxLength, pattern validations with retry loops and messaging; reuse server errors on first prompt (`TUI_TDD.md:175-190`).
- [x] **Task 2.4 – Submission serialization**
  - Build state into `map[string]any` (dotted paths), apply optional `SubmitTransformer`, and emit JSON (default), form-urlencoded, and PrettyText while setting `ContentType` accordingly (`TUI_TDD.md:165-190`).

---

## Phase 3. Relationships & Option Loading
**Why**: support belongsTo/hasOne/hasMany prompts with either inline enums or optional HTTP option fetches (`TUI_TDD.md:190-205`, `TUI_TDD.md:135-155`).

**Tasks**
- [x] **Task 3.1 – Relationship option sourcing**
  - Use `Field.Enum` when present; otherwise, when `WithHTTPClient` is set, load options from `relationship.endpoint.*` metadata (method, params, label/value fields, resultsPath). Cache per session to avoid duplicate calls.
- [x] **Task 3.2 – Relationship prompting**
  - Map belongsTo/hasOne to Select/Input fallback; hasMany to MultiSelect or repeated ID input when no options are available. Apply `relationship.current` defaults (`TUI_TDD.md:135-155`, `TUI_TDD.md:190-205`).

---

## Phase 4. Integration Surface (CLI & Orchestrator)
**Why**: expose the renderer to consumers and allow configuration from the stock CLI (`TUI_TDD.md:225-250`).

**Tasks**
- [x] **Task 4.1 – Registry wiring**
  - Register `tui` in `cmd/formgen-cli` and `examples/cli` registries; add flags `--renderer tui`, `--tui-format json|form|pretty`, `--tui-no-fetch`, `--tui-non-interactive`, `--tui-theme` (`TUI_TDD.md:225-240`).
- [ ] **Task 4.2 – Wrapper guidance**
  - Optional helper to return structured `map[string]any` alongside bytes for wrappers that don’t want to decode JSON; document if added.

---

## Phase 5. Testing & Fixtures
**Why**: ensure deterministic behaviour without TTY and guard against regressions (`TUI_TDD.md:240-260`).

**Tasks**
- [x] **Task 5.1 – Stub driver + unit tests**
  - Build a `StubDriver` that replays scripted answers and captures info/error messages; add unit tests for mapping, validation loops, prefill/errors, arrays/objects, abort handling.
- [x] **Task 5.2 – Relationship tests**
  - Tests for option fetch success/failure, caching, and fallbacks to raw IDs; include fixtures in `pkg/renderers/tui/testdata/`.
- [x] **Task 5.3 – Serialization tests**
  - Golden tests for JSON and form-urlencoded outputs with stable ordering; PrettyText snapshot for `--inspect` flow.
- [ ] **Task 5.4 – CLI wiring test**
  - Non-interactive test for `examples/cli -renderer tui -inspect` using the stub driver to ensure flags pipe through.

---

## Phase 6. Documentation
**Why**: guide consumers on usage, configuration, and integration patterns (`TUI_TDD.md:225-250`).

**Tasks**
- [x] **Task 6.1 – Docs updates**
  - Add a TUI section to `go-form-gen.md` and `client/README.md` covering renderer selection, formats, flags/env vars, offline defaults, and relationship fetch opt-in.
- [x] **Task 6.2 – Examples**
  - Add a short example snippet showing `orchestrator.Request{Renderer: "tui"}` and consuming the returned JSON in a Go CLI flow.

---

## Phase 7. Nice-to-Haves (post-v1)
**Why**: improvements that can land after the core renderer ships.

**Tasks**
- [ ] **Task 7.1 – YAML output format**
  - Add optional YAML serializer if operator workflows benefit.
- [ ] **Task 7.2 – Conditional visibility**
  - Support basic conditional field visibility driven by UI schema rules if needed.
- [ ] **Task 7.3 – Alternate drivers**
  - Ship an experimental Bubble Tea driver behind an opt-in option to support full-screen TUIs.
