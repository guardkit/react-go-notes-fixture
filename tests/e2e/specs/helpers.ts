import {
  expect,
  type Locator,
  type Page,
  type Response,
  type Route,
} from "@playwright/test";

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
// has for making a note, and the flag the note is stored with is decided by the
// column default of apps/api/migrations/0003_add_done.sql, never by the form.
export async function createNote(page: Page, label: string): Promise<void> {
  await page.getByLabel("New note").fill(label);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(noteRow(page, label)).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
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
// slice: the flag is storage's own - Create never takes it as a parameter
// (apps/api/internal/notes/store.go) and the column is NOT NULL DEFAULT false -
// so no flow the person or a spec can use writes a note already done. The
// request, the token, the response parsing and the rendering all stay the
// app's own; only the one field no write path can reach is set on the way back,
// and it is set on the members the API itself just handed over.
//
// Nothing is written to storage with the flag set, so the API-boundary sweep of
// notes-done.spec.ts - which reads the same table through the browser's own
// unintercepted request - still finds every note answered not-done.
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
