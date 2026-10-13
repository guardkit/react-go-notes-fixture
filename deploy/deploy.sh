#!/usr/bin/env bash
# The fixture's vetted deploy step (deploy/profile.yaml compose.script).
# Forge runs it with no arguments; every input arrives in the environment.
# deploy/settings.sh lists exactly what Forge sends in each leg.
#
# Modes (exactly one signal; none is refused):
#   CANDIDATE=1        build and start the candidate project (candidate.env:
#                      COMPOSE_PROJECT_NAME, CANDIDATE_PORT) and print
#                      CHECKED_ARTIFACT=<image ids>
#   CANDIDATE_DOWN=1   take down the named candidate project, only if what
#                      runs there is DEPLOY_IDENTITY's (or nothing runs)
#   PROMOTE=1          start the live project (notes-live, port 8960) from
#                      exactly the images in DEPLOY_ARTIFACT, never a rebuild
#   PREVIEW=1 / FACTORY_PREVIEW_PORT
#                      retained review preview: internal listener on
#                      FACTORY_PREVIEW_INTERNAL_PORT, public listener on the
#                      slot port guarded by FACTORY_PREVIEW_TOKEN, issuers and
#                      callbacks for both addresses
#   RUNNING_IDENTITY=1 read-only: print what the live project runs
#   TEARDOWN=1         remove the named COMPOSE_PROJECT_NAME (manual/controls)
#   REVERT=1           refused: this profile keeps no rollback image
# A teardown never defaults its project: CANDIDATE_DOWN and TEARDOWN refuse
# unless COMPOSE_PROJECT_NAME names one, and never the live project.
#
# Labels: FACTORY_RESOURCE_LABEL when set (forge.preview.lease=<lease> for a
# preview), else react-go-notes.deploy=<project>; built images carry it too.
# Never guardkit.fixture.owner, so the disposable sweeps leave deployments
# alone.
#
# Per-project settings (database and admin passwords, the test password, the
# realm and the instance id) live in a mode-600 file (deploy/settings.sh
# settings_file), created on first start and reused so the data volume keeps
# working; candidate down and teardown delete it.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
source scripts/lib.sh
source deploy/settings.sh

deploy_mode
case "$mode" in
none) die "no deploy mode is set: expected CANDIDATE, CANDIDATE_DOWN, PROMOTE, PREVIEW, RUNNING_IDENTITY or TEARDOWN" ;;
revert) die "REVERT: this fixture keeps no rollback image (its profile has no rollback_image_ref)" ;;
esac

prefix="${NOTES_IMAGE_PREFIX:-react-go-notes}"
export NOTES_IMAGE_PREFIX="$prefix"
services=(postgres api proxy)

# --- Which project ---------------------------------------------------------------
case "$mode" in
running) project="${COMPOSE_PROJECT_NAME:-$notes_live_project}" ;;
candidate) project="${COMPOSE_PROJECT_NAME:-$notes_live_project-cand}" ;;
promote)
  [[ -z "${COMPOSE_PROJECT_NAME:-}" || "$COMPOSE_PROJECT_NAME" == "$notes_live_project" ]] ||
    die "PROMOTE deploys the live project $notes_live_project, not $COMPOSE_PROJECT_NAME"
  project="$notes_live_project"
  ;;
candidate_down | teardown | preview)
  [[ -n "${COMPOSE_PROJECT_NAME:-}" ]] || die "$mode needs COMPOSE_PROJECT_NAME to name its project; refusing to guess"
  project="$COMPOSE_PROJECT_NAME"
  ;;
esac
if [[ "$mode" == candidate || "$mode" == candidate_down || "$mode" == teardown || "$mode" == preview ]]; then
  [[ "$project" != "$notes_live_project" ]] || die "$mode refuses to act on the live project $notes_live_project"
fi
if [[ "$mode" == promote ]]; then
  [[ -n "${DEPLOY_IDENTITY:-}" ]] || die "PROMOTE needs DEPLOY_IDENTITY"
  [[ "${DEPLOY_ARTIFACT:-}" =~ ^postgres=sha256:[0-9a-f]{64},api=sha256:[0-9a-f]{64},proxy=sha256:[0-9a-f]{64}$ ]] ||
    die "PROMOTE needs DEPLOY_ARTIFACT, the image ids a candidate printed as CHECKED_ARTIFACT; refusing to rebuild"
fi
export COMPOSE_PROJECT_NAME="$project"
require_project
state_file="$(settings_file "$project")"
require_docker
export COMPOSE_FILE="$root/compose.yaml"

# Forge's identity text is "<name>@<fingerprint>". An image tag cannot carry
# '@', so the tag is the text with '@' turned into '-'; the text itself is
# kept on the api container as the label react-go-notes.identity and is what
# DEPLOYED_IDENTITY and RUNNING_IDENTITY report.
tag_for() { printf '%s' "${1//@/-}"; }

# The project's running api container id, or "" when none runs.
running_api() {
  docker ps --filter "label=com.docker.compose.project=$project" \
    --filter "label=com.docker.compose.service=api" --format '{{.ID}}'
}

# The tag the running api container was created from, or "" when none runs.
running_tag() {
  local id created_from
  id="$(running_api)"
  [[ -n "$id" ]] || return 0
  created_from="$(docker inspect --format '{{.Config.Image}}' "$id")"
  [[ "$created_from" == "$prefix-api:"* ]] || die "the api container in $project was not created from a $prefix-api image"
  printf '%s' "${created_from#"$prefix-api:"}"
}

# The identity text on the running api container, "" when none runs. Without
# the label, a tag of Forge's shape "<name>-<16 hex>" is read back as
# "<name>@<16 hex>" (as api_test does: the fingerprint has no dash, so the
# last dash was the '@'); anything else cannot be accounted for.
running_identity() {
  local id text tag
  id="$(running_api)"
  [[ -n "$id" ]] || return 0
  text="$(docker inspect --format '{{index .Config.Labels "react-go-notes.identity"}}' "$id")"
  if [[ -z "$text" ]]; then
    tag="$(running_tag)"
    [[ "$tag" =~ ^(.+)-([0-9a-f]{16})$ ]] ||
      die "the api container in $project carries no react-go-notes.identity label and its tag $tag is not Forge's shape"
    text="${BASH_REMATCH[1]}@${BASH_REMATCH[2]}"
  fi
  printf '%s' "$text"
}

# --- Read-only: what is running? ---------------------------------------------------
if [[ "$mode" == running ]]; then
  marker="${RUNNING_AS:-RUNNING_IDENTITY}"
  if ! answer="$(running_identity 2>&1)"; then
    echo "${marker}_UNKNOWN=${answer//$'\n'/ }"
  elif [[ -z "$answer" ]]; then
    echo "$marker=none"
  else
    echo "$marker=$answer"
  fi
  exit 0
fi

# --- Taking a project down ---------------------------------------------------------
take_down() {
  export FACTORY_RESOURCE_LABEL="${FACTORY_RESOURCE_LABEL:-react-go-notes.deploy=$project}"
  # Interpolation needs every required name; values are irrelevant to `down`.
  local name
  for name in NOTES_IMAGE_TAG POSTGRES_PASSWORD APP_DB_PASSWORD KEYCLOAK_DB_PASSWORD KEYCLOAK_ADMIN_PASSWORD \
    OIDC_ISSUERS NOTES_REALM NOTES_RUN_ID NOTES_INTERNAL_PORT; do
    export "$name=${!name:-teardown}"
  done
  docker compose down -v --remove-orphans --timeout 10
  rm -f "$state_file"
  echo "deploy: removed $project" >&2
}
if [[ "$mode" == candidate_down ]]; then
  [[ -n "${DEPLOY_IDENTITY:-}" ]] || die "CANDIDATE_DOWN needs DEPLOY_IDENTITY to name which candidate"
  # Compared as image tags: the one form both a running candidate and the
  # identity handed over always have.
  current="$(running_tag)"
  if [[ -n "$current" && "$current" != "$(tag_for "$DEPLOY_IDENTITY")" ]]; then
    die "the candidate in $project is $current, not $(tag_for "$DEPLOY_IDENTITY") ($DEPLOY_IDENTITY); leaving it for its own build"
  fi
  take_down
  exit 0
fi
if [[ "$mode" == teardown ]]; then
  take_down
  exit 0
fi

# --- Settings for this project ---------------------------------------------------
# The settings folder is private from the moment it exists; the settings file
# is created under umask 077 below (the umask is set before it is opened).
(umask 077 && mkdir -p "$(dirname "$state_file")")
chmod 700 "$(dirname "$state_file")"
if [[ ! -f "$state_file" ]]; then
  instance="$(date -u +%Y%m%d%H%M%S)-$(rand_hex 6)"
  (
    umask 077
    {
      for name in POSTGRES_PASSWORD APP_DB_PASSWORD KEYCLOAK_DB_PASSWORD KEYCLOAK_ADMIN_PASSWORD NOTES_TEST_PASSWORD; do
        printf '%s=%s\n' "$name" "$(rand_token 24)"
      done
      printf 'NOTES_RUN_ID=%s\nNOTES_REALM=notes-%s\n' "$instance" "$instance"
    } >"$state_file"
  )
fi
while IFS='=' read -r name value; do
  [[ "$name" =~ ^(POSTGRES_PASSWORD|APP_DB_PASSWORD|KEYCLOAK_DB_PASSWORD|KEYCLOAK_ADMIN_PASSWORD|NOTES_TEST_PASSWORD|NOTES_RUN_ID|NOTES_REALM)$ ]] ||
    die "unexpected line in $state_file"
  export "$name=$value"
done <"$state_file"

# Forge's preview overlay carries no identity: a preview is named by its lease.
default_identity="manual-$NOTES_RUN_ID"
if [[ "$mode" == preview && "${FACTORY_RESOURCE_LABEL:-}" == forge.preview.lease=* ]]; then
  default_identity="preview-${FACTORY_RESOURCE_LABEL#*=}"
fi
identity="${DEPLOY_IDENTITY:-$default_identity}"
[[ "$identity" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,100}(@[A-Za-z0-9]{1,26})?$ ]] ||
  die "DEPLOY_IDENTITY must be <name> or <name>@<fingerprint> (letters, digits, '.', '_', '-')"
tag="$(tag_for "$identity")"
export NOTES_IMAGE_TAG="$tag" NOTES_IDENTITY="$identity"
export NOTES_CLIENT_ID=notes-web

# --- Addresses, issuers and callbacks for this mode (see the README table) ---------
shows_password="${NOTES_HINT_SHOWS_PASSWORD:-}"
case "$mode" in
preview)
  COMPOSE_FILE="$root/compose.yaml:$root/compose.preview.yaml"
  [[ "${FACTORY_RESOURCE_LABEL:-}" == forge.preview.lease=* ]] ||
    die "a preview needs FACTORY_RESOURCE_LABEL=forge.preview.lease=<lease>"
  [[ "${FACTORY_PREVIEW_PORT:-}" =~ ^[0-9]{2,5}$ ]] || die "FACTORY_PREVIEW_PORT is required for a preview"
  : "${FACTORY_PREVIEW_URL:?FACTORY_PREVIEW_URL is required for a preview}"
  : "${FACTORY_PREVIEW_INTERNAL_PORT:?FACTORY_PREVIEW_INTERNAL_PORT is required for a preview}"
  [[ "${FACTORY_PREVIEW_TOKEN:-}" =~ ^[A-Za-z0-9_-]{16,128}$ ]] ||
    die "FACTORY_PREVIEW_TOKEN must be 16-128 URL-safe characters"
  [[ "$FACTORY_PREVIEW_URL" =~ ^https://([A-Za-z0-9.-]+):([0-9]{1,5})/?$ ]] ||
    die "FACTORY_PREVIEW_URL must be https://<host>:<port>"
  [[ "${BASH_REMATCH[2]}" == "$FACTORY_PREVIEW_PORT" ]] ||
    die "FACTORY_PREVIEW_URL's port must be the slot port $FACTORY_PREVIEW_PORT"
  export FACTORY_PREVIEW_PUBLIC_HOST="${BASH_REMATCH[1]}:${BASH_REMATCH[2]}"
  public="https://$FACTORY_PREVIEW_PUBLIC_HOST"
  export NOTES_INTERNAL_PORT="$FACTORY_PREVIEW_INTERNAL_PORT"
  internal="http://127.0.0.1:$NOTES_INTERNAL_PORT"
  # Both issuers of this run's realm, which share one signing key.
  export OIDC_ISSUERS="$internal/auth/realms/$NOTES_REALM,$public/auth/realms/$NOTES_REALM"
  export NOTES_REDIRECT_ORIGINS="$internal,$public"
  # A preview's own sign-in hint shows this instance's test password, so a
  # reviewer on the private network can sign in (never in the bundle or on the card).
  shows_password="${NOTES_HINT_SHOWS_PASSWORD:-1}"
  ;;
candidate)
  export NOTES_INTERNAL_PORT="${CANDIDATE_PORT:?CANDIDATE_PORT is required for a candidate}"
  ;;
promote)
  export NOTES_INTERNAL_PORT="$notes_live_port"
  ;;
esac
[[ "$NOTES_INTERNAL_PORT" =~ ^[0-9]{4,5}$ ]] || die "invalid internal port: $NOTES_INTERNAL_PORT"
if [[ "$mode" != preview ]]; then
  internal="http://127.0.0.1:$NOTES_INTERNAL_PORT"
  export OIDC_ISSUERS="$internal/auth/realms/$NOTES_REALM"
  export NOTES_REDIRECT_ORIGINS="$internal"
fi
export FACTORY_RESOURCE_LABEL="${FACTORY_RESOURCE_LABEL:-react-go-notes.deploy=$project}"
[[ "$FACTORY_RESOURCE_LABEL" =~ ^[A-Za-z0-9._-]+=[A-Za-z0-9._-]+$ ]] || die "FACTORY_RESOURCE_LABEL must be key=value"
if truthy "$shows_password"; then
  export NOTES_SIGNIN_HINT="Test users: alice and bob. Password for this instance: $NOTES_TEST_PASSWORD"
else
  export NOTES_SIGNIN_HINT="Test users: alice and bob."
fi
require_plain NOTES_SIGNIN_HINT

image_ids() {
  local service cid image out=""
  for service in "${services[@]}"; do
    cid="$(docker compose ps -q "$service")"
    [[ -n "$cid" ]] || die "service $service is not running in $project"
    image="$(docker inspect --format '{{.Image}}' "$cid")"
    out+="${out:+,}$service=$image"
  done
  printf '%s' "$out"
}

# --- Build, or adopt exactly the checked images ---------------------------------
if [[ "$mode" == promote ]]; then
  IFS=',' read -r -a parts <<<"$DEPLOY_ARTIFACT"
  for part in "${parts[@]}"; do
    docker tag "${part#*=}" "$prefix-${part%%=*}:$tag"
  done
  docker compose up -d --no-build --pull never --wait --wait-timeout 180
else
  # Forge lays the candidate or preview out from `git archive`, so nothing is
  # installed yet: the locked install (with its lockfile guard) runs first, and
  # a failed install stops the deploy before any image is built.
  bash scripts/install.sh >&2
  bash scripts/build-artifacts.sh >&2
  docker compose build >&2
  docker compose up -d --wait --wait-timeout 180
fi

# --- Explicit migrations, seed and realm ------------------------------------------
bash scripts/migrate.sh >&2
bash scripts/seed.sh >&2
bash scripts/keycloak-config.sh >&2

# The images carry the tag form; the api container carries the identity text,
# which the read-only question above reads back.
running="$(image_ids)"
if [[ "$mode" == promote && "$running" != "$DEPLOY_ARTIFACT" ]]; then
  die "the running images ($running) are not the checked artifact ($DEPLOY_ARTIFACT)"
fi
for service in "${services[@]}"; do
  docker image inspect "$prefix-$service:$tag" >/dev/null ||
    die "image $prefix-$service:$tag is missing"
done

echo "deploy: $project ($mode) up on $internal (instance $NOTES_RUN_ID)" >&2
if [[ "$mode" == candidate || "$mode" == preview ]]; then
  echo "${CHECKED_AS:-CHECKED_ARTIFACT}=$running"
fi
echo "${REPORTED_AS:-DEPLOYED_IDENTITY}=$identity"
