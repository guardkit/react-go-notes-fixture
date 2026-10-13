// Runtime settings served by the proxy at /config.json, so one built bundle
// serves every run, realm and address. Nothing secret is ever in the bundle.
export type RuntimeConfig = {
  realm: string;
  clientId: string;
  runId: string;
  signInHint: string;
};

type Fetcher = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

function text(value: object, key: string, pattern?: RegExp): string {
  const field: unknown = (value as Record<string, unknown>)[key];
  if (typeof field !== "string" || (pattern && !pattern.test(field))) {
    throw new Error(`Invalid runtime setting: ${key}`);
  }
  return field;
}

export function parseConfig(value: unknown): RuntimeConfig {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error("Invalid runtime settings");
  }
  const name = /^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$/;
  return {
    realm: text(value, "realm", name),
    clientId: text(value, "clientId", name),
    runId: text(value, "runId"),
    signInHint: text(value, "signInHint"),
  };
}

export async function loadConfig(
  fetcher: Fetcher = fetch,
): Promise<RuntimeConfig> {
  const response = await fetcher("/config.json", { cache: "no-store" });
  if (!response.ok) throw new Error("Runtime settings unavailable");
  const body: unknown = await response.json();
  return parseConfig(body);
}
