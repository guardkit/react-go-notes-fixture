#!/usr/bin/env bash
# A disposable stack for a DIRECT Playwright run (tests/e2e/global-setup.ts):
#
#   scripts/stack.sh up            start one, print its settings as KEY=VALUE
#   scripts/stack.sh down STATE    remove it (STATE is the NOTES_STACK_STATE
#                                  file `up` printed)
#
# scripts/check-feature.sh is the canonical connected check; this exists so
# `npx playwright test --config tests/e2e/playwright.config.ts` also works on
# its own. It uses the same lifecycle pieces (scripts/stack-lib.sh): its own
# compose project (notes-pw-<run>), the owner label
# guardkit.fixture.owner=${GUARDKIT_RUN_OWNER:-notes-direct-<run>}, a free
# loopback port, readiness bounded to 180 s with a run-identity check, fresh
# migrations, the seed and the realm. A failed or interrupted `up` removes
# what it started.
#
# Output: stdout carries only the settings; stderr only short status lines.
# Everything the build, compose, migrations and realm print goes to
# .notes-runs/<run>/stack.log. A caller killed on a timeout leaves stdout and
# stderr as closed pipes; SIGPIPE is ignored here, so a failed status write
# never stops a teardown, and compose never writes to those pipes at all.
set -euo pipefail
trap '' PIPE
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
source scripts/lib.sh
source scripts/stack-lib.sh

# Status lines go to the caller's stderr as it was at start (fd 3), also from
# the EXIT trap, which can run while a step's output is redirected to the log.
exec 3>&2
status_line() { printf 'stack: %s\n' "$*" >&3 || true; }

refuse_without_docker() {
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    return 0
  fi
  die "A direct Playwright run needs a stack, and Docker is not reachable here. Run the browser journeys with scripts/check-feature.sh where Docker is available, or set NOTES_BASE_URL (with NOTES_RUN_ID, E2E_USERNAME and E2E_PASSWORD) to a running stack."
}

up() {
  refuse_without_docker
  source scripts/require-tools.sh
  require_docker
  require_command setsid
  stack_identity pw notes-direct
  # Globals, not locals: the EXIT trap (undo) runs after `up` has returned.
  local name
  dir="$root/.notes-runs/$RUN_ID"
  state="$dir/stack.env"
  log="$dir/stack.log"
  # The run folder is private (0700) and stack.env, which holds the passwords,
  # is created 0600: the umask is set BEFORE the file is opened, because a
  # redirection on the subshell itself would open it under the caller's umask.
  (
    umask 077
    mkdir -p "$dir"
    chmod 700 "$dir"
    {
      for name in COMPOSE_PROJECT_NAME COMPOSE_FILE FACTORY_RESOURCE_LABEL NOTES_IMAGE_TAG NOTES_RUN_ID NOTES_REALM \
        NOTES_CLIENT_ID NOTES_INTERNAL_PORT OIDC_ISSUERS NOTES_REDIRECT_ORIGINS NOTES_SIGNIN_HINT \
        POSTGRES_PASSWORD APP_DB_PASSWORD KEYCLOAK_DB_PASSWORD KEYCLOAK_ADMIN_PASSWORD NOTES_TEST_PASSWORD; do
        printf 'export %s=%q\n' "$name" "${!name}"
      done
    } >"$state"
  )
  status_line "$COMPOSE_PROJECT_NAME on $stack_base (owner $stack_owner); log $log"

  started=0
  images_armed=0
  # shellcheck disable=SC2329 # invoked by the traps
  undo() {
    local status=$?
    trap - EXIT
    trap '' INT TERM
    status_line "start failed (exit $status); see $log"
    # The settings file holds this stack's passwords; once the start has
    # failed it is removed whatever happens next. stack.log stays for
    # diagnosis (it holds no secrets: they travel on stdin or in this file).
    rm -f "$state"
    if ((started)); then
      if stack_down "$log"; then
        status_line "removed $COMPOSE_PROJECT_NAME after the failed start"
      else
        status_line "could not remove $COMPOSE_PROJECT_NAME; see $log"
      fi
    elif ((images_armed)); then
      # The image build began but nothing started: remove this run's images.
      stack_remove_images "$log"
      status_line "removed this run's images after the failed build"
    fi
    exit "$status"
  }
  trap undo EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  {
    images_armed=1
    # shellcheck disable=SC2119 # no runner wrapper here
    stack_build
    started=1
    # shellcheck disable=SC2119 # no runner wrapper here
    stack_up
    bash scripts/migrate.sh
    bash scripts/seed.sh
    bash scripts/keycloak-config.sh
  } >>"$log" 2>&1 </dev/null

  # The settings are written while the trap is armed: if the caller has gone
  # (the write fails), the stack is removed rather than left for nobody.
  printf 'NOTES_BASE_URL=%s\nNOTES_RUN_ID=%s\nE2E_USERNAME=alice\nE2E_PASSWORD=%s\nNOTES_STACK_STATE=%s\n' \
    "$stack_base" "$RUN_ID" "$NOTES_TEST_PASSWORD" "$state"
  trap - EXIT INT TERM
  status_line "ready"
}

down() {
  local state="${1:?usage: scripts/stack.sh down STATE}"
  [[ "$state" == "$root/.notes-runs/"*/stack.env && -f "$state" ]] || die "not a stack state file: $state"
  trap '' INT TERM
  # shellcheck source=/dev/null
  source "$state"
  require_project
  local dir status=0
  dir="$(dirname "$state")"
  stack_down "$dir/stack.log" || status=$?
  if ((status == 0)); then
    rm -rf "$dir"
    status_line "removed $COMPOSE_PROJECT_NAME"
  else
    status_line "could not remove $COMPOSE_PROJECT_NAME (exit $status); see $dir/stack.log"
  fi
  return "$status"
}

case "${1:-}" in
up) up ;;
down) down "${2:-}" ;;
*) die "usage: scripts/stack.sh up | down STATE" ;;
esac
