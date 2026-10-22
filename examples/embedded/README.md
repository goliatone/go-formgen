# Embedded form primitives example (Phase 1)

A runnable, generic host for the embedded-form primitives. It compiles a
synthetic JSON Schema definition, renders it with the vanilla renderer, and
drives the browser controller through the complete Phase 1 flow:

compile/render → hydrate → edit/cancel → scoped validation → server issues →
typed snapshot → destroy.

No private data, no persistence, no telemetry, no CDN. The library adds no
HTTP protocol; the `/api/validate` route and `adapter.js` in this directory are
host-owned and exist only to demonstrate the seams.

## Run

```bash
go run ./examples/embedded
```

Open the printed `http://127.0.0.1:8089` address. Use `--addr` to change the
listener, `--render` to print the page HTML to stdout without serving, and
`--csp` to send a restrictive `Content-Security-Policy` (`default-src 'self'`,
`script-src 'self'`, `style-src 'self'`; no inline or remote scripts or
styles): the page carries no inline scripts or `style` attributes, the
descriptor is a JSON data block, the vanilla stylesheet is served from
`vanilla.AssetsFS()`, and the vanilla grid layout travels as
`data-formgen-columns`/`-gutter`/`-span` attributes that the served stylesheet
matches ([Styling Guide, §5](../../docs/GUIDE_STYLING.md#5-responsive-layouts)),
so the policy needs no `'unsafe-inline'`.

```bash
go test ./examples/embedded/
go test -tags example ./examples/ -run 'TestExamplesCompile|TestExamplesRun/embedded'
```

## What the page exercises

| Button | Controller call | Case |
| --- | --- | --- |
| Hydrate sample (silent) | `setValues(values, { notify: false })` | C13: widgets, mirrors, and applicability settle; zero `onChange` callbacks |
| Capture snapshot | `controller.snapshot()` | C08, C10, C15: typed values with explicit presence (absent vs false, null vs "", [] vs missing), retained values apart |
| Simulate edit | `setValues(values, { notify: true })` | C07: one settled notification; `level` cleared because `enabled` is false |
| Cancel (restore snapshot) | `controller.restore(snapshot, { notify: false })` | C08: controls, presence, retained values, and applicability return to the snapshot without recursion |
| Validate colors only | `validate({ paths: ["colors"] })` + `setIssues` | C09: missing unrelated required fields do not block the step |
| Validate all | `validate()` + `setIssues` | C09: final validation catches missing values |
| Send to server | host `fetch` of `effective().values` → Go `ParseJSON` + `ValidateConditional` → `setIssues` | C03/C07 (server applies the same conditions), C11 (precise paths), focus fallback |
| Clear issues | `clearIssues()` | values preserved |
| Capture and lock | disable controls, then `controller.snapshot()` | C15: locked controls keep their typed values in the snapshot while raw collection omits them |
| Destroy | `destroy()` | C12: late server responses are ignored, listeners detached |

The `note` field uses a registered extension control: the Go component
`note-counter` (registered through `components.Registry.Register`) renders the
control, and `adapter.js` registers the matching browser factory with
`registerComponent("note-counter", ...)`. The server runs
`renderer.Preflight(form)` after registration and before rendering, so a
misspelled or unregistered component name fails at startup with its definition
path; removing the Go registration reproduces that path. Browser-side,
`FormgenRelationships.preflightComponents(root)` lists each wrapper and whether
a factory enhances it.

## Shared applicability (Phase 2)

The schema declares `level` applicable only while `enabled` is true, required
whenever `enabled` is answered, and cleared when inapplicable
(`x-formgen.conditions`, `formgen.conditions/v1`). The descriptor carries that
definition to the browser, where `Formgen.attach(root, { model })` compiles it
once, hides the inapplicable control, follows conditional requiredness, clears
the value under the `clear` policy, drops obsolete issues, and reports
`meta.applicable` on every settled change. `controller.effective()` returns the
effective values, retained inactive values, applicability, and requiredness; the
page sends `effective().values` to the server, which evaluates the same
definition through `submission.ValidateConditional`. The earlier host-side
applicability adapter is gone.

## Metadata spelling used here

- Field labels: `"x-formgen": { "label": "Enable feature" }`.
- Extension control: `"x-formgen": { "component.name": "note-counter" }`
  (inline schema metadata; operation UI schemas use `fields.<path>.component`).
- Native choice controls: `"component.name": "choice"` on `enabled` renders a
  Yes/No radio group that starts unanswered with typed boolean values,
  `"component.name": "scale"` with `component.config` `lowLabel`/`highLabel` on
  `level` derives the 1–10 radios from `minimum`/`maximum`, and
  `"component.name": "choice-multiple"` on `colors` renders a checkbox group
  (`colors[]`). All option values use the typed enum codec, so `false` and `0`
  survive parsing in Go and normalization in the browser.
- Conditions: root `"x-formgen": { "conditions": { "version": "formgen.conditions/v1", "rules": [...] } }`
  with absolute canonical paths such as `level`.
- `uniqueItems` is a model validation rule (`validations[].kind`); the JSON
  Schema keyword is not adopted, so the schema here declares `minItems`/`maxItems`.

## Phase 1 limitations

- `enabled` uses the `choice` component, so an unanswered question stays
  missing (C01) instead of collapsing to `false`; `level` stays inapplicable
  until Yes is selected. Choice groups render readonly as disabled native
  controls; capture values before locking.
- Keyboard operation, label activation, and visible focus rely on native radio
  and checkbox semantics plus the `formgen-choice*` CSS hooks; they were
  exercised in jsdom-based tests, Go rendering tests, and the recorded Chromium
  smoke below, not with a screen reader.
- Reactive applicability targets each control's component wrapper (or a
  matching group element) with `hidden`; hosts that style applicability
  differently pass `conditions: { hideInapplicable: false }` and use the
  `data-formgen-applicable` markers.
- Browser integers must be safe JavaScript integers; larger values report a
  `type` issue instead of rounding. Pattern parity is limited to the portable
  subset; Go uses RE2.
- Nothing here establishes WKWebView, VoiceOver, or application acceptance.
  The flow was exercised with jsdom-based tests, Go tests, and the recorded
  Chromium smoke; other engines, screen readers, and host integration remain a
  maintainer trial.

## Evidence map (C01–C16)

| Case | Where it is asserted |
| --- | --- |
| C01 missing required | Go `TestSharedEmbeddedConformance/C01`, `TestValidateEndpoint/C01_missing`; Vitest `embedded values: shared fixture parity › C01`, `embedded typed snapshots › distinguishes absent from false…` |
| C02 false succeeds | Go `…/C02`; Vitest fixture parity `C02`, `embedded choice components › decodes typed selections…` |
| C03 conditional requiredness | Go `…/C03`; Vitest `embedded conditions › C03`, `embedded reactive conditions › validates applicable scope…` |
| C04 typed value | Go `…/C04`; Vitest parity `C04`, choice components test |
| C05 range | Go `…/C05`, `C05-min-bound`, `C05-max-bound`; Vitest parity, `TestValidateEndpoint/C05_range` |
| C06 enum, count, duplicates | Go `…/C06*`, `TestUniqueItemsUsesTypedJSONEquality`, `TestUniqueItemsParityAcrossParseMapAndParseJSON`, `TestUniqueItemsArbitraryExponentsAcrossValidationPipeline`, `TestChoiceMultipleRendersCheckboxGroupWithTypedItems`; Vitest parity `C06*`, `uses typed JSON equality…` |
| C07 clear and inactive input | Go `…/C07`, `TestValidateEndpoint/C07…`; Vitest `embedded conditions › C07`, `embedded reactive conditions › settles applicability…` |
| C08 snapshot/cancel | Vitest `embedded hydration options › restores a host snapshot…`, `embedded reactive conditions › settles after silent hydration, replace restoration, and reset`, `embedded typed snapshots › restores snapshots…` |
| C09 scoped validation | Vitest `embedded values: validation › validates only the selected scope…`, `embedded model composition`, `embedded conditions › limits scoped conditional validation…` |
| C10 zero and empty strings | Vitest `embedded values: normalization › …(C10)`, choice components test, typed snapshots test; Go `TestScaleComponentDerivesBoundsAndKeepsZero` |
| C11 nested/repeated issues | Vitest `embedded issue presentation › displays, focuses, and clears precise nested and repeated issues…` |
| C12 isolation and teardown | Vitest `embedded hydration options › keeps roots isolated…`, `embedded reactive conditions › stops settling after destroy…`, typed snapshots and choice tests |
| C13 silent hydration, no loops | Vitest `embedded hydration options › suppresses subscribers…`, `lets a subscriber normalize values…`, `keeps widget, mirror, dependency, and component synchronization…` |
| C14 definition diagnostics | Go `TestSharedEmbeddedConformance` invalid definitions, `TestPreflight*`; Vitest `embedded conditions › rejects invalid definition…`, `embedded reactive conditions › surfaces definition diagnostics…` |
| C15 locked payload | Vitest `embedded typed snapshots › keeps locked values after controls are disabled…`; the page's Capture and lock action |
| C16 keyboard-operable choices | Go `TestChoiceComponent*`/`TestScaleComponent*` (fieldset, legend, ids, states); Vitest choice tests (label activation, group errors, focus); Chromium smoke below. No screen-reader evidence. |

Run everything with:

```bash
go test ./pkg/submission ./pkg/conditions ./pkg/model ./pkg/renderers/vanilla/... ./examples/embedded/
```

```bash
cd client && npm test
```

## Profile (jsdom, not budgets)

`FORMGEN_PROFILE=1 npm test -- --threads=false tests/embedded-profile.test.ts`
measured on the development machine with jsdom; the controller adds three root
listeners regardless of size and emits zero callbacks for silent hydration and
exactly one per user action.

| Form | attach | silent hydrate | edit | toggle (30 rules) | validate | snapshot |
| --- | --- | --- | --- | --- | --- | --- |
| 6 fields, 1 rule | 14 ms | 2 ms | 1 ms | 2 ms | 1 ms | 0.4 ms |
| 300 fields, 30 rules | 19 ms | 17 ms | 11 ms | 21 ms | 3 ms | 4 ms |

Numeric budgets are a maintainer and host decision; these are observations.

## Browser smoke (recorded)

Engine: Chromium 152.0.7977.167 (the Claude desktop app's built-in browser,
macOS), served with `--csp`. The flow was last re-run on 2026-10-22 against the
shipped bundles after the integrated review fixes (`role="radiogroup"`,
checkbox-group legend hint) and the radio-member `aria-required` correction
(IR-2026-10-22-03). Steps and observed outcomes:

- Attach: controller attached, `level` inapplicable (`hidden`,
  `data-formgen-hidden-by="applicability"`) before any answer; the Yes radio's
  `labels` resolve to "Yes"; `enabled` is a `role="radiogroup"` fieldset
  labelled by its legend with `aria-required="true"` and radios that carry
  native `required` only; `colors` is a plain fieldset with
  `data-formgen-required` and the visually hidden "(required)" legend hint. No
  inline `style` attributes, one `application/json` data block, no console
  errors or CSP reports.
- Label click on No: `level` inapplicable again and cleared by the clear
  policy. ArrowLeft on the focused radio moves the selection back to Yes:
  `level` applicable, visible and required (`aria-required` on the fieldset,
  `required` on its radios, no member `aria-required`). Each radio activation
  produced two `ui` notifications because Chromium fires both `input` and
  `change`; that is the preserved legacy per-event contract, not a loop.
- Hydrate: zero callbacks; immutable reference captured. Snapshot: five present
  paths. Simulate edit: one settled `hydrate` notification, `level` cleared by
  the clear policy. Cancel: restored with presence intact.
- Full validation with `level` unanswered and blue unchecked: `required:level`
  and `required:colors` inline (`aria-invalid` and `data-validation-state` on
  the radiogroup, `data-validation-state` on the checkbox fieldset and
  `aria-invalid` on its members), focus on the first `level` radio, server
  agreement (two issues, then one after re-checking blue). With `level`
  answered: `required:colors` only, focus on the first checkbox.
- Capture and lock: typed snapshot kept the five locked values; Destroy: late
  results ignored.

Findings fixed during the smokes: a snapshot taken after locking every control
evaluated conditions on the raw collection and dropped the locked `level`; the
presence overlay now precedes condition evaluation (regression test added).
The settled requiredness pass wrote `aria-required` on radio members; it now
writes it on the radiogroup fieldset only, and radio members keep native
`required` (runtime and renderer regressions added). The `--render` harness
check in `examples/examples_test.go` matched the summary hook only through the
page's former inline CSS; it now checks the served page's own contract.
Finding recorded for the renderer during the first smoke: under
`style-src 'self'` Chromium blocked six inline `style` attributes from the
vanilla grid layout (cosmetic). Resolved: the renderer now emits
`data-formgen-*` layout attributes and the example sends `style-src 'self'`. A
second Chromium 152 smoke with that policy showed no CSP messages in the
console, zero `style` attributes in the DOM, a 12-track grid with a 24px gap,
every field spanning the full width, and probe attributes (`data-formgen-span`,
`data-formgen-start`, `data-formgen-span-lg`) placing fields as the stylesheet
contract describes. Not covered: WKWebView, VoiceOver or other screen readers,
physical devices.

## Versions

- Module: see `.version` at the repository root (0.33.2 at the time of writing).
- Runtime bundle: `@goliatone/formgen-runtime` 0.0.19 (`client/package.json`),
  served from `formgen.RuntimeAssetsFS()`.
- Go: module minimum from `go.mod` (1.25).
- Shared contract: `formgen.descriptor/v1`, `formgen.conditions/v1`,
  `testdata/embedded/v1.json`.
