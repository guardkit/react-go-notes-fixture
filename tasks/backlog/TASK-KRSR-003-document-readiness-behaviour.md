---
id: TASK-KRSR-003
title: Document the realm setup readiness behaviour
task_type: documentation
parent_review: TASK-REV-6AC9
feature_id: FEAT-6AC9
wave: 2
implementation_mode: direct
complexity: 2
dependencies:
  - TASK-KRSR-001
scaffolding_kind: documentation
---

Document the realm setup's readiness behaviour in the README's scripts table entry for `keycloak-config.sh`.

## The words of the request this task serves

_none — this is a documentation task_

## Files to Create

- _none_

## Files to Modify

- `README.md`

## Acceptance Criteria

- [ ] The README's `scripts/migrate.sh`, `seed.sh`, `keycloak-config.sh`, `build-artifacts.sh` table row (`README.md:43`) notes that the realm setup waits for the admin login to be ready, retrying only on 503 or a refused connection under a 2-minute budget and failing straight away on any other error.
- [ ] The README states that the admin password is passed to the realm setup on the exec's stdin and never appears on a command line or in output.

## Implementation Notes

- Keep the edit to the existing table row and a short note; do not restructure the README.