---
paths:
  - "apps/web/**"
  - "tests/e2e/**"
---

# React web and browser tests

- Smoke gates use `bash scripts/check.sh`. Browser journeys are the declared `feature_check` (`bash scripts/check-feature.sh`), which GuardKit runs itself after the last wave with its own long limit; do not put them in `smoke_gates`. If a plan must, it sets `timeout: 600`.
- A direct `npx playwright test` starts its own isolated stack (`scripts/stack.sh`) and removes it afterwards. It needs Docker, takes about a minute, and is not the check that counts.
