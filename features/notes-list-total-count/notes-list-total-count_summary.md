# Feature Spec Summary: Notes List Total Count

**Stack**: generic
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 3 total (1 smoke, 0 regression)
**Assumptions**: 4 total (0 high / 0 medium / 4 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers rendering a total-notes count in a dedicated region immediately above the notes list, sourced from the same collection that populates the list. The stack is undeclared and defaults to `generic`; no codebase scan or fleet-memory retrieval was performed in this headless run.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 2 |
| Boundary conditions (@boundary) | 0 |
| Negative cases (@negative) | 1 |
| Edge cases (@edge-case) | 0 |

## Deferred Items

None.

## Open Assumptions (low confidence)

- ASSUM-001: A notes list view already exists and renders a collection of notes; the request adds a count to it rather than introducing the list itself.
- ASSUM-002: "Total number of notes" means the size of the note set the list is currently showing, not a per-user global count independent of the view.
- ASSUM-003: The total count is already available to the view from the data used to render the list, so no new backend endpoint or query is required.
- ASSUM-004: The count is a non-interactive text summary placed above the list; it is not a filter control, link, or sortable affordance.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes List Total Count" --context features/notes-list-total-count/notes-list-total-count_summary.md

## Verifier routing (proposed)

- "Total note count is displayed above the notes list" → toolchain
- "Count is sourced from the same collection that renders the list" → toolchain
- "Count is not rendered inline with list items" → toolchain