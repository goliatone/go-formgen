#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

ARGS=("$@")
if [[ ${#ARGS[@]} -eq 0 ]]; then
  ARGS=(./...)
fi

export UPDATE_GOLDENS=1

GO_BIN="${GO_BIN:-/Users/goliatone/.g/go/bin/go}"

GOLDEN_PACKAGES=(
  ./pkg/renderers/preact
  ./pkg/renderers/vanilla
  ./pkg/orchestrator
)

if [[ -z "${GOCACHE:-}" ]]; then
  export GOCACHE="${ROOT_DIR}/.gocache"
elif [[ "${GOCACHE}" != /* ]]; then
  export GOCACHE="${ROOT_DIR}/${GOCACHE}"
fi

if [[ -z "${GOMODCACHE:-}" ]]; then
  export GOMODCACHE="${ROOT_DIR}/.gomodcache"
elif [[ "${GOMODCACHE}" != /* ]]; then
  export GOMODCACHE="${ROOT_DIR}/${GOMODCACHE}"
fi

runs_all=false
for arg in "${ARGS[@]}"; do
  if [[ "${arg}" == "./..." ]]; then
    runs_all=true
    break
  fi
done

if [[ "${runs_all}" == false ]]; then
  for pkg in "${GOLDEN_PACKAGES[@]}"; do
    "${GO_BIN}" test "${pkg}"
  done
fi

"${GO_BIN}" test "${ARGS[@]}"
"${GO_BIN}" test -tags example ./examples/...
