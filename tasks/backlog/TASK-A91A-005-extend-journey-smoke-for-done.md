---
id: TASK-A91A-005
title: Extend the notes journey and smoke specs for the done flag
task_type: testing
scaffolding_kind: tests
parent_review: FEAT-A91A
feature_id: FEAT-A91A
wave: 4
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-A91A-003
  - TASK-A91A-004
---

Extend the existing Playwright journey and smoke specs to assert the note list still loads end-to-end now that each note carries a `done` field.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- `tests/e2e/specs/notes-done.spec.ts`

## Files to Modify

- `tests/e2e/specs/journey.spec.ts`

## Acceptance Criteria

- `tests/e2e/specs/journey.spec.ts` still passes after the `done` field is added to the API response: the sign-in → create → reload → see-it-again journey is unchanged and green (do not weaken existing assertions; the request does not ask to change them).
- Add `tests/e2e/specs/notes-done.spec.ts` asserting that, after creating a note, `GET /api/notes` (as observed by the browser flow at `tests/e2e/specs/journey.spec.ts:186`) returns the created note carrying a `done` field with value `false`.
- The new spec covers every note in the fetched list by iterating the collection the journey's own listing produces (not by naming a hard-coded subset), so a note present in the list is checked.
- The specs run inside the existing disposable stack via `bash scripts/check-feature.sh` (the declared feature_check) and pass.
- Test-only change: no production source under `apps/api` or `apps/web/src` is modified by this task.

## Implementation Notes

- This is a test task: it adds coverage for the delivered read path (TASK-A91A-003) and the regenerated type (TASK-A91A-004); it builds on the finished surface, so it runs last.
- The `done` field is not surfaced in the note list UI by this slice (the request is read-path API/contract/types only), so the new spec asserts the field at the `GET /api/notes` boundary the browser observes, following the existing `anonymous.get("/api/notes")` pattern at `tests/e2e/specs/journey.spec.ts:186`.
- Reuse the existing helpers in `tests/e2e/specs/helpers.ts` for fetching `/api/notes` and reading the collection, so the check finds the members itself when it runs rather than naming a fixed subset.
- Run the existing journey and smoke specs first (`bash scripts/check-feature.sh`) and change only what the `done` field needs; do not touch the auth/401 assertions the request does not ask to change.