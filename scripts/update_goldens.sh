#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

GO_BIN="${GO_BIN:-/Users/goliatone/.g/go/bin/go}"
ARGS=("$@")

export UPDATE_GOLDENS=1

GOLDEN_PACKAGES=(
  ./pkg/renderers/vanilla
  ./pkg/renderers/preact
  ./pkg/orchestrator
)

# Normalise caches to keep local runs reproducible.
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

"${GO_BIN}" test "${ARGS[@]:-./...}"

# Update client goldens if the script exists and can run.
if [[ -x "${ROOT_DIR}/client/scripts/update_goldens.sh" ]]; then
  (cd "${ROOT_DIR}/client" && ./scripts/update_goldens.sh "$@") || echo "Client golden update skipped (see above for errors)."
fi
