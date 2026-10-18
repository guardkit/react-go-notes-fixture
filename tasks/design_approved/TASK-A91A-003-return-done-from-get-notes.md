---
complexity: 4
dependencies:
- TASK-A91A-001
- TASK-A91A-002
feature_id: FEAT-A91A
id: TASK-A91A-003
implementation_mode: task-work
parent_review: FEAT-A91A
status: design_approved
task_type: feature
title: Return the done flag from GET /api/notes
wave: 2
---

Make `GET /api/notes` return each note's stored `done` flag on the wire, and make note creation leave `done` false by default, so the delivered surface exposes the flag exactly as the migration stored it and the contract declares it.

## The words of the request this task serves

List lines: 2, 4

"GET /api/notes returns each note's done flag" — this task delivers that at the read surface. A newly created note returns `done false`; each stored value round-trips verbatim (`true` and `false`); a completed note is never omitted from the list; every returned note carries a `done` field that is neither missing nor null; the existing `id`, `label`, and `created` fields are still returned unchanged alongside `done`; and `GET /api/notes` never accepts a `done` value from the caller (it is read-only). This task uses TASK-A91A-001's `notes.done` column and TASK-A91A-002's `Note.done` contract property.

## Files to Create

- `apps/api/internal/notes/done_test.go`

## Files to Modify

- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/store.go`

## Acceptance Criteria

- `GET /api/notes` returns, for each note, a `done` JSON field that equals the stored value: a note stored with `done=false` is returned `done false`, a note stored with `done=true` is returned `done true` (both `@smoke` states of the Scenario Outline).
- A note created through `POST /api/notes` returns and persists with `done false` by default.
- A completed note (`done=true`) is returned in the list; the `done` flag never hides a note from `GET /api/notes` (both a false and a true note are returned together).
- Every note in a mixed `GET /api/notes` response includes a `done` field; no note is returned with a missing or null `done` field (non-null column guarantees this at the boundary).
- A returned note still carries its `id`, `label`, and `createdAt` unchanged, in addition to the new `done` field (the flag is additive).
- `GET /api/notes` does not read any `done` value from the request; the stored flag is unchanged by the read (the notes read path stays read-only — ASSUM-003).
- The wire response validates against the contract updated by TASK-A91A-002 (the `contractRouter` at `apps/api/internal/notes/contract_test.go:20-38` accepts a note response with `done` and refuses one missing it).
- Add `apps/api/internal/notes/done_test.go` covering all of the above at the handler boundary (the surface the person uses), including the "does not displace existing fields" and "does not accept a done value" cases.
- One check covers "each note": it finds the members itself when it runs, from `apps/api/internal/notes/store.go` (the `List` query at `store.go:64` returns the owner's notes from the `notes` table), and fails if it finds none.
- All modified Go files pass the project's configured lint/format checks with zero errors.

## Implementation Notes

- Look first at `apps/api/internal/notes/handler.go` (the `GET /api/notes` handler around `handler.go:46`) and `apps/api/internal/notes/store.go` (the `List`/`Create` queries around `store.go:31,64`); change only what is missing for `done`, following the existing note-shape code.
- Add a `Done bool` field to the Go `Note` type and serialize it as JSON key `done` (contract-ordered `id`, `label`, `createdAt`, then `done`) so it round-trips verbatim.
- In `store.go`, include `done` in the `SELECT` column list and bind `done = false` in the `INSERT` (rely on the DB default from TASK-A91A-001); do not add a `done` scan from user input.
- Do NOT add any request body / query parsing for `done` on the read path — the endpoint stays read-only.
- Extend the existing table-driven response-shape tests (see `handler_test.go:188-203` for the exact wire-shape assertion style) so the note JSON gains the `done` key.
- Run the existing handler, store, and contract tests first (`bash scripts/check.sh`) and change only what the `done` field needs.
- Consumer context: this surface consumes TASK-A91A-001's `notes.done` column and TASK-A91A-002's `Note.done` contract property.
- Before writing the "each note" check, search the whole repository for further members of the notes set (any file that reads or writes the `notes` table or the `Note` type) and ensure the check covers them. The descriptor reports 48 files matched "note" but only 24 were listed; search beyond the listed candidates.

## Seam Tests

This surface consumes the `done` column (TASK-A91A-001) and the `Note.done` contract property (TASK-A91A-002); the two `## Files to Modify` are the delivered read path.

```python
"""Seam test: verify the delivered GET /api/notes response honors both contracts."""
import pytest


@pytest.mark.seam
@pytest.mark.integration_contract("done")
def test_get_notes_carries_done():
    """Verify GET /api/notes serializes each note's stored done flag.

    Contract: notes.done is BOOLEAN NOT NULL DEFAULT false (TASK-A91A-001) and
    Note.done is a required boolean in the contract (TASK-A91A-002); the read
    path must emit a non-null boolean 'done' per note and never omit a note.
    Producers: TASK-A91A-001, TASK-A91A-002
    """
    # Delivered surface: one serialized note from the handler's list response.
    note_json = {"id": "1", "label": "x", "createdAt": "2026-10-12T09:30:00Z", "done": False}

    # Consumer side: what the delivered surface must guarantee.
    assert "done" in note_json, "every note must carry a done field"
    assert isinstance(note_json["done"], bool), "done must be a boolean"
    assert note_json["done"] is False, "a default note returns done false"
```