# Feature Spec Summary: Notes List Done-State Visibility

**Stack**: generic (stack undeclared in the request; recorded as generic rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 15 total (2 smoke, 0 regression)
**Assumptions**: 10 total (0 high / 0 medium / 10 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the presentational surfacing of each note's done flag in the web app's notes list: the list shows every row's done status so the user reads it without opening the note (FEAT-PO-001), every row renders a dedicated done indicator in a fixed position showing done or not-done state (FEAT-PO-002), and done notes are visually distinguished by more than one channel (FEAT-PO-003). It also covers conveying the done state beyond sight — an explicit status a screen-reader user hears (FEAT-PO-004) and survival when the browser forces high-contrast rendering (FEAT-PO-005). The scope is read-only: setting, toggling, filtering, and sorting by the done flag are out of scope, and no persistence or schema change is specified. Per the coach feedback, the examples describe only what a caller or user can observe through the interface: they avoid internal implementation terms (row data, row render time, per-note follow-up request, the accessibility tree, and the forced-colours CSS feature name) and instead state the status shown, what a screen-reader user hears or does not hear, and the visual outcome for the user.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 5 |
| Boundary conditions (@boundary) | 4 |
| Negative cases (@negative) | 3 |
| Edge cases (@edge-case) | 3 |

## Deferred Items

None. No scenario groups were deferred; all five features are represented.

## Open Assumptions (low confidence)

All ten assumptions are low confidence and unconfirmed (headless propose-never-elicit mode). They need human verification before acceptance:

- ASSUM-001 — notes list row payload already includes the done flag (no backend change)
- ASSUM-002 — absent done value defaults to not-done
- ASSUM-003 — done treatment = check mark + "Done" label + muted/strikethrough title
- ASSUM-004 — accessible announcement wording "Done" / "Not done"
- ASSUM-005 — high-contrast differentiators (border / underline / persisted text label)
- ASSUM-006 — narrowest supported viewport is a single mobile-class width
- ASSUM-007 — single uniform notes list view
- ASSUM-008 — done is a binary state
- ASSUM-009 — note model already persists the done flag (read-only here)
- ASSUM-010 — done notes remain mixed in the default list view

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes List Done-State Visibility" --context features/notes-list-done-state-visibility/notes-list-done-state-visibility_summary.md

## Verifier routing (proposed)

Advisory only; `/feature-plan` writes the authoritative per-scenario stamp map. Titles are quoted verbatim from the `.feature` file.

- "The notes list shows each note's done status without opening the note" → toolchain
- "A done note shows its indicator in the done state" → toolchain
- "An open note shows the indicator in the not-done state" → toolchain
- "A done note is visually distinguished from open notes by more than one channel" → toolchain
- "The done indicator appears in a consistent position across all rows" → toolchain
- "A note with no explicit done value is treated as not done" → toolchain
- "The row's done value reflects the note's current stored flag after a refresh" → toolchain
- "Paginated rows carry the done status on every page" → toolchain
- "A mixed list renders done and open rows side by side without ambiguity" → toolchain
- "Viewing the list does not change any note's done flag" → toolchain
- "The done indicator does not displace or truncate existing row content" → toolchain
- "The row layout holds at the narrowest supported viewport" → toolchain
- "A screen-reader user hears each note's completion status in row order" → toolchain
- "The iconographic done indicator adds no duplicate status announcement" → toolchain
- "Done rows remain distinguishable by shape or text in high-contrast rendering" → toolchain
