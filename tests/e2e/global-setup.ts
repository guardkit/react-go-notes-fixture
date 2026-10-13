import { spawnSync } from "node:child_process";
import path from "node:path";
import { request } from "@playwright/test";

const root = path.resolve(import.meta.dirname, "..", "..");

// scripts/check-feature.sh (the canonical connected check) and the preview
// driver start the stack themselves and set NOTES_BASE_URL. A DIRECT
// `npx playwright test` has no stack, so it gets its own isolated one
// (scripts/stack.sh), whose settings reach the workers through process.env,
// and the returned teardown removes it after the run, passed or failed.
function startOwnStack(): () => Promise<void> {
  console.log(
    "No NOTES_BASE_URL: starting an isolated stack for this direct run " +
      "(scripts/check-feature.sh is the canonical way to run browser journeys).",
  );
  const up = spawnSync("bash", ["scripts/stack.sh", "up"], {
    cwd: root,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "inherit"],
    timeout: 600_000,
  });
  if (up.status !== 0) {
    throw new Error(
      `could not start a stack for this direct Playwright run (exit ${up.status ?? up.signal}); ` +
        "see the output above, or run bash scripts/check-feature.sh",
    );
  }
  let state = "";
  for (const line of up.stdout.split("\n")) {
    const match = /^([A-Z0-9_]+)=(.*)$/.exec(line);
    if (!match) continue;
    const [, key, value] = match as unknown as [string, string, string];
    if (key === "NOTES_STACK_STATE") state = value;
    else process.env[key] = value;
  }
  if (!state || !process.env.NOTES_BASE_URL) {
    throw new Error("scripts/stack.sh up did not report its stack");
  }
  return () => {
    const down = spawnSync("bash", ["scripts/stack.sh", "down", state], {
      cwd: root,
      stdio: "inherit",
      timeout: 300_000,
    });
    if (down.status !== 0) {
      return Promise.reject(
        new Error(
          `removing the direct run's stack failed (exit ${down.status ?? down.signal})`,
        ),
      );
    }
    return Promise.resolve();
  };
}

// Refuses to test an instance that is not this run's: both the proxy's
// runtime settings and the API's health must report NOTES_RUN_ID.
async function checkIdentity(): Promise<void> {
  const baseURL = process.env.NOTES_BASE_URL;
  const runId = process.env.NOTES_RUN_ID;
  if (!baseURL || !runId) {
    throw new Error("NOTES_BASE_URL and NOTES_RUN_ID are required");
  }
  const api = await request.newContext({ baseURL });
  try {
    const config = await api.get("/config.json");
    const health = await api.get("/api/health");
    const configBody = (await config.json()) as { runId?: unknown };
    const healthBody = (await health.json()) as { run?: unknown };
    if (!config.ok() || configBody.runId !== runId) {
      throw new Error(`proxy at ${baseURL} is not run ${runId}`);
    }
    if (!health.ok() || healthBody.run !== runId) {
      throw new Error(`API behind ${baseURL} is not run ${runId}`);
    }
  } finally {
    await api.dispose();
  }
}

// The teardown runs at most once: an interrupt and a failed identity check
// can both ask for it, and the second call must not try a stack that is gone.
function once(fn: () => Promise<void>): () => Promise<void> {
  let running: Promise<void> | undefined;
  return () => (running ??= fn());
}

export default async function globalSetup(): Promise<
  (() => Promise<void>) | undefined
> {
  const teardown = process.env.NOTES_BASE_URL
    ? undefined
    : once(startOwnStack());
  // Until Playwright holds the teardown, an interrupt here would leave the
  // stack behind: remove it on SIGINT or SIGTERM, then let the signal end
  // the run as usual.
  const onSignal = (signal: NodeJS.Signals): void => {
    removeListeners();
    teardown?.().then(
      () => process.kill(process.pid, signal),
      (error: unknown) => {
        console.error("removing the direct run's stack failed:", error);
        process.kill(process.pid, signal);
      },
    );
  };
  const removeListeners = (): void => {
    process.off("SIGINT", onSignal);
    process.off("SIGTERM", onSignal);
  };
  if (teardown) {
    process.once("SIGINT", onSignal);
    process.once("SIGTERM", onSignal);
  }
  try {
    await checkIdentity();
  } catch (error) {
    removeListeners();
    try {
      await teardown?.();
    } catch (teardownError) {
      console.error(
        "removing the direct run's stack also failed:",
        teardownError,
      );
    }
    throw error;
  }
  removeListeners();
  return teardown;
}
