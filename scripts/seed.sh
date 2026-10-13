#!/usr/bin/env bash
# Seeds synthetic notes owned by a subject no test user has, so the browser
# journey can confirm one owner never sees another's notes. Idempotent.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib.sh
require_command docker
require_project
docker compose exec -T postgres psql -X -q -v ON_ERROR_STOP=1 -U notes_app -d notes <<'SQL'
INSERT INTO notes (owner_sub, label)
SELECT 'seed:synthetic', label
FROM (VALUES ('Seeded synthetic note one'), ('Seeded synthetic note two')) AS seed(label)
WHERE NOT EXISTS (
  SELECT 1 FROM notes n WHERE n.owner_sub = 'seed:synthetic' AND n.label = seed.label
);
SQL
echo "seed: synthetic notes present"
