#!/usr/bin/env bash
# Idempotently configures this run's realm with kcadm inside the running
# keycloak container: the realm, the public PKCE client, the audience mapper
# that puts the API's audience into access tokens, and two test users.
# Running it again keeps existing users (same ids) and refreshes settings.
#
# Prints one "user-id <username> <id>" line per test user.
#
# Environment: COMPOSE_PROJECT_NAME, NOTES_REALM, KEYCLOAK_ADMIN_PASSWORD,
# NOTES_TEST_PASSWORD, NOTES_REDIRECT_ORIGINS (comma-separated origins, e.g.
# http://127.0.0.1:41001), optional NOTES_CLIENT_ID (notes-web) and
# NOTES_AUDIENCE (notes-api).
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib.sh
require_command docker
require_project

export NOTES_CLIENT_ID="${NOTES_CLIENT_ID:-notes-web}"
export NOTES_AUDIENCE="${NOTES_AUDIENCE:-notes-api}"
for name in NOTES_REALM NOTES_CLIENT_ID NOTES_AUDIENCE; do
  [[ "${!name:-}" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$ ]] || die "$name must be a plain name"
done
[[ "${KEYCLOAK_ADMIN_PASSWORD:-}" =~ ^[A-Za-z0-9]{16,}$ ]] || die "KEYCLOAK_ADMIN_PASSWORD must be 16+ letters or digits"
[[ "${NOTES_TEST_PASSWORD:-}" =~ ^[A-Za-z0-9]{12,}$ ]] || die "NOTES_TEST_PASSWORD must be 12+ letters or digits"

# Exactly the callback URLs of the contexts this run serves (README table).
redirects=""
IFS=',' read -r -a origins <<<"${NOTES_REDIRECT_ORIGINS:?NOTES_REDIRECT_ORIGINS is required}"
for origin in "${origins[@]}"; do
  [[ "$origin" =~ ^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || die "invalid redirect origin: $origin"
  redirects+="${redirects:+,}\"$origin/*\""
done
export NOTES_REDIRECTS_JSON="[$redirects]"

# Secrets travel on the exec's stdin, ahead of the script body, never on a
# command line on the host.
{
  for name in NOTES_REALM NOTES_CLIENT_ID NOTES_AUDIENCE NOTES_REDIRECTS_JSON KEYCLOAK_ADMIN_PASSWORD NOTES_TEST_PASSWORD; do
    printf 'export %s=%q\n' "$name" "${!name}"
  done
  # The readiness helper is function definitions only, and the keycloak image
  # has no copy of the repository, so it goes down the same stdin stream as
  # the script body that uses it.
  cat scripts/keycloak/lib/readiness.sh
  cat scripts/keycloak/configure-realm.inside.sh
} | docker compose exec -T keycloak bash -s
