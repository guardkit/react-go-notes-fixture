#!/usr/bin/env bash
# Post-deploy health (profile health_checks): the proxy, the API and Keycloak
# answer on this target's internal address as this project's instance.
# Forge runs it with candidate.env after the candidate, the preview overlay
# after a preview, and nothing after a promote (deploy/settings.sh), so the
# target is chosen from those settings, not from a mode signal.
# Exit code is the verdict.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
source scripts/lib.sh
source deploy/settings.sh
require_command curl

if [[ -n "${FACTORY_PREVIEW_PORT:-}" ]]; then
  port="${FACTORY_PREVIEW_INTERNAL_PORT:?FACTORY_PREVIEW_INTERNAL_PORT is required}"
  project="${COMPOSE_PROJECT_NAME:?COMPOSE_PROJECT_NAME is required for a preview}"
elif [[ -n "${CANDIDATE_PORT:-}" ]] || truthy "${CANDIDATE:-}"; then
  port="${CANDIDATE_PORT:?CANDIDATE_PORT is required}"
  project="${COMPOSE_PROJECT_NAME:-$notes_live_project-cand}"
else
  port="$notes_live_port"
  project="${COMPOSE_PROJECT_NAME:-$notes_live_project}"
fi
state_file="$(settings_file "$project")"
[[ -r "$state_file" ]] || die "no deploy settings for $project ($state_file)"
run_id="$(grep '^NOTES_RUN_ID=' "$state_file" | cut -d= -f2)"
realm="$(grep '^NOTES_REALM=' "$state_file" | cut -d= -f2)"
base="http://127.0.0.1:$port"

deadline=$(($(date +%s) + 120))
while (($(date +%s) < deadline)); do
  if fetch "$base/config.json" 2>/dev/null | grep -Fq "\"runId\":\"$run_id\"" &&
    fetch "$base/api/health" 2>/dev/null | grep -Fq "\"run\":\"$run_id\"" &&
    fetch "$base/auth/realms/$realm/.well-known/openid-configuration" 2>/dev/null |
    grep -Fq "\"issuer\":\"$base/auth/realms/$realm\""; then
    echo "healthy: $project on $base"
    exit 0
  fi
  sleep 2
done
die "unhealthy: $project on $base is not instance $run_id within 120 s"
