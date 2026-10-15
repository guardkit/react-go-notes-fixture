---
id: TASK-NTVL-002
title: "Extend the contract test table for the refused over-long title"
task_type: testing
parent_review: TASK-REV-D267
feature_id: FEAT-D267
wave: 2
implementation_mode: direct
complexity: 2
dependencies:
  - TASK-NTVL-001
scaffolding_kind: tests
---

Add a create-with-over-long-title row to the contract test table so the enforced boundary is also asserted through the response-contract validator.

## The words of the request this task serves

_none — this is a tests task_

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/contract_test.go`

## Acceptance Criteria

- `TestImplementedResponsesMatchTheContract` (`apps/api/internal/notes/contract_test.go:57-86`) carries a new case that POSTs `/api/notes` with a title of 201 characters and expects 400, and `checkResponse` accepts the response body as contract-conformant.
- All existing contract table cases (health, list, create, create invalid, create anonymous, create failure) remain green unchanged.

## Implementation Notes

- The table entries are struct literals of shape `{name, method, path, body, token, store, want}`; reuse the `fakeStore` and `goodToken` fixtures already in this file.
- Run the existing `apps/api/internal/notes` tests first; this task only extends `contract_test.go` — the behaviour under test already exists after TASK-NTVL-001.
- If the contract validator rejects the message body shape, do not edit `contract/openapi.yaml` (out of scope — Go only); report the finding instead of changing the contract.