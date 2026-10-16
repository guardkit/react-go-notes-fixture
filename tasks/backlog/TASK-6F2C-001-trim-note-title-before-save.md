---
id: TASK-6F2C-001
title: Trim note title before saving in Go API
task_type: feature
parent_review: TASK-REV-6F2C
feature_id: FEAT-6F2C
wave: 1
implementation_mode: direct
complexity: 3
dependencies: []
---

Change the Go notes API so that a note's title is trimmed of leading and trailing whitespace before it is persisted, on both the create and update persistence paths, and the trimmed value is what is persisted and returned in the API response. The interior of the title is left unchanged.

## The words of the request this task serves

> The notes API should trim leading and trailing spaces from a note's title before saving it. Change only the Go API and its Go tests.

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/handler.go`
- `apps/api/internal/notes/store.go`

## Acceptance Criteria

- The create path trims leading and trailing whitespace from the submitted title before the value is written to storage, and the API response for `POST /api/notes` returns the trimmed title.
- The update path trims leading and trailing whitespace from the submitted replacement title before the value is written to storage, and the API response for the update endpoint returns the trimmed title.
- Interior whitespace of the title (including internal spaces and all non-space characters) is preserved: the trim removes ONLY leading and trailing whitespace.
- A title with no leading or trailing whitespace is returned unchanged.
- A title composed entirely of whitespace yields an empty title in the API response.
- For the surrounding-space boundary (leading, trailing, and both), the returned title equals the trimmed title at every create response.
- The change touches only the Go API implementation; no schema migration, client code, or cross-service code is modified.

## Implementation Notes

- Start from the note title decode and persistence path in `apps/api/internal/notes/handler.go` and the write path in `apps/api/internal/notes/store.go`; change only what is missing rather than rewriting the surrounding logic.
- Apply the trim immediately before the value is written to storage (the last step before persistence), so it covers both create and update.
- Use the standard library `strings.TrimSpace` for the trim.
- Do not move the existing blank-title rejection (`apps/api/internal/notes/validation.go`, which already trims before checking for blankness) — its behaviour stays as-is; only the persisted-and-returned title is now trimmed.
- Confirm the response body reflects the trimmed value, not the value submitted by the client.