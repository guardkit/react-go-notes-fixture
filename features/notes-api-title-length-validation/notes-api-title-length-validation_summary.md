# Feature Spec Summary: Notes API Title Length Validation

**Stack**: generic (no stack declared in the request; not auto-detected — no codebase scan is performed in this mode)
**Generated**: 2026-08-14T00:00:00Z
**Scenarios**: 6 total (2 smoke, 0 regression)
**Assumptions**: 4 total (0 high / 0 medium / 4 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the request-boundary rule for the Go notes API: a note-creation request whose title exceeds 200 characters is refused with HTTP 400 Bad Request and a clear message naming the title field and the 200-character limit, and nothing is persisted. Titles of exactly 200 characters or fewer pass validation and proceed to normal creation, so the limit is inclusive. Scope is deliberately narrow — the request forbids inventing adjacent work such as storage-layer limits, client-side checks, configurability, or changes to other services, and the rule is stated for creation only.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 2 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 0 |

## Deferred Items

None. No scenario groups were deferred.

## Open Assumptions (low confidence)

- ASSUM-001 — The notes API exposes an HTTP note-creation endpoint implemented in Go where request validation can be enforced.
- ASSUM-002 — The 200-character limit is counted in characters (Unicode code points), not bytes.
- ASSUM-003 — The rule applies only to note creation, not to updates of existing notes.
- ASSUM-004 — The "clear message" is a plain response body naming the title field and the 200-character limit, with no prescribed structured JSON error envelope.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes API Title Length Validation" --context features/notes-api-title-length-validation/notes-api-title-length-validation_summary.md

## Verifier routing (proposed)

- "A note with a title longer than 200 characters is refused" → hurl
- "A note with a title of exactly 200 characters is accepted" → hurl
- "A note with a short title is accepted" → hurl
- "A title at the exact 200-character limit is accepted" → hurl
- "A title one character over the 200-character limit is refused" → hurl
- "A refused note-creation request persists nothing" → toolchain
