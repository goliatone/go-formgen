# formgen

Go library for turning OpenAPI 3.x operations into HTML forms with pluggable renderers.

## Documentation

- [Architecture & Guides](go-form-gen.md)
- [API Reference](https://pkg.go.dev/github.com/goliatone/formgen)
- [Task & Roadmap Tracker](GEN_TSK.md)

## Installation

```bash
go get github.com/goliatone/formgen
```

Requires Go 1.21+.

## Quick Start

```bash
go run ./examples/basic
```

See `examples/cli` for a flag-driven generator, `examples/http` for a minimal server, and `examples/multi-renderer` for writing renderer outputs (and preact assets) to disk.
Sample OpenAPI fixtures live under `examples/fixtures/`.

## Testing & Tooling

```
./taskfile dev:test            # go test ./... with cached modules
./taskfile dev:test:contracts  # contract + integration suites (renderer coverage)
./taskfile dev:test:examples   # compile example binaries with -tags example (vanilla + Preact)
./taskfile dev:ci              # vet + optional golangci-lint (includes example build)
./taskfile dev:goldens         # regenerate vanilla/Preact snapshots via scripts/update_goldens.sh
./scripts/update_goldens.sh    # refresh vanilla/Preact snapshots and rerun example builds
```

## Troubleshooting

Offline environments should avoid enabling HTTP loader options. Bundle validations fail if the renderer templates are incomplete—reuse `formgen.EmbeddedTemplates()` when in doubt.

## License

MIT © Goliat One
