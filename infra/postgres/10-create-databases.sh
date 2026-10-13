#!/usr/bin/env bash
# Runs once, on an empty data volume, as the image's init step. Two databases,
# each owned by its own login role; neither role can connect to the other's.
set -euo pipefail
: "${APP_DB_PASSWORD:?APP_DB_PASSWORD is required}"
: "${KEYCLOAK_DB_PASSWORD:?KEYCLOAK_DB_PASSWORD is required}"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -v app_pw="$APP_DB_PASSWORD" -v kc_pw="$KEYCLOAK_DB_PASSWORD" <<'SQL'
CREATE ROLE notes_app LOGIN PASSWORD :'app_pw';
CREATE ROLE keycloak LOGIN PASSWORD :'kc_pw';
CREATE DATABASE notes OWNER notes_app;
CREATE DATABASE keycloak OWNER keycloak;
REVOKE CONNECT, TEMPORARY ON DATABASE notes FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE keycloak FROM PUBLIC;
GRANT CONNECT ON DATABASE notes TO notes_app;
GRANT CONNECT ON DATABASE keycloak TO keycloak;
SQL
