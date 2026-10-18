---
complexity: 5
dependencies: []
feature_id: FEAT-7FC3
id: TASK-7FC3-001
implementation_mode: task-work
parent_review: TASK-REV-7FC3
status: design_approved
task_type: feature
title: Add interactive done-toggle control to the notes list
wave: 1
---

Add an interactive tick/untick completion control to each note row in the web app's notes list that saves the done state through the existing PATCH /api/notes/{id} endpoint and shows the saved state after a reload.

## The words of the request this task serves

List lines: 1, 2, 3, 4

"Let the user tick a note as done or not done from the web app's notes list, saving it through PATCH /api/notes/{id} and showing the saved state after a reload"

This task delivers the interactive control itself: the checkbox/tick that the user clicks to flip a note's done state, the optimistic update that reflects the tick immediately, the call to `updateNoteDone` (which already exists in `apps/web/src/api.ts`) to persist the change through PATCH /api/notes/{id}, and the reconciliation to the server-confirmed state. It also ensures the control renders the persisted done state on load and after reload, which the existing done-flag rendering (App.tsx:19-25) already supports once the flag arrives from the server.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/App.tsx`

## Acceptance Criteria

- [ ] Each note row in the notes list displays an interactive completion control (checkbox or tick button) that the user can click to toggle the note's done state
- [ ] Clicking the control on a not-done note marks it as done: the control appears ticked and the note's title is visually distinguished as done (struck-through/dimmed, using the existing `doneRowStyle` and `doneTitleStyle` from App.tsx)
- [ ] Clicking the control on a done note returns it to not done: the control appears unticked and the title is no longer visually distinguished
- [ ] The control is operable directly from the notes list without opening the note detail view
- [ ] Toggling the control calls `updateNoteDone` (from `apps/web/src/api.ts`) with the note's id and the new done state, which sends PATCH /api/notes/{id} with the done flag
- [ ] The control updates optimistically while the PATCH is in flight and reconciles to the server-confirmed state on response (per ASSUM-003)
- [ ] If the PATCH fails, the control reverts to its previous state and an error is shown
- [ ] After a page reload, the control renders the persisted done state from the server's GET /api/notes response (the done flag is already returned by the read path per ASSUM-004)
- [ ] The completion control accepts only the two binary states (done/not done) with no intermediate status
- [ ] All modified files pass project-configured lint/format checks with zero errors

## Implementation Notes

- The `updateNoteDone` function already exists in `apps/web/src/api.ts` and is tested in `apps/web/src/api.test.ts:73`. It sends PATCH /api/notes/{id} with `{done: true/false}` and returns the updated note. Use this function directly — do not create a new API call.
- The done flag rendering styles (`doneRowStyle`, `doneTitleStyle`, `doneFlagStyle`) already exist in `apps/web/src/App.tsx:19-25`. Reuse them for the visual distinction when a note is done.
- The notes list state is managed via `setNotes` in App.tsx. When the user toggles a note, optimistically update the note in the `notes` array, call `updateNoteDone`, and reconcile with the server response.
- The existing `done-flag.test.tsx` tests the rendering of done/not-done states but not the interactive toggle. This task adds the interactive control; the unit tests for it are in TASK-7FC3-002.
- The backend PATCH endpoint is fully implemented (handler.go:105, store.UpdateDone at patch_done_test.go:356). No backend changes are needed.
- The contract at `contract/openapi.yaml:64` declares the PATCH /api/notes/{id} operation. The generated types in `apps/web/src/api-types.ts` already include it.
- Handle the `SignInRequiredError` case: if the PATCH returns 401, sign the user out (consistent with the existing `submit` function pattern at App.tsx:78).
- The control should be accessible: use a proper `<input type="checkbox">` or a button with `role="checkbox"` and `aria-checked` so screen readers announce the state.