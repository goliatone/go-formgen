# CLI/TUI Form Renderer Technical Design (TUI_TDD)

This document specifies the interactive terminal renderer for formgen. It targets a new renderer under `pkg/renderers/tui` that can be selected from the orchestrator/CLI (`--renderer tui`) and outputs a submission payload after guiding the user through the form described by `FormModel`.

---

## 1. Overview

- A new renderer (`pkg/renderers/tui`) consumes `model.FormModel` + `render.RenderOptions` and drives a terminal UI session (prompt/selection loops) instead of emitting HTML.
- Prompts are picked from field metadata/UI hints (`Field.Type`, `Format`, `Enum`, `Relationship`, `Validations`, `UIHints`) and respect UI schema decorations (order, labels, descriptions, defaults, required).
- Prefilled values (`RenderOptions.Values`) and server errors (`RenderOptions.Errors`) render inline before the first prompt; subsequent validation loops reuse the same messaging.
- The renderer returns a serialised submission payload (JSON by default; form-urlencoded optional) as `[]byte` with an appropriate `ContentType()`, keeping the `render.Renderer` contract intact for the orchestrator.
- A pluggable prompt driver abstracts the actual TUI library; the default driver is built on `github.com/AlecAivazis/survey` (lightweight prompts, no CGO, easy test stubbing). Tests can inject a fake driver for deterministic prompts without a real TTY.

## 2. Goals

1. Let `formgen` render fully interactive CLI forms driven by OpenAPI + UI schema without additional caller code.
2. Support the core field matrix (string/number/bool/enum/array/object/relationship) with validation, defaults, and inline errors.
3. Provide a clean prompt-driver interface so teams can swap/skin TUI stacks without touching form logic.
4. Keep orchestration/API compatibility: `orchestrator.Generate` gains a new renderer name; no changes to loader/parser/model/decorators required.
5. Ship docs + tests (goldens + driver fakes) so behaviour is reproducible and CI-safe.

## 3. Non-Goals

- Rich text editing, file uploads, or mouse-driven widgets in v1.
- Terminal GUI layout engines (split panes, scrollback virtualisation); stay in prompt/list patterns.
- Bidirectional binding to running processes (e.g., live preview of HTTP responses).
- Replacing the HTML renderers; this is an additive renderer.

## 4. Inputs & Dependencies

- **Form model**: `model.FormModel` as built today (fields, validations, metadata, UI hints, relationships).
- **Render options**: `render.RenderOptions` (prefill values, server errors, method override).
- **UI schema**: same decorators already applied by the orchestrator; we may add CLI-specific UI hints (see §5.3) but do not change the decorator interface.
- **Prompt driver**: an interface the renderer calls to show inputs/selects/confirmations; default implementation uses `survey` (small dep tree, cross-platform, builtin validation). Must support non-interactive mode for tests.
- **Dependencies we reuse**: dotted-path helpers for values/errors, validation rules in `Field.Validations`, relationship metadata in `Field.Relationship` + `Field.Metadata`.
- **Optional HTTP**: relationship option fetching may need HTTP; default is offline unless explicitly enabled in renderer options to avoid surprises in CLI contexts.

## 5. Contracts

### 5.1 Renderer API

- Package: `pkg/renderers/tui`.
- Exports `func New(opts ...Option) (render.Renderer, error)` implementing:
  - `Name()` → `"tui"`
  - `ContentType()` → `"application/json"` (default) or `"application/x-www-form-urlencoded"` when configured.
  - `Render(ctx, form model.FormModel, opts render.RenderOptions)` → runs prompts and returns the serialised submission.
- Renderer options:
  - `WithPromptDriver(driver PromptDriver)` (default driver wraps `survey`)
  - `WithOutputFormat(format OutputFormat)` where `OutputFormat` ∈ {`JSON`, `FormURLEncoded`, `PrettyText` (human-readable review)}.
  - `WithHTTPClient(client *http.Client)` (optional, for relationship fetches).
  - `WithSubmitTransformer(func(values map[string]any) (map[string]any, error))` to adjust payloads before serialisation.
  - `WithTheme(Theme)` optional minimal theming (prefix/suffix strings, colors if supported by driver).

### 5.2 Prompt Driver Interface

Abstract to keep renderer logic testable:

```go
type PromptDriver interface {
    Input(ctx context.Context, cfg InputConfig) (string, error)
    Password(ctx context.Context, cfg InputConfig) (string, error)         // masked
    Confirm(ctx context.Context, cfg ConfirmConfig) (bool, error)
    Select(ctx context.Context, cfg SelectConfig) (int, error)             // returns selected index
    MultiSelect(ctx context.Context, cfg SelectConfig) ([]int, error)      // indices
    TextArea(ctx context.Context, cfg TextAreaConfig) (string, error)
    Repeat(ctx context.Context, cfg RepeatConfig) ([][]byte, error)        // used for array/object loops; driver can simply loop Input calls
    Info(ctx context.Context, msg string) error                            // prints info/validation messages
}
```

Drivers must respect `ctx` for cancellation and return `ErrAborted` on Ctrl+C so the renderer can surface a clean exit.

### 5.3 CLI UI Hints (new metadata, optional)

Augment `Field.Metadata`/`UIHints` via UI schema decorators (no API change):

- `cli.widget`: `input|textarea|password|select|multiselect|toggle|confirm|chips-lite` (default derived from type/format/enum).
- `cli.placeholder`: string fallback when `Field.Placeholder` absent.
- `cli.help`: long-form description shown before prompt (falls back to `Field.Description`).
- `cli.requiredMessage`: custom validation message for required fields.
- `cli.secret`: `"true"` to force password prompt even when format is plain string.
- `cli.enum.label` / `cli.enum.value`: override keys when enum is derived from relationship options.
- `cli.layout.order`: integer ordering override (ties resolved by original field order).
- `cli.repeat.label`: singular noun for array add/remove prompts (default uses field label).

If absent, renderer defaults to existing fields/labels/descriptions.

### 5.4 Field → Prompt Mapping

- **string**: `Input` (default) or `Password` when `Format == "password"` or `cli.secret`. `textarea` for `Format == "textarea"` or `UIHints["input"] == "textarea"`.
- **number/integer**: `Input` with numeric validation and optional min/max from `Validations`.
- **boolean**: `Confirm` (y/n toggle). If `UIHints["input"] == "switch"`, treat as confirm with custom label text.
- **enum**: `Select` (single) or `MultiSelect` (when array + enum or relationship has-many). Options from `Field.Enum` or relationship fetch.
- **array**: `Repeat` semantics; driver loops until user stops. Items use the mapped widget for `Items`.
- **object**: nested prompts in a sub-session; dotted paths preserved for submission.
- **relationship**:
  - belongsTo/hasOne → `Select` or `Input` (typeahead-style prompt when options not preloaded).
  - hasMany → `MultiSelect` or `Repeat` of selections.
  - Options come from `Field.Metadata["relationship.endpoint.*"]` + optional HTTP fetch; default fallback uses `Field.Enum` if present.

### 5.5 Validation & Error Surfacing

- Required fields and numeric/text bounds enforced client-side using `Field.Required` + `Field.Validations` (`min/max`, `minLength/maxLength`, `pattern`).
- `RenderOptions.Errors` displayed before the first prompt for each field and reused after failed validation.
- On invalid entry, driver shows the message and re-prompts the same field; renderer keeps previous value for editing.
- Patterns: compile regex once; on failure, show either schema-supplied `pattern` or `cli.requiredMessage`.

### 5.6 Submission Encoding

- Renderer accumulates `map[string]any` using dotted paths for nested fields (consistent with `RenderOptions.Values` keys).
- Output formats:
  - `JSON` (default): `encoding/json` with stable ordering for tests.
  - `FormURLEncoded`: flattened key/value pairs respecting dotted paths; arrays use `field[]=...` convention.
  - `PrettyText`: human-friendly summary (for `--inspect` style usage).
- `ContentType()` reflects the chosen format so downstream tooling can pipe to HTTP clients.

### 5.7 Relationship Option Fetching (optional)

- If `WithHTTPClient` is provided and `Field.Metadata` includes `relationship.endpoint.url`, the renderer can fetch options before prompting.
- Respect existing endpoint contract (`EndpointConfig`/`EndpointOverride`): method, params, auth hints (header/cookie/custom), `labelField`, `valueField`, optional `resultsPath`.
- Offline default: skip fetch when no HTTP client; fall back to `Field.Enum` or prompt for raw ID input.
- Cache per session to avoid duplicate calls for the same endpoint + params.

## 6. Architecture & Components

```
pkg/renderers/tui/
  renderer.go       // implements render.Renderer, orchestrates session
  options.go        // TUI-specific options, output formats, WithHTTPClient, etc.
  driver.go         // PromptDriver interface + default driver implementation
  state.go          // holds current values, errors, and helpers for dotted paths
  validate.go       // maps Field.Validations to runtime checks
  fields.go         // per-field prompting strategies
  relationship.go   // option loading (fetch/enum), selection helpers
  serialize.go      // JSON/form-urlencoded emitters
  errors.go         // typed errors (ErrAborted, ErrValidation)
  testdata/         // golden fixtures for deterministic sessions
```

**Renderer flow**:
1. Sort fields (respecting `cli.layout.order` when present).
2. Prefill state from `RenderOptions.Values`.
3. For each field:
   - Show help/description + any server errors.
   - Pick prompt type (see §5.4).
   - Run prompt via `PromptDriver`, validate, loop on errors.
   - Persist value into state map using dotted path.
4. After all fields, run optional `SubmitTransformer`, serialise, return bytes.

**Driver abstraction**:
- Default driver wraps `survey`; must support non-interactive testing by consuming scripted answers.
- Tests inject a `StubDriver` with pre-seeded responses and capture info/error messages for assertions.

## 7. UX & Interaction Flows

### 7.1 Arrays
- Show existing values (from prefill) and let users `add`, `edit`, `remove`, or `done`.
- Each element uses the item prompt mapping; indices preserved in submission order.

### 7.2 Objects (nested)
- Enter a sub-session; breadcrumbs show the path (e.g., `author.email`).
- Validation/errors scoped to nested fields but share the same driver instance.

### 7.3 Relationship Fields
- If options available (enum or fetched), show `Select/MultiSelect`.
- If no options and no HTTP, fallback to raw ID `Input` with label hint (`enter author ID`).
- Has-many: multi-select when options known; otherwise repeat prompt for IDs.

### 7.4 Cancel/Abort
- Ctrl+C (or driver equivalent) returns `ErrAborted`; renderer stops and surfaces a friendly message (no partial output).

## 8. Configuration Surface

- `formgen.Generate` uses `"tui"` renderer when requested; no orchestrator changes.
- `cmd/formgen-cli`: add `--renderer tui` plus TUI-specific flags:
  - `--tui-format json|form` (maps to `OutputFormat`)
  - `--tui-non-interactive` (fail fast when stdin not a TTY)
  - `--tui-theme minimal|ascii` (optional theme presets)
  - `--tui-no-fetch` (disable relationship HTTP fetches)
  - `--inspect` can print the collected map before serialisation (PrettyText)
- Environment overrides allowed (e.g., `FORMGEN_TUI_NO_FETCH=1`).

## 9. Testing Strategy

- **Unit tests** for:
  - Field → prompt mapping with/without UI hints.
  - Validation rules (required, min/max, length, pattern) and retry loops.
  - Prefill application and server error rendering from `RenderOptions`.
  - Array/object flows (add/edit/remove).
  - Relationship option fetch: happy path, HTTP failure fallback to raw input, caching.
  - Output serialisation for JSON vs form-urlencoded (stable ordering).
  - Abort handling (`ErrAborted` stops session).
- **Integration-like tests**: table-driven scenarios feed a `FormModel` + `RenderOptions` + scripted `StubDriver` responses and assert final output bytes + captured info messages. Place under `pkg/renderers/tui/renderer_test.go` with fixtures in `testdata/`.
- **CLl wiring test**: `go run ./examples/cli -renderer tui -inspect` golden snapshot (non-interactive via stub driver).

## 10. Delivery Plan

1. Scaffold renderer package + driver interfaces + options (default driver: `survey`).
2. Implement core prompt strategies for scalar/enum/bool + validation + prefill/errors.
3. Add array/object flows and submission serialisation.
4. Optional: relationship fetch support (guarded by `WithHTTPClient`).
5. Wire into `cmd/formgen-cli` and `examples/cli` registry.
6. Document usage in `go-form-gen.md` and `client/README.md` (CLI section).
7. Land tests/goldens; ensure `go test ./...` passes without TTY.

## 11. Open Questions

- Should we support YAML output for operators? (Could be added as another `OutputFormat`.)
- Do we need per-field conditional visibility in CLI (e.g., driven by UI schema rules), or is static ordering sufficient for v1?
