---
id: TASK-F552-001
title: "Add the storage write path that sets a note's done flag"
task_type: feature
parent_review: TASK-REV-F552
feature_id: FEAT-F552
wave: 1
implementation_mode: task-work
complexity: 4
dependencies: []
---

Add the owner-scoped store write that sets a note's `done` flag and answers with the updated note, so the PATCH route in TASK-F552-002 has a storage call to make. Today `Store` only offers `Create` and `List`; `List` reads the done column but nothing writes it, so the flag is read-only at the storage layer.

## The words of the request this task serves

List lines: 2

The request's part this task satisfies is the write behind "the signed-in owner can set a note's done flag to true or false". `store.List` already reads `done` (`apps/api/internal/notes/store.go:83`), but no store method sets it — this task adds that write, scoped to the signed-in owner like every other store call, and returns the updated note for the route to answer with.

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/store.go`
- `apps/api/internal/notes/handler_test.go`

## Acceptance Criteria

- A new `Store` method (for example `SetDone(ctx context.Context, owner, id string, done bool) (Note, error)`) is added to the `Store` interface in `apps/api/internal/notes/handler.go`, beside `Create` and `List`, and is implemented by `PGStore` in `apps/api/internal/notes/store.go`.
- The `PGStore` write is scoped to the signed-in owner: the UPDATE filters on both `id` and `owner_sub`, so it can only change a note belonging to the caller and leaves every other row untouched.
- The write persists the `done` boolean column the 0003 migration adds (`apps/api/migrations/0003_add_done.sql`) and then reads the full updated row back, returning a `Note` carrying `ID`, `Label`, `CreatedAt` (UTC) and the new `Done`, matching how `List` shapes a note (`apps/api/internal/notes/store.go:83`).
- When no owned row matches the id, the method reports "not found" to the caller (a sentinel error the route can map to 404) rather than returning a zero note.
- The boolean is stored and read back faithfully for both `true` and `false`; no string coercion is introduced.
- Existing `TestListReadsTheDoneColumnAndCreateWritesNoFlag` stays green: `Create` still writes no `done` column and `List` still reads `id, label, created_at, done`.
- A fake-store test in `apps/api/internal/notes/handler_test.go` exercises the new method for both `true` and `false`, and for a miss (no owned row) returning not-found.
- All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- Follow the existing `store.go` shape: reuse the `queryCtx`/5-second-timeout idiom used by `List`, use the same `%w`-wrapped error style, and use `strconv.FormatInt` when shaping the returned id so it matches how `List` renders ids.
- Do not add a `done` parameter to `Create` — the migration default backfills new rows with false and existing tests pin "Create writes no flag". The only write this task adds is the flag update.
- Keep the write owner-scoped and id-scoped on the same UPDATE so a foreign id is a no-op that answers not-found; the route in TASK-F552-002 relies on this to produce the 404 for someone else's note.
- `go test ./...` from `apps/api/` is the check for this task.