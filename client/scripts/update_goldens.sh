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

# Optional Go packages (skip if directories do not exist).
GOLDEN_PACKAGES=()
for pkg in ./pkg/renderers/preact ./pkg/renderers/vanilla ./pkg/orchestrator; do
  if [[ -d "${pkg}" ]]; then
    GOLDEN_PACKAGES+=("${pkg}")
  fi
done

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

if [[ "${runs_all}" == false && ${#GOLDEN_PACKAGES[@]} -gt 0 ]]; then
  for pkg in "${GOLDEN_PACKAGES[@]}"; do
    "${GO_BIN}" test "${pkg}"
  done
fi

# If there are no Go packages in client, skip Go tests entirely.
if [[ ${#GOLDEN_PACKAGES[@]} -gt 0 || "${runs_all}" == true ]]; then
  "${GO_BIN}" test "${ARGS[@]}"
  "${GO_BIN}" test -tags example ./examples/...
else
  echo "No client Go packages found; skipping Go golden tests."
fi
