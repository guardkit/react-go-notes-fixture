import { test } from "@playwright/test";
import { signIn, signOut, testUser } from "./helpers";

test("sign-in smoke", async ({ page }) => {
  await page.goto("/");
  await signIn(page, testUser());
  await signOut(page);
});
