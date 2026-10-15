---
id: TASK-NTVL-001
title: "Enforce the 200-character note title limit at note creation"
task_type: feature
parent_review: TASK-REV-D267
feature_id: FEAT-D267
wave: 1
implementation_mode: direct
complexity: 3
dependencies: []
---

Add title-length validation to the note-creation handler in the Go API so a request whose title is longer than 200 characters is refused with 400 Bad Request and a clear message before anything reaches persistence, with the Go handler tests covering the boundary.

## The words of the request this task serves

> "The notes API should refuse to create a note whose title is longer than 200 characters, answering 400 Bad Request with a clear message. Change only the Go API and its Go tests."

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/validation.go`
- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/handler_test.go`

## Acceptance Criteria

- A note-creation request whose title is 201 characters long answers 400 Bad Request, and the response carries a clear message naming the title field and the 200-character limit.
- A note-creation request whose title is exactly 200 characters long succeeds (the limit is inclusive), and a short title continues to succeed — existing create-success tests in `apps/api/internal/notes/handler_test.go` stay green.
- A title one character over the limit is refused with the same 400 and message, at the character-count boundary (Unicode code points, not bytes, per ASSUM-002).
- A refused over-long title persists nothing: the handler returns before `store.Create` is called, proven by the fake-store call counter (`store.calls != 0` fails the test), so no over-long note is stored.
- The existing 81-character and 1025-character rejection cases in `TestRejectsInvalidJSONBeforePersistence` (`apps/api/internal/notes/handler_test.go:61-83`) keep answering 400 and keep never reaching persistence — the new rule must not regress them.
- The rule applies only to note creation, not to updates (ASSUM-003); no other route changes behaviour.
- New table-driven cases in the handler test cover both sides of the boundary (200 accepted, 201 refused with the message).
- All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- Look at `apps/api/internal/notes/validation.go` first — it already exists (repository inventory) and is the natural home for the title-length check; change only what is missing there. Then wire the result into the `POST /api/notes` handler in `apps/api/internal/notes/handler.go` (the `decodeLabel` error path at lines 59-72).
- The handler's `decodeLabel` currently collapses every decode/validation failure into the message "Invalid request"; a distinct validation error type is the clean way to emit a message naming the title field and the 200-character limit, without disturbing the existing invalid-JSON message the handler tests pin.
- Count characters as Unicode code points (e.g. `utf8.RuneCountInString`), never bytes (ASSUM-002).
- Run the existing Go tests in `apps/api/internal/notes/` before and after the change — do not re-create existing coverage; extend the `TestRejectsInvalidJSONBeforePersistence`-style table with the boundary cases.
- Scope guard: change only the Go API and its Go tests. Do not touch `apps/web`, `contract/openapi.yaml`, migrations, or e2e specs; do not invent update-path validation, configurability, or a structured JSON error envelope.

## Deferred Planning Decisions

This task changes no decision beyond the plan-level defaults recorded in IMPLEMENTATION-GUIDE.md.