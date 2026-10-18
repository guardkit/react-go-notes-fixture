---
complexity: 4
dependencies:
- TASK-1291-001
feature_id: FEAT-1291
id: TASK-1291-002
implementation_mode: task-work
parent_review: TASK-REV-1291
status: design_approved
task_type: feature
title: Make the done state perceivable beyond sight
wave: 2
---

Make the done state conveyed in the notes list perceivable beyond sight: a screen-reader user hears each note's completion status in row order, the iconographic indicator adds no duplicate announcement, and done rows stay distinguishable under forced high-contrast rendering.

## The words of the request this task serves

List lines: 1, 2, 18, 19, 20

The part of the request this task exists to satisfy: "with done notes clearly marked as done" — the done state must reach the user through channels other than colour alone: a screen-reader user hears each note's completion status in row order named explicitly as "Done" or "Not done", the iconographic indicator adds no duplicate status announcement (the written label carries the meaning once), when the browser forces high-contrast colours that override the page's colours done rows remain distinguishable by shape or text rather than by colour alone and the done indicator's icon and "Done" label stay visible.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/App.tsx`

## Acceptance Criteria

- [ ] A screen-reader user navigating the notes list hears each note's completion status stated in row order alongside the note title, named explicitly as "Done" or "Not done".
- [ ] When the done indicator is rendered as an icon alongside a written label, the user hears the completion status stated once, from the written label, and does not hear the icon announced as a second, redundant status.
- [ ] When the browser forces high-contrast colours that override the page's colours, done rows remain distinguishable from open rows by shape or text rather than by colour alone, and the done indicator's icon and "Done" label stay visible.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- This task builds on the indicator TASK-1291-001 renders in `apps/web/src/App.tsx`; change only what is missing for non-visual perception, leaving the visual treatment from TASK-1291-001 intact.
- Give the written "Done" / "Not done" label the accessible name for the status and mark the decorative check-mark icon as presentational (e.g. `aria-hidden`) so it is not announced a second time.
- Do not rely on colour alone to separate done from open rows: keep a non-colour differentiator (border, underline, or the persisted text label) so the distinction survives forced high-contrast rendering.
- The accessible announcement wording "Done" / "Not done" and the high-contrast differentiators are low-confidence assumptions (ASSUM-004, ASSUM-005) deferred to the human; implement the stated defaults and record them as deferred in the plan.