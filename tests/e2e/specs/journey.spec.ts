import {
  expect,
  test,
  type Locator,
  type Page,
  type Response,
} from "@playwright/test";
import { signIn, signOut, testUser } from "./helpers";

// The summary region delivered by TASK-NLTC-001: the paragraph rendered
// directly above the "Your notes" list, holding the size of the very same
// array the list maps over.
const SUMMARY = "notes-count";

// The number in the summary's "N notes" text. NaN if it is in no such format,
// which then fails every comparison it is used in.
async function displayedTotal(summary: Locator): Promise<number> {
  const text = (await summary.textContent()) ?? "";
  const match = /^(\d+) notes$/.exec(text);
  return match ? Number(match[1]) : Number.NaN;
}

// Resolves with the size of the owner's own collection, counted from the very
// /api/notes response the page asks for once it holds a token. Must be called
// before the action that triggers the request.
function ownerCollection(page: Page): Promise<number> {
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
      return (body.notes as unknown[]).length;
    });
}

// Asserts the total is displayed above the notes list in the format "N notes",
// that N is the number of notes the list actually renders, and that the total
// belongs to the summary region rather than to any note item.
async function expectTotalAboveList(
  summary: Locator,
  list: Locator,
  expected: number,
): Promise<void> {
  await expect(summary).toHaveText(`${expected} notes`);
  await expect(summary).toHaveText(/^\d+ notes$/);
  // The same collection populates both: the digits shown are the number of
  // notes shown in the list below.
  await expect(list.getByRole("listitem")).toHaveCount(expected);
  expect(await displayedTotal(summary)).toBe(
    await list.getByRole("listitem").count(),
  );
  // The total is a summary of the collection, never part of a note item.
  await expect(list.getByTestId(SUMMARY)).toHaveCount(0);
  await expect(
    list.getByText(`${expected} notes`, { exact: true }),
  ).toHaveCount(0);
  // Above the list: the summary is the list's immediately preceding sibling,
  // and it is painted higher on the page.
  await expect(
    list.locator(`xpath=preceding-sibling::*[1][@data-testid='${SUMMARY}']`),
  ).toHaveCount(1);
  const summaryBox = await summary.boundingBox();
  const listBox = await list.boundingBox();
  if (!summaryBox || !listBox) {
    throw new Error("the summary and the notes list must both be rendered");
  }
  expect(summaryBox.y + summaryBox.height).toBeLessThanOrEqual(listBox.y);
}

// The tab title is the same count at the browser's own surface, and exactly
// "Notes (N)" rather than a title that merely contains it, so no other text
// can hide in it. Read once the total above the list has settled: the title
// and the list are committed from the one collection in the one render.
async function expectTabTitle(page: Page, expected: number): Promise<void> {
  const title = await page.title();
  expect(title, "the tab title is the count and nothing else").toBe(
    `Notes (${expected})`,
  );
  expect(title).toMatch(/^Notes \(\d+\)$/);
}

// A marker held on the window object, which only loading a page again can
// clear. Armed before an action and checked after it, it proves the tab title
// moved with the note itself and not with a fresh page.
const NO_RELOAD = "__notesNoReload";

async function armNoReload(page: Page): Promise<void> {
  await page.evaluate((key: string) => {
    (window as unknown as Record<string, unknown>)[key] = true;
  }, NO_RELOAD);
}

function noReloadArmed(page: Page): Promise<boolean> {
  return page.evaluate(
    (key: string) =>
      (window as unknown as Record<string, unknown>)[key] === true,
    NO_RELOAD,
  );
}

test("sign in, create a note, reload, sign out, anonymous API refused", async ({
  page,
  playwright,
  baseURL,
}, testInfo) => {
  const user = testUser();
  const label = `Fixture note ${testInfo.testId.slice(0, 8)} ${Date.now()}`;

  await page.goto("/");
  await expect(page.getByTestId("signin-hint")).toBeVisible();
  const listedAtSignIn = ownerCollection(page);
  await signIn(page, user);
  const list = page.getByRole("list", { name: "Your notes" });
  const summary = page.getByTestId(SUMMARY);
  const seeded = await listedAtSignIn;
  // Seeded notes belong to another owner and must not be shown.
  await expect(list).not.toContainText("Seeded synthetic note");
  await expect(page.getByRole("alert")).toHaveCount(0);

  // The total above the list is the size of this owner's own collection.
  await expectTotalAboveList(summary, list, seeded);
  // The tab title carries that same count from the first showing of the list.
  await expectTabTitle(page, seeded);
  await page.screenshot({
    path: testInfo.outputPath("notes-count-before-create.png"),
  });
  await testInfo.attach("notes-count-before-create", {
    path: testInfo.outputPath("notes-count-before-create.png"),
    contentType: "image/png",
  });

  await armNoReload(page);
  await page.getByLabel("New note").fill(label);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(list.getByText(label)).toBeVisible();
  // One note added to the collection is one note more above the list.
  await expectTotalAboveList(summary, list, seeded + 1);
  // The note moved the tab title with it on the page already open: the marker
  // armed before the save is still there, so nothing reloaded the page.
  await expectTabTitle(page, seeded + 1);
  expect(await noReloadArmed(page)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("after-create.png") });
  await testInfo.attach("after-create", {
    path: testInfo.outputPath("after-create.png"),
    contentType: "image/png",
  });

  // Tokens live in memory, so a reload signs in again through Keycloak's
  // session; the note must come back from PostgreSQL.
  await page.reload();
  // The reload clears the marker: the check made of it above was a real one.
  expect(await noReloadArmed(page)).toBe(false);
  const listedAfterReload = ownerCollection(page);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
  await expect(list.getByText(label)).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
  // The reloaded view's total is the reloaded collection's size, so the count
  // and the list come back from the same source together.
  const persisted = await listedAfterReload;
  expect(persisted).toBe(seeded + 1);
  await expectTotalAboveList(summary, list, persisted);
  // A reloaded page titles its tab from the collection it reloaded, so the
  // tab and the list come back from the one source together.
  await expectTabTitle(page, persisted);
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
