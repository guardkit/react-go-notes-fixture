import {
  expect,
  test,
  type APIRequestContext,
  type Locator,
  type Page,
  type TestInfo,
} from "@playwright/test";
import {
  createNote,
  doneIndicator,
  isDoneWire,
  noteRow,
  noteRows,
  notesList,
  readStoredCollection,
  serveNotesWithDoneFlags,
  signIn,
  statusGlyph,
  statusWord,
  testUser,
  titleCell,
  DONE_WORD,
  OPEN_WORD,
  type NoteWire,
} from "./helpers";

// The done flag at the surface the person actually reads: the rows of the notes
// list. The unit tests of apps/web/src/App.test.tsx pin the same flag on the
// rendered component; these checks start where those stop and assert what a
// browser really paints - the state word a row shows, the treatment a done row
// carries and an open row does not, the column the indicators line up in, and
// what the indicator leaves untouched beside it.
//
// The rows are the ones the app renders for notes this spec made through the
// app's own form (helpers.createNote). Only the flag itself cannot come from
// the app: POST /api/notes never takes it and the column defaults to false, so
// helpers.serveNotesWithDoneFlags sets that one field on the members the API
// itself handed back, on their way to the browser. Nothing is stored done,
// which is what the untouched-flag check below depends on: it reads the kept
// state through an out-of-band request the route never sees.

// The two states, in the words the row owes the person. A state nobody can read
// is not a state, so every check here reads the word rather than a class name.
const STATE_WORDS = [DONE_WORD, OPEN_WORD] as const;

// A title long enough to fill a row at the narrowest supported viewport: the
// row where a displacing indicator would show, if one existed.
const LONGEST_TITLE = 80;

// A title unique to this run, so a row matched by its title is this test's row
// and never one an earlier run left in the owner's collection. The words of
// the two states never appear in a title, so no assertion on a state word can
// be satisfied by a note's own title.
function uniqueLabel(testInfo: TestInfo, kind: string): string {
  return `flag ${testInfo.testId.slice(0, 8)}-${Date.now()} ${kind}`;
}

function longestLabel(testInfo: TestInfo): string {
  const base = `${uniqueLabel(testInfo, "long")}-`;
  return `${base}${"x".repeat(Math.max(0, LONGEST_TITLE - base.length))}`;
}

// Every /api call the page makes from the moment this is armed. A row that
// fetched its note, or a list that wrote a flag back, shows up here.
type ApiCall = { method: string; path: string };

function trackApiCalls(page: Page): ApiCall[] {
  const calls: ApiCall[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.pathname.startsWith("/api/")) {
      calls.push({ method: request.method(), path: url.pathname });
    }
  });
  return calls;
}

// What one row shows of its note, as the browser painted it: the two things a
// person reads off a row (the state and the title) plus the boxes they occupy.
type RowView = {
  label: string;
  word: string;
  glyph: string;
  title: string;
  indicator: { x: number; y: number; width: number; height: number };
  titleBox: { x: number; y: number; width: number; height: number };
};

async function viewRow(page: Page, label: string): Promise<RowView> {
  const row = noteRow(page, label);
  await expect(row, `one row shows "${label}"`).toHaveCount(1);
  const indicator = doneIndicator(row);
  const title = titleCell(row);
  await expect(indicator, `"${label}" shows one status cell`).toHaveCount(1);
  await expect(indicator).toBeVisible();
  const indicatorBox = await indicator.boundingBox();
  const titleBox = await title.boundingBox();
  if (!indicatorBox || !titleBox) {
    throw new Error(`"${label}" shows a status cell and a title, both painted`);
  }
  return {
    label,
    word: await statusWord(row),
    glyph: await statusGlyph(row),
    title: ((await title.textContent()) ?? "").replace(/\s+/g, " ").trim(),
    indicator: indicatorBox,
    titleBox,
  };
}

// The row content that carries text, and whether the browser had to clip any of
// it. The sweep takes whatever the row renders, so a row that later gains a
// preview or a timestamp is measured by the same rule as the title is.
type PaintedText = {
  text: string;
  clipped: boolean;
  overflow: string;
  x: number;
  y: number;
  width: number;
  height: number;
};

async function paintedTextOf(row: Locator): Promise<PaintedText[]> {
  return row.locator("> *").evaluateAll((cells) =>
    cells.map((cell) => ({
      text: (cell.textContent ?? "").replace(/\s+/g, " ").trim(),
      clipped: cell.scrollWidth > cell.clientWidth + 1,
      overflow: getComputedStyle(cell).textOverflow,
      x: cell.getBoundingClientRect().x,
      y: cell.getBoundingClientRect().y,
      width: cell.getBoundingClientRect().width,
      height: cell.getBoundingClientRect().height,
    })),
  );
}

// The list as the person sees it: notes made through the app's own form, then
// the list shown again with the named notes answered done. Tokens live in
// memory, so the showing after the reload signs in again through the session
// Keycloak still holds, exactly as the journey spec does.
type ListView = {
  // The members the app was handed for the list it rendered.
  served: NoteWire[];
  // The same members read straight from the API before the list was shown.
  stored: NoteWire[];
  // The authorization header value the app itself used for its own reads.
  authorization: string;
  // Every /api call made while the list was being shown.
  calls: ApiCall[];
};

async function showList(
  page: Page,
  request: APIRequestContext,
  labels: string[],
  doneLabels: readonly string[],
): Promise<ListView> {
  const user = testUser();
  await page.goto("/");
  await expect(page.getByTestId("signin-hint")).toBeVisible();
  await signIn(page, user);
  await expect(notesList(page)).toHaveCount(1);
  await expect(page.getByRole("alert")).toHaveCount(0);

  // The token the app reads its own notes with, taken from the request the
  // form's save makes - never a token the spec minted for itself.
  const saved = page.waitForRequest(
    (made) => made.method() === "POST" && made.url().endsWith("/api/notes"),
  );
  for (const label of labels) {
    await createNote(page, label);
  }
  const authorization = (await saved).headers()["authorization"] ?? "";
  expect(authorization, "the app reads its notes signed in").toMatch(
    /^Bearer \S+/,
  );
  const stored = await readStoredCollection(request, authorization);

  const served = await serveNotesWithDoneFlags(page, doneLabels);
  const listed = served.next();
  const calls = trackApiCalls(page);
  await page.reload();
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByTestId("signed-in-user")).toHaveText(user.username);
  await expect(page.getByRole("alert")).toHaveCount(0);
  const members = await listed;
  expect(
    members.length,
    "the list was filled with the notes this spec made",
  ).toBeGreaterThanOrEqual(labels.length);
  return { served: members, stored, authorization, calls };
}

// The notes every check below reads: two answered done, one left open, and one
// of those carrying the longest title the form accepts, so the row with the
// least room to spare is in every sweep.
function fixture(testInfo: TestInfo): {
  labels: string[];
  done: string[];
  open: string[];
  longest: string;
} {
  const longest = longestLabel(testInfo);
  const done = [uniqueLabel(testInfo, "one"), longest];
  const open = [uniqueLabel(testInfo, "two")];
  return { labels: [...done, ...open], done, open, longest };
}

test("every row presents its note's done status while the note stays unopened", async ({
  page,
  request,
}, testInfo) => {
  const { labels, done } = fixture(testInfo);
  const { served, calls } = await showList(page, request, labels, done);

  // The rows are the collection: one row per member, and every member the one
  // read that filled the list named.
  await expect(noteRows(page)).toHaveCount(served.length);
  const listed = served.map((note) => note.label);
  for (const label of labels) {
    expect(listed, `the list holds the note "${label}"`).toContain(label);
  }

  for (const note of served) {
    const row = noteRow(page, note.label);
    const indicator = doneIndicator(row);
    // The status is on the row and shown: visible with nothing clicked,
    // hovered or expanded, because this test performs no such action.
    await expect(
      indicator,
      `"${note.label}" shows its status without the note being opened`,
    ).toBeVisible();
    expect(
      await statusWord(row),
      `the row of "${note.label}" presents the flag that note carries`,
    ).toBe(isDoneWire(note) ? DONE_WORD : OPEN_WORD);
  }

  // Nothing was opened to get it: the list is filled by its own one read, and
  // no note was fetched or written beside or behind that read.
  const beyondTheList = calls.filter(
    (call) => !(call.method === "GET" && call.path === "/api/notes"),
  );
  expect(
    beyondTheList,
    "showing the list asks for the collection once and for no note of its own",
  ).toEqual([]);
  // The person never left the list: no detail view was navigated to.
  expect(new URL(page.url()).pathname, "the list is still the page shown").toBe(
    "/",
  );
});

test("a done note shows the indicator in its done state and an open note in the not-done state", async ({
  page,
  request,
}, testInfo) => {
  const { labels, done, open } = fixture(testInfo);
  await showList(page, request, labels, done);

  const doneRow = await viewRow(page, done[0] ?? "");
  const openRow = await viewRow(page, open[0] ?? "");

  // The done note's row states the done state, in words a person reads.
  expect(doneRow.word, "a done note reads as done").toBe(DONE_WORD);
  // The open note's row states the not-done state rather than showing nothing
  // at all: absence of markup is never how an open note says so.
  expect(openRow.word, "an open note reads as not done").toBe(OPEN_WORD);
  // The two rows are on the page together, so the two states are both really
  // rendered rather than one of them being an empty locator.
  await expect(noteRow(page, done[0] ?? "")).toBeVisible();
  await expect(noteRow(page, open[0] ?? "")).toBeVisible();
  // The states are told apart: an open row is never read as done, and a done
  // row never falls back to the open wording.
  expect(doneRow.word).not.toBe(openRow.word);
  expect(STATE_WORDS).toContain(doneRow.word);
  expect(STATE_WORDS).toContain(openRow.word);
  // The indicator is the row's own, so one note's flag cannot be read where
  // another note's is read.
  expect(doneRow.indicator.y).not.toBe(openRow.indicator.y);
});

test("a done note is distinguished from an open one by more than one channel", async ({
  page,
  request,
}, testInfo) => {
  const { labels, done, open } = fixture(testInfo);
  await showList(page, request, labels, done);

  const doneRow = await viewRow(page, done[0] ?? "");
  const openRow = await viewRow(page, open[0] ?? "");
  const doneTitle = titleCell(noteRow(page, done[0] ?? ""));
  const openTitle = titleCell(noteRow(page, open[0] ?? ""));
  const doneStyle = await doneTitle.evaluate((element) => {
    const shown = getComputedStyle(element);
    return {
      decoration: shown.textDecorationLine,
      weight: shown.fontWeight,
      color: shown.color,
    };
  });
  const openStyle = await openTitle.evaluate((element) => {
    const shown = getComputedStyle(element);
    return {
      decoration: shown.textDecorationLine,
      weight: shown.fontWeight,
      color: shown.color,
    };
  });

  // Channel one: the written label. The done row says the done word; the open
  // row says the other word, so the cue is present in both states and differs
  // between them rather than being missing from one.
  expect(doneRow.word).toBe(DONE_WORD);
  expect(openRow.word).toBe(OPEN_WORD);

  // The channels that tell the two rows apart without colour, counted from what
  // the browser painted rather than from a list of properties assumed to differ.
  const nonColour: string[] = [];
  if (doneRow.word !== openRow.word) nonColour.push("state word");
  if (doneRow.glyph !== openRow.glyph) nonColour.push("glyph");
  if (doneStyle.decoration !== openStyle.decoration) {
    nonColour.push("title decoration");
  }
  if (doneStyle.weight !== openStyle.weight) {
    nonColour.push("title weight");
  }
  // "Clearly marked" is more than one channel: the written state plus at least
  // one row-level treatment, none of them a colour.
  expect(
    nonColour,
    "a done row is set apart by more than colour alone",
  ).toEqual(expect.arrayContaining(["state word", "title decoration"]));
  expect(
    nonColour.length,
    "a done row is set apart by more than one channel",
  ).toBeGreaterThanOrEqual(2);
  // The row-level treatment the request asked for, named rather than implied:
  // the done title is struck through and the open title is not.
  expect(doneStyle.decoration, "a done title is struck through").toContain(
    "line-through",
  );
  expect(openStyle.decoration, "an open title is not struck through").toBe(
    "none",
  );
  // The written state is never carried by a decorative glyph alone: the glyph
  // is hidden from assistive technology, and the word stands without it.
  expect(doneRow.glyph, "the done row shows a glyph").not.toBe("");
  await expect(
    doneIndicator(noteRow(page, done[0] ?? "")).locator("[aria-hidden='true']"),
    "the glyph is decoration, not the state itself",
  ).toHaveCount(1);
});

test("the indicator holds one position across every row, and a mixed list reads unambiguously", async ({
  page,
  request,
}, testInfo) => {
  const { labels, done, open } = fixture(testInfo);
  const { served } = await showList(page, request, labels, done);

  // Every row, read in the order the list paints them.
  const rows = noteRows(page);
  const total = await rows.count();
  expect(total, "the list shows one row for every member").toBe(served.length);
  const shown: { title: string; word: string; left: number }[] = [];
  for (let index = 0; index < total; index += 1) {
    const row = rows.nth(index);
    await expect(
      row.locator("> *"),
      "a row is made of cells a person reads across",
    ).not.toHaveCount(0);
    const indicator = doneIndicator(row);
    await expect(
      indicator,
      "every row carries its own status cell, and only one",
    ).toHaveCount(1);
    // The status cell leads its row: the indicator is the first thing the eye
    // meets, on this row and on every other row.
    const leads = await row
      .locator("> *")
      .first()
      .evaluate((cell) =>
        /^(?:\S+\s+)?(?:Not done|Done)$/.test(
          (cell.textContent ?? "").replace(/\s+/g, " ").trim(),
        ),
      );
    expect(leads, "the status cell leads its row").toBe(true);
    const box = await indicator.boundingBox();
    expect(box, "the status cell is painted").not.toBeNull();
    const rowBox = await row.boundingBox();
    const titleBox = await titleCell(row).boundingBox();
    expect(rowBox, "the row is painted").not.toBeNull();
    expect(titleBox, "the title is painted").not.toBeNull();
    // The status cell sits at the row's own left edge, on the same line as the
    // title and before it: one column of statuses, read beside the title it
    // belongs to rather than stacked under it or pushed after it.
    expect(
      Math.abs(box.x - (rowBox?.x ?? Number.NaN)),
      "the status cell starts where the row starts",
    ).toBeLessThanOrEqual(1);
    expect(
      Math.abs(box.y - (titleBox?.y ?? Number.NaN)),
      "the status cell meets the eye on the title's own line",
    ).toBeLessThanOrEqual(4);
    expect(
      box.x,
      "the status cell is read before the title, not after it",
    ).toBeLessThanOrEqual((titleBox?.x ?? 0) + 1);
    shown.push({
      title: ((await titleCell(row).textContent()) ?? "")
        .replace(/\s+/g, " ")
        .trim(),
      word: await statusWord(row),
      left: box?.x ?? Number.NaN,
    });
  }

  // One column of statuses: every indicator starts at the same x, so the list
  // scans as a column rather than as a scatter of statuses.
  const lefts = [...new Set(shown.map((row) => Math.round(row.left)))];
  expect(lefts, "the indicators all start at one edge").toHaveLength(1);

  // A mixed list, read without ambiguity: each row says exactly one of the two
  // states, and the rows that say done are exactly the notes carried done.
  const expected = new Set(done);
  for (const label of labels) {
    const matches = shown.filter((row) => row.title === label);
    expect(matches, `one row shows "${label}"`).toHaveLength(1);
    const row = matches[0];
    if (!row) throw new Error(`no row shows "${label}"`);
    expect(STATE_WORDS, `"${label}" reads as one of the two states`).toContain(
      row.word,
    );
    expect(
      row.word,
      `"${label}" reads as ${expected.has(label) ? DONE_WORD : OPEN_WORD}`,
    ).toBe(expected.has(label) ? DONE_WORD : OPEN_WORD);
  }
  // No row says both states, and none says neither: the words appear once each
  // in their own row, never together in one row.
  for (const label of labels) {
    const text = ((await noteRow(page, label).textContent()) ?? "").replace(
      /\s+/g,
      " ",
    );
    expect(
      text.includes(DONE_WORD) && text.includes(OPEN_WORD),
      `one row says both states: "${text}"`,
    ).toBe(false);
  }
  // The done rows and the open rows are all on the page at once: a person
  // comparing them never has to scroll one of them into the other's place.
  for (const label of [...done, ...open]) {
    await expect(
      noteRow(page, label),
      `"${label}" is shown while the other states are shown too`,
    ).toBeInViewport();
  }
});

test("viewing the list leaves every note's done flag untouched", async ({
  page,
  request,
}, testInfo) => {
  const { labels, done } = fixture(testInfo);
  const { served, stored, authorization, calls } = await showList(
    page,
    request,
    labels,
    done,
  );

  // The list really did show done rows, so the comparison below is made of a
  // view that presented the flag rather than of an empty screen.
  const shownDone = served.filter(isDoneWire).map((note) => note.label);
  expect(shownDone.length, "a done row was shown").toBeGreaterThan(0);
  for (const label of shownDone) {
    expect(await statusWord(noteRow(page, label))).toBe(DONE_WORD);
  }

  // The state as it is kept, read outside the browser and so outside the route
  // that answered the flags: the same members with the same flags before the
  // list was shown and after it was looked at.
  const flags = (notes: NoteWire[]): string[] =>
    notes.map((note) => `${note.id}|${note.label}|${isDoneWire(note)}`).sort();
  const before = flags(stored);
  const after = flags(await readStoredCollection(request, authorization));
  expect(after, "looking at the list changed no note's done flag").toEqual(
    before,
  );

  // And the view asked for nothing beyond its one read: a list that wrote the
  // flag back would show up here as a method other than the collection's GET.
  const writes = calls.filter((call) => call.method !== "GET");
  expect(writes, "showing the list writes nothing back").toEqual([]);
});

test("the indicator neither displaces nor truncates what the row shows", async ({
  page,
  request,
}, testInfo) => {
  const { labels, done, longest } = fixture(testInfo);
  await showList(page, request, labels, done);

  for (const width of [1280, 320]) {
    await page.setViewportSize({ width, height: 1024 });
    for (const label of labels) {
      const row = noteRow(page, label);
      await expect(row, `one row shows "${label}"`).toHaveCount(1);
      const rowView = await viewRow(page, label);

      // The title is whole: the row shows the note's entire title, with no
      // clipping and no ellipsis standing in for what did not fit.
      expect(rowView.title, `"${label}" shows its whole title`).toBe(label);
      expect(
        await row
          .locator("> *")
          .evaluateAll((cells) =>
            cells
              .filter((cell) => cell.scrollWidth > cell.clientWidth + 1)
              .map((cell) =>
                (cell.textContent ?? "").replace(/\s+/g, " ").trim(),
              ),
          ),
        `nothing in the row of "${label}" is cut off at ${width}px`,
      ).toEqual([]);
      const cells = await paintedTextOf(row);
      for (const cell of cells) {
        expect(
          cell.clipped,
          `"${cell.text}" is not clipped at ${width}px`,
        ).toBe(false);
        expect(
          cell.text,
          `"${label}" keeps its text whole at ${width}px`,
        ).not.toContain("…");
        expect(
          cell.overflow === "ellipsis" && cell.clipped,
          `"${cell.text}" is not elided at ${width}px`,
        ).toBe(false);
      }

      // The indicator sits beside the row's content rather than over it: the
      // two boxes never share any area, and the status cell keeps its own room.
      const a = rowView.indicator;
      const b = rowView.titleBox;
      const overlaps =
        a.x < b.x + b.width &&
        b.x < a.x + a.width &&
        a.y < b.y + b.height &&
        b.y < a.y + a.height;
      expect(overlaps, `the indicator does not cover "${label}"`).toBe(false);
      expect(a.width, "the status cell keeps its own room").toBeGreaterThan(0);
      expect(a.height, "the status cell is not squeezed away").toBeGreaterThan(
        0,
      );
      expect(
        b.width,
        `the title of "${label}" still has room at ${width}px`,
      ).toBeGreaterThan(0);
      expect(
        b.height,
        `the title of "${label}" keeps its line`,
      ).toBeGreaterThan(0);
      // The whole row, indicator and content together, is inside the viewport
      // the person is looking at: nothing was pushed out of sight.
      expect(
        a.x,
        `the indicator of "${label}" is on the page at ${width}px`,
      ).toBeGreaterThanOrEqual(0);
      expect(
        b.x + b.width,
        `the title of "${label}" ends inside the page at ${width}px`,
      ).toBeLessThanOrEqual(width);
    }
  }
  expect(
    (await viewRow(page, longest)).title,
    "the longest title survives every viewport",
  ).toBe(longest);
});
