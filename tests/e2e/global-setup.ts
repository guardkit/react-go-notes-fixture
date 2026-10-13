import { request } from "@playwright/test";

// Refuses to test an instance that is not this run's: both the proxy's
// runtime settings and the API's health must report NOTES_RUN_ID.
export default async function globalSetup(): Promise<void> {
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
