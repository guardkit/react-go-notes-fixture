---
id: TASK-F552-002
title: "Wire PATCH /api/notes/{id} through Routes"
task_type: feature
parent_review: TASK-REV-F552
feature_id: FEAT-F552
wave: 2
implementation_mode: task-work
complexity: 6
dependencies:
  - TASK-F552-001
---

Wire `PATCH /api/notes/{id}` in `Routes` so the signed-in owner sets a note's done flag and gets the updated note back, answering 404 for a note that does not exist or belongs to someone else and 401 before any mutation when unauthenticated. This is the delivered surface the person uses and the owner of every approved scenario.

## The words of the request this task serves

List lines: 1, 3, 4, 5, 6, 7

The request asks the API to "Add PATCH /api/notes/{id} so the signed-in owner can set a note's done flag to true or false; it returns the updated note, and answers 404 for a note that does not exist or belongs to someone else". This task creates that route in `Routes` (`apps/api/internal/notes/handler.go:30`) and states the observable behaviour at it: the accepted `done` body of `true` or `false` is applied through the storage write from TASK-F552-001 and the updated note is returned; missing or foreign ids are answered 404; an unauthenticated request is refused 401 before any mutation. It consumes the `SetDone` method produced by TASK-F552-001 and uses its returned note as the response body.

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/validation.go`
- `apps/api/internal/notes/handler_test.go`

## Acceptance Criteria

- `Routes` registers `PATCH /api/notes/{id}` and wraps it in the existing `signedIn(authn, ...)` helper, exactly like `GET /api/notes`, so the handler receives the owner's subject and cannot run unauthenticated.
- The route parses the `{id}` path segment and a JSON request body, and only a `done` value that is the JSON boolean `true` or `false` is accepted; a non-boolean or missing `done` is rejected with 400 and no mutation happens (validation lives in `validation.go`).
- On an accepted request the route calls the storage write added by TASK-F552-001 for the caller's owner and the requested id, and returns `200` with the updated note as JSON — carrying the note's `id` and its new `done` state and every other existing field, with all fields other than `done` unchanged.
- The two toggle directions both work at the surface: `done` set to `true` returns a note with `done` true, and `done` set to `false` returns a note with `done` false.
- Re-sending the same value is idempotent: two `PATCH` calls with `done: true` both return the note with `done` true and neither corrupts it.
- A note id that does not exist is answered `404`; the note collection is not created or modified by the failed call.
- A note belonging to another user is answered `404` — never `403` — so existence is not leaked, and the foreign note's `done` flag is left untouched.
- An unauthenticated `PATCH` is answered `401` (consistent with the existing `GET /api/notes` 401 `SignInRequired`, ASSUM-001) and the note's `done` flag remains unchanged.
- Handler tests in `apps/api/internal/notes/handler_test.go` cover each behaviour above against the real `Routes` handler, following the `request(Routes(store, fakeAuth{}, "run-1"), ...)` shape already used by `listedNotes`, using a fake store whose `SetDone` records the owner/id/done it was called with.
- The existing listing and create handler tests stay green.
- All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- This task depends on TASK-F552-001 and calls its `SetDone`; the acceptance criteria above use that write, so the storage method lands underneath the route rather than beside it.
- Map the storage not-found sentinel to `404` here; keep the two negative cases (missing vs foreign) on one code path since both arrive as not-found from the owner-scoped write.
- Use the existing `writeJSON`/`writeError` helpers already in `handler.go`; do not introduce a new response encoder.
- Do not touch the OpenAPI contract or `api-types.ts` — that is TASK-F552-003.
- `go test ./...` from `apps/api/` is the check for this task.