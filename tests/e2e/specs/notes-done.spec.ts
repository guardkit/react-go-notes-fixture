import { expect, test } from "@playwright/test";
import { ownerNotes, signIn, signOut, testUser } from "./helpers";

// The done flag is decided by the column default of
// apps/api/migrations/0003_add_done.sql (BOOLEAN NOT NULL DEFAULT false) and
// read back verbatim by the store. Nothing in this slice lets a caller hand
// over a done value, so every note this owner owns - the one created below and
// any created before it - is answered not-done, and never answered without the
// field.
//
// The flag is not surfaced in the note list UI by this slice, so it is asserted
// at the GET /api/notes boundary the browser itself crosses: the members come
// from the very response the page asked for, which is how the list the person
// sees is populated.
test("the note list still loads, and every note it returns carries a done flag of false", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = `Done flag note ${testInfo.testId.slice(0, 8)} ${Date.now()}`;

  await page.goto("/");
  await expect(page.getByTestId("signin-hint")).toBeVisible();
  await signIn(page, user);

  const list = page.getByRole("list", { name: "Your notes" });
  // Notes owned by another subject never reach this list.
  await expect(list).not.toContainText("Seeded synthetic note");
  await expect(page.getByRole("alert")).toHaveCount(0);

  await page.getByLabel("New note").fill(label);
  await page.getByRole("button", { name: "Save" }).click();
  // The list still loads end-to-end now that every note carries the flag: the
  // saved note is rendered and nothing reported a load failure.
  await expect(list.getByText(label)).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);

  // Tokens live in memory, so the list is refetched from PostgreSQL only once
  // the page has signed in again: the response armed here is the GET /api/notes
  // the browser itself makes to populate the list.
  await page.reload();
  const fetched = ownerNotes(page);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
  const notes = await fetched;

  // The sweep below is only a check if the collection holds anything at all,
  // and it holds the note this spec just created through the browser.
  expect(
    notes.length,
    "the fetched list holds the owner's notes",
  ).toBeGreaterThan(0);
  const created = notes.filter((note) => note.label === label);
  expect(
    created,
    "the note created through the browser is in the fetched list",
  ).toHaveLength(1);

  // Every member of that collection is checked - not a subset named here.
  for (const [position, note] of notes.entries()) {
    const where = `note ${position + 1} of the fetched list`;
    expect(
      Object.prototype.hasOwnProperty.call(note, "done"),
      `${where} carries a done field at all`,
    ).toBe(true);
    expect(
      typeof note.done,
      `${where} carries done as a boolean, never as null or a string`,
    ).toBe("boolean");
    expect(note.done, `${where} is answered not-done`).toBe(false);
  }

  // The note created above, named out of the collection it was found in.
  expect(created[0]?.done, "the created note is answered not-done").toBe(false);

  // The rows the person is shown are the members that were just checked.
  await expect(list.getByRole("listitem")).toHaveCount(notes.length);

  await signOut(page);
});
