---
id: TASK-A91A-004
title: Regenerate the web types from the contract
task_type: feature
parent_review: FEAT-A91A
feature_id: FEAT-A91A
wave: 3
implementation_mode: direct
complexity: 2
dependencies:
  - TASK-A91A-002
  - TASK-A91A-003
---

Regenerate `apps/web/src/api-types.ts` from the updated contract so the generated web `Note` type carries the `done` boolean field — with the field arriving only through regeneration, never by hand-editing the generated file.

## The words of the request this task serves

List lines: 6, 7

"and the generated web types updated to match" — this task is the web-types half. The generated `Note` type gains a `done` boolean field, matching the `done` the read endpoint returns (TASK-A91A-003), and the field reaches the web type only through regeneration from the contract (TASK-A91A-002), with no generated file hand-edited.

## Files to Create

- `_none_`

## Files to Modify

- `apps/web/src/api-types.ts`

## Acceptance Criteria

- Regenerate `apps/web/src/api-types.ts` from `contract/openapi.yaml` via the project's generation step (`npm run contract:generate`, backed by `tools/contract/generate.mjs`), so the generated `components["schemas"]["Note"]` type includes a `done` boolean field marked present (non-optional), matching the required `done` the read endpoint returns.
- The `done` field in `api-types.ts` originates solely from regeneration: no generated file is hand-edited (the change in `git diff apps/web/src/api-types.ts` is exactly what the generator emits from the contract).
- The generated `Note` type still carries `id`, `label`, and `createdAt` unchanged alongside the new `done` field (the additive change is reflected in the type).
- `npm run contract:check` (drift check) passes: the committed `api-types.ts` matches what regeneration produces from the contract.
- `npm --workspace notes-web run typecheck` (see `scripts/check.sh:25`) passes with the new `done` field.
- Run the existing web type tests and drift check first (`bash scripts/check.sh`) and change only what regeneration produces.

## Implementation Notes

- `apps/web/src/api-types.ts` is a generated file (see the note at the top of `contract/openapi.yaml:1` and `README.md:34`); never edit it by hand — run `npm run contract:generate` and commit the regenerated output.
- The generated file already maps `/api/notes` to `components["schemas"]["NoteList"]` (`apps/web/src/api-types.ts:21-28`); confirm the regenerated `Note` schema now includes `done: boolean`.
- If the generated diff shows more than the `done` field, investigate before committing — the generator should be deterministic and the only intended change is `done`.
- This task builds on the contract updated by TASK-A91A-002 and reflects the field the read endpoint (TASK-A91A-003) now returns.

## Seam Tests

This task consumes TASK-A91A-002's contract `Note.done` declaration; the generated type is the consumer of that declaration.

```python
"""Seam test: verify the 'contract' -> generated-type done field from TASK-A91A-002."""
import pytest


@pytest.mark.seam
@pytest.mark.integration_contract("contract")
def test_generated_note_has_done():
    """Verify api-types.ts Note carries the done boolean from the contract.

    Contract: Note.done is a required boolean in contract/openapi.yaml
    (TASK-A91A-002); regeneration must surface it as a non-optional boolean on
    the generated web Note type.
    Producer: TASK-A91A-002
    """
    # Consumer side: the regenerated generated type.
    generated_types = ""  # read apps/web/src/api-types.ts

    assert "done" in generated_types, "generated Note type must carry done"
    assert "boolean" in generated_types, "done must be typed boolean"
```