# Feature Spec Summary: Notes List Done-Flag Visibility

**Stack**: generic (stack undeclared in the request; recorded as generic rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 12 total (2 smoke, 0 regression)
**Assumptions**: 9 total (0 high / 0 medium / 9 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the display-layer change that shows each note's done flag in the web app's notes list and clearly marks done notes so completed notes are visually separable from outstanding ones at a glance. It spans rendering the flag per row (FEAT-PO-001), the visually distinct done treatment (FEAT-PO-002), conveying the done state beyond color alone via a textual done cue (FEAT-PO-003), and exposing the done state to assistive technology (FEAT-PO-004). Toggling the done state, filtering, reordering/grouping, and note detail/edit views are out of scope.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 6 |
| Boundary conditions (@boundary) | 3 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None. All four groups were proposed and accepted in headless mode; no group was marked for deferral.

## Open Assumptions (low confidence)

All nine assumptions are low confidence and unconfirmed (headless propose-never-elicit mode). They need human verification before acceptance:

- ASSUM-001 — notes list already receives the done flag (no API change)
- ASSUM-002 — single notes list view where all notes are displayed together
- ASSUM-003 — done treatment is a 'Done' textual label plus differentiated row styling
- ASSUM-004 — done notes keep their current list position (no reordering/grouping)
- ASSUM-005 — unset/null done flag is treated as not-done
- ASSUM-006 — non-color done cue is a 'Done' textual label rather than an icon
- ASSUM-007 — done state exposed via accessible label or ARIA state
- ASSUM-008 — done flag can change while the list is open, causing a re-render
- ASSUM-009 — done flag is a boolean-like completed state already on the note model

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes List Done-Flag Visibility" --context features/notes-list-done-flag-visibility/notes-list-done-flag-visibility_summary.md

## Verifier routing (proposed)

- "Every note row reflects its done flag" → playwright
- "Showing the done flag does not alter other list behaviour" → playwright
- "Done notes are visually distinct from not-done notes" → playwright
- "Done state is conveyed by a non-color cue as well as styling" → playwright
- "A non-color textual cue conveys the done state alongside styling" → playwright
- "Screen reader announces the done state of each row" → playwright
- "Rows render in the not-done presentation when the flag is not set" → playwright
- "A note whose done flag is set renders in the done presentation" → playwright
- "Not-done rows carry no done cue" → playwright
- "A list of only not-done notes still shows each flag" → playwright
- "The done treatment is never applied to a not-done note" → playwright
- "Announced done state stays in sync when the list re-renders" → playwright
