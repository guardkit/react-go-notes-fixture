# Feature Spec Summary: Notes Count in Browser Tab Title

**Stack**: generic (no stack declared in the request; not auto-detected in headless mode)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 10 total (3 smoke, 0 regression)
**Assumptions**: 3 total (0 high / 0 medium / 3 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers the web-app-only change of showing the current note count in the browser tab title in the exact format "Notes (N)" (e.g. "Notes (3)"), derived from note data the web app already holds. It pins the initial-load rendering (including the empty state "Notes (0)"), the live update of the title when a note is added or deleted without a page reload, and the guarantee that the title never diverges from the visible note list. No backend, API, or storage behaviour is specified, per the request's scope constraint.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 6 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 1 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None. All scenarios are included; there is no interactive curation in this headless run.

## Open Assumptions (low confidence)

- ASSUM-001 — The web app already has the full note count in client-side state; no backend/API change is needed.
- ASSUM-002 — The app is a browser-based web app where document.title can be set dynamically.
- ASSUM-003 — The title format is exactly "Notes (N)" with the literal prefix "Notes", including "Notes (0)".

## Verifier routing (proposed)

- "Tab title shows the note count on initial load" → toolchain
- "Tab title shows zero notes on initial load" → toolchain
- "Adding a note updates the tab title immediately" → toolchain
- "Deleting the last note updates the tab title to zero" → toolchain
- "Tab title never diverges from the visible note list" → toolchain
- "Showing the count changes nothing outside the web app" → toolchain
- "Tab title renders the exact format for any note count" → toolchain
- "Deleting the last note leaves no stale count in the tab title" → toolchain
- "Tab title contains only the required count format" → toolchain
- "Rapid add and delete settle on the correct count" → toolchain

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Count in Browser Tab Title" --context notes-count-in-browser-tab-title/notes-count-in-browser-tab-title_summary.md