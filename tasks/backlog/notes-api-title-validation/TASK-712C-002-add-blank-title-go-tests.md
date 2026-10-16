---
id: TASK-712C-002
title: Add Go tests covering every blank-title scenario at the create endpoint
task_type: testing
parent_review: TASK-REV-712C
feature_id: FEAT-712C
wave: 2
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-712C-001
scaffolding_kind: tests
---

Add Go tests that exercise the note-creation endpoint for every blank-title case in the approved specification, asserting the 400 status, the clear message naming the title field, and that nothing is persisted, plus the unchanged success path for well-formed titles.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/handler_test.go`
- `apps/api/internal/notes/contract_test.go`

## Acceptance Criteria

- [ ] A Go test asserts that a note-creation request with an absent title answers HTTP 400 Bad Request, carries a clear message identifying the title as the invalid field, and persists nothing.
- [ ] A Go test asserts that a note-creation request with an empty-string title answers HTTP 400 Bad Request, carries a clear message identifying the title as the invalid field, and persists nothing.
- [ ] A Go test asserts that a note-creation request with a whitespace-only title (spaces, tabs, newlines) answers HTTP 400 Bad Request, carries a clear message identifying the title as the invalid field, and persists nothing.
- [ ] A Go test asserts that a note-creation request with a title that is blank only after trimming answers HTTP 400 Bad Request, carries a clear message identifying the title as the invalid field, and persists nothing.
- [ ] A Go test asserts that a note-creation request with a single-character title answers the existing success status and persists a note.
- [ ] A Go test asserts that a note-creation request with a well-formed title answers the existing success status and persists a note, unchanged from before.
- [ ] The tests use the existing `fakeStore` and `request` helpers so persistence is verified by the store's call count, matching the existing test style.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- This task depends on TASK-712C-001: the handler behaviour it asserts must already be in place. Run the existing Go tests first (`go test ./...` under `apps/api/`) to confirm the baseline before adding cases.
- Extend the existing table-driven tests in `apps/api/internal/notes/handler_test.go` and `apps/api/internal/notes/contract_test.go` rather than creating new files, keeping the `fakeStore`/`request`/`Routes` helpers already in use.
- The existing `TestRejectsInvalidJSONBeforePersistence` already covers some malformed bodies; add the blank-title cases where they belong (a blank title is a valid JSON body that must be refused by the validation rule, not by the decoder).
- Do not assert that any directory, table, or config is empty or absent as a boundary check — later work in this feature fills the create path, and such an assertion would turn red when it succeeds.