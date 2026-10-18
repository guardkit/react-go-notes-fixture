---
complexity: 6
dependencies:
- TASK-4384-001
feature_id: FEAT-4384
id: TASK-4384-002
implementation_mode: task-work
parent_review: TASK-REV-4384
status: design_approved
task_type: feature
title: Add the PATCH /api/notes/{id} handler and route
wave: 2
---

Add the PATCH /api/notes/{id} route and handler that lets the signed-in owner set a note's done flag to true or false, returns the updated note as 200, and answers an indistinguishable 404 for a note that does not exist or belongs to someone else.

## The words of the request this task serves

List lines: 1, 2, 3, 4, 5, 6

This task delivers the surface the person uses: "Add PATCH /api/notes/{id} so the signed-in owner can set a note's done flag to true or false; it returns the updated note, and answers 404 for a note that does not exist or belongs to someone else." It owns every scenario's observable behaviour at the PATCH endpoint.

## Files to Create

- `apps/api/internal/notes/patch_done_test.go`

## Files to Modify

- `apps/api/internal/notes/handler.go`

## Acceptance Criteria

- [ ] A PATCH /api/notes/{id} route is registered and requires the bearer token (401 with the existing challenge when the token is missing or refused).
- [ ] "Owner marks a note done and receives the updated note": PATCH /api/notes/note-1 with body `{"done":true}` for a note that is not done answers 200 with a body containing the note id, the label "Buy milk", and `done` set to true.
- [ ] "Owner marks a note not done and receives the updated note": PATCH /api/notes/note-1 with body `{"done":false}` for a note that is done answers 200 with a body containing the note id, the label "Buy milk", and `done` set to false.
- [ ] "Only the done flag is mutated by the update": the response body carries the note's existing label unchanged; the handler never accepts or writes a label on this path.
- [ ] "Updating a note that does not exist answers 404": PATCH /api/notes/missing-1 with body `{"done":true}` answers 404, and the body does not indicate whether the note exists or belongs to another user.
- [ ] "Updating another user's note answers 404": PATCH /api/notes/foreign-1 (a note belonging to someone else) with body `{"done":true}` answers 404, byte-for-byte indistinguishable from the missing-note 404.
- [ ] "Setting done to its current value succeeds idempotently": PATCH /api/notes/note-1 with `{"done":true}` when the note is already done answers 200 with `done` true.
- [ ] "The toggled done flag persists for a later read": after a successful PATCH setting done true, a subsequent GET /api/notes includes that note with done true (the handler uses the store update from TASK-4384-001 and the existing list read, so the toggled value is what the row holds).
- [ ] The success response status is 200 and the body is the full updated note (id, label, created_at, done) — ASSUM-001/ASSUM-002.
- [ ] The handler calls the store's owner-scoped update from TASK-4384-001 exactly once per request and maps its not-found signal to the 404.
- [ ] The existing `NewNote` write schema keeps only the label property (no `done`), as `apps/api/internal/notes/contract_done_test.go:72` pins; this task does not change that test.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- Reuse the existing auth path: the handler resolves the owner from the bearer token's `sub` via the `Authenticator` interface (`apps/api/internal/notes/handler.go:36-39`), exactly as POST and GET do.
- The request body for this endpoint carries only `done` (a boolean); reject a body that is not a decodable JSON object with `done` as a boolean as 400, reusing the existing invalid-request message shape.
- The 404 must be produced from the store's not-found signal so the missing-note and foreign-note cases cannot diverge; do not branch on existence separately.
- The id is a string path parameter (ASSUM-003); the existing tests already use string ids such as "1" and "7".
- Write the handler tests in the new `patch_done_test.go` using the existing `request(...)` and `fakeStore` helpers in `apps/api/internal/notes/done_test.go`; extend `fakeStore` to record the update call (owner, id, done) so the owner-scoping and single-call properties are checkable.
- Keep the read slice read-only and the write path label-only: do not add `done` to `NewNote` or to the list envelope — `TestContractKeepsTheDoneFlagOffTheWritePathAndTheEnvelope` (`apps/api/internal/notes/contract_done_test.go:72`) will fail if you do.