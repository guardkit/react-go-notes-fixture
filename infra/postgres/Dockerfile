# PostgreSQL 17.7 with an init script that creates separate app and Keycloak
# databases and roles. COPY only: the build needs no network.
FROM postgres:17.7-alpine@sha256:bb377b7239d2774ac8cc76f481596ce96c5a6b5e9d141f6d0a0ee371a6e7c0f2
COPY 10-create-databases.sh /docker-entrypoint-initdb.d/10-create-databases.sh
