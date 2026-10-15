# Keycloak Realm Setup Readiness (FEAT-6AC9)

Make the realm setup wait for Keycloak's admin login to be ready so app start-up, the checks target, the local stack and every deploy stop failing on a Keycloak that reports itself healthy slightly before its admin login is usable.

## Tasks

| Task | Wave | Mode | Complexity |
|------|------|------|------------|
| TASK-KRSR-001 — Add bounded readiness wait to the Keycloak realm setup | 1 | task-work | 6 |
| TASK-KRSR-002 — Add tests for the realm setup readiness wait | 2 | task-work | 4 |
| TASK-KRSR-003 — Document the realm setup readiness behaviour | 2 | direct | 2 |

## Execution

- **Wave 1:** TASK-KRSR-001 (foundation — the readiness wait in the realm setup).
- **Wave 2:** TASK-KRSR-002 and TASK-KRSR-003 in parallel (tests and docs, both building on the wait).

See `IMPLEMENTATION-GUIDE.md` for diagrams, integration contracts and deferred planning decisions.