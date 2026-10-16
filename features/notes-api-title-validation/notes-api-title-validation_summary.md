# Feature Spec Summary: Notes API Title Validation

**Stack**: generic (no stack declared in the request; no codebase scan performed in headless mode)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 11 total (2 smoke, 0 regression)
**Assumptions**: 6 total (0 high / 0 medium / 6 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification pins the server-side rule that the notes API refuses to create a note whose title is absent, the empty string, or whitespace-only after trimming, answering HTTP 400 Bad Request with a clear message naming the title as the invalid field and persisting nothing. A note with a non-blank title continues to be created and answered exactly as before. Scope is confined to the Go API and its Go tests; no client, schema, migration, or non-Go surface is in scope. The request names no HTTP method or path, so the endpoint is left unspecified rather than guessed.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 3 |
| Negative cases (@negative) | 3 |
| Edge cases (@edge-case) | 2 |

## Deferred Items

None. No scenario groups were deferred; all proposed scenarios are included.

## Open Assumptions (low confidence)

- ASSUM-001 — The note-creation operation is an HTTP endpoint accepting a title, and it currently accepts empty or blank titles without complaint.
- ASSUM-002 — Blank means the title is absent, the empty string, or whitespace-only once surrounding whitespace is trimmed.
- ASSUM-003 — The 400 response carries a clear message naming the title as the invalid field.
- ASSUM-004 — The rule is enforced in the Go API request-handling path rather than in a database constraint or a client-side check.
- ASSUM-005 — The rule applies only to note creation; update/edit operations and other fields are out of scope.
- ASSUM-006 — Existing successful note-creation behaviour and status codes are unchanged for non-blank titles; the new rule only adds a rejection path.

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes API Title Validation" --context notes-api-title-validation/notes-api-title-validation_summary.md

## Verifier routing (proposed)

- "A note with a non-blank title is created successfully" → hurl
- "A note with a non-blank title is answered exactly as before" → hurl
- "A note-creation request with a blank title is rejected with a clear message" → hurl
- "A title of a single non-whitespace character is accepted" → hurl
- "A title of a single whitespace character is rejected as blank" → hurl
- "A title with surrounding whitespace around a non-blank value is accepted" → hurl
- "A note-creation request with an absent title is rejected" → hurl
- "A note-creation request with an empty-string title is rejected" → hurl
- "A note-creation request with a whitespace-only title is rejected" → hurl
- "A title of only tab and newline characters is rejected as blank" → hurl
- "A note with a very long non-blank title is created successfully" → hurl