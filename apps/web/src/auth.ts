import {
  InMemoryWebStorage,
  UserManager,
  WebStorageStateStore,
} from "oidc-client-ts";
import type { RuntimeConfig } from "./config";

// Authorization code flow with PKCE (S256, oidc-client-ts's default for the
// code flow). Tokens stay in memory only. The short-lived sign-in state (the
// PKCE verifier and state) must survive the redirect to Keycloak and back, so
// it lives in this tab's sessionStorage, never in shared localStorage.
export function createUserManager(
  config: RuntimeConfig,
  origin: string,
): UserManager {
  return new UserManager({
    authority: `${origin}/auth/realms/${config.realm}`,
    client_id: config.clientId,
    redirect_uri: `${origin}/`,
    post_logout_redirect_uri: `${origin}/`,
    response_type: "code",
    scope: "openid profile",
    userStore: new WebStorageStateStore({ store: new InMemoryWebStorage() }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
    automaticSilentRenew: false,
    monitorSession: false,
    loadUserInfo: false,
  });
}

export function isSignInCallback(search: string): boolean {
  const params = new URLSearchParams(search);
  return params.has("state") && (params.has("code") || params.has("error"));
}
