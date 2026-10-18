import {
  expect,
  test,
  type Locator,
  type Page,
  type TestInfo,
} from "@playwright/test";
import {
  createNote,
  noteRow,
  noteRows,
  notesList,
  ownerNotes,
  serveNotesWithDoneFlags,
  signIn,
  signOut,
  testUser,
  type NoteWire,
  type TestUser,
} from "./helpers";

// The done flag at the surface the person actually reads: the rows of the
// notes list. notes-done.spec.ts pins the flag at the GET /api/notes boundary
// this list is populated from; these checks start where that one stops and
// assert what the rendered rows show - the word "Done" on a done row and
// nowhere else, and a row treatment a not-done row never shares.
//
// The rows are the ones the app renders for notes this spec made through the
// app's own form (helpers.createNote). Only the flag itself cannot come from
// the app: it is decided by the column default of
// apps/api/migrations/0003_add_done.sql and no flow writes it, so
// helpers.serveNotesWithDoneFlags sets that one field on the members the API
// itself handed back, on the way to the browser. Nothing is stored done, so
// notes-done.spec.ts still finds every note of this owner answered not-done
// when it reads the same table through an unintercepted request.

// The word that carries the state on its own, so the two states stay
// distinguishable without colour and to a screen reader.
const DONE_WORD = "Done";
// The row treatment the done note's row carries and a not-done row never does.
const DONE_ROW_CLASS = "note-row-done";

// What the eye tells apart in a row, read off the rendered row: the properties
// that carry the state without colour, plus the class the treatment is applied
// through and the number of "Done" cues the row shows.
type RowLook = {
  classes: string[];
  opacity: string;
  fontStyle: string;
  color: string;
  titleDecoration: string;
  doneCues: number;
};

// The properties a single scan of the list reads without relying on colour.
const EYE_KEYS = ["opacity", "fontStyle", "titleDecoration"] as const;
type EyeKey = (typeof EYE_KEYS)[number];

// The presentation a row has when nothing has been applied to it: what the
// browser makes of a plain list item. A not-done row keeps every one of these;
// a done row gives up at least one of them.
const STANDARD_ROW: Record<EyeKey, string> = {
  opacity: "1",
  fontStyle: "normal",
  titleDecoration: "none",
};

// The properties the two states are told apart by: never colour alone.
function eyeDifferences(a: RowLook, b: RowLook): EyeKey[] {
  return EYE_KEYS.filter((key) => a[key] !== b[key]);
}

function standardDifferences(look: RowLook): EyeKey[] {
  return EYE_KEYS.filter((key) => look[key] !== STANDARD_ROW[key]);
}

// Everything a row shows, as the browser rendered it. The row is addressed by
// the title it displays, and the count is pinned first: a sweep that walks the
// members the app was handed is only a sweep of the whole list if each title
// names exactly one row and the rows number the members.
async function lookAt(page: Page, label: string): Promise<RowLook> {
  const row = noteRow(page, label);
  await expect(row, `exactly one row shows "${label}"`).toHaveCount(1);
  const title = row.getByText(label, { exact: true });
  await expect(title, `"${label}" shows its title`).toHaveCount(1);
  const rendered = await row.evaluate((element: Element) => {
    const shown = getComputedStyle(element);
    return {
      classes: element.className.split(/\s+/).filter(Boolean),
      opacity: shown.opacity,
      fontStyle: shown.fontStyle,
      color: shown.color,
    };
  });
  return {
    ...rendered,
    titleDecoration: await title.evaluate(
      (element: Element) => getComputedStyle(element).textDecorationLine,
    ),
    doneCues: await row.getByText(DONE_WORD, { exact: true }).count(),
  };
}

// A row in the not-done presentation: no "Done" cue anywhere in it, none of the
// done row's treatment on it, and the browser's own styling kept.
async function expectNotDoneRow(page: Page, label: string): Promise<RowLook> {
  const look = await lookAt(page, label);
  expect(
    look.doneCues,
    `the not-done row "${label}" shows no "${DONE_WORD}" cue`,
  ).toBe(0);
  expect(
    look.classes,
    `the not-done row "${label}" carries no done treatment`,
  ).not.toContain(DONE_ROW_CLASS);
  for (const key of EYE_KEYS) {
    expect(
      look[key],
      `the not-done row "${label}" keeps the standard ${key}`,
    ).toBe(STANDARD_ROW[key]);
  }
  return look;
}

// A row in the done presentation: the word "Done" is shown, and the row is
// taken out of the standard presentation by something a scan reads without
// colour.
async function expectDoneRow(page: Page, label: string): Promise<RowLook> {
  const cue = noteRow(page, label).getByText(DONE_WORD, { exact: true });
  await expect(
    cue,
    `the done row "${label}" shows its "${DONE_WORD}" cue`,
  ).toHaveCount(1);
  await expect(cue).toBeVisible();
  const look = await lookAt(page, label);
  expect(look.doneCues, `the done row "${label}" shows exactly one cue`).toBe(
    1,
  );
  expect(
    look.classes,
    `the done row "${label}" carries the done treatment`,
  ).toContain(DONE_ROW_CLASS);
  expect(
    standardDifferences(look),
    `the done row "${label}" does not keep the standard presentation`,
  ).not.toHaveLength(0);
  return look;
}

// A title unique to this run, and free of the cue word so no assertion on the
// cue can be satisfied by a title of its own.
function uniqueLabel(testInfo: TestInfo, kind: string): string {
  return `visibility note ${testInfo.testId.slice(0, 8)}-${Date.now()} ${kind}`;
}

// Signs in and makes each note through the app's own form, so the rows checked
// below are the rows the person's own flow put in the list.
async function signInAndCreate(
  page: Page,
  user: TestUser,
  labels: string[],
): Promise<void> {
  await page.goto("/");
  await expect(page.getByTestId("signin-hint")).toBeVisible();
  await signIn(page, user);
  // The list is on the page the person is looking at. It is empty until the
  // first note is made below, so it is the element that is asserted here, not
  // its box - an empty list has nothing to paint.
  await expect(notesList(page)).toHaveCount(1);
  await expect(page.getByRole("alert")).toHaveCount(0);
  for (const label of labels) {
    await createNote(page, label);
  }
}

// The list as the person sees it on a fresh load. Tokens live in memory, so the
// rows come from the app's own GET /api/notes once it has signed in again
// through Keycloak's still-living session; `listed` must be armed before the
// reload, exactly as ownerNotes asks to be.
async function showListAgain(
  page: Page,
  user: TestUser,
  listed: Promise<NoteWire[]>,
): Promise<NoteWire[]> {
  await page.reload();
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
  await expect(page.getByRole("alert")).toHaveCount(0);
  return await listed;
}

// The rows the person is shown are exactly the members the app was handed: one
// row per member, and no member without a row.
async function expectRowsMatchMembers(
  page: Page,
  notes: NoteWire[],
): Promise<void> {
  await expect(
    noteRows(page),
    "the list shows one row for every member the app was handed",
  ).toHaveCount(notes.length);
  expect(
    notes.length,
    "the sweep below is only a sweep if the list holds members at all",
  ).toBeGreaterThan(0);
}

// Both rows on the page at once, so the difference between them is something a
// single scan of the list meets rather than one the person hunts for by
// scrolling from one to the other.
async function expectBothRowsInOneScan(
  page: Page,
  rows: Locator[],
): Promise<void> {
  const viewport = page.viewportSize();
  expect(viewport, "the list is shown in a viewport").not.toBeNull();
  for (const row of rows) {
    await expect(row).toBeVisible();
  }
  // The last of the rows is brought into view; the rest have to be seen with
  // it, or the two states do not meet the eye in one scan.
  await rows.at(-1)?.scrollIntoViewIfNeeded();
  for (const row of rows) {
    const box = await row.boundingBox();
    expect(box, "the row is painted").not.toBeNull();
    expect(box?.height, "the row has a height to scan").toBeGreaterThan(0);
    expect(box?.width, "the row has a width to scan").toBeGreaterThan(0);
    expect(box?.y, "the row starts inside the viewport").toBeGreaterThanOrEqual(
      0,
    );
    expect(
      (box?.y ?? 0) + (box?.height ?? 0),
      "the row ends inside the viewport: no scrolling between the rows",
    ).toBeLessThanOrEqual(viewport?.height ?? 0);
  }
}

test("a done row shows the Done label and is styled differently from the not-done row", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const doneLabel = uniqueLabel(testInfo, "alpha");
  const openLabel = uniqueLabel(testInfo, "beta");
  await signInAndCreate(page, user, [doneLabel, openLabel]);

  const handler = await serveNotesWithDoneFlags(page, [doneLabel]);
  const notes = await showListAgain(page, user, handler.next());
  await expectRowsMatchMembers(page, notes);

  const byLabel = new Map(notes.map((note) => [note.label, note]));
  expect(
    byLabel.get(doneLabel)?.done,
    "the done note reaches the browser marked done",
  ).toBe(true);
  expect(
    byLabel.get(openLabel)?.done,
    "the other note reaches the browser marked not-done",
  ).toBe(false);

  const done = await expectDoneRow(page, doneLabel);
  const open = await expectNotDoneRow(page, openLabel);

  // Styled differently from the not-done row, in a way a scan reads without
  // colour, and by colour as well.
  expect(
    eyeDifferences(done, open),
    "the done row and the not-done row are told apart without colour",
  ).not.toHaveLength(0);
  expect(
    done.color,
    "the done row is also coloured differently from the not-done row",
  ).not.toBe(open.color);

  // The two rows meet the eye together, in the one list, in the one viewport.
  await expectBothRowsInOneScan(page, [
    noteRow(page, doneLabel),
    noteRow(page, openLabel),
  ]);
  expect(
    await noteRow(page, doneLabel).evaluate((element: Element) =>
      element.closest("ul")?.getAttribute("aria-label"),
    ),
    "both rows are in the notes list the person reads",
  ).toBe("Your notes");

  await page.screenshot({ path: testInfo.outputPath("done-and-not-done.png") });
  await testInfo.attach("done-and-not-done", {
    path: testInfo.outputPath("done-and-not-done.png"),
    contentType: "image/png",
  });

  await signOut(page);
});

test("the not-done row shows no Done label and keeps the standard row styling", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const firstLabel = uniqueLabel(testInfo, "alpha");
  const secondLabel = uniqueLabel(testInfo, "beta");
  await signInAndCreate(page, user, [firstLabel, secondLabel]);

  const notes = await showListAgain(page, user, ownerNotes(page));
  await expectRowsMatchMembers(page, notes);

  const first = await expectNotDoneRow(page, firstLabel);
  const second = await expectNotDoneRow(page, secondLabel);

  // No "Done" cue anywhere in either row, and nothing that sets the two apart:
  // the standard row styling is the one treatment every not-done row gets.
  await expect(
    noteRow(page, firstLabel).getByText(DONE_WORD, { exact: true }),
  ).toHaveCount(0);
  await expect(
    noteRow(page, secondLabel).getByText(DONE_WORD, { exact: true }),
  ).toHaveCount(0);
  expect(second, "two not-done rows are shown identically").toEqual(first);

  // The standard row is the row the app leaves alone: a plain note row, with
  // none of the done treatment and the browser's own styling kept.
  expect(first.classes, "the not-done row is a plain note row").not.toContain(
    DONE_ROW_CLASS,
  );
  expect(
    standardDifferences(first),
    "the not-done row keeps the browser's own row styling",
  ).toHaveLength(0);

  await page.screenshot({ path: testInfo.outputPath("not-done-rows.png") });
  await testInfo.attach("not-done-rows", {
    path: testInfo.outputPath("not-done-rows.png"),
    contentType: "image/png",
  });

  await signOut(page);
});

test("the done treatment and the Done cue appear on done rows only", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const doneLabel = uniqueLabel(testInfo, "alpha");
  const openLabels = [
    uniqueLabel(testInfo, "beta"),
    uniqueLabel(testInfo, "gamma"),
  ];
  await signInAndCreate(page, user, [doneLabel, ...openLabels]);

  const handler = await serveNotesWithDoneFlags(page, [doneLabel]);
  const notes = await showListAgain(page, user, handler.next());
  await expectRowsMatchMembers(page, notes);

  // The members marked done are exactly the one this spec asked to be marked,
  // so the sweep below has both states to tell apart.
  const doneLabels = notes
    .filter((note) => note.done)
    .map((note) => note.label);
  expect(doneLabels, "one member of the list is handed over as done").toEqual([
    doneLabel,
  ]);

  // Every member, in the order the app was handed them: a done member's row
  // carries the cue and the treatment, a not-done member's row carries neither.
  for (const note of notes) {
    if (note.done) {
      await expectDoneRow(page, note.label);
    } else {
      await expectNotDoneRow(page, note.label);
    }
  }

  // The cue and the treatment exist in the list only on the done rows: counted
  // across the whole list, not per row, so no row escapes the count.
  await expect(
    notesList(page).getByText(DONE_WORD, { exact: true }),
    `only the done row shows "${DONE_WORD}"`,
  ).toHaveCount(doneLabels.length);
  await expect(
    notesList(page).locator(`.${DONE_ROW_CLASS}`),
    "only the done row carries the done treatment",
  ).toHaveCount(doneLabels.length);

  // And the not-done rows sit next to it untouched.
  for (const label of openLabels) {
    await expectNotDoneRow(page, label);
  }

  await page.screenshot({ path: testInfo.outputPath("cue-on-done-only.png") });
  await testInfo.attach("cue-on-done-only", {
    path: testInfo.outputPath("cue-on-done-only.png"),
    contentType: "image/png",
  });

  await signOut(page);
});

test("a list of only not-done notes shows every row in the not-done presentation", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const created = [
    uniqueLabel(testInfo, "alpha"),
    uniqueLabel(testInfo, "beta"),
    uniqueLabel(testInfo, "gamma"),
  ];
  await signInAndCreate(page, user, created);

  const notes = await showListAgain(page, user, ownerNotes(page));
  await expectRowsMatchMembers(page, notes);

  // The premise: this list holds no done note at all, and it holds the notes
  // this spec just made through the app's own form.
  expect(
    notes.every((note) => note.done === false),
    "no member of the list is marked done",
  ).toBe(true);
  const labels = notes.map((note) => note.label);
  for (const label of created) {
    expect(
      labels,
      `the note created as "${label}" was not omitted from the list`,
    ).toContain(label);
  }

  // Nothing in the list is marked done: no cue and no treatment anywhere.
  await expect(
    notesList(page).getByText(DONE_WORD, { exact: true }),
    `no row of an all not-done list shows "${DONE_WORD}"`,
  ).toHaveCount(0);
  await expect(
    notesList(page).locator(`.${DONE_ROW_CLASS}`),
    "no row of an all not-done list carries the done treatment",
  ).toHaveCount(0);

  // Every member is shown, in the not-done presentation, and no row is hidden:
  // each one is painted, with an area of its own, in the list the person reads.
  for (const note of notes) {
    await expectNotDoneRow(page, note.label);
    const row = noteRow(page, note.label);
    await expect(row, `the row "${note.label}" is visible`).toBeVisible();
    const box = await row.boundingBox();
    expect(box, `the row "${note.label}" is painted`).not.toBeNull();
    expect(box?.height, `the row "${note.label}" has a height`).toBeGreaterThan(
      0,
    );
    expect(box?.width, `the row "${note.label}" has a width`).toBeGreaterThan(
      0,
    );
  }
  expect(await noteRows(page).count(), "no row was dropped from the list").toBe(
    notes.length,
  );

  await page.screenshot({ path: testInfo.outputPath("all-not-done.png") });
  await testInfo.attach("all-not-done", {
    path: testInfo.outputPath("all-not-done.png"),
    contentType: "image/png",
  });

  await signOut(page);
});
