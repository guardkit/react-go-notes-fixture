import {
  expect,
  test,
  type Locator,
  type Page,
  type Response,
  type TestInfo,
} from "@playwright/test";
import {
  createNote,
  noteRow,
  notesList,
  ownerNotes,
  signIn,
  signOut,
  testUser,
  type NoteWire,
  type TestUser,
} from "./helpers";

// The done flag as the person changes it, at the surface they change it from:
// the tick at the head of each row of the notes list.
//
// notes-done.spec.ts pins the flag at the GET /api/notes boundary;
// notes-done-visibility.spec.ts pins the two presentations a row can be shown
// in. Both had to reach for helpers.serveNotesWithDoneFlags because, in their
// slice, nothing could write the flag: it was storage's own column default.
// This slice has PATCH /api/notes/{id}, so these checks need no fixture at all
// - every done row they assert was made done by the very control they assert,
// through the app's own click handler, the app's own fetch and the live
// backend. Nothing here is intercepted or answered by the spec: the flag each
// row ends up showing is the flag PostgreSQL holds, and the reload checks below
// read it back through the app's own unintercepted GET /api/notes.
//
// What is asserted is the whole journey the request asked for: the tick in the
// list, the state the control and the title show once the save has come back,
// the same state after the page is thrown away and the person signs in again,
// and the reverse direction.

// The word that carries the state on its own, so the two states stay
// distinguishable without colour and to a screen reader.
const DONE_WORD = "Done";
// The row treatment a done row carries and a not-done row never does.
const DONE_ROW_CLASS = "note-row-done";
// The control itself, and the extra class it carries only while ticked.
const TOGGLE_CLASS = "done-toggle";
const TOGGLE_ON_CLASS = "done-toggle-on";
// The two - and only two - answers a completion control may give.
const TICKED = "true";
const UNTICKED = "false";

// What the eye tells apart in a row, read off the rendered row and its control.
type RowLook = {
  checked: string | null;
  rowClasses: string[];
  toggleClasses: string[];
  ticks: number;
  titleDecoration: string;
  rowOpacity: string;
  rowFontStyle: string;
  rowColor: string;
  doneCues: number;
};

// The properties a single scan of the list reads without relying on colour.
const EYE_KEYS = [
  "titleDecoration",
  "rowOpacity",
  "rowFontStyle",
  "ticks",
] as const;
type EyeKey = (typeof EYE_KEYS)[number];

// The presentation a row has when nothing has been applied to it: what the
// browser makes of a plain list item and a plain button.
const STANDARD_ROW: Record<EyeKey, string | number> = {
  titleDecoration: "none",
  rowOpacity: "1",
  rowFontStyle: "normal",
  ticks: 0,
};

function eyeDifferences(a: RowLook, b: RowLook): EyeKey[] {
  return EYE_KEYS.filter((key) => a[key] !== b[key]);
}

function standardDifferences(look: RowLook): EyeKey[] {
  return EYE_KEYS.filter((key) => look[key] !== STANDARD_ROW[key]);
}

// The completion control of one row, addressed by role: a row carries exactly
// one, so the role names it without the spec choosing an element by markup.
function doneControl(page: Page, label: string): Locator {
  return noteRow(page, label).getByRole("checkbox");
}

// The title of one row, addressed by the very text the person reads.
function doneTitle(page: Page, label: string): Locator {
  return noteRow(page, label).getByText(label, { exact: true });
}

// Everything a row shows, as the browser rendered it. The row is addressed by
// the title the spec made for it, and the counts are pinned first: an
// assertion about "the" control or "the" title only means something once
// exactly one of each is on the page.
async function lookAt(page: Page, label: string): Promise<RowLook> {
  const row = noteRow(page, label);
  await expect(row, `exactly one row shows "${label}"`).toHaveCount(1);
  const control = doneControl(page, label);
  await expect(
    control,
    `the row "${label}" carries exactly one completion control`,
  ).toHaveCount(1);
  const title = doneTitle(page, label);
  await expect(title, `"${label}" shows its title`).toHaveCount(1);
  const rendered = await row.evaluate((element: Element) => {
    const shown = getComputedStyle(element);
    return {
      rowClasses: element.className.split(/\s+/).filter(Boolean),
      rowOpacity: shown.opacity,
      rowFontStyle: shown.fontStyle,
      rowColor: shown.color,
    };
  });
  const controlState = await control.evaluate((element: Element) => ({
    checked: element.getAttribute("aria-checked"),
    toggleClasses: element.className.split(/\s+/).filter(Boolean),
    ticks: element.querySelectorAll("svg").length,
  }));
  return {
    ...rendered,
    ...controlState,
    titleDecoration: await title.evaluate(
      (element: Element) => getComputedStyle(element).textDecorationLine,
    ),
    doneCues: await row.getByText(DONE_WORD, { exact: true }).count(),
  };
}

// The row as it is when the note is done: the control ticked and saying so, the
// title taken out of the standard presentation, and the word "Done" shown.
async function expectTickedRow(page: Page, label: string): Promise<RowLook> {
  const control = doneControl(page, label);
  await expect(control, `the control of "${label}" is ticked`).toBeChecked();
  await expect(
    control,
    `the control of "${label}" answers its state as ticked`,
  ).toHaveAttribute("aria-checked", TICKED);
  await expect(
    control,
    `the ticked control of "${label}" says what pressing it would do`,
  ).toHaveAccessibleName(`Mark "${label}" as not done`);
  const look = await lookAt(page, label);
  expect(look.checked, `the control of "${label}" is ticked`).toBe(TICKED);
  expect(
    look.toggleClasses,
    `the ticked control of "${label}" carries the ticked treatment`,
  ).toContain(TOGGLE_ON_CLASS);
  expect(look.ticks, `the ticked control of "${label}" shows its tick`).toBe(1);
  expect(
    look.rowClasses,
    `the done row "${label}" carries the done treatment`,
  ).toContain(DONE_ROW_CLASS);
  expect(
    look.titleDecoration,
    `the done title "${label}" is struck through`,
  ).toBe("line-through");
  expect(
    look.doneCues,
    `the done row "${label}" shows exactly one "${DONE_WORD}" cue`,
  ).toBe(1);
  return look;
}

// The row as it is when the note is not done: the control unticked and saying
// so, no tick, no done treatment on the row, the title left in the standard
// presentation, and no "Done" cue anywhere in the row.
async function expectUntickedRow(page: Page, label: string): Promise<RowLook> {
  const control = doneControl(page, label);
  await expect(
    control,
    `the control of "${label}" is not ticked`,
  ).not.toBeChecked();
  await expect(
    control,
    `the control of "${label}" answers its state as unticked`,
  ).toHaveAttribute("aria-checked", UNTICKED);
  await expect(
    control,
    `the unticked control of "${label}" says what pressing it would do`,
  ).toHaveAccessibleName(`Mark "${label}" as done`);
  const look = await lookAt(page, label);
  expect(look.checked, `the control of "${label}" is unticked`).toBe(UNTICKED);
  expect(
    look.toggleClasses,
    `the unticked control of "${label}" carries no ticked treatment`,
  ).not.toContain(TOGGLE_ON_CLASS);
  expect(
    look.toggleClasses,
    `the unticked control of "${label}" is still the same control`,
  ).toContain(TOGGLE_CLASS);
  expect(look.ticks, `the unticked control of "${label}" shows no tick`).toBe(
    0,
  );
  expect(
    look.rowClasses,
    `the not-done row "${label}" carries no done treatment`,
  ).not.toContain(DONE_ROW_CLASS);
  expect(
    look.titleDecoration,
    `the not-done title "${label}" is not struck through`,
  ).toBe("none");
  expect(
    look.doneCues,
    `the not-done row "${label}" shows no "${DONE_WORD}" cue`,
  ).toBe(0);
  for (const key of EYE_KEYS) {
    expect(
      look[key],
      `the not-done row "${label}" keeps the standard ${key}`,
    ).toBe(STANDARD_ROW[key]);
  }
  return look;
}

// The one PATCH the app sends for a single note: the path the id is written
// into, and nothing else the page could be asking for.
function isPatchToNote(response: Response): boolean {
  return (
    response.request().method() === "PATCH" &&
    new URL(response.url()).pathname.startsWith("/api/notes/")
  );
}

// Presses the completion control of one row and hands back the note the service
// answered with. The wait is armed before the press, so the note returned is
// the answer to this press and not to an earlier one; the caller presses again
// only once this has come back, which is also how the app's own optimistic
// state settles.
async function pressControl(
  page: Page,
  label: string,
  press: () => Promise<void>,
): Promise<NoteWire> {
  const saved = page.waitForResponse(isPatchToNote);
  await press();
  const response = await saved;
  expect(
    response.status(),
    `the save for "${label}" was accepted by the service`,
  ).toBe(200);
  const note = (await response.json()) as NoteWire;
  expect(note.label, `the service answered for the note "${label}"`).toBe(
    label,
  );
  expect(
    typeof note.done,
    `the service answered "${label}" with a boolean flag`,
  ).toBe("boolean");
  await expect(
    page.getByRole("alert"),
    `ticking "${label}" reported no error`,
  ).toHaveCount(0);
  return note;
}

function clickControl(page: Page, label: string): Promise<NoteWire> {
  return pressControl(page, label, () => doneControl(page, label).click());
}

// A title unique to this run, and free of the cue word so no assertion on the
// cue can be satisfied by a title of its own.
function uniqueLabel(testInfo: TestInfo, kind: string): string {
  return `toggle note ${testInfo.testId.slice(0, 8)}-${Date.now()} ${kind}`;
}

// The notes each test made through the app's own form, recorded as they are
// made so the undo below can reach every one of them by title.
const madeByTest = new Map<string, string[]>();

// Signs in and makes each note through the app's own form, so the rows ticked
// below are the rows the person's own flow put in the list - and so every one
// of them starts not-done, the flag the column default gives a new note.
async function signInAndCreate(
  page: Page,
  testInfo: TestInfo,
  user: TestUser,
  labels: string[],
): Promise<void> {
  await page.goto("/");
  await expect(page.getByTestId("signin-hint")).toBeVisible();
  await signIn(page, user);
  await expect(notesList(page)).toHaveCount(1);
  await expect(page.getByRole("alert")).toHaveCount(0);
  for (const label of labels) {
    await createNote(page, label);
  }
  const made = madeByTest.get(testInfo.testId) ?? [];
  madeByTest.set(testInfo.testId, [...made, ...labels]);
}

// The list as the person sees it on a fresh load. Tokens live in memory, so the
// rows come from the app's own GET /api/notes - unintercepted, from PostgreSQL -
// once it has signed in again through Keycloak's still-living session.
// `listed` must be armed before the reload, exactly as ownerNotes asks to be.
async function showListAgain(
  page: Page,
  user: TestUser,
  listed: Promise<NoteWire[]>,
): Promise<NoteWire[]> {
  await page.reload();
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(notesList(page)).toHaveCount(1);
  return await listed;
}

// The one member of the list the app was just handed that this spec made, named
// out of the collection rather than assumed: this is the state the server holds,
// read through the app's own request.
function storedNote(notes: NoteWire[], label: string): NoteWire {
  const members = notes.filter((note: NoteWire) => note.label === label);
  expect(
    members,
    `the note "${label}" is in the list the app was handed`,
  ).toHaveLength(1);
  const note = members[0] as NoteWire;
  expect(
    Object.prototype.hasOwnProperty.call(note, "done"),
    `the stored note "${label}" carries a done field at all`,
  ).toBe(true);
  expect(
    typeof note.done,
    `the stored note "${label}" carries done as a boolean`,
  ).toBe("boolean");
  return note;
}

// Puts a note back the way the person's own flow made it - not-done - before
// the test lets go of the page. The owner and their collection are shared by
// every spec in this run, so a note left ticked here would show up as a second
// done row in a later test's list, and as a done note in the unintercepted
// boundary sweep of notes-done.spec.ts. The undo goes out through the same
// control and the same live PATCH.
async function restoreNotDone(page: Page, label: string): Promise<void> {
  const control = doneControl(page, label);
  if ((await control.getAttribute("aria-checked")) === TICKED) {
    const saved = await clickControl(page, label);
    expect(saved.done, `the undo stored "${label}" as not-done again`).toBe(
      false,
    );
  }
  await expectUntickedRow(page, label);
}

// The undo that runs whatever the test did next. Every test here ends with its
// own restoreNotDone, but a test that fails on the way to it would otherwise
// hand the next spec a done note nobody made - and this owner's whole collection
// is read row by row by notes-done.spec.ts, which expects none. So the sweep of
// every note this test made lives in a hook rather than in any one test body.
// It is best effort by design: a cleanup must never become the failure a run
// reports, and a page already signed out simply has no control left to press,
// which is what the happy path looks like from here.
test.afterEach(async ({ page }, testInfo) => {
  const labels = madeByTest.get(testInfo.testId) ?? [];
  madeByTest.delete(testInfo.testId);
  for (const label of labels) {
    try {
      const control = doneControl(page, label);
      if ((await control.count()) !== 1) continue;
      if ((await control.getAttribute("aria-checked")) !== TICKED) continue;
      const saved = page
        .waitForResponse(isPatchToNote, { timeout: 15_000 })
        .catch(() => null);
      await control.click();
      await saved;
    } catch (cleanup) {
      // The test already reported what went wrong; a cleanup that cannot reach
      // the page would only bury that report under a second failure.
      console.warn(
        `cleanup could not put "${label}" back as not-done`,
        cleanup,
      );
    }
  }
});

async function attach(page: Page, testInfo: TestInfo, name: string) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path });
  await testInfo.attach(name, {
    path,
    contentType: "image/png",
  });
}

test("ticking a not-done note ticks the control and distinguishes its title as done", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = uniqueLabel(testInfo, "tick");
  const openLabel = uniqueLabel(testInfo, "open");
  await signInAndCreate(page, testInfo, user, [label, openLabel]);

  // The premise: a note made through the form arrives not-done, and the control
  // is already there to be pressed - nothing has to be opened to reach it.
  const before = await expectUntickedRow(page, label);
  const open = await expectUntickedRow(page, openLabel);

  const saved = await clickControl(page, label);
  expect(saved.done, `the service stored "${label}" as done`).toBe(true);

  // The control now reads as ticked, and the title is distinguished as done.
  const ticked = await expectTickedRow(page, label);
  expect(
    eyeDifferences(ticked, before),
    `ticking "${label}" changed what the row shows`,
  ).not.toHaveLength(0);
  expect(
    eyeDifferences(ticked, open),
    `the ticked row "${label}" is told apart from the unticked row without colour`,
  ).not.toHaveLength(0);
  expect(
    standardDifferences(ticked),
    `the ticked row "${label}" no longer keeps the standard presentation`,
  ).not.toHaveLength(0);

  // The other note is left exactly as it was: the save touched one row only.
  await expectUntickedRow(page, openLabel);

  await attach(page, testInfo, "ticked-row");
  await restoreNotDone(page, label);
  await signOut(page);
});

test("unticking a done note unticks the control and leaves the title undistinguished", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = uniqueLabel(testInfo, "untick");
  const openLabel = uniqueLabel(testInfo, "open");
  await signInAndCreate(page, testInfo, user, [label, openLabel]);

  // A done note in the list, made done by the control itself through the live
  // PATCH - no fixture, so the untick below reverses a state the server holds.
  const tickedSave = await clickControl(page, label);
  expect(tickedSave.done, `the service stored "${label}" as done`).toBe(true);
  const done = await expectTickedRow(page, label);

  const saved = await clickControl(page, label);
  expect(saved.done, `the service stored "${label}" as not-done`).toBe(false);

  // The control reads as unticked again and the title has given up everything
  // that marked it done.
  const unticked = await expectUntickedRow(page, label);
  expect(
    eyeDifferences(unticked, done),
    `unticking "${label}" changed what the row shows`,
  ).not.toHaveLength(0);
  const open = await expectUntickedRow(page, openLabel);
  expect(
    unticked,
    `the unticked row "${label}" is shown exactly like a note that was never ticked`,
  ).toEqual(open);

  await attach(page, testInfo, "unticked-row");
  await signOut(page);
});

test("a ticked note is still ticked after the page is reloaded", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = uniqueLabel(testInfo, "reload-tick");
  await signInAndCreate(page, testInfo, user, [label]);

  await expectUntickedRow(page, label);
  const saved = await clickControl(page, label);
  expect(saved.done, `the service stored "${label}" as done`).toBe(true);
  await expectTickedRow(page, label);

  // The page and everything it held in memory are thrown away; the rows come
  // back from the app's own GET /api/notes once it has signed in again.
  const listed = ownerNotes(page);
  const notes = await showListAgain(page, user, listed);

  // The state is the server's, not the click's: the very response the reloaded
  // page was handed still says the note is done.
  expect(
    storedNote(notes, label).done,
    `the reloaded list reads "${label}" back from the server as done`,
  ).toBe(true);

  // And the control the person sees on that reloaded page is ticked, with the
  // title distinguished as done, without anyone pressing anything again.
  await expectTickedRow(page, label);
  // The cue the list shows, counted across every row, is the cue the server's
  // own members call for: one per done member, and no done member without one.
  // (Other tests' notes share this owner's list, so the count is taken from
  // what the server answered rather than from what this test made.)
  const doneMembers = notes
    .filter((note: NoteWire) => note.done)
    .map((note: NoteWire) => note.label);
  expect(
    doneMembers,
    `the server answered "${label}" as the done member it is`,
  ).toContain(label);
  await expect(
    notesList(page).getByText(DONE_WORD, { exact: true }),
    "the reloaded list shows the Done cue on exactly the members the server holds as done",
  ).toHaveCount(doneMembers.length);

  await attach(page, testInfo, "ticked-after-reload");
  await restoreNotDone(page, label);
  await signOut(page);
});

test("an unticked note is still unticked after the page is reloaded", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = uniqueLabel(testInfo, "reload-untick");
  await signInAndCreate(page, testInfo, user, [label]);

  // Tick it, then undo the tick: the reverse direction, saved both times.
  const tickedSave = await clickControl(page, label);
  expect(tickedSave.done, `the service stored "${label}" as done`).toBe(true);
  await expectTickedRow(page, label);
  const untickedSave = await clickControl(page, label);
  expect(untickedSave.done, `the service stored "${label}" as not-done`).toBe(
    false,
  );
  await expectUntickedRow(page, label);

  const listed = ownerNotes(page);
  const notes = await showListAgain(page, user, listed);

  expect(
    storedNote(notes, label).done,
    `the reloaded list reads "${label}" back from the server as not-done`,
  ).toBe(false);

  // The untick is what survives the reload: no tick, no cue, no struck title,
  // while the rows the server does hold as done keep theirs.
  await expectUntickedRow(page, label);
  const doneMembers = notes
    .filter((note: NoteWire) => note.done)
    .map((note: NoteWire) => note.label);
  expect(
    doneMembers,
    `the server holds "${label}" as not-done, whatever else this owner has`,
  ).not.toContain(label);
  await expect(
    notesList(page).getByText(DONE_WORD, { exact: true }),
    "the reloaded list shows the Done cue on exactly the members the server holds as done",
  ).toHaveCount(doneMembers.length);
  await expect(
    noteRow(page, label).getByText(DONE_WORD, { exact: true }),
    `the reloaded list shows no "${DONE_WORD}" cue for the unticked "${label}"`,
  ).toHaveCount(0);

  await attach(page, testInfo, "unticked-after-reload");
  await signOut(page);
});

test("the completion control works from the list itself, with no note detail view opened", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = uniqueLabel(testInfo, "in-place");
  await signInAndCreate(page, testInfo, user, [label]);

  const url = page.url();
  const row = noteRow(page, label);
  const control = doneControl(page, label);

  // The control is in the row the person is already looking at, and it is the
  // only thing in that row they could mistake for a way into the note: nothing
  // has to be opened, clicked through or expanded first.
  const listLabel = await control.evaluate((element: Element) =>
    element.closest("ul")?.getAttribute("aria-label"),
  );
  expect(
    listLabel,
    `the control of "${label}" sits inside the notes list the person reads`,
  ).toBe("Your notes");
  await expect(
    row.getByRole("link"),
    `the row "${label}" links to no detail view`,
  ).toHaveCount(0);
  await expect(
    row.getByRole("button"),
    `the row "${label}" offers no other button to reach the note by`,
  ).toHaveCount(0);
  await expect(
    row.getByRole("textbox"),
    `the row "${label}" hides no editor behind a detail view`,
  ).toHaveCount(0);
  await expect(
    page.getByRole("dialog"),
    "the app opens no detail view to change the state",
  ).toHaveCount(0);
  await expect(
    control,
    `the control of "${label}" is the row's one and only control`,
  ).toHaveCount(1);
  await expect(
    control,
    `the control of "${label}" is ready to press`,
  ).toBeVisible();
  await expect(control, `the control of "${label}" is enabled`).toBeEnabled();

  // It answers the keyboard from the list too: focus it where it stands and
  // press it, with no pointer and no view change.
  const saved = await pressControl(page, label, async () => {
    await control.focus();
    await page.keyboard.press("Space");
  });
  expect(saved.done, `the keyboard press stored "${label}" as done`).toBe(true);
  await expectTickedRow(page, label);

  // And the person is still on the list they started on: same page, same view,
  // every row still shown, no detail view to come back from.
  expect(page.url(), "pressing the control navigated nowhere").toBe(url);
  await expect(page.getByRole("heading", { name: "Notes" })).toBeVisible();
  await expect(notesList(page)).toHaveCount(1);
  await expect(row, `the row "${label}" is still in the list`).toHaveCount(1);
  await expect(
    page.getByRole("dialog"),
    "no detail view appeared to change the state",
  ).toHaveCount(0);
  await expect(
    page.getByRole("alert"),
    "changing the state in place reported no error",
  ).toHaveCount(0);

  await attach(page, testInfo, "in-place-toggle");
  await restoreNotDone(page, label);
  await signOut(page);
});

test("the completion control has exactly two states and reaches no third one", async ({
  page,
}, testInfo) => {
  const user = testUser();
  const label = uniqueLabel(testInfo, "binary");
  await signInAndCreate(page, testInfo, user, [label]);

  // One control per row, and it starts in one of the two states.
  const control = doneControl(page, label);
  await expect(
    noteRow(page, label).getByRole("checkbox"),
    `the row "${label}" has one completion control, not one per state`,
  ).toHaveCount(1);
  const answers: string[] = [];
  const wire: boolean[] = [];

  // Four presses: the control alternates, and nothing else is ever answered.
  for (const [press, wanted] of [true, false, true, false].entries()) {
    const saved = await clickControl(page, label);
    expect(
      saved.done,
      `press ${press + 1} stored "${label}" as ${wanted ? "done" : "not-done"}`,
    ).toBe(wanted);
    wire.push(saved.done);
    await expect(
      control,
      `press ${press + 1} left the control of "${label}" ${wanted ? "ticked" : "unticked"}`,
    ).toHaveAttribute("aria-checked", wanted ? TICKED : UNTICKED);
    const look = await lookAt(page, label);
    answers.push(look.checked ?? "<missing>");
    expect(
      [TICKED, UNTICKED],
      `press ${press + 1} answered the state of "${label}" with a state the control has`,
    ).toContain(look.checked);
    expect(
      look.ticks,
      `press ${press + 1} shows the tick on "${label}" exactly when it is ticked`,
    ).toBe(wanted ? 1 : 0);
    expect(
      look.doneCues,
      `press ${press + 1} shows the "${DONE_WORD}" cue on "${label}" exactly when it is ticked`,
    ).toBe(wanted ? 1 : 0);
    expect(
      look.titleDecoration,
      `press ${press + 1} strikes the title of "${label}" exactly when it is ticked`,
    ).toBe(wanted ? "line-through" : "none");
  }

  // The two states are all the control ever answered - no indeterminate, no
  // missing state, no third value on the wire either.
  expect(
    [...new Set(answers)].sort(),
    `the control of "${label}" answered only the two states, and both of them`,
  ).toEqual([UNTICKED, TICKED]);
  await expect(
    control,
    `the control of "${label}" is never indeterminate`,
  ).not.toHaveAttribute("aria-checked", "mixed");
  for (const [position, value] of wire.entries()) {
    expect(
      typeof value,
      `press ${position + 1} sent "${label}" a boolean flag, never a third value`,
    ).toBe("boolean");
  }

  // An even number of presses leaves the note as it was found, and the server
  // agrees with the unticked control the person is left looking at.
  await expectUntickedRow(page, label);
  const notes = await showListAgain(page, user, ownerNotes(page));
  expect(
    storedNote(notes, label).done,
    `the server holds "${label}" as not-done after the round trip`,
  ).toBe(false);
  await expectUntickedRow(page, label);

  await attach(page, testInfo, "two-states-only");
  await signOut(page);
});
