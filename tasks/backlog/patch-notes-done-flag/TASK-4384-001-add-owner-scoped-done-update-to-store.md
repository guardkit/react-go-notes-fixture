---
id: TASK-4384-001
title: Add the owner-scoped done-flag update to the notes store
task_type: feature
parent_review: TASK-REV-4384
feature_id: FEAT-4384
wave: 1
implementation_mode: task-work
complexity: 4
dependencies: []
---

Add an owner-scoped update method to the notes store that sets a note's done flag and returns the updated note, answering a not-found signal when no row matches both the id and the signed-in owner.

## The words of the request this task serves

List lines: 2

This task serves "the signed-in owner can set a note's done flag to true or false" at the storage layer: the update must be scoped to the signed-in owner (the token's `sub`, exactly as `Create` and `List` already are — `apps/api/internal/notes/handler.go:30-34`), so a note belonging to someone else is indistinguishable from a note that does not exist.

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/store.go`
- `apps/api/internal/notes/handler.go`

## Acceptance Criteria

- [ ] The `Store` interface gains an update method that takes the owner, the note id, and the done value, and returns the updated `Note` (or a not-found signal).
- [ ] The PostgreSQL adapter issues an UPDATE on the notes table scoped to both the id and the owner, so a foreign note and a missing note both yield the not-found signal — never a different answer.
- [ ] The update sets only the done column; the label and created_at columns are never written by this path.
- [ ] The returned `Note` carries the id, label, created_at, and the new done value read back from the row.
- [ ] The existing `Create` and `List` methods keep their current signatures and behaviour (the read slice stays read-only and the write path stays label-only, as `apps/api/internal/notes/contract_done_test.go:72` pins).
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- The `Note` struct already carries `Done bool` (`apps/api/internal/notes/done_test.go:32`); reuse it — do not add a new type.
- The done column is `NOT NULL DEFAULT false` from `apps/api/migrations/0003_add_done.sql`; no new migration is needed.
- Keep the owner scoping in the SQL WHERE clause (id AND owner), mirroring how `List` scopes by owner, so the indistinguishable-404 property is enforced at the database rather than in the handler.
- The handler's `Store` interface is the consumer boundary; extend it here so the handler task (TASK-4384-002) can call the new method without touching storage internals.