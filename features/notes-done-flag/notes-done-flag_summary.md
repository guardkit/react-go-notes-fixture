# Feature Spec Summary: Notes Done Flag

**Stack**: generic (no stack signal supplied; `--stack` unset, so `generic` is recorded rather than guessed)
**Generated**: 2026-07-09T14:32:00Z
**Scenarios**: 12 total (2 smoke, 0 regression)
**Assumptions**: 6 total (0 high / 0 medium / 6 low confidence)
**Review required**: Yes

REVIEW REQUIRED: all assumptions unconfirmed (--auto mode)

## Scope

This specification covers a single vertical slice that introduces a boolean `done` completion flag on notes: a new migration adds a non-null `done` column defaulting to false (backfilling pre-existing notes to not-done), `GET /api/notes` serializes each note's `done` value without omitting completed notes, the API contract declares `done` as a boolean present on every note, and the web client types are regenerated from that contract so the client note type carries the `done` field. The slice is read-path only — no create/update/toggle endpoint for the flag is specified. The input is well specified for the happy path and the contract/types loop; the open questions (backfill semantics, required-vs-optional in the contract, mutation scope, ownership scoping, and whether regeneration is automated) are carried as low-confidence assumptions rather than invented.

## Scenario Counts by Category

| Category | Count |
|----------|-------|
| Key examples (@key-example) | 6 |
| Boundary conditions (@boundary) | 3 |
| Negative cases (@negative) | 2 |
| Edge cases (@edge-case) | 1 |

## Deferred Items

None. No scenario groups were deferred; every proposed scenario is included.

## Open Assumptions (low confidence)

- ASSUM-001 — Pre-existing note rows are backfilled to done=false when the migration runs.
- ASSUM-002 — The done flag is a simple boolean, not a richer status enum.
- ASSUM-003 — This slice is read-path only; no mutation endpoint for the done flag is in scope.
- ASSUM-004 — The contract declares done as a required boolean present on every note.
- ASSUM-005 — The web types are regenerated from the contract as part of this change, not hand-edited.
- ASSUM-006 — The done flag is scoped to the signed-in owner; the read endpoint returns only the caller's own notes.

## Verifier routing (proposed)

- "A newly created note is not done by default" → hurl
- "A note that existed before the migration is not done" → toolchain
- "GET /api/notes returns each note's stored done flag" → hurl
- "A completed note is not omitted from the notes list" → hurl
- "The done flag carries exactly the two boolean states" → hurl
- "Every note in the response carries a done field" → hurl
- "The done flag does not displace the existing note fields" → hurl
- "The notes read endpoint does not accept a done value from the caller" → hurl
- "A signed-in owner cannot read another owner's notes" → hurl
- "The contract declares the done flag on the note object" → toolchain
- "The generated web note type includes the done flag" → toolchain
- "The done field reaches the web type only through regeneration" → toolchain

## Integration with /feature-plan

This summary can be passed to `/feature-plan` as a context file:

    /feature-plan "Notes Done Flag" --context features/notes-done-flag/notes-done-flag_summary.md
