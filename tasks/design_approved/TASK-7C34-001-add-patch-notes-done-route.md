---
complexity: 7
dependencies: []
feature_id: FEAT-7C34
id: TASK-7C34-001
implementation_mode: task-work
parent_review: TASK-REV-7C34
status: design_approved
task_type: feature
title: Add the PATCH /api/notes/{id} done-flag route with the owner-scoped update
wave: 1
---

Add `PATCH /api/notes/{id}` to the notes API: the signed-in owner sets a note's `done` flag to true or false, the handler returns the updated note, and a note that does not exist or belongs to another user answers 404.

## The words of the request this task serves

List lines: 1, 2, 3, 4, 5

"Add PATCH /api/notes/{id} so the signed-in owner can set a note's done flag to true or false; it returns the updated note, and answers 404 for a note that does not exist or belongs to someone else"

## Files to Create

- `apps/api/internal/notes/patch_test.go`

## Files to Modify

- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/store.go`
- `apps/api/internal/notes/contract_test.go`
- `apps/api/internal/notes/validation.go`
- `contract/openapi.yaml`
- `apps/web/src/api-types.ts`
- `apps/api/internal/notes/handler_test.go`

## Acceptance Criteria

- The router registers `PATCH /api/notes/{id}` behind the existing `signedIn(authn, ...)` guard, so a missing or invalid bearer token still answers 401 with the existing challenge (the existing 401-without-challenge refusal at `apps/api/internal/notes/contract_test.go:126` stays green — keep it).
- A signed-in owner PATCHing their own note with `{"done": true}` answers 200 and returns the updated note carrying its `id`, `label`, `createdAt` and `done` true; PATCHing with `{"done": false}` returns `done` false (owner re-opens a completed note).
- The updated note's non-mutable fields survive the update: PATCHing only `done` on a note stored with label "Buy groceries" returns the same `label` and `createdAt` (ASSUM-002), and `done` is the sole mutable field a client may set.
- PATCHing a note id the owner does not own — a missing id, or an id owned by another user — answers 404, and both cases return the byte-identical body so the answer reveals nothing about whether the note exists (ASSUM-003); the two 404 cases are checked with an equality assertion on the body.
- Both accepted boolean values are accepted per the Scenario Outline (`false→true`, `true→false`); a `done` value that is absent, null, or not a boolean is refused with 400.
- The `Store` interface gains a single owner-scoped method (`UpdateDone(ctx, owner, id string, done bool) (Note, error)`); the PostgreSQL adapter scopes the UPDATE to `WHERE id = $id AND owner = $owner` so a foreign note cannot be reached, and the handler never trusts a caller-supplied owner or id beyond the token's `sub`.
- `contract/openapi.yaml` documents `PATCH /api/notes/{id}` (200 Note, 404, 401, 400, 500) with bearer security, and `contract_test.go` is extended so the contract validator accepts a valid 200/404 PATCH response and refuses undocumented fields or a non-boolean `done` on this route, mirroring the existing required-boolean rule at `apps/api/internal/notes/contract_test.go:116`.
- `apps/web/src/api-types.ts` is regenerated from the updated OpenAPI document and carries the new PATCH operation.
- `apps/api/internal/notes/handler_test.go`'s existing done-flag listing tests (`TestListingReturnsTheDoneFlagForADoneNote`, `TestEveryNoteInTheListingCarriesItsOwnDoneFlag` at `apps/api/internal/notes/handler_test.go:547`, `TestListingReturnsNotDoneForANoteStoredBeforeTheDoneFlag`) remain green — the listing keeps answering the flag beside `id`, `label`, `createdAt`.
- Existing tests the request does not ask to change stay passing: do not weaken the contract refusals; if a listed test legitimately must change because the request changes that behaviour, change it in this task and name the change here.
- All modified files pass the project-configured lint/format checks with zero errors (run the repo's own check entrypoints from `scripts/check.sh`; do not create a separate lint task).

## Implementation Notes

- Follow the existing route pattern in `apps/api/internal/notes/handler.go` (`Routes`, the `signedIn` wrapper, `writeJSON`/`writeError`); reuse the existing note JSON shape so the returned note matches the listing's fields byte-for-byte in structure.
- Model the fake store's update method on the existing `fakeStore{list: ...}` used by `listedNotes` at `apps/api/internal/notes/handler_test.go:468`; add the PATCH-path tests in a new `patch_test.go` file to keep the large existing handler suite untouched (Block 17: keep what the project's checks pin).
- The `done` column already exists (migration `0003_add_done.sql`), so no new migration is required; read `0003_add_done.sql` first and change only what is missing on the write path.
- The 404 body: use the shared `writeError` helper so the "does not exist" and "belongs to someone else" paths are literally the same call, which is what satisfies ASSUM-003.
- Do not stub a listing test asserting the write path is absent — the write path lands in this same task.
- Lint/format verification is part of this task's Coach Validation commands, not a separate task.