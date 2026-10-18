---
id: TASK-9018-001
title: Add the done column migration
task_type: feature
parent_review: TASK-REV-9018
feature_id: FEAT-9018
wave: 1
implementation_mode: direct
complexity: 2
dependencies: []
---

Add a new migration that adds a `done` column to the `notes` table, `NOT NULL DEFAULT false`, so every row — including rows created before the migration — reads back as `false`.

## The words of the request this task serves

List lines: 1

"a new migration adds a done column that is false by default" — this task is the migration itself.

## Files to Create

- `apps/api/migrations/0003_add_done.sql`

## Files to Modify

- _none_

## Acceptance Criteria

- [ ] `apps/api/migrations/0003_add_done.sql` exists and is the next migration after `0002_add_created_at.sql`.
- [ ] The migration runs `ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT false;` (mirroring the `0002` pattern of `NOT NULL DEFAULT`), so pre-existing rows get `false` rather than `NULL`.
- [ ] The migration is idempotent-safe under the connected check's three-step apply (upgrade from `0001` with seeded rows, fresh install, idempotent re-run) that `scripts/check-feature.sh` already performs.
- [ ] `scripts/check-feature.sh`'s `schema_migrations` count assertions still pass with the new migration present.
- [ ] All modified files pass project-configured lint/format checks with zero errors: `gofmt` and `go vet` (Go), `npm run lint` (ESLint) and Prettier (web), and `npm run contract:check` (contract drift) — the aggregate `bash scripts/check.sh` must exit 0.

## Implementation Notes

- Follow the shape of `apps/api/migrations/0002_add_created_at.sql` (`ALTER TABLE notes ADD COLUMN … NOT NULL DEFAULT …;`). The API never migrates on start (`apps/api/internal/notes/store.go:16`); migrations are applied explicitly by `scripts/migrate.sh`, so no Go-side migration code is needed here.
- Do not add an index on `done`: the request names no filtering or sorting by the flag (ASSUM-004 keeps this read-only), and an unused index is out of scope.
- The migration is verified by the state it produces in `GET /api/notes` (the scenarios), not by inspecting the schema — that read-side proof lives in TASK-9018-002.
- Run the project's own checks before finishing: `gofmt -l .` and `go vet ./...` in `apps/api`, `npm run lint` and Prettier for the web workspace, and `npm run contract:check` at the repo root (the aggregate `bash scripts/check.sh`).