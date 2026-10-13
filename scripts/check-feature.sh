#!/usr/bin/env bash
# The connected check: browser, API, PostgreSQL and Keycloak on a fresh,
# per-run compose project. Any failure is a non-zero exit; there is
# no mock, skip or reuse fallback.
#
#   1. allocate a run identity and a free loopback port;
#   2. build and start the stack, readiness bounded to 180 s, and confirm the
#      responding instance belongs to this run;
#   3. apply migrations explicitly: an upgrade from 0001 with seeded rows, a
#      fresh install, and an idempotent re-run;
#   4. seed synthetic notes;
#   5. configure the realm twice and confirm the users kept their ids;
#   6. run Playwright (Chromium journey, Firefox and WebKit sign-in smoke);
#   7. copy evidence to FACTORY_EVIDENCE_DIR (default test-results/<run>);
#   8. on exit, interrupt or termination: `compose down -v` of this run's
#      project only, plus this run's images.
#
# Every container, network and volume carries
# guardkit.fixture.owner=${GUARDKIT_RUN_OWNER:-notes-check-<run>}, so the
# factory's disposable sweeps can remove what a killed run leaves behind.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
source scripts/require-tools.sh
source scripts/lib.sh
require_docker
require_command setsid

say() { printf 'check-feature: %s\n' "$*"; }

# --- 1. Run identity -------------------------------------------------------
RUN_ID="$(date -u +%Y%m%d%H%M%S)-$(rand_hex 6)"
owner="${GUARDKIT_RUN_OWNER:-notes-check-$RUN_ID}"
[[ "$owner" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || die "GUARDKIT_RUN_OWNER must be letters, digits, '.', '_' or '-'"
export NOTES_RUN_ID="$RUN_ID"
export COMPOSE_PROJECT_NAME="notes-chk-$RUN_ID"
export COMPOSE_FILE="$root/compose.yaml"
export FACTORY_RESOURCE_LABEL="guardkit.fixture.owner=$owner"
export NOTES_IMAGE_TAG="chk-$RUN_ID"
export NOTES_REALM="notes-$RUN_ID"
export NOTES_CLIENT_ID=notes-web
export NOTES_INTERNAL_PORT
NOTES_INTERNAL_PORT="$(free_port)"
base="http://127.0.0.1:$NOTES_INTERNAL_PORT"
# A disposable check accepts only its own internal issuer.
export OIDC_ISSUERS="$base/auth/realms/$NOTES_REALM"
export NOTES_REDIRECT_ORIGINS="$base"
export NOTES_SIGNIN_HINT="Test users: alice and bob."
for name in POSTGRES_PASSWORD APP_DB_PASSWORD KEYCLOAK_DB_PASSWORD KEYCLOAK_ADMIN_PASSWORD NOTES_TEST_PASSWORD; do
  export "$name=$(rand_token 24)"
done
unset OIDC_AUDIENCE FACTORY_PREVIEW_PORT FACTORY_PREVIEW_URL FACTORY_PREVIEW_TOKEN COMPOSE_PROFILES

work="$root/.notes-runs/$RUN_ID"
evidence="${FACTORY_EVIDENCE_DIR:-$root/test-results/$RUN_ID}"
mkdir -p "$work" "$evidence"
evidence="$(cd "$evidence" && pwd)"
[[ -z "$(ls -A "$evidence")" ]] || die "evidence folder is not empty: $evidence"
say "run=$RUN_ID project=$COMPOSE_PROJECT_NAME port=$NOTES_INTERNAL_PORT owner=$owner"
say "evidence=$evidence"

# --- 8. Cleanup, armed before anything starts --------------------------------
started=0
child=0
outcome=FAIL
cleanup() {
  local status=$?
  # A second interrupt or termination must not abort `compose down`.
  trap - EXIT
  trap '' INT TERM
  if ((child > 0)); then
    kill -TERM "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
  fi
  if [[ -d "$work/playwright" ]]; then
    cp -a "$work/playwright/." "$evidence/" 2>/dev/null || true
  fi
  if ((started)); then
    docker compose logs --no-color --timestamps >"$evidence/compose.log" 2>&1 || true
    docker compose ps -a >"$evidence/compose-ps.txt" 2>&1 || true
    # In its own session, so a signal sent to this run's whole process group
    # (a cancelled build) cannot kill the teardown half-way.
    local down_status=0
    setsid docker compose down -v --remove-orphans --timeout 10 >&2 &
    wait "$!" || down_status=$?
    if ((down_status != 0)); then
      echo "check-feature: cleanup of $COMPOSE_PROJECT_NAME failed (exit $down_status)" >&2
      status=1
    fi
    docker image rm "${NOTES_IMAGE_PREFIX:-react-go-notes}-postgres:$NOTES_IMAGE_TAG" \
      "${NOTES_IMAGE_PREFIX:-react-go-notes}-api:$NOTES_IMAGE_TAG" \
      "${NOTES_IMAGE_PREFIX:-react-go-notes}-proxy:$NOTES_IMAGE_TAG" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
  ((status == 0)) || outcome=FAIL
  printf '{"run":"%s","project":"%s","port":%s,"owner":"%s","outcome":"%s","exit":%s}\n' \
    "$RUN_ID" "$COMPOSE_PROJECT_NAME" "$NOTES_INTERNAL_PORT" "$owner" "$outcome" "$status" >"$evidence/run.json"
  say "$outcome run=$RUN_ID exit=$status"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Long steps run in the background so a signal is handled at once rather than
# after the step finishes; the trap then stops the step and cleans up.
run_long() {
  "$@" &
  child=$!
  local status=0
  wait "$child" || status=$?
  child=0
  return "$status"
}

# --- 2. Build, start, bounded readiness, run identity ------------------------
say "step 2: build and start"
run_long bash scripts/build-artifacts.sh
run_long docker compose build
started=1
deadline=$(($(date +%s) + 180))
run_long docker compose up -d --wait --wait-timeout 180
ready=0
while (($(date +%s) < deadline)); do
  if fetch "$base/config.json" 2>/dev/null | grep -Fq "\"runId\":\"$RUN_ID\"" &&
    fetch "$base/api/health" 2>/dev/null | grep -Fq "\"run\":\"$RUN_ID\"" &&
    fetch "$base/auth/realms/master/.well-known/openid-configuration" 2>/dev/null |
    grep -Fq "\"issuer\":\"$base/auth/realms/master\""; then
    ready=1
    break
  fi
  sleep 2
done
((ready)) || die "stack not ready within 180 s, or the instance on port $NOTES_INTERNAL_PORT is not run $RUN_ID"
say "ready: proxy, API and Keycloak answer as run $RUN_ID on $base"

# --- 3. Migrations ----------------------------------------------------------
say "step 3: migrations"
psql_as() {
  local user="$1" db="$2"
  shift 2
  docker compose exec -T postgres psql -X -q -At -v ON_ERROR_STOP=1 -U "$user" -d "$db" "$@"
}
migration_count="$(find apps/api/migrations -maxdepth 1 -name '[0-9][0-9][0-9][0-9]_*.sql' | wc -l)"
first_version="$(basename "$(find apps/api/migrations -maxdepth 1 -name '0001_*.sql' | head -n 1)" .sql)"
[[ -n "$first_version" ]] || die "migration 0001 is missing"

# Upgrade: 0001 only, rows seeded, then the rest; rows and constraints survive.
psql_as postgres postgres -c "CREATE DATABASE notes_upgrade OWNER notes_app" </dev/null
bash scripts/migrate.sh --database notes_upgrade --to 0001
psql_as notes_app notes_upgrade -c "INSERT INTO notes (owner_sub, label) VALUES ('upgrade:a', 'kept one'), ('upgrade:a', 'kept two'), ('upgrade:b', 'kept three')" </dev/null
bash scripts/migrate.sh --database notes_upgrade
[[ "$(psql_as notes_app notes_upgrade -c "SELECT count(*) FROM notes WHERE label LIKE 'kept %'" </dev/null)" == 3 ]] ||
  die "upgrade lost seeded rows"
[[ "$(psql_as notes_app notes_upgrade -c "SELECT count(*) FROM notes WHERE created_at IS NULL" </dev/null)" == 0 ]] ||
  die "upgrade left rows without the new column's default"
[[ "$(psql_as notes_app notes_upgrade -c "SELECT count(*) FROM schema_migrations" </dev/null)" == "$migration_count" ]] ||
  die "upgrade did not record every migration"
if psql_as notes_app notes_upgrade -c "INSERT INTO notes (owner_sub, label) VALUES ('upgrade:a', '')" </dev/null 2>/dev/null; then
  die "upgrade lost the label constraint"
fi
if psql_as notes_app notes_upgrade -c "INSERT INTO notes (label) VALUES ('no owner')" </dev/null 2>/dev/null; then
  die "upgrade lost the owner constraint"
fi
say "upgrade from $first_version kept 3 rows and both constraints"

# Fresh install, then a re-run that must apply nothing.
bash scripts/migrate.sh
[[ "$(psql_as notes_app notes -c "SELECT count(*) FROM schema_migrations" </dev/null)" == "$migration_count" ]] ||
  die "fresh install did not record every migration"
rerun="$(bash scripts/migrate.sh)"
[[ "$rerun" == *"migrate: 0 applied to notes"* ]] || die "a second migration run applied something: $rerun"
say "fresh install applied $migration_count migrations; re-run applied none"

# --- 4. Seed ---------------------------------------------------------------
say "step 4: seed"
bash scripts/seed.sh

# --- 5. Realm, twice --------------------------------------------------------
say "step 5: realm configuration (twice)"
first="$(bash scripts/keycloak-config.sh | grep '^user-id ')"
second="$(bash scripts/keycloak-config.sh | grep '^user-id ')"
[[ "$(wc -l <<<"$first")" == 2 ]] || die "expected two test users, got: $first"
[[ "$first" == "$second" ]] || die "a second realm configuration changed the users: $first / $second"
say "realm $NOTES_REALM configured twice; users kept their ids"

# --- 6. Browser ------------------------------------------------------------
say "step 6: Playwright"
# control-hook: before-browser
run_long env NOTES_BASE_URL="$base" E2E_USERNAME=alice E2E_PASSWORD="$NOTES_TEST_PASSWORD" \
  FACTORY_EVIDENCE_DIR="$work/playwright" \
  npx --no-install playwright test --config tests/e2e/playwright.config.ts

# --- 7. Evidence ------------------------------------------------------------
say "step 7: evidence"
cp -a "$work/playwright/." "$evidence/"
rm -rf "$work/playwright"
printf '%s\n' "$first" >"$evidence/keycloak-users.txt"
outcome=PASS
