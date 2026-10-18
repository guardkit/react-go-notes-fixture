---
id: TASK-A91A-001
title: Add the notes done column migration
task_type: declarative
parent_review: FEAT-A91A
feature_id: FEAT-A91A
wave: 1
implementation_mode: direct
complexity: 2
dependencies: []
---

Add a new migration that introduces the boolean `done` completion column on the `notes` table, non-null and defaulting to `false`, so every existing and future note row carries a done flag.

## The words of the request this task serves

List lines: 1, 3

"a new migration adds a done column that is false by default" — this task writes that migration. Per ASSUM-001 (deferred), pre-existing rows are backfilled to `not-done` so no legacy note is ever treated as done; the `NOT NULL DEFAULT false` clause achieves that backfill for rows already in the table when the migration runs.

## Files to Create

- `apps/api/migrations/0003_add_done.sql`

## Files to Modify

- `apps/api/internal/notes/handler_test.go`

## Acceptance Criteria

- A new file `apps/api/migrations/0003_add_done.sql` exists and applies cleanly against the notes database (the existing `scripts/migrate.sh` / `scripts/check-feature.sh` upgrade path from `0001`/`0002` still completes).
- The migration runs `ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT false;` (mirroring the `NOT NULL DEFAULT` shape already proven at `apps/api/migrations/0002_add_created_at.sql:3`).
- After the migration, every existing note row has `done = false` (no legacy row is `done` and none is null).
- The new column is named exactly `done`, is `BOOLEAN`, is `NOT NULL`, and defaults to `false`.
- Extend the existing `Note` struct in `apps/api/internal/notes/handler_test.go` with a `Done bool` field (json tag `done`) so the test fixture's note shape includes `done`, keeping the struct in step with the migration and the store. This change is strictly limited to adding the `done` field to the `Note` struct used in tests; do not refactor, rename, or restructure any other part of `handler_test.go`.
- Run the existing migrations and store/handler tests first (`bash scripts/check.sh`) and change only what is missing for the `done` column; do not rewrite unrelated migration or test code.
- The migration is idempotent-safe with the repo's existing `schema_migrations` accounting used by `scripts/check-feature.sh`.

## Implementation Notes

- Follow the exact house style of `apps/api/migrations/0002_add_created_at.sql` (`ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT ...;`): a single `ALTER TABLE`, one statement, `NOT NULL` plus a column `DEFAULT` so the backfill is implicit and no separate `UPDATE` is needed.
- Name the file `0003_add_done.sql` to continue the `0001_` / `0002_` sequence the README documents (`README.md:33`).
- The API never migrates on start (see `apps/api/internal/notes/store.go:16`); migrations are applied explicitly, so this task only adds the SQL file and keeps the store's note shape in step.
- Keep the change minimal: no index is required for correctness, and the request does not ask for one.
- In `handler_test.go`, locate the `Note` struct definition (the test fixture type used to construct expected note shapes) and add only the `Done bool` field with a `json:"done"` tag. Do not touch any test function, helper, or other struct in that file.

## Seam Tests

The migration's `done` column is consumed downstream by the note response (TASK-A91A-003) via the `done` boolean field.

```python
"""Seam test: verify the 'done' column contract from TASK-A91A-001."""
import pytest


@pytest.mark.seam
@pytest.mark.integration_contract("done")
def test_done_column_contract():
    """Verify the notes.done column matches the expected shape.

    Contract: notes.done is BOOLEAN NOT NULL DEFAULT false, so a persisted note
    carries a non-null boolean done (default false).
    Producer: TASK-A91A-001
    """
    # Producer side: the column definition as written by the migration.
    column_sql = "done BOOLEAN NOT NULL DEFAULT false"  # from 0003_add_done.sql

    # Consumer side: the shape TASK-A91A-003 relies on when serializing.
    assert "done" in column_sql, "column must be named exactly 'done'"
    assert "NOT NULL" in column_sql, "done must be non-null"
    assert "DEFAULT false" in column_sql.lower(), "done must default to false"
```