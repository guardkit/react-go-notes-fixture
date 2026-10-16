# Feature Spec Summary: Notes API Title Normalization

**Stack**: generic
**Generated**: 2026-08-14T12:00:00Z
**Scenarios**: 8 total (2 smoke, 0 regression)
**Assumptions**: 4 total (0 high / 0 medium / 4 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers server-side normalization of note titles in the Go notes API: a note's title is trimmed of leading and trailing whitespace before it is persisted, on both the create and update paths, so that stored titles never carry accidental surrounding whitespace. The trimmed value is what is persisted and returned, while the interior of the title (including internal spaces and all non-space characters) is left unchanged. Change scope is limited to the Go API implementation and its Go tests; no schema migrations, client changes, or cross-service edits are in scope.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 3 |
| Boundary conditions (@boundary) | 3 |
| Negative cases (@negative) | 1 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None.

## Open Assumptions (low confidence)

- ASSUM-001: The trim applies to both note creation and note update operations
- ASSUM-002: Trimming removes all surrounding whitespace (Go strings.TrimSpace semantics), not only literal space characters
- ASSUM-003: A title that is entirely whitespace is saved as an empty string rather than rejected as empty
- ASSUM-004: The notes API is implemented in Go and has an existing create/update persistence path where a title trim can be applied before saving

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes API Title Normalization" --context features/notes-api-title-normalization/notes-api-title-normalization_summary.md

## Verifier routing (proposed)

- "Creating a note with a leading-space title stores the trimmed title" → hurl
- "Updating a note with a trailing-space title stores the trimmed title" → hurl
- "Interior spacing of a title is preserved when trimming" → toolchain
- "A title with no surrounding whitespace is stored unchanged" → toolchain
- "A title composed entirely of spaces is stored as an empty title" → toolchain
- "Surrounding spaces are removed from a title before saving" → toolchain
- "A title that trims to empty is saved as an empty title rather than rejected" → hurl
- "Surrounding whitespace other than spaces is removed from a title" → toolchain
