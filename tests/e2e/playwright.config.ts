import path from "node:path";
import { defineConfig, devices } from "@playwright/test";

// The stack is started by scripts/check-feature.sh (the canonical path) or
// by Forge's preview leg, and addressed through NOTES_BASE_URL. A direct
// `npx playwright test` without it gets its own isolated stack from
// global-setup.ts, which sets NOTES_BASE_URL before the workers load this
// file again. There is deliberately no `webServer` block, so Playwright never
// reuses a server it did not start for this run (the design's
// `reuseExistingServer: false`); global-setup.ts also refuses to run unless
// the responding instance reports this run's identity.
const baseURL = process.env.NOTES_BASE_URL;
const evidence = path.resolve(
  process.env.FACTORY_EVIDENCE_DIR ??
    path.join(import.meta.dirname, "..", "..", "test-results"),
);
// The key journey is always recorded; FACTORY_RECORD_VIDEO=1 records the
// cross-browser smoke too.
const recordAll = process.env.FACTORY_RECORD_VIDEO === "1";
const smokeVideo = recordAll ? "on" : "retain-on-failure";

export default defineConfig({
  testDir: "./specs",
  globalSetup: "./global-setup.ts",
  forbidOnly: true,
  retries: 0,
  workers: 1,
  fullyParallel: false,
  timeout: 120_000,
  expect: { timeout: 20_000 },
  outputDir: path.join(evidence, "test-output"),
  reporter: [
    ["list"],
    [
      "html",
      { outputFolder: path.join(evidence, "playwright-report"), open: "never" },
    ],
    ["json", { outputFile: path.join(evidence, "results.json") }],
  ],
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "on",
    ignoreHTTPSErrors: false,
  },
  projects: [
    {
      name: "chromium-journey",
      testMatch: /journey\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], video: "on" },
    },
    {
      // The done flag is asserted at the GET /api/notes boundary the browser
      // crosses, so it runs on the same browser as the journey.
      name: "chromium-notes-done",
      testMatch: /notes-done\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], video: "retain-on-failure" },
    },
    {
      // The same flag at the surface the person reads: the rendered rows of the
      // notes list. It runs on the same browser as the boundary check above, so
      // the two halves of the claim are made of one stack.
      name: "chromium-notes-done-visibility",
      testMatch: /notes-done-visibility\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], video: "retain-on-failure" },
    },
    {
      // The same flag as the person changes it: the tick in the row, the PATCH
      // it sends, and the state the list shows after the page is reloaded. It
      // runs on the same browser as the two checks above, so the flag the
      // control writes, the flag the row shows and the flag the boundary
      // answers are all read off one stack.
      name: "chromium-notes-done-toggle",
      testMatch: /notes-done-toggle\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], video: "retain-on-failure" },
    },
    {
      name: "firefox-smoke",
      testMatch: /smoke\.spec\.ts/,
      use: { ...devices["Desktop Firefox"], video: smokeVideo },
    },
    {
      name: "webkit-smoke",
      testMatch: /smoke\.spec\.ts/,
      use: { ...devices["Desktop Safari"], video: smokeVideo },
    },
  ],
});
