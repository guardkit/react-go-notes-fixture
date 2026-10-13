#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/require-tools.sh
# Locked installs only: npm ci refuses a lockfile/manifest mismatch, and read-only
# Go modules plus go mod verify refuse go.mod/go.sum drift or tampered downloads.
# `go mod download` can still silently re-add a dropped go.sum line, so the
# committed manifests and locks are hashed first and must be unchanged after.
locks=(apps/api/go.mod apps/api/go.sum package-lock.json)
before="$(sha256sum "${locks[@]}")"
# --ignore-scripts: Playwright browsers come from PLAYWRIGHT_BROWSERS_PATH, never
# from a download during install.
npm ci --ignore-scripts --no-audit --no-fund
(
  cd apps/api
  go mod download
  go mod verify
  # Resolves every package, test dependencies included, so a missing module
  # fails here rather than in the first check.
  go list -deps -test ./... >/dev/null
)
after="$(sha256sum "${locks[@]}")"
if [[ "$before" != "$after" ]]; then
  echo "install changed a committed lockfile; commit the intended change instead:" >&2
  diff <(printf '%s\n' "$before") <(printf '%s\n' "$after") >&2 || true
  exit 1
fi
