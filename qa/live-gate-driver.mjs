// The fixture's live-gate driver (deploy/profile.yaml live_gate.driver).
//
// Runs the same Playwright journey as scripts/check-feature.sh against an
// already running instance (a candidate, a preview's internal URL or the live
// app), keeps the evidence, and prints one results envelope as JSON on stdout
// for Forge's project_driver_answer. Everything else goes to stderr.
// Exit codes follow Forge's map: 0 pass, 1 fail, 3 instrument_fail,
// 4 environment_fail.
//
// Inputs (environment):
//   NOTES_BASE_URL        the instance's browser URL, e.g. http://127.0.0.1:41001.
//                         In a preview, FACTORY_PREVIEW_INTERNAL_PORT (always in
//                         Forge's fixed preview overlay) wins:
//                         http://127.0.0.1:<that port>
//   FACTORY_EVIDENCE_DIR  new or empty folder for the evidence (default
//                         qa/evidence/<time>); written once
//   E2E_USERNAME, E2E_PASSWORD, NOTES_RUN_ID
//                         or, when absent, read from the deploy settings file
//                         NOTES_STATE_FILE, or NOTES_STATE_DIR/<project>.env
//                         with project COMPOSE_PROJECT_NAME, NOTES_PROJECT or
//                         notes-live (the file deploy/deploy.sh writes; a
//                         preview's is <project>-<lease>.env in .notes-state/)
// The verdict and evidence come from qa/journey-report.mjs, which checks the
// media on disk: passing tests with the key journey's video, every screenshot
// or the HTML report missing are reported as instrument_fail, never pass.
// The envelope carries what Forge's review preview reads: app_url,
// report_ref, screenshot_refs, media_refs (video), trace_refs and outcome
// (passed, failed, skipped, flaky; null when unknown), with references
// relative to the evidence folder.
// Arguments: Forge appends --feature, --target and --gates; they are recorded.
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readdir, readFile, stat, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { REPORT_REF, summariseReport } from "./journey-report.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const EXIT = { pass: 0, fail: 1, instrument_fail: 3, environment_fail: 4 };
const runId = `live-${new Date().toISOString().replace(/[-:.]/g, "").slice(0, 15)}`;

function log(message) {
  process.stderr.write(`live-gate-driver: ${message}\n`);
}

function finish(envelope) {
  process.stdout.write(`${JSON.stringify(envelope)}\n`);
  process.exit(EXIT[envelope.verdict]);
}

function failed(verdict, reason, extra = {}) {
  log(reason);
  finish({
    verdict,
    run_id: runId,
    reason,
    app_url: null,
    evidence_index_ref: "",
    screenshot_refs: [],
    trace_refs: [],
    media_refs: [],
    outcome: { passed: null, failed: null, skipped: null, flaky: null },
    gates: [],
    ...extra,
  });
}

function argsRecord(argv) {
  const record = {};
  for (let i = 0; i < argv.length; i += 1) {
    const flag = argv[i];
    if (flag.startsWith("--")) {
      const value =
        argv[i + 1] && !argv[i + 1].startsWith("--") ? argv[(i += 1)] : "";
      record[flag.slice(2)] = value;
    }
  }
  return record;
}

async function settings() {
  const env = process.env;
  if (env.E2E_USERNAME && env.E2E_PASSWORD && env.NOTES_RUN_ID) {
    return {
      username: env.E2E_USERNAME,
      pw: env.E2E_PASSWORD,
      runId: env.NOTES_RUN_ID,
    };
  }
  // Same rule as deploy/settings.sh settings_file: a preview's settings are
  // per lease and live in its own laid-out tree (.notes-state/).
  let project = env.COMPOSE_PROJECT_NAME || env.NOTES_PROJECT || "notes-live";
  let dir =
    env.NOTES_STATE_DIR ||
    path.join(os.homedir(), ".local", "state", "react-go-notes");
  const lease = /^forge\.preview\.lease=(.+)$/.exec(
    env.FACTORY_RESOURCE_LABEL ?? "",
  );
  if (lease) {
    project = `${project}-${lease[1]}`;
    dir = env.NOTES_STATE_DIR || path.join(root, ".notes-state");
  }
  const file = env.NOTES_STATE_FILE || path.join(dir, `${project}.env`);
  const values = {};
  for (const line of (await readFile(file, "utf8")).split("\n")) {
    const at = line.indexOf("=");
    if (at > 0) values[line.slice(0, at)] = line.slice(at + 1);
  }
  if (!values.NOTES_TEST_PASSWORD || !values.NOTES_RUN_ID) {
    throw new Error(`${file} lacks NOTES_TEST_PASSWORD or NOTES_RUN_ID`);
  }
  return {
    username: "alice",
    pw: values.NOTES_TEST_PASSWORD,
    runId: values.NOTES_RUN_ID,
  };
}

async function answersAs(baseURL, expected) {
  const get = async (p) => {
    const response = await fetch(new URL(p, baseURL), {
      signal: AbortSignal.timeout(10_000),
    });
    if (!response.ok) throw new Error(`${p} answered ${response.status}`);
    return response.json();
  };
  const config = await get("/config.json");
  const health = await get("/api/health");
  if (config.runId !== expected || health.run !== expected) {
    throw new Error(`instance at ${baseURL} is not ${expected}`);
  }
}

async function filesUnder(dir) {
  const out = [];
  for (const entry of await readdir(dir, {
    withFileTypes: true,
    recursive: true,
  })) {
    if (entry.isFile()) out.push(path.join(entry.parentPath, entry.name));
  }
  return out.sort();
}

const args = argsRecord(process.argv.slice(2));
const previewPort = process.env.FACTORY_PREVIEW_INTERNAL_PORT;
if (previewPort !== undefined && !/^[0-9]{2,5}$/.test(previewPort)) {
  failed(
    "instrument_fail",
    "FACTORY_PREVIEW_INTERNAL_PORT must be a port number",
  );
}
const baseURL = previewPort
  ? `http://127.0.0.1:${previewPort}`
  : process.env.NOTES_BASE_URL;
if (!baseURL || !/^https?:\/\/[^/]+\/?$/.test(baseURL)) {
  failed(
    "instrument_fail",
    "NOTES_BASE_URL must be the instance's origin, e.g. http://127.0.0.1:41001",
  );
}

let user;
try {
  user = await settings();
} catch (error) {
  failed("instrument_fail", `no test sign-in settings: ${error.message}`);
}
try {
  await answersAs(baseURL, user.runId);
} catch (error) {
  failed(
    "environment_fail",
    `instance not reachable as expected: ${error.message}`,
    { app_url: baseURL },
  );
}

const evidence = path.resolve(
  process.env.FACTORY_EVIDENCE_DIR || path.join(root, "qa", "evidence", runId),
);
await mkdir(evidence, { recursive: true });
if ((await readdir(evidence)).length > 0) {
  failed(
    "instrument_fail",
    `evidence folder is not empty (written once): ${evidence}`,
  );
}

log(`running the journey against ${baseURL}; evidence in ${evidence}`);
const exitCode = await new Promise((resolve) => {
  const child = spawn(
    "npx",
    [
      "--no-install",
      "playwright",
      "test",
      "--config",
      "tests/e2e/playwright.config.ts",
    ],
    {
      cwd: root,
      env: {
        ...process.env,
        NOTES_BASE_URL: baseURL,
        NOTES_RUN_ID: user.runId,
        E2E_USERNAME: user.username,
        E2E_PASSWORD: user.pw,
        FACTORY_EVIDENCE_DIR: evidence,
        FACTORY_RECORD_VIDEO: "1",
      },
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  child.stdout.pipe(process.stderr);
  child.stderr.pipe(process.stderr);
  child.on("error", (error) => {
    log(`could not start Playwright: ${error.message}`);
    resolve(-1);
  });
  child.on("close", (code) => resolve(code ?? -1));
});

let report;
try {
  report = JSON.parse(
    await readFile(path.join(evidence, "results.json"), "utf8"),
  );
} catch {
  failed(
    "instrument_fail",
    `Playwright left no JSON report (exit ${exitCode})`,
    { app_url: baseURL },
  );
}

const rel = (file) => path.relative(evidence, file);
const summary = summariseReport({ report, evidence, exitCode });
const index = [];
for (const file of await filesUnder(evidence)) {
  const data = await readFile(file);
  index.push({
    path: rel(file),
    bytes: (await stat(file)).size,
    sha256: createHash("sha256").update(data).digest("hex"),
  });
}
await writeFile(
  path.join(evidence, "index.json"),
  `${JSON.stringify({ run_id: runId, app_url: baseURL, instance: user.runId, files: index }, null, 2)}\n`,
);

if (summary.reason) log(summary.reason);
finish({
  verdict: summary.verdict,
  run_id: runId,
  ...(summary.reason ? { reason: summary.reason } : {}),
  app_url: baseURL,
  instance: user.runId,
  arguments: args,
  evidence_dir: evidence,
  evidence_index_ref: "index.json",
  report_ref: REPORT_REF,
  screenshot_refs: summary.screenshots,
  trace_refs: summary.traces,
  media_refs: summary.videos,
  missing_evidence: summary.missing,
  outcome: summary.outcome,
  gates: [
    {
      gate_id: "browser-journey",
      exit_code: exitCode,
      assertions: summary.assertions,
    },
  ],
});
