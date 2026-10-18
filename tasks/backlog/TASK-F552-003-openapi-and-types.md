---
id: TASK-F552-003
title: "Publish the done-flag contract in OpenAPI and generated types"
task_type: declarative
parent_review: TASK-REV-F552
feature_id: FEAT-F552
wave: 3
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-F552-002
---

Add the `PATCH /api/notes/{id}` operation to `contract/openapi.yaml` and regenerate `apps/web/src/api-types.ts` from `tools/contract/generate.mjs`, so the published contract describes the done-flag toggle the API now serves. This task builds no code surface and owns no scenario; it documents the shape TASK-F552-002 delivers.

## The words of the request this task serves

List lines: 1, 4, 5, 6, 7

The request's PATCH endpoint is published through the OpenAPI contract. This task adds the `patch` operation for `/api/notes/{id}` to `contract/openapi.yaml`, reusing the existing `Note` response schema and the `SignInRequired` / `Unavailable` responses already used by the sibling operations, and regenerates `apps/web/src/api-types.ts` so the client types describe the toggle, the 404 responses, and the 401.

## Files to Create

- _none_

## Files to Modify

- `contract/openapi.yaml`
- `apps/web/src/api-types.ts`

## Acceptance Criteria

- `contract/openapi.yaml` declares a `patch` operation on `/api/notes/{id}` with a bearer security requirement, a request body carrying the boolean `done` field, a `200` response returning the updated `Note`, and a `404` response alongside the `401` `SignInRequired` and `500` `Unavailable` responses the sibling operations use.
- `apps/web/src/api-types.ts` is regenerated from the contract through `tools/contract/generate.mjs` (never hand-edited) and reflects the new `patch` operation and its responses.
- No existing OpenAPI path or operation is changed.
- `npm run contract` (or the project's contract-generation command) reproduces `api-types.ts` with no diff.

## Implementation Notes

- Run the existing contract generator rather than editing `api-types.ts` by hand; a hand-edit will be overwritten on the next generate.
- `npm run contract` and `npm run lint` are the checks for this task.