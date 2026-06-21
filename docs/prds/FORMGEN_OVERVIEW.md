# FORMGEN Overview (Phase 0)

Planning notes for the admin settings expansion of formgen. This aligns the available FORMGEN TDD with pending specs in adjacent projects and assigns provisional ownership so workstreams stay unblocked.

## Inputs and References
- `FORMGEN_TDD.md` – authoritative scope for admin settings integration.
- `go-form-gen.md` – current architecture, renderer behaviours, and testing approach.
- `GO_SETTINGS_TDD`, `GO_MEDIA_TDD`, `EXPORT_TDD`, `THEMING_TDD` – not present in this repo. Owners should attach or summarise these as soon as they are available so extension semantics and theming contracts are unambiguous.

## Ownership and Boundaries
| Area | Primary owner | Partners | Notes |
| --- | --- | --- | --- |
| x-admin extensions → model/renderers | formgen core | go-settings | Formgen parses `x-admin-*`, threads them into `FormModel`, and exposes them to renderers. go-settings must continue emitting the metadata and confirm shapes once `GO_SETTINGS_TDD` is available. |
| Widget/renderer registry + defaults | formgen core | go-settings, go-media, go-export | Formgen builds the runtime registry (priorities + `x-admin-widget` mapping) and ships built-ins (toggle, select+chips, code/JSON/key-value editors). Adapters register domain widgets (media/export/settings) when their TDDs land. |
| Visibility evaluator + context payload | formgen core | go-settings | Formgen defines the interface, wiring, and default no-op. go-settings provides the evaluator implementation (e.g., go-options) and supplies context in adapters. |
| Provenance + prefill metadata | formgen core | go-settings | Formgen extends render options and UI to display provenance/read-only hints. go-settings controls payload shape and labels; awaiting `GO_SETTINGS_TDD` confirmation. |
| Error mapping + submission helpers | formgen core | go-errors, go-settings, go-export | Formgen owns path→field mapping, renderer surfacing, and helpers for CSRF/auth/version/method overrides. Downstream services must emit go-errors-compatible payloads. |
| Theming/templates (vanilla + Preact) | formgen core | theming platform (per `THEMING_TDD`) | Formgen exposes theme providers/template roots and respects injected tokens/assets. The theming spec is pending; confirm token + partial contract once provided. |
| JSON/object editor + partial generation | formgen core | go-settings | Formgen ships the richer editor and partial generation (tags/groups/sections). Settings adapters declare schema hints and subsets. |
| Adapter bridge (registry/evaluator injection) | formgen core | go-settings, go-media, go-export | Formgen keeps injection hooks stable; adapters register widgets/evaluators and pass provenance/errors/prefill data. Missing TDDs should define adapter-side responsibilities. |

## Umbrella Issue Stubs (tracking)
- [ ] Admin extension mapping into `FormModel` + renderers (Tasks 1.1–1.3); owner: formgen core; dependency: `GO_SETTINGS_TDD`.
- [ ] Widget/renderer registry with built-ins and `x-admin-widget` wiring (Tasks 2.1–2.3); owner: formgen core; partners: go-settings/go-media/go-export adapters.
- [ ] Visibility evaluator integration and rule evaluation flow (Tasks 3.1–3.3); owner: formgen core; partner: go-settings evaluator.
- [ ] Provenance + prefill metadata surfacing (Tasks 4.1–4.3); owner: formgen core; partner: go-settings for payload shape.
- [ ] Error integration + submission helpers (Tasks 5.1–5.3); owner: formgen core; partners: go-errors/go-settings/go-export.
- [ ] Theming/templates + theme provider hooks (Tasks 6.1–6.3); owner: formgen core; partner: theming spec once `THEMING_TDD` is available.
- [ ] JSON/object editor improvements (Tasks 7.1–7.3); owner: formgen core; partner: go-settings for schema hints.
- [ ] Partial form generation by tags/groups/sections (Tasks 8.1–8.2); owner: formgen core; partner: go-settings for grouping metadata.
- [ ] Adapter registration for runtime widgets/evaluators (Tasks 9.2–9.3); owner: formgen core; partners: go-settings/go-media/go-export adapters.

## Dependencies and Actions
- Blockers: missing `GO_SETTINGS_TDD`, `GO_MEDIA_TDD`, `EXPORT_TDD`, `THEMING_TDD` – import or summarise to unblock interface finalisation.
- Tests/CI: placeholders landed under `pkg/orchestrator` to keep the upcoming goldens and contract tests visible in the matrix.
- Coordination: confirm which team owns adapter delivery for settings/media/export once their TDDs arrive, and align on theming token contract before wiring renderer options.
