# Notes fixture (React, Go, PostgreSQL, Keycloak)

A small notes app with sign-in. It is **neutral verification infrastructure** for a
software factory's web-app readiness (the factory's web-readiness design). It is not a
product. It has no trainer, learner, assessment or grading concepts.

It proves that the factory's checks work on a real browser, API, database and identity
provider:
- install and the native gates;
- the connected journey;
- migrations;
- the contract;
- concurrent runs and their cleanup;
- the review preview and the evidence shown before a merge decision.

## Where it came from

- **Starting point.** The `react-go-monorepo` 1.1.0 template's literal layout, rendered
  with the template's own `materialise()` (GuardKit `5ba7cd28`,
  `tests/integration/verify_react_go_template.py`). That render is the first fixture
  commit, so later changes show as a diff.
- **Kept from the template:** `scripts/require-tools.sh` (exact Go 1.26.9, Staticcheck
  2026.2.1 and Node 22.23.2 guards), `scripts/install.sh`, `scripts/check.sh` (Go tests
  with `-count=1`) and the `.guardkit/config.yaml` declaration.
- **Added here:** sign-in, owner-scoped notes, migrations, the contract, the containers
  and the connected check.

## What is in it

| Path | What it does |
| --- | --- |
| `apps/api` | Go `net/http` API. `GET /api/health` reports liveness and the run id. `GET /api/notes` and `POST /api/notes` require a valid access token and act for its `sub`. See "Token checks" below. Server timeouts, request contexts, graceful shutdown. |
| `apps/api/migrations` | `0001` creates `notes` with an owner; `0002` adds `created_at` with a default. |
| `contract/openapi.yaml` | The contract's source. `apps/web/src/api-types.ts` is generated from it (`npm run contract:generate`) and committed. `npm run contract:check` regenerates it and fails on drift. The Go test `contract_test.go` validates every implemented response against the same file, and shows that forbidden responses are refused. |
| `apps/web` | React and Vite in strict TypeScript. Sign-in uses oidc-client-ts 3.5.0 (authorisation code with PKCE S256), with tokens kept in memory and the short-lived sign-in state in this tab's sessionStorage (never localStorage). Lists and creates notes; signs out. Runtime settings (realm, client, run id, sign-in hint) come from the proxy's `/config.json`. Nothing secret is in the bundle. |
| `compose.yaml` | PostgreSQL 17.7 (separate app and Keycloak databases and roles), Keycloak 26.7.4 at `/auth`, the API (distroless runtime image, no compiler), and Caddy 2.10.2. Caddy serves the built web files, `/api` and `/auth` on one port. |
| `compose.preview.yaml` | Adds the token-guarded public listener for a retained preview. |
| `infra/` | The postgres init script and the proxy's Caddyfile. |
| `scripts/check.sh` | The aggregate native check: gofmt, `go mod verify`, vet, Staticcheck, race tests with `-v` (including token and contract tests), Go build, contract drift, `tsc`, ESLint, Prettier, Vitest, the Vite build, end-to-end typecheck, the driver's verdict tests and shell syntax. The declared ran-markers, byte-identical to the template (GuardKit `4f368202`), need a Go `--- PASS:` line and no cached `ok … (cached)` replay, and the root also needs Vitest's `Tests … N passed`. |
| `scripts/check-feature.sh` | The connected check (see below). Smoke gates use `bash scripts/check.sh`. Browser journeys are the declared `feature_check` (`bash scripts/check-feature.sh`, timeout 1800 s), which GuardKit runs itself after the last wave with its own long limit; do not put them in `smoke_gates`. If a plan must, it sets `timeout: 600`. (Also in `.claude/rules`.) |
| `scripts/coverage.sh` | The declared coverage (root): both suites, totals printed. Measured, never thresholded: exit 0 means the tests passed with coverage measured. The `web` component declares `npm run coverage` (Vitest with V8 coverage) and the `api` component the Go `-coverprofile` and `go tool cover -func` total. Reports stay in git-ignored `coverage/` folders. |
| `scripts/stack-lib.sh`, `scripts/stack.sh` | The disposable stack's lifecycle pieces, shared by `check-feature.sh` and by a direct Playwright run (below). |
| `scripts/migrate.sh`, `seed.sh`, `keycloak-config.sh`, `build-artifacts.sh` | The explicit steps they use. |
| `tests/e2e` | Playwright 1.63.0. Chromium runs the full journey: sign in on the real Keycloak page, create a note, reload and see it, sign out, and get 401 from the API anonymously or with a forged token. Firefox and WebKit run a sign-in smoke. HTML and JSON reports, video always for the journey, traces kept on failure. |
| `deploy/` | `profile.yaml` in Forge's format, with `identity`, the shared `sandbox` and the `preview` section (`enabled`, `slots`, `app_ttl_hours`, `evidence_days`, `slot_wait_minutes`, as Forge's parser reads them). `settings.sh` lists every signal Forge sends in each leg and maps it to one mode, project, port and settings file. `deploy.sh` handles each mode; see "Deploy modes" below. Then the health check. |
| `qa/live-gate-driver.mjs` | Runs the same journey against a running instance and prints Forge's results envelope. Its verdict comes from `qa/journey-report.mjs` (tested by `node --test` in `check.sh`). If the key journey's video, every screenshot, or the HTML report is missing or empty on disk, a passing run is reported as `instrument_fail`, never `pass`; a failing run stays `fail`. In a preview it addresses `http://127.0.0.1:<FACTORY_PREVIEW_INTERNAL_PORT>`. The envelope carries `app_url`, `report_ref`, `screenshot_refs`, `media_refs` (video), `trace_refs` and `outcome` (passed, failed, skipped, flaky; null when unknown), with references relative to the evidence folder. |

**Token checks.** The API accepts a token only when all of these hold:
- the RS256 signature verifies against keys from `OIDC_JWKS_URL`;
- the issuer is exactly one listed in `OIDC_ISSUERS`;
- the audience includes `OIDC_AUDIENCE`;
- the token has not expired.

It uses go-oidc v3.21.0 for the claim checks, with the fixture's own key set
(`internal/auth/keyset.go`):
- RS256 only;
- keys cached by key id;
- a token naming an unknown key id causes at most one key-set fetch per 10 s, so
  unauthenticated requests cannot make the API hammer Keycloak;
- one fetch runs at a time, outside the lock and on its own 10 s background context.
  So a cached key never waits behind a fetch, and a cancelled request never leaves the
  API without keys.

Anonymous, wrong-audience, expired, unknown-key, unlisted-issuer, `alg=none` and
HS256-with-the-public-key tokens get 401. The API logs why it refused a token (for
example `expected audience`), never the token itself.

## Addresses and sign-in in each context

Keycloak sits behind Caddy at `/auth` (`KC_HTTP_RELATIVE_PATH=/auth`). It takes its
browser-facing address from the forwarded headers Caddy sets (`KC_PROXY_HEADERS=xforwarded`,
`KC_HOSTNAME_STRICT=false`). Caddy **overwrites** those headers on each listener, so a
client cannot choose the issuer. Keycloak and the API are reachable only through Caddy or
the compose network.

| Context | Browser URL | Issuer the API accepts | Callback (redirect) URLs | Discovery and keys |
| --- | --- | --- | --- | --- |
| Disposable check, in the sandbox VM | `http://127.0.0.1:<P>/` on Caddy's internal listener | `http://127.0.0.1:<P>/auth/realms/<realm>` | `http://127.0.0.1:<P>/*` | The browser uses the URL in the first column. The API fetches keys from `http://keycloak:8080/auth/realms/<realm>/protocol/openid-connect/certs` on the compose network. |
| Preview journey recorded for the merge decision, in the VM | `http://127.0.0.1:<I>/` on the preview's internal listener | as above with `<I>` | `http://127.0.0.1:<I>/*` | as above |
| A reviewer on the private network | `https://<public host>:<S>/` (a private HTTPS front on the sandbox host → `sbx ports` → Caddy's public listener `<S>`, which sets `X-Forwarded-Proto: https` and the public host). The coordinator supplies the host (`review_preview.public_host`); it is not in this repository | `https://<public host>:<S>/auth/realms/<realm>` | `https://<public host>:<S>/*` | as above |

- **`<P>`** is a free loopback port chosen per run.
- **Preview ports.** A preview uses the slot port `<S>` (8950 or 8951) publicly, and an
  internal port `<I>` (8970 or 8971) that is not published.
- **Issuers.** A preview's API accepts exactly the internal and public issuers of its own
  run's realm, which share one signing key. A disposable check accepts only its internal
  issuer.
- **Realm names** are unique per run or instance.
- **Link token.** On the public listener, `?preview=<FACTORY_PREVIEW_TOKEN>` sets the
  cookie `factory_preview_<slot>` (Secure, HttpOnly) and redirects to `/`. It always goes
  to `/`, never to the requested path, so the link cannot be used as an open redirect. A
  missing, wrong or old token gets `410 — this preview link is for a different or expired
  candidate`. The cookie is stripped before any request reaches Keycloak or the API.
- **Proxy hygiene.** Each listener drops a client's `Forwarded`, `X-Forwarded-Prefix`,
  `X-Forwarded-Server`, `X-Forwarded-Ssl` and `X-Real-IP` headers, and sets its own
  `X-Forwarded-*`. Every `/auth/admin*` path gets 404.
- **Sign-in hint.** The preview's own hint shows the instance's test password, so a
  reviewer on the private network can sign in. `deploy/deploy.sh` does this in its preview mode only
  (`NOTES_HINT_SHOWS_PASSWORD` defaults to 1 there), because Forge passes a preview only
  its fixed `FACTORY_*` settings. Disposable checks and other deploys show user names
  only. The password is never in the bundle or on the merge card.
- **Per-lease settings.** A preview's realm, instance id and passwords are kept per lease
  in `.notes-state/<project>-<lease>.env`, inside the preview's own laid-out tree. A later
  occupant of the same slot never reuses an earlier one's settings, and the file goes
  when Forge removes the tree. Forge removes everything else of a preview by its label
  (containers, networks, volumes and the built images, which carry the same label); it
  does not run `deploy.sh` for that.

## The connected check, `scripts/check-feature.sh`

1. **Identity.** Allocates a run id, project `notes-chk-<run>` and a free loopback port.
2. **Start.** Builds the web bundle and a static API binary with the pinned toolchain,
   then builds the copy-only images and runs `compose up`. Readiness is bounded to 180 s.
   The run then confirms the proxy, API and Keycloak answer **as this run**: `/config.json`
   and `/api/health` carry the run id, and Keycloak's discovery issuer is this port's.
3. **Migrations.**
   - Upgrade: `0001` only, three rows seeded, then `0002`. The rows, the new column's
     default and both constraints must survive.
   - Fresh install.
   - Re-run, which must apply nothing.
4. **Seed.** Synthetic notes owned by another subject. The journey checks that the test
   user cannot see them.
5. **Realm.** Configured twice with `kcadm` inside the Keycloak container: realm, public
   PKCE client, audience mapper and users `alice` and `bob`. The users must keep their
   ids between the two runs.
6. **Browser.** Playwright: the Chromium journey, then Firefox and WebKit smoke.
7. **Evidence.** Copied to `FACTORY_EVIDENCE_DIR` (default `test-results/<run>/`): HTML
   and JSON reports, screenshots, video, failure traces, compose logs and `run.json`.
8. **Cleanup.** A trap on exit, interrupt or termination stops the running step, then
   runs `compose down -v` for **this run's project only** and removes this run's images.
   It ignores further interrupts while doing so, and runs `compose down` in its own
   session (`setsid`). A second Ctrl-C or TERM, even one sent to the whole process
   group, cannot abort it.

**A direct `npx playwright test`.** It is not the canonical path, but it works on its
own:
- With `NOTES_BASE_URL` unset, `tests/e2e/global-setup.ts` brings up an isolated stack
  with `scripts/stack.sh up`. It uses the same pieces as above: project `notes-pw-<run>`,
  owner `${GUARDKIT_RUN_OWNER:-notes-direct-<run>}`, a free loopback port, bounded
  readiness with the run-identity check, fresh migrations, the seed and the realm.
- The returned global teardown removes the stack after a pass, a failure or an interrupt,
  including an interrupt during the identity check that follows the start. An `up` that
  fails part-way removes what it started.
- Build, compose, migration and realm output goes to `.notes-runs/<run>/stack.log`, and
  only short status lines go to the console. A run that is killed (closed pipes) cannot
  stop its own teardown.
- Without a reachable Docker, the run is refused with a plain message that points to
  `scripts/check-feature.sh`.
- With `NOTES_BASE_URL` set, as `check-feature.sh` and the preview driver set it, nothing
  changes.

Every container, network and volume carries `guardkit.fixture.owner=$GUARDKIT_RUN_OWNER`,
with a run-unique fallback. If a killed run leaves anything behind, the factory's
disposable sweeps remove it. Deployments and previews use other labels
(`react-go-notes.deploy=<project>`, or `forge.preview.lease=<lease>` from Forge), so
those sweeps leave them alone.

## Deploy modes

What Forge sends `deploy/deploy.sh` in each leg is listed in `deploy/settings.sh`, read
from Forge `9abec6a9`. Exactly one signal is accepted; none, or two at once, is refused.

**Identity.** Forge's `DEPLOY_IDENTITY` is `<name>@<fingerprint>`, for example
`j-0123456789ab@969cd0c23004c8f7`. An image tag cannot carry `@`, so:
- images are tagged with `@` turned into `-`;
- the api container keeps the original text as the label `react-go-notes.identity`;
- `DEPLOYED_IDENTITY` and `RUNNING_IDENTITY` report the original text;
- the candidate teardown compares identities as tags.

A container without the label is read back from a tag of Forge's shape
(`<name>-<16 hex>` → `<name>@<16 hex>`), as api_test does. Anything else answers
`RUNNING_IDENTITY_UNKNOWN`.

| Signal (who sends it) | What `deploy.sh` does |
| --- | --- |
| `CANDIDATE=1` + `candidate.env` + `DEPLOY_IDENTITY` (Forge's candidate check) | Runs the locked install (Forge lays the tree out from `git archive`, so nothing is installed yet), then builds and starts `notes-live-cand` on 8961 and prints `CHECKED_ARTIFACT=postgres=…,api=…,proxy=…`. A failed install, or a lockfile it would rewrite, stops the deploy before anything is built |
| `CANDIDATE_DOWN=1` + `candidate.env` + `DEPLOY_IDENTITY` (candidate teardown) | `down -v` of the **named** candidate project, and deletes its settings. Refuses if no project is named, if the project is `notes-live`, if there is no identity, or if another build's candidate is running there |
| `PROMOTE=1` + `DEPLOY_IDENTITY` + `DEPLOY_ARTIFACT` (promote) | Re-tags exactly the checked images and starts `notes-live` on 8960 with `--no-build`, then confirms the running images are the artifact. Refuses without an artifact; never rebuilds |
| `PREVIEW=1` + Forge's fixed preview overlay (preview up) | Runs the locked install first, as for a candidate. Then the retained preview: both listeners, both issuers, the password-showing hint, settings in the preview's tree. Images are named `preview-<lease>`. Refuses the project name `notes-live` |
| `RUNNING_IDENTITY=1` (what is running) | Read-only. Prints `RUNNING_IDENTITY=<identity>` or `none` |
| `TEARDOWN=1` (manual use and the controls) | `down -v` of the **named** `COMPOSE_PROJECT_NAME` only, never `notes-live` |
| `REVERT=1` | Refused: this profile keeps no rollback image |

The live port (8960) and project (`notes-live`) live in `deploy/settings.sh`, because the
promote leg carries no overlay. The health check picks its target from the settings
Forge passes it: `candidate.env`, the preview overlay, or nothing after a promote.

## Memory

Every service has a hard memory limit, with no extra swap:

| Service | Limit |
| --- | --- |
| Keycloak | 1 GiB, with its heap capped at 512 MiB through `JAVA_OPTS_KC_HEAP` |
| PostgreSQL | 512 MiB |
| API | 256 MiB |
| Caddy | 128 MiB |

One stack therefore stays under 2 GiB, which fits a preview in a 12 GiB sandbox VM.
Without the heap cap, Keycloak's JVM would size its heap from the host's memory.

## Images and network needs

The three built images (postgres init, API and proxy) only copy files: there are no
`RUN` steps. The web bundle and the static Go binary are built beforehand by
`scripts/build-artifacts.sh` with the declared toolchain, for the Docker engine's
architecture. So image builds need no npm or Go proxy access inside Docker.

A sandbox without container-registry access needs these base images loaded into its
engine beforehand:

- `postgres:17.7-alpine@sha256:bb377b7239d2774ac8cc76f481596ce96c5a6b5e9d141f6d0a0ee371a6e7c0f2`
- `quay.io/keycloak/keycloak:26.7.4@sha256:82a77884f3af238beab1e7afd63b5f530e1b5c0590bd7aa60b40a40463e29b2c`
- `caddy:2.10.2-alpine@sha256:4c6e91c6ed0e2fa03efd5b44747b625fec79bc9cd06ac5235a779726618e530d`
- `gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab`

`scripts/install.sh` needs the npm registry and the Go module proxy, or warm caches. It
fails if installing changed `apps/api/go.mod`, `apps/api/go.sum` or `package-lock.json`.
That matters because `go mod download` would otherwise quietly re-add a dropped `go.sum`
line.

## The sandbox it deploys into

`deploy/profile.yaml` names the factory's one shared sandbox, `api-test-deploy`, the same
sandbox the factory's `guardkit/api_test` reference project uses. This fixture's ports in
it are:

| Port | Use |
| --- | --- |
| 8960 | live app |
| 8961 | candidate |
| 8950, 8951 | preview slots (public listener) |
| 8970, 8971 | preview internal listeners (not published) |

None of them collides with api_test's ports (8901, 8902, 8924, 8925).

The factory's build runner and deploy helper in that sandbox belong to the sandbox, not to
this repository, so this profile does not declare them.

## Verification so far (12–13 October 2026)

- **Native checks.** `scripts/install.sh` and `scripts/check.sh` pass with the pinned
  toolchains. That covers the Go token tests with locally generated keys, the contract
  tests, Vitest and the live-gate driver's verdict tests. ShellCheck is clean.
  - Every `deploy.sh` mode was exercised through a recording docker shim.
  - The proxy's link-token and forwarded-header rules were exercised with a real Caddy
    2.10.2 binary.
- **Connected checks, in the factory's runner image.**
  - `scripts/check-feature.sh` passes end to end on a real Docker engine: compose up,
    migrations on PostgreSQL, the realm configured twice, Playwright in three browsers,
    evidence and cleanup.
  - The factory's adverse controls fail it for the right reason: a broken route, a broken
    migration, a wrong audience, an unreachable Keycloak, and a disconnected button.
  - Overlapping runs, including two with one owner, and a TERM mid-run, stay isolated and
    clean up after themselves.
- **One cleanup gap, in the factory's sweep.** After a SIGKILL, the factory's restart
  sweep currently removes the run's containers but not its network and volume. That is
  pending work in the factory, not in this fixture.

**Not yet run:**
- `deploy/deploy.sh` against a real engine (it has only run through the shim);
- the driver's pass path against a live preview;
- the sweep that must keep a live preview while removing a killed run;
- sign-in from another machine on the private network.
