# go-formgen – Admin Settings Integration TDD

This document captures the changes needed in go-formgen to serve admin settings forms (via go-settings/go-admin) while remaining reusable. Assume these changes will be delivered by the go-formgen team.

## Background and Goals
- **Current state**: go-formgen turns OpenAPI 3.x operations into `FormModel`s and renders via vanilla/Preact/TUI. It supports template overrides and UI schema metadata but has limited first-class support for rich admin controls, conditional visibility, and vendor extensions tailored to settings.
- **Desired state**: Generate customizable admin settings forms from go-settings exported schemas with minimal glue. Map vendor extensions to layout/widgets, support conditional visibility, provenance-aware prefill, and error integration. Keep go-formgen router/UI agnostic and adapter-friendly.

## Scope
### In Scope
- Extension handling for admin metadata (`x-admin-*`) to drive layout, grouping, widgets, visibility.
- Runtime widget/field renderer registry for custom controls (toggles, code editor, JSON editor).
- Conditional visibility/show-hide rules evaluated at render/runtime using pluggable evaluators.
- Prefill with values + provenance/labels (e.g., scope source).
- Error ingestion compatible with go-errors field paths.
- Theming/config APIs for template roots/partials and UI schema overrides.
- JSON/object editor improvements suitable for settings payloads.
- Submission helpers for CSRF/auth/versioning fields (optional but recommended).

### Out of Scope
- New SPA frontend; existing renderers (vanilla/Preact/TUI) remain, with enhancements.
- Backend persistence (handled by go-settings); formgen only renders/serializes.

## Requirements & Constraints
- Remain OpenAPI-first but accept vendor extensions for admin layout/widgets.
- No hard dependency on go-settings/go-admin; changes must be generic.
- Pluggable evaluators for conditional visibility (host can supply go-options/goja/etc.).
- Renderer-extensible without forking: register new widgets/templates at runtime.
- Preserve offline-friendly loader behavior; avoid mandatory HTTP fetches.

## Proposed Changes
1) **Vendor Extension Mapping**
   - Recognize `x-admin-group`, `x-admin-tags`, `x-admin-widget`, `x-admin-visibility-rule`, `x-admin-help`, `x-admin-placeholder`, `x-admin-readonly`, `x-admin-order`.
   - Surface them in `FormModel`/field metadata so renderers can layout sections/tabs and pick widgets.

2) **Custom Widget/Renderer Registry**
   - Runtime registration API with priorities; map field kinds or `x-admin-widget` to renderer IDs.
   - Provide built-in widgets: toggle/switch (bool), select+chips (array/string enums), code editor (json/string), JSON/object editor (schema-aware), key/value map editor.

3) **Conditional Visibility**
   - Add optional `VisibilityRule` on fields/sections; evaluate using a pluggable evaluator interface (default no-op). Allow host to inject evaluator (e.g., go-options) and context (scope, current values).
   - Renderers receive evaluated visibility flags to show/hide/disable.

4) **Provenance & Prefill Metadata**
   - Extend `RenderOptions.Values` to accept metadata per field (value + provenance label, e.g., `scope=tenant`, `source=defaults`).
   - Renderers can display tooltips/badges with provenance and dim/lock inherited values when marked readonly.

5) **Error Integration**
   - Accept error payloads as map[path][]string and map to field IDs; ensure compatibility with go-errors path format.
   - Support form-level errors.

6) **Theming & Templates**
   - Expose options to supply template roots/partials per renderer without forking (vanilla/Preact).
   - Support UI schema overrides for grouping/order/action bars; keep existing UI schema path.

7) **JSON/Object Editing**
   - Ship richer JSON/object editor widget (with schema-based validation hints) for arbitrary map/object fields; support collapsed/expanded view and pretty-print.

8) **Submission Helpers**
   - Optional helper to emit hidden fields for CSRF/auth/version (e.g., `if-match` or `version`), and to wire form actions/method overrides conveniently.

9) **Partial Form Generation**
   - Ability to generate a subset of fields by tag/group (e.g., render only `x-admin-group=notifications`), to support tabbed settings UIs without multiple OpenAPI docs.

## Interfaces & Injection
- Add interfaces:
  - `VisibilityEvaluator` with `Eval(fieldID string, rule string, ctx VisibilityContext) (bool, error)`.
  - `WidgetRegistry` APIs for registering widget renderers and mapping strategies.
- Keep orchestration pluggable: orchestrator options to set evaluator, widget registry, template roots, default renderer.

## Acceptance Criteria
- Contract tests showing vendor extensions flow into `FormModel` and render outputs.
- Runtime widget registration + custom widget snapshot tests (vanilla/Preact).
- Visibility rules honored in renderers when evaluator provided; no-ops when absent.
- Error mapping covers go-errors style paths.
- JSON/object editor available and snapshot-tested.
- Theming override tests (custom templates directory) pass without touching core templates.
- Partial generation by tag/group works for both vanilla and Preact renderers.

## Notes for go-settings Integration
- go-settings will export schemas with `x-admin-*` metadata and prefill values + provenance. It will inject a visibility evaluator (go-options powered) and register custom widgets via a `formgenadapter`. Renderers should be able to consume provenance to show “from tenant scope” badges and disable inherited fields as configured.

## Runtime: File Uploader Hydration + Go-Consumable Assets
We want to reuse go-formgen’s runtime `file_uploader` component in go-admin (create + edit forms) without requiring per-application uploader JS/templates, and without requiring consumers to run an npm build step to serve the runtime.

### Background (Current State)
- Vanilla renderer template renders a plain input for `file_uploader`:
  - `pkg/renderers/vanilla/templates/components/file_uploader.tmpl` emits `<input type="text" name="{{ field.name }}" ... value="{{ field.default }}">`.
- Runtime component lives at `client/src/components/file-uploader/index.ts` and is registered as `data-component="file_uploader"` via `client/src/components/registry.ts`.
- Today the runtime component:
  - Finds the underlying input and changes `this.input.type = "hidden"`.
  - Clears the field at init: `this.input.value = ""` (this breaks edit forms with prefilled values).
  - Builds custom UI (button/dropzone, preview for image, file list, progress UI, remove).
  - Uploads via `fetch` with `FormData` field name exactly `file`.
  - Expects JSON payload containing at least `{ url: string }`.
  - Serializes uploaded URLs back into the form by writing hidden input(s):
    - `multiple=false`: writes `input.name=originalName` and `input.value=url`.
    - `multiple=true`: writes `input.name="${originalName}[]"` (if missing `[]`) and creates sibling hidden inputs for additional URLs.
- Prebuilt runtime output exists today under `client/dist/*`, but consumption from Go apps is not first-class (no top-level helper that exposes an `fs.FS` for mounting).

### Goals
1) **Prefilled-value hydration** (edit form support): if the underlying markup includes existing value(s), the runtime must render them as “already uploaded” on init.
2) **Upload contract remains unchanged**: `POST` (configurable method ok) to `uploadEndpoint`, body `FormData` with field name `file`, response JSON includes `{ url }` at minimum.
3) **Go-consumable runtime assets**: prebuilt bundles are committed in-repo and exposed via a Go helper `fs.FS` so apps can serve them without npm.
4) **Optional**: orchestrator/renderer auto-injects required runtime scripts when a form uses components like `file_uploader`.

### Non-Goals
- Changing the upload API or response shape beyond optional additional fields.
- Perfect upload progress via `fetch` (indeterminate progress is acceptable).
- Building a new SPA; this is purely runtime + integration plumbing.

### Canonical Multi-Value Encoding (Decision)
For `multiple=true`, the canonical HTML encoding is **repeated inputs** with `name="field[]"`, one per URL value (standard HTML form encoding, Go-friendly).

Example (edit form, two existing URLs):
```html
<div data-component="file_uploader" data-component-config='{"multiple":true,"uploadEndpoint":"/api/uploads"}'>
  <input type="text" name="photos[]" value="/assets/uploads/a.png">
  <input type="hidden" name="photos[]" value="/assets/uploads/b.png">
</div>
```

Notes:
- The “first” input remains the canonical input the runtime owns and will convert to `type="hidden"`.
- Additional inputs can be `type="hidden"` or `type="text"`; the runtime must read them and then remove/consume them so it fully owns serialization after init.

### Hydration Rules (Automatic, No App Glue)
Hydration is automatic by reading existing input values during component construction.

#### Single (`multiple=false`)
- Read `input.value` before clearing.
- If it is a non-empty string, treat it as an existing uploaded URL:
  - Create an internal entry with status `uploaded`.
  - Show preview for `variant="image"` when `preview=true`.
  - Render a file list row in “uploaded” state.
- “Remove” must remove the entry and re-serialize:
  - Clear the hidden input’s value.
  - Hide the preview when no uploaded entries remain.

#### Multiple (`multiple=true`)
The component must support edit forms that encode existing values via repeated inputs.

Hydration algorithm (spec-level):
1) Capture `originalName` from the first discovered input (today `this.originalName = input.name`).
2) Derive:
   - `logicalName` = `originalName` with one trailing `[]` stripped if present.
   - `multiName` = `${logicalName}[]`.
3) Collect candidate inputs within the component root element:
   - Include inputs whose `name` is either `originalName` or `multiName` (this supports markup that uses either `field` or `field[]` as the first input name).
   - Read values in DOM order and keep only non-empty strings.
4) Consume extra inputs:
   - Keep the first input as the component-owned input.
   - Remove additional inputs from the DOM after values are captured (prevents duplicate submissions and ensures the runtime owns serialization).
5) For each captured URL, create an internal “uploaded” entry and then call the existing `serializeFiles()` path to write the canonical runtime encoding:
   - When `multiple=true` and at least one URL exists, ensure the primary hidden input has `name=multiName` and `value=firstUrl`, and create one hidden input per remaining URL in the hidden container.
   - When the last URL is removed, restore the “empty” encoding: `input.name=originalName` (no forced `[]`) and `input.value=""`, hidden container cleared.

#### Hydrated entry shape
Hydrated entries must behave the same as freshly uploaded ones (remove, preview selection, re-serialize).

Implementation constraint: current `FileEntry` expects a `File`. For hydrated entries, the runtime can create a synthetic `File` with:
- `name`: best-effort derived from the URL (basename), falling back to the URL string,
- `size`: `0`,
- `type`: `""` (or inferred if desired, not required).

The associated `UploadedFile` should minimally include:
- `url`: the hydrated URL (required),
- `name`/`originalName`: derived (optional),
- `size`/`contentType`: best-effort (optional).

### Serialization Contract (Must Remain Stable)
The runtime remains the owner of form serialization:
- It always converts the source input to `type="hidden"`.
- It writes values back into input(s) exactly as it does today:
  - Single: one hidden input with the URL in `value`.
  - Multiple: `name="field[]"` repeated inputs; first value on the owned input, rest in a hidden container after it.
- If a custom `serialize` hook is provided via config, hydration must still end with `serializeFiles()` so the hook can take over.

### Upload Contract (Unchanged)
- Request: `fetch(uploadEndpoint, { method, body: FormData })`
  - Body must include `formData.append("file", file, file.name)`
- Response: JSON payload with at least `{ url: string }`
- On success: add entry as uploaded and serialize URLs using the same path as today.

### Go-Consumable Runtime Assets (Committed + Embeddable)
Prebuilt bundles are committed to the repo and exposed to Go consumers with a helper `fs.FS`.

#### Distribution
- Generate browser bundles via `client`’s build (`npm run build:bundle`) and commit the distributable artifacts under `pkg/runtime/assets/...` (including source maps and framework shims).
- The Go module must include these committed assets in tagged releases so `go get` consumers can serve them without npm.

#### Go API (Proposed)
Add a top-level helper mirroring `EmbeddedTemplates()`:
- `formgen.RuntimeAssetsFS() fs.FS`
  - Returns an `fs.FS` rooted at the distributable runtime asset directory (so mounting does not require path surgery).
  - Intended for `http.FileServerFS`.

#### Mounting example (Go)
```go
// Serve go-formgen runtime assets (no npm build step).
mux.Handle("/runtime/",
  http.StripPrefix("/runtime/",
    http.FileServerFS(formgen.RuntimeAssetsFS()),
  ),
)
```

#### Browser usage (non-module)
The committed browser bundle should be usable via a plain `<script>` tag. Consumers should:
1) Include the runtime script (example name; keep in sync with the embedded bundle),
2) Call `initRelationships()` once per page (it also calls `initComponents()` for `data-formgen-auto-init` roots).

```html
<script src="/runtime/formgen-relationships.min.js" defer></script>
<script>
  window.addEventListener("DOMContentLoaded", function () {
    if (window.FormgenRelationships) window.FormgenRelationships.initRelationships();
  });
</script>
```

### Optional: Auto-inject runtime scripts when used
If the orchestrator/renderer already computes a `component_scripts` list, it should automatically include the runtime script when a form includes `file_uploader`.
- The script list must dedupe entries (multi-component forms should not add duplicates).
- Prefer a single “runtime entrypoint” script that covers multiple components to avoid script sprawl.

### Acceptance Criteria
- Given:
```html
<div data-formgen-auto-init>
  <div data-component="file_uploader" data-component-config='{"variant":"image","uploadEndpoint":"/api/uploads","preview":true}'>
    <input type="text" name="profile_picture" value="/assets/uploads/foo.png">
  </div>
</div>
```
After init:
- The input becomes hidden.
- The UI shows an image preview of `/assets/uploads/foo.png`.
- The file list contains a single entry in “uploaded” state.
- “Remove” clears the serialized value(s) and hides the preview.

Multiple mode acceptance:
- Given repeated inputs `name="photos[]"` with prefilled values, the runtime hydrates all entries and “Remove” updates the repeated hidden inputs accordingly.

### Tests (Client)
Update/add tests under `client/tests/file-uploader.test.ts` to cover:
- Hydration from single input value.
- Hydration from repeated inputs (`multiple=true`) including consuming extra inputs.
- Remove behavior in both single and multiple modes (DOM serialization + preview/list updates).
