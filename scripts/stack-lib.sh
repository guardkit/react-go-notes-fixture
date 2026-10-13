# shellcheck shell=bash
# shellcheck disable=SC2154,SC2034 # root comes from the sourcing script; RUN_ID, stack_owner and stack_base are read there
# The disposable stack's lifecycle pieces, shared by scripts/check-feature.sh
# (the canonical connected check) and scripts/stack.sh (the stack a direct
# Playwright run brings up for itself). Sourced after scripts/lib.sh, from the
# fixture root. Every piece acts on this run's own compose project only.

# stack_identity KIND OWNER_FALLBACK: a run id, a free loopback port, a
# project named notes-KIND-<run>, the owner label and fresh secrets, all
# exported for compose. Sets RUN_ID, stack_owner and stack_base.
stack_identity() {
  local kind="$1" fallback="$2" name
  RUN_ID="$(date -u +%Y%m%d%H%M%S)-$(rand_hex 6)"
  stack_owner="${GUARDKIT_RUN_OWNER:-$fallback-$RUN_ID}"
  [[ "$stack_owner" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] ||
    die "GUARDKIT_RUN_OWNER must be letters, digits, '.', '_' or '-'"
  export NOTES_RUN_ID="$RUN_ID"
  export COMPOSE_PROJECT_NAME="notes-$kind-$RUN_ID"
  export COMPOSE_FILE="$root/compose.yaml"
  export FACTORY_RESOURCE_LABEL="guardkit.fixture.owner=$stack_owner"
  export NOTES_IMAGE_TAG="$kind-$RUN_ID"
  export NOTES_REALM="notes-$RUN_ID"
  export NOTES_CLIENT_ID=notes-web
  export NOTES_INTERNAL_PORT
  NOTES_INTERNAL_PORT="$(free_port)"
  stack_base="http://127.0.0.1:$NOTES_INTERNAL_PORT"
  # A disposable stack accepts only its own internal issuer.
  export OIDC_ISSUERS="$stack_base/auth/realms/$NOTES_REALM"
  export NOTES_REDIRECT_ORIGINS="$stack_base"
  export NOTES_SIGNIN_HINT="Test users: alice and bob."
  for name in POSTGRES_PASSWORD APP_DB_PASSWORD KEYCLOAK_DB_PASSWORD KEYCLOAK_ADMIN_PASSWORD NOTES_TEST_PASSWORD; do
    export "$name=$(rand_token 24)"
  done
  unset OIDC_AUDIENCE FACTORY_PREVIEW_PORT FACTORY_PREVIEW_URL FACTORY_PREVIEW_TOKEN COMPOSE_PROFILES
}

# stack_build [RUNNER...]: the web bundle, the static API binary and the
# copy-only images. RUNNER (e.g. check-feature.sh's run_long) wraps each long
# command so a signal is handled at once.
stack_build() {
  "$@" bash scripts/build-artifacts.sh
  "$@" docker compose build
}

# stack_up [RUNNER...]: start the stack and wait, at most 180 s in all, until
# the proxy, the API and Keycloak answer as THIS run on its own port.
stack_up() {
  local deadline ready=0
  deadline=$(($(date +%s) + 180))
  "$@" docker compose up -d --wait --wait-timeout 180
  while (($(date +%s) < deadline)); do
    if fetch "$stack_base/config.json" 2>/dev/null | grep -Fq "\"runId\":\"$RUN_ID\"" &&
      fetch "$stack_base/api/health" 2>/dev/null | grep -Fq "\"run\":\"$RUN_ID\"" &&
      fetch "$stack_base/auth/realms/master/.well-known/openid-configuration" 2>/dev/null |
      grep -Fq "\"issuer\":\"$stack_base/auth/realms/master\""; then
      ready=1
      break
    fi
    sleep 2
  done
  ((ready)) || die "stack not ready within 180 s, or the instance on port $NOTES_INTERNAL_PORT is not run $RUN_ID"
}

# stack_remove_images LOG: removes this run's images, by this run's own tag
# only (NOTES_IMAGE_TAG is unique per run), never another run's. A tag that
# was never built is simply absent. Callers arm this BEFORE the image build
# starts: `compose build` can tag one service's image and then fail on the
# next, and that image must not be left behind.
stack_remove_images() {
  local log="${1:?stack_remove_images needs a log file}" prefix="${NOTES_IMAGE_PREFIX:-react-go-notes}"
  docker image rm "$prefix-postgres:$NOTES_IMAGE_TAG" "$prefix-api:$NOTES_IMAGE_TAG" \
    "$prefix-proxy:$NOTES_IMAGE_TAG" >>"$log" 2>&1 </dev/null || true
}

# stack_down LOG: `compose down -v` of this run's project, then this run's
# images. Its output goes to LOG, never to the caller's stdout or stderr: a
# caller killed on a timeout leaves those as closed pipes, and a write there
# would end compose with SIGPIPE before the teardown finished. It runs in its
# own session, so a signal sent to the caller's whole process group cannot
# stop it half-way either. Returns compose's exit status.
stack_down() {
  local log="${1:?stack_down needs a log file}" status=0
  setsid docker compose down -v --remove-orphans --timeout 10 >>"$log" 2>&1 </dev/null &
  wait "$!" || status=$?
  stack_remove_images "$log"
  return "$status"
}
