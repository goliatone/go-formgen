# testsupport helpers

Utilities in this package centralise the fixture workflow described in `go-form-gen.md:268-301`.

- `LoadDocumentFromPath` and `LoadDocument` construct `pkg/openapi.Document` wrappers for OpenAPI fixtures.
- `LoadFormModel`/`MustLoadFormModel` and `MustLoadOperations` read golden JSON snapshots into their typed forms.
- `CompareGolden` wraps `cmp.Diff` so tests surface human-friendly diffs.
- `WriteMaybeGolden`, `WriteGolden`, and `WriteFormModel` honour the `UPDATE_GOLDENS` toggle used by contract tests.

To refresh every golden in the project, run:

```bash
./scripts/update_goldens.sh
```

Pass package patterns (for example `./internal/model`) to limit the update scope.

