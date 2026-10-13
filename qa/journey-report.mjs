// Reads a Playwright JSON report and the evidence folder into the parts of
// the live-gate envelope: per-test assertions, evidence references (relative
// to the evidence folder), outcome counts (unknown stays null) and the
// verdict. Media are checked on disk: the key journey's video and at least
// one screenshot must exist and be non-empty, as must the HTML report, or a
// run whose tests passed is reported as instrument_fail, never as pass.
import { statSync } from "node:fs";
import path from "node:path";

export const KEY_JOURNEY = "chromium-journey";
export const REPORT_REF = "playwright-report/index.html";

const STATUS = {
  expected: "pass",
  unexpected: "fail",
  flaky: "flaky",
  skipped: "skipped",
};

function collectTests(suite, out) {
  for (const spec of suite.specs ?? []) {
    for (const test of spec.tests ?? []) out.push({ title: spec.title, test });
  }
  for (const child of suite.suites ?? []) collectTests(child, out);
  return out;
}

function present(file) {
  try {
    const info = statSync(file);
    return info.isFile() && info.size > 0;
  } catch {
    return false;
  }
}

export function summariseReport({ report, evidence, exitCode }) {
  const inside = (file) => {
    const relative = path.relative(evidence, path.resolve(evidence, file));
    return relative && !relative.startsWith("..") && !path.isAbsolute(relative)
      ? relative
      : null;
  };
  const tests = (report.suites ?? []).flatMap((suite) =>
    collectTests(suite, []),
  );
  const screenshots = [];
  const traces = [];
  const videos = [];
  let keyJourneyVideo = false;
  const assertions = tests.map(({ title, test }) => {
    const own = { screenshots: [], traces: [], videos: [] };
    for (const result of test.results ?? []) {
      for (const attachment of result.attachments ?? []) {
        const ref = attachment.path ? inside(attachment.path) : null;
        if (!ref || !present(path.join(evidence, ref))) continue;
        if (attachment.contentType === "image/png") own.screenshots.push(ref);
        else if (attachment.name === "trace") own.traces.push(ref);
        else if (attachment.contentType?.startsWith("video/"))
          own.videos.push(ref);
      }
    }
    if (test.projectName === KEY_JOURNEY && own.videos.length > 0) {
      keyJourneyVideo = true;
    }
    screenshots.push(...own.screenshots);
    traces.push(...own.traces);
    videos.push(...own.videos);
    return {
      id: `${test.projectName} › ${title}`,
      status: STATUS[test.status] ?? "fail",
      evidence_ref: own.videos[0] ?? own.screenshots[0] ?? null,
    };
  });

  const stats = report.stats ?? {};
  const count = (value) =>
    Number.isInteger(value) && value >= 0 ? value : null;
  const outcome = {
    passed: count(stats.expected),
    failed: count(stats.unexpected),
    skipped: count(stats.skipped),
    flaky: count(stats.flaky),
  };

  const missing = [];
  if (!keyJourneyVideo) missing.push(`the ${KEY_JOURNEY} video`);
  if (screenshots.length === 0) missing.push("a screenshot");
  if (!present(path.join(evidence, REPORT_REF)))
    missing.push("the HTML report");

  const testsPassed =
    exitCode === 0 &&
    outcome.failed === 0 &&
    (outcome.passed ?? 0) > 0 &&
    (report.errors ?? []).length === 0;
  let verdict = "pass";
  let reason = null;
  if (!testsPassed) {
    verdict = "fail";
    reason = `the browser journey failed (exit ${exitCode})`;
  } else if (missing.length > 0) {
    verdict = "instrument_fail";
    reason = `the tests passed but the evidence is incomplete: missing ${missing.join(", ")}`;
  }
  if (verdict === "fail" && missing.length > 0) {
    reason += `; also missing ${missing.join(", ")}`;
  }
  return {
    verdict,
    reason,
    missing,
    assertions,
    screenshots,
    traces,
    videos,
    outcome,
  };
}
