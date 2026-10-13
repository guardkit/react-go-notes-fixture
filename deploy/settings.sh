# shellcheck shell=bash
# shellcheck disable=SC2034 # notes_live_*, mode: read by the scripts that source this
# Sourced by deploy/deploy.sh and deploy/healthcheck.sh: one place that maps
# what Forge sends a project step to a mode, a compose project, a port and a
# settings file. qa/live-gate-driver.mjs applies the same settings-file rule.
#
# What Forge sends (forge src/forge/deploy/stage.py, runbook_builder.py,
# executor/shell_steps.py, review_preview.py at 9abec6a9):
#   candidate check  CANDIDATE=1 + candidate.env + DEPLOY_IDENTITY
#   candidate down   CANDIDATE_DOWN=1 + candidate.env + DEPLOY_IDENTITY
#   promote          PROMOTE=1 + DEPLOY_IDENTITY + DEPLOY_ARTIFACT
#   revert           REVERT=1 (+ ROLLBACK_IMAGE_REF); this profile has none
#   what is running  RUNNING_IDENTITY=1
#   preview up       PREVIEW=1 + COMPOSE_PROJECT_NAME + FACTORY_PREVIEW_* +
#                    FACTORY_RESOURCE_LABEL=forge.preview.lease=<lease> +
#                    FACTORY_EVIDENCE_DIR
#   health checks    candidate.env (candidate), the preview overlay (preview),
#                    nothing (after a promote)
# TEARDOWN=1 is this project's own manual and control teardown.

# The live app: its project and its loopback port inside the sandbox.
notes_live_project="${NOTES_PROJECT:-notes-live}"
notes_live_port="${NOTES_LIVE_PORT:-8960}"

truthy() { [[ "${1:-}" =~ ^(1|true|yes)$ ]]; }

# Sets `mode` to one of: running candidate candidate_down promote revert
# preview teardown none. Two or more signals are refused.
deploy_mode() {
  local found=()
  truthy "${RUNNING_IDENTITY:-}" && found+=(running)
  truthy "${CANDIDATE:-}" && found+=(candidate)
  truthy "${CANDIDATE_DOWN:-}" && found+=(candidate_down)
  truthy "${PROMOTE:-}" && found+=(promote)
  truthy "${REVERT:-}" && found+=(revert)
  truthy "${TEARDOWN:-}" && found+=(teardown)
  if truthy "${PREVIEW:-}" || [[ -n "${FACTORY_PREVIEW_PORT:-}" ]]; then
    found+=(preview)
  fi
  ((${#found[@]} <= 1)) || die "more than one deploy mode is set (${found[*]}); refusing to guess"
  mode="${found[0]:-none}"
}

# The settings file for `project` in this invocation. A preview's live in its
# own laid-out tree (.notes-state/), which Forge removes with the preview, and
# are per lease; everything else under NOTES_STATE_DIR or ~/.local/state.
settings_file() {
  local project="$1" dir name="$1"
  if [[ "${FACTORY_RESOURCE_LABEL:-}" == forge.preview.lease=* ]]; then
    name="$project-${FACTORY_RESOURCE_LABEL#*=}"
    dir="${NOTES_STATE_DIR:-$root/.notes-state}"
  else
    dir="${NOTES_STATE_DIR:-$HOME/.local/state/react-go-notes}"
  fi
  [[ "$name" =~ ^[A-Za-z0-9._-]+$ ]] || die "invalid settings name: $name"
  printf '%s/%s.env' "$dir" "$name"
}
