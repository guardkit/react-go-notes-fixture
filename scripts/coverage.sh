#!/usr/bin/env bash
# Measures and prints coverage for both suites. No percentage threshold: exit 0
# means the tests passed and coverage was measured. Reports stay in the
# git-ignored apps/api/coverage and apps/web/coverage folders.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/require-tools.sh
(
  cd apps/api
  mkdir -p coverage
  go test -race -count=1 -coverprofile=coverage/go.out ./...
  go tool cover -func=coverage/go.out | tail -n 1
)
npm --workspace sample-web run coverage
