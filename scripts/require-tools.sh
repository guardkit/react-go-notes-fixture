# shellcheck shell=bash
# Sourced by scripts/install.sh and scripts/check.sh: exact toolchain pins, no silent substitution.
export GOTOOLCHAIN=local
export GOWORK=off
export GOFLAGS=-mod=readonly
command -v go >/dev/null || { echo 'go is required' >&2; exit 1; }
command -v staticcheck >/dev/null || { echo 'staticcheck is required' >&2; exit 1; }
command -v node >/dev/null || { echo 'node is required' >&2; exit 1; }
command -v npm >/dev/null || { echo 'npm is required' >&2; exit 1; }
[[ "$(go version | awk '{print $3}')" == go1.26.9 ]] || { echo 'Go 1.26.9 is required' >&2; exit 1; }
[[ "$(staticcheck -version)" == 'staticcheck 2026.2.1 (0.8.1)' ]] || { echo 'Staticcheck 2026.2.1 is required' >&2; exit 1; }
[[ "$(node --version)" == v22.23.2 ]] || { echo 'Node 22.23.2 is required' >&2; exit 1; }
[[ "$(go env CGO_ENABLED)" == 1 ]] || { echo 'CGO is required for race detection' >&2; exit 1; }
