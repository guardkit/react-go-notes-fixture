import { expect, type Page, type Response } from "@playwright/test";

export type TestUser = { username: string; password: string };

// One note exactly as GET /api/notes answers it. The contract makes all four
// fields required of every note (contract/openapi.yaml, Note), so a member
// read back without one of them is a broken read path, not a different kind
// of note.
export type NoteWire = {
  id: string;
  label: string;
  createdAt: string;
  done: boolean;
};

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

// Resolves with the owner's own collection, read from the very /api/notes
// response the page asks for once it holds a token - the browser's own listing,
// so a spec checks the members the app was actually handed rather than a subset
// named in the spec. Must be called before the action that triggers the request.
export function ownerNotes(page: Page): Promise<NoteWire[]> {
  return page
    .waitForResponse(
      (response) =>
        response.request().method() === "GET" &&
        response.url().endsWith("/api/notes") &&
        response.status() === 200,
    )
    .then(async (response: Response) => {
      const body = (await response.json()) as { notes?: unknown };
      expect(body.notes, "the API answers with a notes array").toBeInstanceOf(
        Array,
      );
      return body.notes as NoteWire[];
    });
}
