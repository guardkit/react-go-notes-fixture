import { expect, test } from "@playwright/test";
import { signIn, signOut, testUser } from "./helpers";

test("sign in, create a note, reload, sign out, anonymous API refused", async ({
  page,
  playwright,
  baseURL,
}, testInfo) => {
  const user = testUser();
  const label = `Fixture note ${testInfo.testId.slice(0, 8)} ${Date.now()}`;

  await page.goto("/");
  await expect(page.getByTestId("signin-hint")).toBeVisible();
  await signIn(page, user);
  const list = page.getByRole("list", { name: "Your notes" });
  // Seeded notes belong to another owner and must not be shown.
  await expect(list).not.toContainText("Seeded synthetic note");
  await expect(page.getByRole("alert")).toHaveCount(0);

  await page.getByLabel("New note").fill(label);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(list.getByText(label)).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("after-create.png") });
  await testInfo.attach("after-create", {
    path: testInfo.outputPath("after-create.png"),
    contentType: "image/png",
  });

  // Tokens live in memory, so a reload signs in again through Keycloak's
  // session; the note must come back from PostgreSQL.
  await page.reload();
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
  await expect(list.getByText(label)).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await page.screenshot({ path: testInfo.outputPath("after-reload.png") });
  await testInfo.attach("after-reload", {
    path: testInfo.outputPath("after-reload.png"),
    contentType: "image/png",
  });

  await signOut(page);
  // After sign-out Keycloak's session is gone: signing in shows its login page.
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.locator("#username")).toBeVisible();

  const anonymous = await playwright.request.newContext({ baseURL });
  try {
    const listed = await anonymous.get("/api/notes");
    expect(listed.status()).toBe(401);
    const created = await anonymous.post("/api/notes", {
      data: { label: "anonymous" },
    });
    expect(created.status()).toBe(401);
    const forged = await anonymous.get("/api/notes", {
      headers: { authorization: "Bearer not-a-token" },
    });
    expect(forged.status()).toBe(401);
  } finally {
    await anonymous.dispose();
  }
});
