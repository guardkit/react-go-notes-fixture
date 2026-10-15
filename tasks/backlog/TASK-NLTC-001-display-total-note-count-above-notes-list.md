---
id: TASK-NLTC-001
title: Display total note count above the notes list
task_type: feature
parent_review: TASK-REV-8BFC
feature_id: FEAT-8BFC
wave: 1
implementation_mode: direct
complexity: 3
dependencies: []
---

Add a non-interactive text summary above the notes list in the notes list view that shows the total number of notes, derived from the same collection that populates the list.

## The words of the request this task serves

> Show the total number of notes above the notes list.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/App.tsx`

## Acceptance Criteria

- [ ] The notes list view renders a text summary above the notes list showing the total number of notes in the format "N notes" (e.g. "12 notes" when the list contains 12 notes).
- [ ] The total number of notes displayed equals the number of notes in the collection that populates the list (the count is derived from the same `notes` array the list renders from, so the two can never disagree).
- [ ] The total number of notes is not displayed as part of any individual note in the notes list (the count lives in a summary region above the list, not inline with list items).
- [ ] The count is a non-interactive text summary (not a filter control, link, or sortable affordance).
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- The notes list view lives in `apps/web/src/App.tsx`; the list is populated from the `notes` array returned by `listNotes` in `apps/web/src/api.ts` (the API response shape is `{ notes: [...] }`, see `apps/web/src/api.test.ts:48`). Derive the count as `notes.length` from that same array — do not add a new endpoint or query (ASSUM-003).
- Place the summary element in the markup before the list element so it renders above the list. Keep it plain text (e.g. a `<p>` or `<span>`), not a button or link (ASSUM-004).
- Format the count as "N notes" (e.g. "12 notes").
- Do not render the count inside any individual note item (the negative scenario).
- Run the existing web unit tests (`apps/web/src/*.test.ts`) and the e2e suite after the change to confirm nothing regressed.