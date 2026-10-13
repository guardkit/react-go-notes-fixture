// node --test qa/: the driver's verdict never reads pass with media missing.
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { after, test } from "node:test";
import { REPORT_REF, summariseReport } from "./journey-report.mjs";

const evidence = mkdtempSync(path.join(os.tmpdir(), "journey-report-"));
after(() => rmSync(evidence, { recursive: true, force: true }));

function file(relative, content = "data") {
  const full = path.join(evidence, relative);
  mkdirSync(path.dirname(full), { recursive: true });
  writeFileSync(full, content);
  return full;
}

file(REPORT_REF, "<html></html>");
const video = file("test-output/journey/video.webm");
const shot = file("test-output/journey/after-create.png");
const smokeShot = file("test-output/smoke/test-finished-1.png");
const emptyVideo = file("test-output/journey/empty.webm", "");

function report({
  journeyAttachments,
  smokeAttachments = [],
  journey = "expected",
  stats,
}) {
  return {
    suites: [
      {
        specs: [
          {
            title: "journey",
            tests: [
              {
                projectName: "chromium-journey",
                status: journey,
                results: [{ attachments: journeyAttachments }],
              },
            ],
          },
        ],
        suites: [
          {
            specs: [
              {
                title: "smoke",
                tests: [
                  {
                    projectName: "firefox-smoke",
                    status: "expected",
                    results: [{ attachments: smokeAttachments }],
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
    stats: stats ?? {
      expected: journey === "expected" ? 2 : 1,
      unexpected: journey === "expected" ? 0 : 1,
      skipped: 0,
      flaky: 0,
    },
    errors: [],
  };
}
const videoAttachment = (p = video) => ({
  name: "video",
  contentType: "video/webm",
  path: p,
});
const shotAttachment = (p = shot) => ({
  name: "screenshot",
  contentType: "image/png",
  path: p,
});

test("all evidence present and tests passed: pass", () => {
  const got = summariseReport({
    report: report({
      journeyAttachments: [videoAttachment(), shotAttachment()],
      smokeAttachments: [shotAttachment(smokeShot)],
    }),
    evidence,
    exitCode: 0,
  });
  assert.equal(got.verdict, "pass");
  assert.deepEqual(got.videos, ["test-output/journey/video.webm"]);
  assert.equal(got.screenshots.length, 2);
  assert.deepEqual(got.outcome, { passed: 2, failed: 0, skipped: 0, flaky: 0 });
});

test("key journey video attachment missing: instrument_fail", () => {
  const got = summariseReport({
    report: report({ journeyAttachments: [shotAttachment()] }),
    evidence,
    exitCode: 0,
  });
  assert.equal(got.verdict, "instrument_fail");
  assert.match(got.reason, /chromium-journey video/);
});

test("video listed but absent or empty on disk: instrument_fail", () => {
  for (const p of [path.join(evidence, "gone.webm"), emptyVideo]) {
    const got = summariseReport({
      report: report({
        journeyAttachments: [videoAttachment(p), shotAttachment()],
      }),
      evidence,
      exitCode: 0,
    });
    assert.equal(got.verdict, "instrument_fail", p);
  }
});

test("video outside the evidence folder does not count", () => {
  const outside = path.join(os.tmpdir(), "elsewhere.webm");
  writeFileSync(outside, "x");
  try {
    const got = summariseReport({
      report: report({
        journeyAttachments: [videoAttachment(outside), shotAttachment()],
      }),
      evidence,
      exitCode: 0,
    });
    assert.equal(got.verdict, "instrument_fail");
  } finally {
    rmSync(outside, { force: true });
  }
});

test("no screenshot at all: instrument_fail", () => {
  const got = summariseReport({
    report: report({ journeyAttachments: [videoAttachment()] }),
    evidence,
    exitCode: 0,
  });
  assert.equal(got.verdict, "instrument_fail");
  assert.match(got.reason, /a screenshot/);
});

test("HTML report missing: instrument_fail", () => {
  rmSync(path.join(evidence, REPORT_REF));
  try {
    const got = summariseReport({
      report: report({
        journeyAttachments: [videoAttachment(), shotAttachment()],
      }),
      evidence,
      exitCode: 0,
    });
    assert.equal(got.verdict, "instrument_fail");
    assert.match(got.reason, /HTML report/);
  } finally {
    file(REPORT_REF, "<html></html>");
  }
});

test("failed journey stays fail, and names missing media", () => {
  const got = summariseReport({
    report: report({ journeyAttachments: [], journey: "unexpected" }),
    evidence,
    exitCode: 1,
  });
  assert.equal(got.verdict, "fail");
  assert.match(got.reason, /also missing/);
});

test("unknown counts stay null and never read as pass", () => {
  const got = summariseReport({
    report: report({
      journeyAttachments: [videoAttachment(), shotAttachment()],
      stats: {},
    }),
    evidence,
    exitCode: 0,
  });
  assert.deepEqual(got.outcome, {
    passed: null,
    failed: null,
    skipped: null,
    flaky: null,
  });
  assert.equal(got.verdict, "fail");
});
