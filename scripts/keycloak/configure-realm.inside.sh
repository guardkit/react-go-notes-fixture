# shellcheck shell=bash
# Runs inside the keycloak container (bash -s), after the exported settings.
# kcadm talks to Keycloak directly on localhost; the admin console is not
# served through the proxy.
set -euo pipefail
# The image's own paths; KCADM and KCADM_SERVER let a test stand in a
# command shim and a stub endpoint for a live Keycloak.
kc="${KCADM:-/opt/keycloak/bin/kcadm.sh}"
kc_server="${KCADM_SERVER:-http://localhost:8080/auth}"
cfg="/tmp/kcadm-$$.config"
out="/tmp/kcadm-$$.out"
trap 'rm -f "$cfg" "$out"' EXIT
# Commands get /dev/null as stdin unless piped: stdin is this script.
k() { "$kc" "$@" --config "$cfg" </dev/null; }
kf() { "$kc" "$@" --config "$cfg"; }

# The bounded readiness wait for the two calls below (scripts/keycloak/
# lib/readiness.sh). scripts/keycloak-config.sh concatenates it ahead of this
# script, because the keycloak image has no copy of the repository; run
# straight from a checkout, it is sourced from disk.
if ! declare -F kc_wait_ready >/dev/null; then
  # shellcheck source=lib/readiness.sh
  source "$(dirname "${BASH_SOURCE[0]}")/lib/readiness.sh"
fi

# Only the login and the first admin call wait for readiness: 503 or a
# refused connection is retried inside one 2-minute budget, anything else -
# a wrong password, a 500 - fails straight away. The password reaches the
# login through the stdin transport of scripts/keycloak-config.sh, never a
# command line on the host, and never appears in a message.
login_with_readiness "$out"

kc_wait_ready "first admin call (realm list)" "$kc_server/admin/realms" "$out" -- \
  k get realms --fields realm --format csv --noquotes
if ! grep -Fxq "$NOTES_REALM" "$out"; then
  k create realms -s "realm=$NOTES_REALM" -s enabled=true -s sslRequired=external >/dev/null
  echo "keycloak: created realm $NOTES_REALM" >&2
fi

client_json="$(printf '{"clientId":"%s","name":"Notes fixture","enabled":true,"protocol":"openid-connect","publicClient":true,"standardFlowEnabled":true,"implicitFlowEnabled":false,"directAccessGrantsEnabled":false,"serviceAccountsEnabled":false,"redirectUris":%s,"webOrigins":["+"],"attributes":{"pkce.code.challenge.method":"S256","post.logout.redirect.uris":"+"}}' \
  "$NOTES_CLIENT_ID" "$NOTES_REDIRECTS_JSON")"
client_id() { k get clients -r "$NOTES_REALM" -q "clientId=$NOTES_CLIENT_ID" --fields id --format csv --noquotes; }
cid="$(client_id)"
if [[ -z "$cid" ]]; then
  printf '%s' "$client_json" | kf create clients -r "$NOTES_REALM" -f - >/dev/null
  cid="$(client_id)"
  echo "keycloak: created client $NOTES_CLIENT_ID" >&2
else
  # An update with -f sends this body as it is, so it carries the id.
  printf '{"id":"%s",%s' "$cid" "${client_json#\{}" | kf update "clients/$cid" -r "$NOTES_REALM" -f - >/dev/null
fi
[[ "$cid" =~ ^[0-9a-f-]{36}$ ]] || { echo "keycloak: client id not found" >&2; exit 1; }

mapper_json="$(printf '{"name":"notes-api-audience","protocol":"openid-connect","protocolMapper":"oidc-audience-mapper","config":{"included.custom.audience":"%s","access.token.claim":"true","id.token.claim":"false","introspection.token.claim":"true"}}' "$NOTES_AUDIENCE")"
# The Keycloak image has no awk: read "id,name" lines in bash.
mappers="$(k get "clients/$cid/protocol-mappers/models" -r "$NOTES_REALM" --fields id,name --format csv --noquotes)"
mid=""
while IFS=, read -r one_id one_name; do
  if [[ "$one_name" == notes-api-audience ]]; then mid="$one_id"; fi
done <<<"$mappers"
if [[ -z "$mid" ]]; then
  printf '%s' "$mapper_json" | kf create "clients/$cid/protocol-mappers/models" -r "$NOTES_REALM" -f - >/dev/null
  echo "keycloak: created audience mapper for $NOTES_AUDIENCE" >&2
else
  # Keycloak refuses a mapper update whose body has no id ("mapping with id
  # null does not exist").
  printf '{"id":"%s",%s' "$mid" "${mapper_json#\{}" |
    kf update "clients/$cid/protocol-mappers/models/$mid" -r "$NOTES_REALM" -f - >/dev/null
fi

user_id() { k get users -r "$NOTES_REALM" -q "username=$1" -q exact=true --fields id --format csv --noquotes; }
for pair in alice:Alice bob:Bob; do
  username="${pair%%:*}"
  first="${pair##*:}"
  uid="$(user_id "$username")"
  if [[ -z "$uid" ]]; then
    printf '{"username":"%s","enabled":true,"emailVerified":true,"firstName":"%s","lastName":"Tester","email":"%s@example.test","requiredActions":[]}' \
      "$username" "$first" "$username" | kf create users -r "$NOTES_REALM" -f - >/dev/null
    uid="$(user_id "$username")"
    echo "keycloak: created user $username" >&2
  fi
  [[ "$uid" =~ ^[0-9a-f-]{36}$ ]] || { echo "keycloak: user $username not found" >&2; exit 1; }
  printf '{"type":"password","value":"%s","temporary":false}' "$NOTES_TEST_PASSWORD" |
    kf update "users/$uid/reset-password" -r "$NOTES_REALM" -f - -n >/dev/null
  echo "user-id $username $uid"
done
