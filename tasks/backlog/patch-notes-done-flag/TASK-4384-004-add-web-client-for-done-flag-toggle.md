---
id: TASK-4384-004
title: Add the web client for the done-flag toggle
task_type: feature
parent_review: TASK-REV-4384
feature_id: FEAT-4384
wave: 4
implementation_mode: direct
complexity: 3
dependencies:
  - TASK-4384-002
  - TASK-4384-003
---

Add a web API client function that sends PATCH /api/notes/{id} with the done flag and returns the updated note, so the browser can toggle completion state and refresh its view from the returned note.

## The words of the request this task serves

List lines: 1, 2, 3

This task serves the client side of "the signed-in owner can set a note's done flag to true or false; it returns the updated note": the web app gets a function that calls the new endpoint and hands back the updated note the response carries.

## Files to Create

- _none_

## Files to Modify

- `apps/web/src/api.ts`
- `apps/web/src/api.test.ts`

## Acceptance Criteria

- [ ] `apps/web/src/api.ts` exports a function that sends PATCH /api/notes/{id} with a JSON body `{ "done": <boolean> }`, the bearer token in the Authorization header, and returns the parsed updated note (id, label, created_at, done).
- [ ] The function surfaces a 404 (missing or foreign note) as a distinguishable error to its caller without inventing a body that leaks existence.
- [ ] `apps/web/src/api.test.ts` covers a successful toggle to true and to false, asserting the request URL, method, body, and the returned note, in the style of the existing "lists the signed-in owner's notes" test (`apps/web/src/api.test.ts:55`).
- [ ] The existing client functions (create, list) keep their current behaviour and signatures.

## Implementation Notes

- Follow the existing `api.ts` request helper and the `listNotes`/create patterns already in the file; reuse the same fetch wrapper and error mapping.
- The generated types from TASK-4384-003 (`apps/web/src/api-types.ts`) provide the request/response shapes; consume them rather than redefining the note shape.
- This task adds the client function only; wiring a toggle control into `App.tsx` is out of scope for this feature (the request asks for the endpoint and the returned note, not a new UI control).