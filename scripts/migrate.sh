#!/usr/bin/env bash
# Applies pending migrations from apps/api/migrations in file-name order,
# inside the running postgres container, recording each in schema_migrations.
# An explicit step: the API never migrates on start.
#
#   scripts/migrate.sh [--database NAME] [--to VERSION_PREFIX]
#
# Each file and its schema_migrations row commit in one transaction; a failing
# file stops the run with a non-zero exit and leaves that file unapplied.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib.sh
require_command docker
require_project

database=notes
to=""
while (($#)); do
  case "$1" in
  --database) database="${2:?--database needs a name}"; shift 2 ;;
  --to) to="${2:?--to needs a version prefix}"; shift 2 ;;
  *) die "unknown argument: $1" ;;
  esac
done
[[ "$database" =~ ^[a-z][a-z0-9_]{0,62}$ ]] || die "invalid database name: $database"

psql_app() {
  docker compose exec -T postgres psql -X -q -v ON_ERROR_STOP=1 -U notes_app -d "$database" "$@"
}

shopt -s nullglob
files=(apps/api/migrations/[0-9][0-9][0-9][0-9]_*.sql)
((${#files[@]})) || die "no migration files found"
if [[ -n "$to" ]]; then
  found=0
  for file in "${files[@]}"; do
    [[ "$(basename "$file")" == "$to"* ]] && found=1
  done
  ((found)) || die "--to $to matches no migration file"
fi

psql_app -c "CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())" </dev/null
applied="$(psql_app -At -c "SELECT version FROM schema_migrations ORDER BY version" </dev/null)"

known=""
for file in "${files[@]}"; do
  known+="$(basename "$file" .sql)"$'\n'
done
while IFS= read -r version; do
  [[ -z "$version" ]] && continue
  grep -Fxq "$version" <<<"$known" || die "database $database has applied $version, which has no file here"
done <<<"$applied"

count=0
for file in "${files[@]}"; do
  version="$(basename "$file" .sql)"
  [[ "$version" =~ ^[0-9]{4}_[a-z0-9_]+$ ]] || die "invalid migration name: $version"
  if ! grep -Fxq "$version" <<<"$applied"; then
    echo "migrate: applying $version to $database"
    {
      cat "$file"
      printf "\nINSERT INTO schema_migrations (version) VALUES ('%s');\n" "$version"
    } | psql_app --single-transaction -f -
    count=$((count + 1))
  fi
  if [[ -n "$to" && "$version" == "$to"* ]]; then
    break
  fi
done
echo "migrate: $count applied to $database"
