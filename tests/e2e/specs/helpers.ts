import { expect, type Page } from "@playwright/test";

export type TestUser = { username: string; password: string };

export function testUser(): TestUser {
  const username = process.env.E2E_USERNAME;
  const password = process.env.E2E_PASSWORD;
  if (!username || !password) {
    throw new Error("E2E_USERNAME and E2E_PASSWORD are required");
  }
  return { username, password };
}

// Signs in through the real Keycloak login page behind the same origin.
export async function signIn(page: Page, user: TestUser): Promise<void> {
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(
    /\/auth\/realms\/[^/]+\/protocol\/openid-connect\/auth/,
  );
  await page.locator("#username").fill(user.username);
  await page.locator("#password").fill(user.password);
  await page.locator("#kc-login").click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
}

export async function signOut(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
}
