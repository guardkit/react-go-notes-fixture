---
complexity: 5
dependencies: []
feature_id: FEAT-4C9B
id: TASK-4C9B-001
implementation_mode: task-work
parent_review: TASK-REV-4C9B
status: design_approved
task_type: feature
title: Add the done column migration and expose done through the store and API response
wave: 1
---

Add `apps/api/migrations/0003_add_done.sql` (a `done` boolean column with a false default), expose `done` through the persistence mapping and the `GET /api/notes` response, and pin the delivered surface with the listing's done-flag behaviour.

## The words of the request this task serves

List lines: 1, 2, 5

> "a new migration adds a done column that is false by default, and GET /api/notes returns each note's done flag"

This task is the vertical slice: the migration introduces the column with its false default (backfilling pre-existing rows to false), the store reads and writes `done`, and the `GET /api/notes` listing hands each note's `done` flag to callers alongside its existing `id`, `label`, and `createdAt` fields. It also owns the existing listing surface's proof (apps/api/internal/notes/handler.go:46): the route exists and is changed by this task to return the new field, so its existing behaviour carries one proof line here rather than a separate task.

## Files to Create

- `apps/api/migrations/0003_add_done.sql`

## Files to Modify

- `apps/api/internal/notes/store.go`
- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/handler_test.go`
- `apps/api/internal/notes/contract_test.go`

## Acceptance Criteria

- `apps/api/migrations/0003_add_done.sql` adds a `done` boolean column to `notes` with a false default, following the `0002_add_created_at.sql` pattern (`ALTER TABLE notes ADD COLUMN done BOOLEAN NOT NULL DEFAULT false;`), so pre-existing rows are backfilled to false by the column default.
- The `Note` struct in `apps/api/internal/notes/store.go` carries a `Done bool` field; `List` selects `done` and scans it, and `Create` persists `done` as false (the slice is read-only — no write path sets it true).
- `GET /api/notes` returns each note's `done` flag in the JSON body alongside `id`, `label`, and `createdAt`; no existing field is renamed or removed.
- A note created before the done flag existed is returned by `GET /api/notes` with `done: false` (the migration's default is observed through the listing, per ASSUM-004 — no storage probe).
- A note whose completion state is stored as done is returned by `GET /api/notes` with `done: true` (the done state is seeded as stored state; no caller-facing write path exists in this slice, per ASSUM-003).
- Every note in a listing carries a `done` flag that is never omitted and never null, including a mixed listing of several notes whose completion states are a mix of done and not-done, where each note carries its own flag matching its completion state.
- The existing authenticated listing route at `apps/api/internal/notes/handler.go:46` continues to serve the signed-in owner's notes newest-first after this change (proof: the existing listing assertions in `apps/api/internal/notes/handler_test.go` and the journey at `tests/e2e/specs/journey.spec.ts:163` keep passing with `done` added).
- One check covers "each note": it finds the members itself when it runs, from `apps/api/internal/notes/handler.go`, and fails if it finds none.
- The existing contract tests in `apps/api/internal/notes/contract_test.go` (including `TestContractRefusesForbiddenResponses`) keep passing against the new response shape — the response gains `done` and loses nothing.
- All modified files pass project-configured lint/format checks with zero errors (`gofmt`, `go vet`, Staticcheck).

## Implementation Notes

- Follow the existing migration pattern: `0001` creates `notes` with an owner, `0002` adds `created_at` with a default (`apps/api/migrations/0002_add_created_at.sql:3`); `0003` adds `done` the same way. Migrations are applied explicitly by `scripts/migrate.sh`; `scripts/check-feature.sh` exercises an upgrade from `0001` with seeded rows, a fresh install, and an idempotent re-run — the new migration must survive all three.
- The `List` query in `apps/api/internal/notes/store.go:69` selects `id, label, created_at`; add `done` to the select list and the scan. The `Create` path persists a new note; `done` is false there (read-only slice, ASSUM-003).
- The response shape is pinned in `apps/api/internal/notes/handler_test.go` (the wire-shape test at line 188 asserts the exact JSON body); extend the expected body to include `"done":false` / `"done":true` per case.
- For the done-note and mixed-listing cases, seed the stored state through the test store fakes (`fakeStore`, `storingStore`) — there is no API write path that sets `done` true in this slice.
- Run the existing tests first (`go test ./... -race` in `apps/api`) before writing new ones; the contract test's `contractRouter` (line 23) validates live responses against `contract/openapi.yaml`, so the contract edit in TASK-4C9B-002 must land before the contract check accepts `done` — keep the contract edit in that task, not here.
- To maintain verifiability, interpret "search the whole repository for further members of the each-note set" as ensuring test coverage for all relevant files (handlers, stores, and consumers shaping the listing response), not as a manual audit step: add the found members directly into the automated checks or fixtures. The set probe listed 24 of 48 matching files; 24 matching files were not listed, so confirm no additional response-shaping files were missed during this automated check-coverage pass.
- Keep the env-var surface untouched: this task reads no new environment variables; existing config (`API_LISTEN_ADDR`, `NOTES_RUN_ID`) stays as is.