# Feature Spec Summary: Notes List Done-Flag Visibility

**Stack**: generic (stack undeclared in the request; recorded as generic rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 8 total (2 smoke, 0 regression)
**Assumptions**: 6 total (0 high / 0 medium / 6 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the display-layer change that shows each note's done flag in the web app's notes list and clearly marks done notes so completed notes are visually separable from outstanding ones at a glance. It spans rendering the flag per row (FEAT-PO-001), the visually distinct done treatment (FEAT-PO-002), and the requirement that the distinction survive color-only perception (FEAT-PO-003). Toggling the done state, filtering, reordering/grouping, and note detail/edit views are out of scope.

Worked examples use only observable interface facts drawn from the approved input — the "Done" textual label and visually differentiated row styling from the (deferred) ASSUM-003 treatment — and describe what a user sees (a displayed "Done" label, a differently styled row) rather than asserting internal state such as a set flag.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 3 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 0 |

## Deferred Items

None. All groups were proposed and accepted in headless mode; no group was marked for deferral.

## Open Assumptions (low confidence)

All six assumptions are low confidence and unconfirmed (headless propose-never-elicit mode). They need human verification before acceptance:

- ASSUM-001 — notes list already receives the done flag (no API change)
- ASSUM-002 — single notes list view where all notes are displayed together
- ASSUM-003 — done treatment is a 'Done' textual label plus differentiated row styling
- ASSUM-004 — done notes keep their current list position (no reordering/grouping)
- ASSUM-005 — unset/null done flag is treated as not-done
- ASSUM-006 — done flag is a boolean-like completed state already on the note model

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes List Done-Flag Visibility" --context features/notes-list-done-flag-visibility/notes-list-done-flag-visibility_summary.md

## Verifier routing (proposed)

- "Every note row reflects its done flag" → playwright
- "Showing the done flag does not alter other list behaviour" → playwright
- "Done notes are visually distinct from not-done notes" → playwright
- "Rows render in the not-done presentation when the flag is not set" → playwright
- "A done note is rendered with the done treatment" → playwright
- "Only done rows carry the non-color textual cue" → playwright
- "A list of only not-done notes still shows each note in the not-done presentation" → playwright
- "The done treatment is never applied to a not-done note" → playwright