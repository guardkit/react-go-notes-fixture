---
id: TASK-6F2C-002
title: Go tests for title trimming on create and update
task_type: testing
parent_review: TASK-REV-6F2C
feature_id: FEAT-6F2C
wave: 2
implementation_mode: direct
complexity: 2
dependencies:
  - TASK-6F2C-001
scaffolding_kind: tests
---

Add Go tests in the notes package that assert the create and update API responses return the title with leading and trailing whitespace removed and the interior spacing preserved, and remove no existing behaviour covered by the suite.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/handler_test.go`

## Acceptance Criteria

- A create test asserts that a title with leading spaces returns the trimmed title in the response.
- An update test asserts that a replacement title with trailing spaces returns the trimmed title in the response.
- A create test asserts that a title with leading spaces, trailing spaces, and interior double spaces returns the trimmed title with interior spacing preserved.
- A create test asserts that a title with no surrounding whitespace is returned unchanged.
- A create test asserts that a title composed entirely of spaces returns an empty title.
- A create test table covers each surrounding-space position — leading, trailing, and both — and fails if any position is untested or not wired to the create handler.
- Every existing Go test in `apps/api/internal/notes/` still passes after the change.

## Implementation Notes

- Add cases alongside the existing handler tests in `apps/api/internal/notes/handler_test.go`; follow the `request(...)` and `fakeStore{}` conventions already in that file so the assertions go through the real HTTP handler (the delivered surface), not a helper.
- Cover both the create handler and the update handler; assert the returned body's title field equals the expected trimmed value.
- Do not add or reference any URL, host, port, or endpoint the request did not name; drive the handlers through the in-process router already used by the existing tests.