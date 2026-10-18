---
id: TASK-A91A-002
title: Declare the done flag in the notes API contract
task_type: feature
parent_review: FEAT-A91A
feature_id: FEAT-A91A
wave: 1
implementation_mode: direct
complexity: 2
dependencies: []
---

Declare `done` as a required boolean property on the `Note` schema of the source OpenAPI contract, so the contract — the single source of truth the web types are generated from — describes each note's done flag.

## The words of the request this task serves

List lines: 5

"the contract ... updated to match" and "the contract and the generated web types updated to match" — this task is the contract half: it declares `done` on the note object returned by `GET /api/notes`. Per ASSUM-004 (deferred), `done` is a **required** boolean present on every note, not optional.

## Files to Create

- `apps/api/internal/notes/contract_done_test.go`

## Files to Modify

- `contract/openapi.yaml`

## Acceptance Criteria

- `contract/openapi.yaml`'s `Note` component schema declares a `done` property of type `boolean`, and `done` is listed as required (present on every note), alongside `id`, `label`, and `createdAt`.
- The existing `NewNote` request schema is left unchanged (no `done` on the write path): the read slice is read-only (ASSUM-003, deferred).
- The contract still validates as valid OpenAPI (the existing `contractRouter`/`doc.Validate` path at `apps/api/internal/notes/contract_test.go:20-38` continues to pass).
- Add a contract test in `apps/api/internal/notes/contract_done_test.go` proving that a note response missing `done` is refused, mirroring the existing "missing createdAt" case at `apps/api/internal/notes/contract_test.go:105`, and that a note with `done` true and a note with `done` false are both accepted.
- `npm run contract:check` (contract drift) passes after regeneration in TASK-A91A-004.

## Implementation Notes

- `contract/openapi.yaml` is the source of truth; `apps/web/src/api-types.ts` is generated from it (see `README.md:34`). Edit only the contract here, never the generated file (that is TASK-A91A-004).
- Add `done: { type: boolean }` to `Note.properties` and append `done` to `Note.required`. Match the existing schema formatting used for `createdAt`.
- Do not change the `/api/notes` response envelope (`NoteList`); only the per-note `Note` shape gains `done`.
- Keep the contract the only place the boolean is declared: this is what makes the web type agree by generation, not by hand (ASSUM-005, deferred).

## Seam Tests

The contract's `Note.done` property is consumed by the generated web type (TASK-A91A-004).

```python
"""Seam test: verify the 'contract' done-property contract from TASK-A91A-002."""
import pytest


@pytest.mark.seam
@pytest.mark.integration_contract("contract")
def test_contract_declares_done():
    """Verify contract/openapi.yaml declares Note.done as a required boolean.

    Contract: Note.done is type boolean and is listed in Note.required, so the
    generated web type carries a non-null boolean done on every note.
    Producer: TASK-A91A-002
    """
    # Producer side: the contract file consumed by the generator.
    contract_text = ""  # read contract/openapi.yaml

    # Consumer side: the shape TASK-A91A-004 depends on when generating.
    assert "done" in contract_text, "Note schema must declare a done property"
    assert "boolean" in contract_text, "done must be typed boolean"
```