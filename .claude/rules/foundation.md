# Fixture foundation

- Smoke gates use `bash scripts/check.sh`. Browser journeys are the declared `feature_check` (`bash scripts/check-feature.sh`), which GuardKit runs itself after the last wave with its own long limit; do not put them in `smoke_gates`. If a plan must, it sets `timeout: 600`.
- Coverage is measured, never thresholded: `bash scripts/coverage.sh` (root), `npm run coverage` (apps/web) and the declared Go command (apps/api). Reports stay in git-ignored `coverage/` folders.
