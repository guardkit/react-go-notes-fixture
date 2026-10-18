#!/usr/bin/env bash
# shellcheck shell=bash
#
# A stand-in for `docker compose exec -T postgres psql ...`, used by
# migrations_apply_test.go to drive scripts/migrate.sh with no container and no
# network. It keeps just enough PostgreSQL state on disk to answer what
# migrate.sh asks for, and it refuses the statements PostgreSQL would refuse:
# a version recorded twice, a column added twice, and a NOT NULL column added
# without a default to a table that already holds rows.
set -uo pipefail

: "${NOTES_PSQL_STUB_STATE:?the test must point NOTES_PSQL_STUB_STATE at a folder}"
state="$NOTES_PSQL_STUB_STATE"

# Everything after the `psql` word is the psql argument list.
psql_args=()
seen=0
for arg in "$@"; do
  if ((seen)); then psql_args+=("$arg"); fi
  [[ "$arg" == psql ]] && seen=1
done
((seen)) || {
  echo "psql stub: expected 'docker compose exec ... psql ...', got: $*" >&2
  exit 2
}

db=""
sql=""
stdin=0
i=0
while ((i < ${#psql_args[@]})); do
  case "${psql_args[i]}" in
  -d)
    db="${psql_args[i + 1]}"
    i=$((i + 2))
    ;;
  -c)
    sql="${psql_args[i + 1]}"
    i=$((i + 2))
    ;;
  -f)
    [[ "${psql_args[i + 1]}" == "-" ]] && stdin=1
    i=$((i + 2))
    ;;
  *) i=$((i + 1)) ;;
  esac
done
[[ -n "$db" ]] || {
  echo "psql stub: no -d database in: ${psql_args[*]}" >&2
  exit 2
}
if ((stdin)); then sql="$(cat)"; fi

versions="$state/$db.versions"
columns="$state/$db.columns"
rows="$state/$db.rows"
log="$state/$db.log"
touch "$versions" "$log"
[[ -f "$rows" ]] || echo 0 >"$rows"

fail() {
  echo "psql stub: ERROR: $1" >&2
  exit 1
}

# One statement at a time, against copies of the state: migrate.sh pipes each
# file with --single-transaction, so a later failure must leave nothing behind.
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
cp "$versions" "$scratch/versions"
cp "$log" "$scratch/log"
[[ -f "$columns" ]] && cp "$columns" "$scratch/columns" || : >"$scratch/columns"
cp "$rows" "$scratch/rows"

IFS=';'
read -r -d '' -a statements <<<"$sql" || true
unset IFS
for statement in "${statements[@]}"; do
  # psql drops `--` comments before it parses, so the stub does the same.
  norm="$(printf '%s\n' "$statement" | sed 's/--.*//' | tr '\n\t' '  ' | tr -s ' ')"
  norm="${norm# }"
  norm="${norm% }"
  [[ -z "$norm" ]] && continue
  upper="${norm^^}"
  printf '%s\n' "$norm" >>"$scratch/log"

  case "$upper" in
  "CREATE TABLE IF NOT EXISTS SCHEMA_MIGRATIONS"*)
    ;;
  "SELECT VERSION FROM SCHEMA_MIGRATIONS"*)
    cat "$scratch/versions"
    continue
    ;;
  "CREATE TABLE NOTES"*)
    : >"$scratch/columns"
    printf '%s\n' id owner_sub label >>"$scratch/columns"
    echo 0 >"$scratch/rows"
    ;;
  "ALTER TABLE NOTES ADD COLUMN"*)
    read -r -a words <<<"$norm"
    column="${words[5]}"
    [[ "${column^^}" == IF ]] && column="${words[7]}"
    column="${column,,}"
    column="${column//[!a-z0-9_]/}"
    [[ -n "$column" ]] || fail "cannot read a column name from: $norm"
    grep -Fxq "$column" "$scratch/columns" &&
      fail "column \"$column\" of relation \"notes\" already exists"
    if [[ "$upper" == *"NOT NULL"* && "$upper" != *DEFAULT* && "$(cat "$scratch/rows")" != 0 ]]; then
      fail "column \"$column\" of relation \"notes\" contains null values"
    fi
    printf '%s\n' "$column" >>"$scratch/columns"
    ;;
  "CREATE INDEX"*)
    ;;
  "INSERT INTO SCHEMA_MIGRATIONS"*)
    [[ "$upper" == *"VALUES"* ]] || fail "no VALUES in: $norm"
    version="${norm#*\'}"
    version="${version%%\'*}"
    [[ -n "$version" ]] || fail "cannot read a version from: $norm"
    grep -Fxq "$version" "$scratch/versions" &&
      fail "duplicate key value violates unique constraint \"schema_migrations_pkey\""
    printf '%s\n' "$version" >>"$scratch/versions"
    ;;
  "INSERT INTO NOTES"*)
    values="${upper#*VALUES}"
    added="$(printf '%s' "$values" | tr -cd '(' | wc -c)"
    ((added > 0)) || fail "no row to insert in: $norm"
    echo $(($(cat "$scratch/rows") + added)) >"$scratch/rows"
    ;;
  "SELECT COUNT(*) FROM SCHEMA_MIGRATIONS"*)
    wc -l <"$scratch/versions" | tr -d ' '
    continue
    ;;
  "SELECT COUNT(*) FROM NOTES WHERE"*"IS NULL"*)
    read -r -a words <<<"$norm"
    column="${words[5]}"
    column="${column,,}"
    column="${column//[!a-z0-9_]/}"
    grep -Fxq "$column" "$scratch/columns" || fail "column \"$column\" does not exist"
    echo 0
    continue
    ;;
  "SELECT COUNT(*) FROM NOTES"*)
    cat "$scratch/rows"
    continue
    ;;
  *)
    fail "the stub does not know this statement: $norm"
    ;;
  esac
done

# The whole stream succeeded: commit it.
mv "$scratch/versions" "$versions"
mv "$scratch/log" "$log"
mv "$scratch/columns" "$columns"
mv "$scratch/rows" "$rows"
