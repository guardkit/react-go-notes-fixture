# Notes API Title Normalization (FEAT-6F2C)

Trim leading and trailing whitespace from a note's title before saving it, on the Go notes API's create and update paths, so stored titles never carry accidental surrounding whitespace. The trimmed value is what is persisted and returned; interior spacing is preserved. Scope: the Go API and its Go tests only.

## Tasks

- TASK-6F2C-001 — Trim note title before saving in Go API (wave 1, feature, complexity 3) — owns all six scenarios.
- TASK-6F2C-002 — Go tests for title trimming on create and update (wave 2, testing, complexity 2).

## Next steps

1. Read `IMPLEMENTATION-GUIDE.md`.
2. Implement wave 1 (`TASK-6F2C-001`), then wave 2 (`TASK-6F2C-002`).
3. Build: `/feature-build FEAT-6F2C`.