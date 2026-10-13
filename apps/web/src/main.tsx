import { createRoot } from "react-dom/client";
import type { User } from "oidc-client-ts";
import { App } from "./App";
import { createUserManager, isSignInCallback } from "./auth";
import { loadConfig } from "./config";

async function start(root: HTMLElement) {
  const config = await loadConfig();
  const manager = createUserManager(config, window.location.origin);
  let user: User | null = null;
  let error = "";
  if (isSignInCallback(window.location.search)) {
    try {
      user = await manager.signinRedirectCallback();
    } catch {
      error = "Sign-in did not complete. Please try again.";
    }
    window.history.replaceState(null, "", "/");
  }
  createRoot(root).render(
    <App
      config={config}
      manager={manager}
      initialUser={user}
      initialError={error}
    />,
  );
}

const root = document.getElementById("root");
if (!root) throw new Error("Missing app root");
start(root).catch(() => {
  root.textContent = "The notes app could not start.";
});
