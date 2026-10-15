#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/require-tools.sh
(
  cd apps/api
  unformatted="$("$(go env GOROOT)/bin/gofmt" -l cmd internal)"
  [[ -z "$unformatted" ]] || { echo "Go files need gofmt: $unformatted" >&2; exit 1; }
  go mod verify
  go vet ./...
  staticcheck ./...
  # -count=1: a cached result is never replayed as a pass. -v prints a
  # `--- PASS:` line per test, which the declared ran-marker requires. The
  # handler tests include the contract check against contract/openapi.yaml.
  go test -race -count=1 -v ./...
  go build ./...
)
# The generated web types must match the contract (drift fails).
npm run contract:check
npm --workspace sample-web run typecheck
npm --workspace sample-web run lint
npm --workspace sample-web run format:check
npm --workspace sample-web run test
npm --workspace sample-web run build
npm --workspace notes-e2e run typecheck
npx --no-install prettier --check tests/e2e/*.ts tests/e2e/specs tools/contract/generate.mjs qa/*.mjs
# The live-gate driver's verdict rules (no pass with the evidence missing).
node --test qa/journey-report.test.mjs
# Shell syntax of the lifecycle scripts (shellcheck is run where available).
# The members of every deploy context - the checks, the local stack and the
# deploy - are discovered here as the glob runs, so a script added later is
# covered without editing this file. A sweep that finds no member is a
# failure, never a silent pass.
shells=()
for script in scripts/*.sh scripts/keycloak/*.sh scripts/keycloak/lib/*.sh \
  deploy/*.sh infra/postgres/*.sh tests/keycloak/*.sh; do
  if [[ -f "$script" ]]; then
    shells+=("$script")
  fi
done
if ((${#shells[@]} == 0)); then
  echo "no shell scripts found by the lifecycle sweep in scripts/check.sh" >&2
  exit 1
fi
for script in "${shells[@]}"; do
  bash -n "$script"
done
# The Keycloak realm-setup readiness tests (TASK-KRSR-002). A command shim
# stands in for kcadm and a stub endpoint for Keycloak, so this needs no
# container, no network and no live instance.
bash tests/keycloak/test_configure_realm_readiness.sh
