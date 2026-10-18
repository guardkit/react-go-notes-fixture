---
id: TASK-7FC3-002
title: Add unit tests for the done-toggle control
task_type: testing
parent_review: TASK-REV-7FC3
feature_id: FEAT-7FC3
wave: 2
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-7FC3-001
---

Add unit tests for the interactive done-toggle control in the notes list, covering both directions of the toggle, the optimistic update, the server reconciliation, and the error revert path.

## The words of the request this task serves

List lines: 1, 2, 3, 4

"Let the user tick a note as done or not done from the web app's notes list, saving it through PATCH /api/notes/{id} and showing the saved state after a reload"

This task tests the interactive control delivered by TASK-7FC3-001: ticking a not-done note marks it done, unticking a done note returns it to not done, the control is operable without opening the detail view, the PATCH is called with the correct payload, the optimistic update reconciles to the server response, and a failed PATCH reverts the control.

## Files to Create

- `apps/web/src/done-toggle.test.tsx`

## Files to Modify

- _none_

## Acceptance Criteria

- [ ] A test verifies that clicking the completion control on a not-done note marks it as done: the control appears ticked and the title is visually distinguished
- [ ] A test verifies that clicking the completion control on a done note returns it to not done: the control appears unticked and the title is no longer visually distinguished
- [ ] A test verifies that the completion control is operable without opening the note detail view (no navigation or modal appears)
- [ ] A test verifies that toggling the control calls `updateNoteDone` with the note's id and the new done state
- [ ] A test verifies that the control updates optimistically while the PATCH is in flight
- [ ] A test verifies that the control reconciles to the server-confirmed state when the PATCH response arrives
- [ ] A test verifies that a failed PATCH reverts the control to its previous state and shows an error
- [ ] A test verifies that the completion control accepts only the two binary states (done/not done)
- [ ] All tests pass with `npm test` in `apps/web/`

## Implementation Notes

- Follow the patterns in `apps/web/src/App.test.tsx` and `apps/web/src/done-flag.test.tsx` for test setup, note creation helpers, and rendering.
- Mock `updateNoteDone` from `apps/web/src/api.ts` to control the PATCH response and test optimistic update / reconciliation / error revert paths.
- The existing `done-flag.test.tsx` covers the static rendering of done/not-done rows. This new file covers the interactive toggle behaviour.
- Use the same `collectionOf`, `viewNotes`, `noteRows` helpers from App.test.tsx where applicable.
- Test both directions: not-done → done and done → not done.