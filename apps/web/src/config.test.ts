import { describe, expect, it } from "vitest";
import { isSignInCallback } from "./auth";
import { loadConfig, parseConfig } from "./config";

const settings = {
  realm: "notes-20261012-abc123",
  clientId: "notes-web",
  runId: "20261012-abc123",
  signInHint: "Test users: alice and bob",
};

describe("runtime settings", () => {
  it("accepts the proxy's settings", async () => {
    const config = await loadConfig((url) => {
      expect(url).toBe("/config.json");
      return Promise.resolve(Response.json(settings));
    });
    expect(config).toEqual(settings);
  });

  it("refuses unsafe or missing values", () => {
    expect(() => parseConfig({ ...settings, realm: "../master" })).toThrow(
      "realm",
    );
    expect(() => parseConfig({ ...settings, clientId: undefined })).toThrow(
      "clientId",
    );
    expect(() => parseConfig(null)).toThrow("Invalid runtime settings");
  });

  it("recognises only a sign-in redirect", () => {
    expect(isSignInCallback("?code=x&state=y")).toBe(true);
    expect(isSignInCallback("?error=access_denied&state=y")).toBe(true);
    expect(isSignInCallback("?code=x")).toBe(false);
    expect(isSignInCallback("")).toBe(false);
  });
});
