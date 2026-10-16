---
complexity: 4
dependencies: []
feature_id: FEAT-712C
id: TASK-712C-001
implementation_mode: task-work
parent_review: TASK-REV-712C
status: design_approved
task_type: feature
title: Enforce blank-title rejection in the Go notes API and its Go tests
wave: 1
---

Make the Go notes API refuse to create a note whose title is absent, the empty string, or whitespace-only after trimming, answering HTTP 400 Bad Request with a clear message naming the title as the invalid field, while leaving well-formed titles answered exactly as before.

## The words of the request this task serves

> The notes API should refuse to create a note with an empty or blank title, answering 400 Bad Request with a clear message. Change only the Go API and its Go tests.

## Files to Create

- _none_

## Files to Modify

- `apps/api/internal/notes/validation.go`
- `apps/api/internal/notes/handler.go`

## Acceptance Criteria

- [ ] A note-creation request whose title is absent answers HTTP 400 Bad Request with a clear message identifying the title as the invalid field, and no note is persisted.
- [ ] A note-creation request whose title is the empty string answers HTTP 400 Bad Request with a clear message identifying the title as the invalid field, and no note is persisted.
- [ ] A note-creation request whose title is whitespace-only (spaces, tabs, newlines) answers HTTP 400 Bad Request with a clear message identifying the title as the invalid field, and no note is persisted.
- [ ] A note-creation request whose title is blank only after trimming surrounding whitespace answers HTTP 400 Bad Request with a clear message identifying the title as the invalid field, and no note is persisted.
- [ ] A note-creation request with a non-blank title (including a single character) answers the existing success status and persists a note, exactly as before.
- [ ] The blank-title rule is enforced in the Go API request-handling path, not in a database constraint or a client-side check.
- [ ] The rule applies only to note creation; update/edit operations and other fields are unchanged.
- [ ] Look at `apps/api/internal/notes/validation.go` first: `ValidateLabel` already trims whitespace and rejects an empty trimmed value (lines 22-31). Change only what is missing against the scenarios above; do not rewrite working code.
- [ ] All modified files pass project-configured lint/format checks with zero errors.

## Implementation Notes

- The existing `ValidateLabel` in `apps/api/internal/notes/validation.go` already trims surrounding whitespace and returns `errInvalidLabel` when the trimmed value is empty. Confirm the create handler in `apps/api/internal/notes/handler.go` calls `ValidateLabel` on note creation and answers 400 with a message naming the title field when it returns `errInvalidLabel`.
- The existing over-long-title path uses `errLabelTooLong` with a specific message; the blank-title path must likewise carry a clear message naming the title as the invalid field rather than the generic "Invalid request" used for undecodable bodies. Verify what message the handler emits for `errInvalidLabel` and adjust only if it does not name the title field.
- Scope is confined to the Go API and its Go tests. Do not touch the web client, database schema, migrations, or any non-Go surface.
- The request names no HTTP method or path; the existing create route (`POST /api/notes` per the existing tests) is the surface — do not invent a new one.

## Seam Tests

No cross-task integration contract is defined for this task; the boundary is the in-process call from the create handler to `ValidateLabel`, exercised directly by the Go tests in TASK-712C-002.