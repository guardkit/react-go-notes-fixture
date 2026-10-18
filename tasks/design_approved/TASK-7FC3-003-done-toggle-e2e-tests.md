---
complexity: 4
dependencies:
- TASK-7FC3-001
feature_id: FEAT-7FC3
id: TASK-7FC3-003
implementation_mode: task-work
parent_review: TASK-REV-7FC3
status: design_approved
task_type: testing
title: Add end-to-end tests for the done-toggle flow
wave: 2
---

Add end-to-end tests for the done-toggle flow covering the full user journey: ticking a note, verifying the saved state after reload, and the reverse direction, using the existing Playwright infrastructure.

## The words of the request this task serves

List lines: 1, 2, 3, 4

"Let the user tick a note as done or not done from the web app's notes list, saving it through PATCH /api/notes/{id} and showing the saved state after a reload"

This task tests the complete user journey at the browser surface: ticking a note in the list, confirming the control reflects the saved state, reloading the page, and confirming the ticked state persists from the server. It also covers the untick direction and the binary-state constraint.

## Files to Create

- `tests/e2e/specs/notes-done-toggle.spec.ts`

## Files to Modify

- _none_

## Acceptance Criteria

- [ ] A test ticks a not-done note in the list and verifies the control appears ticked and the title is visually distinguished as done
- [ ] A test unticks a done note in the list and verifies the control appears unticked and the title is no longer visually distinguished
- [ ] A test ticks a note, reloads the page, and verifies the control still appears ticked (the saved state is read back from the server)
- [ ] A test unticks a note, reloads the page, and verifies the control still appears unticked
- [ ] A test verifies the completion control is operable without opening the note detail view
- [ ] A test verifies the completion control accepts only the two binary states
- [ ] All tests pass with `npx playwright test` in `tests/e2e/`

## Implementation Notes

- Follow the patterns in `tests/e2e/specs/notes-done.spec.ts` and `tests/e2e/specs/notes-done-visibility.spec.ts` for test structure, sign-in flow, and note creation.
- Use the helpers from `tests/e2e/specs/helpers.ts`: `createNote` for creating notes through the app's own form, and `serveNotesWithDoneFlags` for setting up done-state fixtures when needed.
- The `showListAgain` helper from `notes-done-visibility.spec.ts:180` handles the reload-and-re-sign-in flow (tokens live in memory, so a reload requires re-authentication through Keycloak's still-living session).
- For the reload tests, arm the `listed` promise before calling `showListAgain` so the GET /api/notes response can be captured and verified.
- The PATCH /api/notes/{id} endpoint is live in the preview stack (compose.yaml). The tests exercise the real backend, not mocks.
- Use `page.getByRole("checkbox")` or the appropriate test-id to locate the completion control.
- Assert the done state via the "Done" text cue and the struck-through title style, consistent with the existing visibility tests.