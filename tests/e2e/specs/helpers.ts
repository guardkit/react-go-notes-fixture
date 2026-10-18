import {
  expect,
  type APIRequestContext,
  type Locator,
  type Page,
  type Route,
} from "@playwright/test";

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

// One member of the collection as GET /api/notes answers it. The done flag is
// optional in this type because the view treats a member answered without one
// as not done; every check that reads the flag reads it through `isDoneWire`.
export type NoteWire = {
  id: string;
  label: string;
  createdAt: string;
  done?: boolean;
};

export function isDoneWire(note: NoteWire): boolean {
  return note.done === true;
}

// The list the person reads, and the rows in it, addressed by role so a spec
// checks what is shown rather than the markup that shows it.
export function notesList(page: Page): Locator {
  return page.getByRole("list", { name: "Your notes" });
}

export function noteRows(page: Page): Locator {
  return notesList(page).getByRole("listitem");
}

// The one row showing a title the caller made itself, so the title is unique
// to this run. A spec that sweeps rows by title first proves the address names
// exactly one row, which is what makes the sweep cover the whole list.
export function noteRow(page: Page, label: string): Locator {
  return noteRows(page).filter({ hasText: label });
}

// Creates a note through the app's own form - the "New note" field and the
// "Save" button - and waits for the row it adds. This is the only flow the app
// has for making a note.
export async function createNote(page: Page, label: string): Promise<void> {
  await page.getByLabel("New note").fill(label);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(noteRow(page, label)).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
}

// Resolves with the members of the next GET /api/notes the page is answered.
// Must be armed before the action that triggers the request.
export function ownerNotes(page: Page): Promise<NoteWire[]> {
  return page
    .waitForResponse(
      (response) =>
        response.request().method() === "GET" &&
        response.url().endsWith("/api/notes") &&
        response.status() === 200,
    )
    .then(async (response) => {
      const body = (await response.json()) as { notes?: unknown };
      expect(body.notes, "the API answers with a notes array").toBeInstanceOf(
        Array,
      );
      return (body.notes as NoteWire[]).map((note) => note);
    });
}

// The word a row shows for each state. Both carry the same noun, so a check
// that one is shown cannot be satisfied by the other.
export const DONE_WORD = "Done";
export const OPEN_WORD = "Not done";

// The text a status cell carries: the state word, with any decorative glyph the
// row hides from assistive technology in front of it.
const STATE_TEXT = /^(?:\S+\s+)?(?:Not done|Done)$/;

// The status cell one row renders: the row's own cell that says which state the
// note is in. Located by what it says rather than by which child of the row it
// is, so the checks that pin its position are checks and not assumptions.
export function doneIndicator(row: Locator): Locator {
  return row.locator("> span").filter({ hasText: STATE_TEXT }).first();
}

// The state word the row's status cell shows, with the decorative glyph (the
// one marked aria-hidden, so it carries no meaning on its own) taken out. A
// cell that shows neither state word, or shows both, is not a status cell and
// fails here rather than reading as a state the note is not in.
export async function statusWord(row: Locator): Promise<string> {
  const cell = doneIndicator(row);
  await expect(cell, "the row shows a status cell").toHaveCount(1);
  const decorative = (
    await cell.locator("[aria-hidden='true']").allTextContents()
  ).join("");
  const shown = ((await cell.textContent()) ?? "").replace(decorative, " ");
  const word = shown.replace(/\s+/g, " ").trim();
  expect(
    word,
    `the row's status cell shows one state word, not "${shown}"`,
  ).toMatch(/^(?:Done|Not done)$/);
  return word;
}

// The cell a row gives the note's own title.
export function titleCell(row: Locator): Locator {
  return row.locator("> span").filter({ hasNotText: STATE_TEXT }).first();
}

// The glyph the status cell shows beside the state word, if it hides one from
// assistive technology. Two states that show the same glyph differ by less than
// a state word does.
export async function statusGlyph(row: Locator): Promise<string> {
  const cell = doneIndicator(row);
  return (
    (await cell.locator("[aria-hidden='true']").allTextContents()).join("") ??
    ""
  )
    .replace(/\s+/g, " ")
    .trim();
}

// The members of a GET /api/notes this spec answered, handed back to the test
// so it sweeps the very collection the app was given rather than one named in
// the spec.
export type ServedNotes = {
  // Resolves with the members of the next GET /api/notes answered by this
  // handler. Must be called before the action that triggers the request,
  // exactly like ownerNotes.
  next(): Promise<NoteWire[]>;
};

// Answers the browser's own GET /api/notes with the members the API really
// returned, the done flag set on the members whose title is named here.
//
// This is the only honest way to put a done row in front of a browser in this
// slice: the flag is storage's own - POST /api/notes never takes it as a
// parameter (apps/api/internal/notes/handler.go) and the column is NOT NULL
// DEFAULT false (apps/api/migrations/0003_add_done.sql) - so no flow the person
// or a spec can use writes a note already done. The request, the token, the
// response parsing and the rendering all stay the app's own; only the one field
// no write path can reach is set on the way back, and it is set on the members
// the API itself just handed over.
//
// Nothing is written to storage with the flag set, which is what the untouched-
// flag check asserts: it reads the same table through an out-of-band request
// this handler never sees.
export async function serveNotesWithDoneFlags(
  page: Page,
  doneLabels: readonly string[],
): Promise<ServedNotes> {
  const done = new Set(doneLabels);
  let waiters: Array<(notes: NoteWire[]) => void> = [];
  await page.route("**/api/notes", async (route: Route) => {
    if (route.request().method() !== "GET") {
      // The form's own POST stays the app's own: this handler only ever
      // answers the read the list is populated from.
      await route.fallback();
      return;
    }
    const response = await route.fetch();
    const body = (await response.json()) as { notes?: unknown };
    expect(
      body.notes,
      "the API answers the list with a notes array",
    ).toBeInstanceOf(Array);
    const notes = (body.notes as NoteWire[]).map((note: NoteWire) =>
      done.has(note.label) ? { ...note, done: true } : note,
    );
    const pending = waiters;
    waiters = [];
    for (const settle of pending) settle(notes);
    await route.fulfill({
      status: response.status(),
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ notes }),
    });
  });
  return {
    next: () =>
      new Promise<NoteWire[]>((resolve) => {
        waiters.push(resolve);
      }),
  };
}

// Reads the owner's collection straight from the API, outside the browser and
// therefore outside any route the spec armed: the stored state, as the list was
// populated from it. `authorization` is the header value the app itself used.
export async function readStoredCollection(
  request: APIRequestContext,
  authorization: string,
): Promise<NoteWire[]> {
  const response = await request.get("/api/notes", {
    headers: { authorization },
  });
  expect(response.status(), "the owner's own collection is readable").toBe(200);
  const body = (await response.json()) as { notes?: unknown };
  expect(body.notes, "the API answers with a notes array").toBeInstanceOf(
    Array,
  );
  return body.notes as NoteWire[];
}
