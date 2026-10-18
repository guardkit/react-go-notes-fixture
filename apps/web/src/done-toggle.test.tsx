import { afterEach, describe, expect, it, vi } from "vitest";
import type { User, UserManager } from "oidc-client-ts";
import type { Note } from "./api";
import type { RuntimeConfig } from "./config";

// The surface under test is the control a person clicks on a row of the notes
// list to tick a note done or untick it again: what the click does to the row,
// what it asks the notes service to save, and what the row ends up showing once
// the service has answered. The control is the one the list carries on the row
// itself - there is no detail view to open first - so every case here clicks it
// where it stands.
//
// Notes reach the view only through `listNotes`, and the write leaves only
// through `updateNoteDone`, which is where each case stages its answer: a saved
// note, a note that says something other than what the click asked for, a
// failure, or no answer at all while the request is still out. Nothing here
// changes the API, the contract or the store; the flag and the PATCH route
// already exist.
//
// The static presentation of a done row - the Done cue, the struck title, the
// dimmed row - is the subject of done-flag.test.tsx. What this file holds is the
// movement between the two states, in both directions, and who has the last word
// about which state a row shows.
const world = vi.hoisted(() => ({
  collection: [] as Note[],
  nextId: 0,
  // Every write the view made, in the order it made them.
  patches: [] as { id: string; done: boolean; token: string }[],
  // What the next PATCH answers, staged by the case under test.
  answer: undefined as
    | ((id: string, done: boolean) => Promise<Note>)
    | undefined,
}));

// One id space for every note, so a row can never be answered for another note.
function nextNoteId(): string {
  world.nextId += 1;
  return String(world.nextId);
}

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    listNotes: (): Promise<Note[]> =>
      Promise.resolve(world.collection.map((note) => ({ ...note }))),
    updateNoteDone: (
      id: string,
      done: boolean,
      token = "token",
    ): Promise<Note> => {
      world.patches.push({ id, done, token });
      const answer = world.answer;
      if (!answer) {
        return Promise.reject(
          new Error(`no PATCH answer is staged for note ${id}`),
        );
      }
      return answer(id, done);
    },
  };
});

// This workspace installs are locked (`npm ci`) and ship no DOM library, so the
// component tests carry their own minimal DOM: just enough of Node, Element and
// Document for React 19's client renderer to commit, re-render and dispatch.
type Listener = (event: FEvent) => void;

class FNode {
  nodeType: number;
  nodeName: string;
  data = "";
  namespaceURI = "http://www.w3.org/1999/xhtml";
  parentNode: FNode | null = null;
  childNodes: FNode[] = [];
  ownerDocument: FDocument;
  private listeners = new Map<string, Set<Listener>>();

  constructor(nodeType: number, nodeName: string, ownerDocument?: FNode) {
    this.nodeType = nodeType;
    this.nodeName = nodeName;
    this.ownerDocument = (ownerDocument ?? this) as FDocument;
  }

  get firstChild(): FNode | null {
    return this.childNodes[0] ?? null;
  }
  get nextSibling(): FNode | null {
    const parent = this.parentNode;
    if (!parent) return null;
    return parent.childNodes[parent.childNodes.indexOf(this) + 1] ?? null;
  }
  get isConnected(): boolean {
    return this.parentNode ? this.parentNode.isConnected : this.nodeType === 9;
  }
  get textContent(): string {
    if (this.nodeType === 3 || this.nodeType === 8) return this.data;
    return this.childNodes.map((child) => child.textContent).join("");
  }
  set textContent(value: string) {
    for (const child of this.childNodes) child.parentNode = null;
    this.childNodes = [];
    if (value !== "") this.appendChild(new FText(value, this));
  }

  appendChild<T extends FNode>(child: T): T {
    child.parentNode?.removeChild(child);
    const stale = this.childNodes.indexOf(child);
    if (stale >= 0) this.childNodes.splice(stale, 1);
    child.parentNode = this;
    this.childNodes.push(child);
    return child;
  }
  insertBefore<T extends FNode>(child: T, before: FNode | null): T {
    if (before === null) return this.appendChild(child);
    child.parentNode?.removeChild(child);
    const index = this.childNodes.indexOf(before);
    child.parentNode = this;
    if (index < 0) this.childNodes.push(child);
    else this.childNodes.splice(index, 0, child);
    return child;
  }
  removeChild<T extends FNode>(child: T): T {
    const index = this.childNodes.indexOf(child);
    if (index >= 0) {
      this.childNodes.splice(index, 1);
      child.parentNode = null;
    }
    return child;
  }
  remove(): void {
    this.parentNode?.removeChild(this);
  }
  contains(other: FNode | null): boolean {
    let node = other;
    while (node) {
      if (node === this) return true;
      node = node.parentNode;
    }
    return false;
  }
  addEventListener(type: string, listener: Listener): void {
    const listeners = this.listeners.get(type) ?? new Set<Listener>();
    listeners.add(listener);
    this.listeners.set(type, listeners);
  }
  removeEventListener(type: string, listener: Listener): void {
    this.listeners.get(type)?.delete(listener);
  }
  dispatchEvent(event: FEvent): boolean {
    return FNode.bubble(this, event);
  }
  private static bubble(node: FNode | null, event: FEvent): boolean {
    if (node === null) return true;
    event.currentTarget = node;
    for (const listener of [...(node.listeners.get(event.type) ?? [])]) {
      listener(event);
    }
    if (event.propagationStopped) return true;
    return FNode.bubble(node.parentNode, event);
  }
}

class FText extends FNode {
  constructor(text: string, ownerDocument: FNode) {
    super(3, "#text", ownerDocument);
    this.data = text;
  }
  get nodeValue(): string {
    return this.data;
  }
  set nodeValue(value: string) {
    this.data = value;
  }
}

class FElement extends FNode {
  tagName: string;
  attributes = new Map<string, string>();
  style: Record<string, string> = {};
  value: unknown = "";
  checked = false;
  defaultValue = "";

  constructor(tag: string, ownerDocument: FNode) {
    super(1, tag.toUpperCase(), ownerDocument);
    this.tagName = tag.toUpperCase();
  }

  setAttribute(name: string, value: string): void {
    this.attributes.set(name, String(value));
  }
  getAttribute(name: string): string | null {
    return this.attributes.get(name) ?? null;
  }
  removeAttribute(name: string): void {
    this.attributes.delete(name);
  }
  hasAttribute(name: string): boolean {
    return this.attributes.has(name);
  }
  focus(): void {
    this.ownerDocument.activeElement = this;
  }
  blur(): void {
    if (this.ownerDocument.activeElement === this) {
      this.ownerDocument.activeElement = this.ownerDocument.body;
    }
  }
}

class FDocument extends FNode {
  documentElement: FElement;
  head: FElement;
  body: FElement;
  activeElement: FElement;
  // The browser tab title, as `document.title` is for a real document: what
  // the view writes here is what the tab shows.
  title = "";
  defaultView: unknown = null;

  constructor() {
    super(9, "#document");
    this.documentElement = new FElement("html", this);
    this.head = new FElement("head", this);
    this.body = new FElement("body", this);
    this.appendChild(this.documentElement);
    this.activeElement = this.body;
  }
  createElement(tag: string): FElement {
    return new FElement(tag, this);
  }
  createElementNS(namespace: string, tag: string): FElement {
    const element = new FElement(tag, this);
    element.namespaceURI = namespace;
    return element;
  }
  createTextNode(text: string): FText {
    return new FText(text, this);
  }
  createComment(text: string): FNode {
    const comment = new FNode(8, "#comment", this);
    comment.data = text;
    return comment;
  }
  createDocumentFragment(): FNode {
    return new FNode(11, "#document-fragment", this);
  }
}

type FEvent = {
  type: string;
  target: FNode;
  currentTarget: FNode;
  eventPhase: number;
  bubbles: boolean;
  cancelable: boolean;
  defaultPrevented: boolean;
  isTrusted: boolean;
  returnValue: boolean;
  timeStamp: number;
  detail: number;
  propagationStopped: boolean;
  preventDefault: () => void;
  stopPropagation: () => void;
};

const doc = new FDocument();
const win = {
  document: doc,
  addEventListener: (type: string, listener: Listener): void => {
    doc.addEventListener(type, listener);
  },
  removeEventListener: (type: string, listener: Listener): void => {
    doc.removeEventListener(type, listener);
  },
  requestAnimationFrame: (callback: () => void) => setTimeout(callback, 0),
  cancelAnimationFrame: (handle: ReturnType<typeof setTimeout>) =>
    clearTimeout(handle),
  navigator: globalThis.navigator,
  performance: globalThis.performance,
  HTMLIFrameElement: class HTMLIFrameElement {},
  HTMLDocument: class HTMLDocument {},
  Element: FElement,
  Node: FNode,
};
doc.defaultView = win;

// React reads these once, while it is being imported, so they must exist before
// the renderer is loaded.
for (const [name, value] of [
  ["window", win],
  ["document", doc],
  ["Element", FElement],
  ["HTMLElement", FElement],
  ["Node", FNode],
  ["IS_REACT_ACT_ENVIRONMENT", true],
] as [string, unknown][]) {
  Object.defineProperty(globalThis, name, {
    value,
    configurable: true,
    writable: true,
  });
}

const React = await import("react");
const { createRoot } = await import("react-dom/client");
const { App } = await import("./App");

type Root = ReturnType<typeof createRoot>;
// React's typings name the host node it mounts into; the shim above is the
// stand-in for it, so this is the one place the two meet.
type Host = Parameters<typeof createRoot>[0];

const config: RuntimeConfig = {
  realm: "notes-test",
  clientId: "notes-web",
  runId: "test",
  signInHint: "Test users: alice and bob",
};
const signedIn = {
  access_token: "token",
  profile: { preferred_username: "alice" },
} as unknown as User;
const manager = {
  signinRedirect: (): Promise<unknown> => Promise.resolve(undefined),
  signoutRedirect: (): Promise<unknown> => Promise.resolve(undefined),
} as unknown as UserManager;

function documentOrder(node: FNode, into: FNode[] = []): FNode[] {
  into.push(node);
  for (const child of node.childNodes) documentOrder(child, into);
  return into;
}

function findBy(
  root: FNode,
  predicate: (element: FElement) => boolean,
): FElement | undefined {
  for (const node of documentOrder(root)) {
    if (node instanceof FElement && predicate(node)) return node;
  }
  return undefined;
}

function fire(node: FNode, type: string): void {
  const event: FEvent = {
    type,
    target: node,
    currentTarget: node,
    eventPhase: 2,
    bubbles: true,
    cancelable: true,
    defaultPrevented: false,
    isTrusted: false,
    returnValue: true,
    timeStamp: 1,
    detail: 0,
    propagationStopped: false,
    preventDefault: () => {
      event.defaultPrevented = true;
    },
    stopPropagation: () => {
      event.propagationStopped = true;
    },
  };
  node.dispatchEvent(event);
}

let root: Root | null = null;
let live: FElement | null = null;

function closeView(): void {
  root?.unmount();
  root = null;
  live?.parentNode?.removeChild(live);
  live = null;
}

/** Commits `run` and everything it schedules, then hands control back. */
async function flush(run: () => void): Promise<void> {
  await React.act(async () => {
    run();
    await Promise.resolve();
  });
}

/** Renders the notes list view over `notes`, as if a user opened the list. */
async function viewNotes(notes: Note[]): Promise<FElement> {
  world.collection = notes;
  const container = doc.createElement("div");
  doc.body.appendChild(container);
  await flush(() => {
    closeView();
    root = createRoot(container as unknown as Host);
    live = container;
    root.render(
      React.createElement(App, {
        config,
        manager,
        initialUser: signedIn,
        initialError: "",
      }),
    );
  });
  return container;
}

function notesList(container: FElement): FElement {
  const list = findBy(
    container,
    (element) => element.getAttribute("aria-label") === "Your notes",
  );
  if (!list) throw new Error("the notes list is missing from the view");
  return list;
}

function noteRows(container: FElement): FElement[] {
  return documentOrder(notesList(container)).filter(
    (node) => node instanceof FElement && node.tagName === "LI",
  ) as FElement[];
}

/**
 * A note as the service answers it: the fields the contract declares of a
 * `Note`, with the done flag the case under test asks for.
 */
function noteWith(label: string, done: unknown): Note {
  return {
    id: nextNoteId(),
    label,
    createdAt: "2026-10-12T09:30:00Z",
    done,
  } as unknown as Note;
}

/** A note the service answered without a done flag on it at all. */
function flaglessNote(label: string): Note {
  return {
    id: nextNoteId(),
    label,
    createdAt: "2026-10-12T09:30:00Z",
  } as unknown as Note;
}

/** The note as storage holds it once `done` is saved: what a PATCH answers. */
function savedAs(note: Note, done: boolean): Note {
  return {
    id: note.id,
    label: note.label,
    createdAt: note.createdAt,
    done,
  };
}

/** The next PATCH is accepted and answers with `note`, as a save does. */
function patchSaves(note: Note): void {
  world.answer = () => Promise.resolve({ ...note });
}

/** The next PATCH is refused, as it is when the write cannot be taken. */
function patchFails(): void {
  world.answer = () =>
    Promise.reject(new Error("The notes service is unavailable"));
}

type Held = {
  resolve: (note: Note) => void;
  reject: (error: Error) => void;
};

/** The next PATCH stays unanswered until the case settles it: in flight. */
function patchHeld(): Held {
  let resolve!: (note: Note) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<Note>((accept, refuse) => {
    resolve = accept;
    reject = refuse;
  });
  world.answer = () => promise;
  return { resolve, reject };
}

/** The completion control a row offers, found by the role it announces. */
function toggleOf(row: FElement): FElement {
  const control = findBy(
    row,
    (element) => element.getAttribute("role") === "checkbox",
  );
  if (!control) throw new Error("the row offers no completion control");
  return control;
}

/** The row's title: the text that says which note the row is. */
function titleOf(row: FElement): FElement {
  const title = findBy(
    row,
    (element) =>
      element.tagName === "SPAN" &&
      (element.getAttribute("class") ?? "")
        .split(" ")
        .includes("note-row-title"),
  );
  if (!title) throw new Error("the row shows no title");
  return title;
}

/** The drawn mark a ticked control carries, if it carries one. */
function tickMarks(scope: FElement): FElement[] {
  return documentOrder(scope).filter(
    (node) => node instanceof FElement && node.tagName === "SVG",
  ) as FElement[];
}

/** Every element the view announces as an error. */
function alerts(container: FElement): FElement[] {
  return documentOrder(container).filter(
    (node) => node instanceof FElement && node.getAttribute("role") === "alert",
  ) as FElement[];
}

/** Every element whose whole text is the "Done" cue. */
function doneCues(scope: FElement): FElement[] {
  return documentOrder(scope).filter(
    (node) => node instanceof FElement && node.textContent.trim() === "Done",
  ) as FElement[];
}

/** Anything that would be a detail view opened over the list. */
function detailViews(container: FElement): FElement[] {
  return documentOrder(container).filter(
    (node) =>
      node instanceof FElement &&
      (node.tagName === "DIALOG" ||
        ["dialog", "alertdialog", "article", "region"].includes(
          node.getAttribute("role") ?? "",
        )),
  ) as FElement[];
}

/** Anything that would take the person away from the list. */
function navigations(container: FElement): FElement[] {
  return documentOrder(container).filter(
    (node) =>
      node instanceof FElement &&
      (node.tagName === "A" ||
        node.tagName === "NAV" ||
        (node.getAttribute("role") ?? "") === "navigation"),
  ) as FElement[];
}

/** The view's headings, in the order it shows them. */
function headingsOf(container: FElement): string[] {
  const found: string[] = [];
  for (const node of documentOrder(container)) {
    if (node instanceof FElement && /^H[1-6]$/.test(node.tagName)) {
      found.push(node.textContent);
    }
  }
  return found;
}

/**
 * What a person sees as an element's presentation: its class and its styling.
 * A style React has taken back is left out, as an empty one shows nothing.
 */
function treatment(element: FElement): string {
  const style = Object.entries(element.style)
    .filter(([, value]) => value !== "" && value !== undefined)
    .map(([key, value]) => `${key}:${value}`)
    .sort()
    .join(";");
  return `class=${element.getAttribute("class") ?? ""}|style=${style}`;
}

/** Whether the control has been put into a third, in-between state. */
function indeterminate(control: FElement): boolean {
  return (
    (control as unknown as { indeterminate?: boolean }).indeterminate === true
  );
}

/** The row the list shows at `position`, or the failure that says it does not. */
function rowAt(container: FElement, position: number): FElement {
  const row = noteRows(container)[position];
  if (!row) throw new Error(`the list shows no row at position ${position}`);
  return row;
}

/** The one row of a one-note list. */
function onlyRow(container: FElement): FElement {
  const rows = noteRows(container);
  if (rows.length !== 1) {
    throw new Error(`expected one row, found ${rows.length}`);
  }
  return rowAt(container, 0);
}

/** Clicks the control, committing the click and everything it schedules. */
async function click(control: FElement): Promise<void> {
  await flush(() => {
    fire(control, "click");
  });
}

/** Lets every answer the view is waiting on arrive and reach the row. */
async function settle(): Promise<void> {
  for (let turn = 0; turn < 6; turn += 1) {
    await flush(() => undefined);
  }
}

afterEach(async () => {
  await flush(closeView);
  world.collection = [];
  world.patches = [];
  world.answer = undefined;
  doc.title = "";
});

describe("ticking the completion control on a note row", () => {
  it("ticks the control of a not-done note and sets its title apart", async () => {
    const plain = noteWith("a plain note", false);
    const other = noteWith("another plain note", false);
    const container = await viewNotes([plain, other]);
    const row = rowAt(container, 0);
    const untouched = rowAt(container, 1);
    const control = toggleOf(row);
    const title = titleOf(row);
    // Before the click the note reads as not done, in the standard row.
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(tickMarks(control)).toHaveLength(0);
    expect(doneCues(row)).toHaveLength(0);
    expect(treatment(title)).toBe(treatment(titleOf(untouched)));

    patchSaves(savedAs(plain, true));
    await click(control);
    await settle();

    // The control is ticked, by the mark it carries as well as by what it says.
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(tickMarks(control)).toHaveLength(1);
    // The title is now set apart from the one a not-done note wears, and by
    // more than colour: it is struck through, and the row carries the cue.
    expect(treatment(title)).not.toBe(treatment(titleOf(untouched)));
    expect(title.style["textDecorationLine"]).toBe("line-through");
    expect(doneCues(row).map((cue) => cue.textContent.trim())).toEqual([
      "Done",
    ]);
    expect(row.getAttribute("class")).toContain("note-row-done");
    // The note it is has not changed, and the control now offers the other move.
    expect(title.textContent).toBe("a plain note");
    expect(control.getAttribute("aria-label")).toBe(
      'Mark "a plain note" as not done',
    );
    // It was the click that moved the state, and only this note's.
    expect(world.patches).toEqual([
      { id: plain.id, done: true, token: "token" },
    ]);
    expect(toggleOf(untouched).getAttribute("aria-checked")).toBe("false");
  });

  it("unticks the control of a done note and takes the distinction off its title", async () => {
    const finished = noteWith("a finished note", true);
    const open = noteWith("an open note", false);
    const container = await viewNotes([finished, open]);
    const row = rowAt(container, 0);
    const standard = rowAt(container, 1);
    const control = toggleOf(row);
    const title = titleOf(row);
    // Before the click the note reads as done, in the row done notes wear.
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(tickMarks(control)).toHaveLength(1);
    expect(treatment(title)).not.toBe(treatment(titleOf(standard)));
    const doneRow = treatment(row);

    patchSaves(savedAs(finished, false));
    await click(control);
    await settle();

    // The control is unticked again, and the title is back to the one every
    // not-done row wears - no cue, no strike, no dimming.
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(tickMarks(control)).toHaveLength(0);
    expect(treatment(title)).toBe(treatment(titleOf(standard)));
    expect(treatment(row)).toBe(treatment(standard));
    expect(treatment(row)).not.toBe(doneRow);
    expect(doneCues(row)).toHaveLength(0);
    expect(row.textContent).toBe("a finished note");
    expect(control.getAttribute("aria-label")).toBe(
      'Mark "a finished note" as done',
    );
    expect(world.patches).toEqual([
      { id: finished.id, done: false, token: "token" },
    ]);
    expect(toggleOf(standard).getAttribute("aria-checked")).toBe("false");
  });

  it("toggles from the row itself, opening no detail view, dialog or link", async () => {
    const keep = noteWith("keep me", false);
    const target = noteWith("toggle me", true);
    const container = await viewNotes([keep, target]);
    const first = rowAt(container, 0);
    const second = rowAt(container, 1);
    const control = toggleOf(second);
    const list = notesList(container);
    const headings = headingsOf(container);
    expect(headings).toEqual(["Notes"]);

    // The control is part of the row, not of a view it opens: nothing above it
    // is a link, a form or a dialog, so its click cannot be answered by one.
    for (let node = control.parentNode; node !== null; node = node.parentNode) {
      if (node instanceof FElement) {
        expect(["A", "NAV", "FORM", "DIALOG"]).not.toContain(node.tagName);
      }
    }
    expect(detailViews(container)).toHaveLength(0);
    expect(navigations(container)).toHaveLength(0);

    patchSaves(savedAs(target, false));
    await click(control);
    await settle();

    // The click moved the state and nothing else: no dialog or detail panel
    // appeared, no link or navigation appeared, the headings are the ones the
    // list already showed, and the list is still the only list.
    expect(detailViews(container)).toHaveLength(0);
    expect(navigations(container)).toHaveLength(0);
    expect(headingsOf(container)).toEqual(headings);
    expect(
      documentOrder(container).filter(
        (node) =>
          node instanceof FElement &&
          node.getAttribute("aria-label") === "Your notes",
      ),
    ).toHaveLength(1);
    // The same two rows, still children of the list, in the order they had.
    expect(noteRows(container)).toEqual([first, second]);
    expect(first.parentNode).toBe(list);
    expect(second.parentNode).toBe(list);
    expect(noteRows(container).map((row) => titleOf(row).textContent)).toEqual([
      "keep me",
      "toggle me",
    ]);
    // and the row that was clicked is now unticked, in place.
    expect(toggleOf(second).getAttribute("aria-checked")).toBe("false");
    expect(treatment(second)).toBe(treatment(first));
  });

  it("asks the service to save the note's own id and the state it moved to", async () => {
    const keep = noteWith("keep me", false);
    const target = noteWith("toggle me", false);
    const other = noteWith("toggle me too", true);
    const container = await viewNotes([keep, target, other]);
    expect(world.patches).toHaveLength(0);

    patchSaves(savedAs(target, true));
    await click(toggleOf(rowAt(container, 1)));
    await settle();
    // One write, for the note whose control was clicked, carrying the state the
    // control moved to, made as the signed-in person.
    expect(world.patches).toEqual([
      { id: target.id, done: true, token: "token" },
    ]);

    // The other direction, on another note: again that note's own id, and the
    // state it has just left being done.
    patchSaves(savedAs(other, false));
    await click(toggleOf(rowAt(container, 2)));
    await settle();
    expect(world.patches).toHaveLength(2);
    expect(world.patches[1]).toEqual({
      id: other.id,
      done: false,
      token: "token",
    });
    // One write per click, and the note never clicked was never written to.
    expect(world.patches.map((patch) => patch.id)).toEqual([
      target.id,
      other.id,
    ]);
  });
});

describe("what the row shows while the PATCH is out and when it answers", () => {
  it("shows the state the person asked for while the PATCH is still out", async () => {
    const plain = noteWith("a plain note", false);
    const other = noteWith("another plain note", false);
    const container = await viewNotes([plain, other]);
    const row = rowAt(container, 0);
    const control = toggleOf(row);
    const standardRow = treatment(rowAt(container, 1));
    const held = patchHeld();

    await click(control);

    // The write is out and nothing has answered it, yet the row already reads
    // as done: the click is not held up waiting for the service.
    expect(world.patches).toEqual([
      { id: plain.id, done: true, token: "token" },
    ]);
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(tickMarks(control)).toHaveLength(1);
    expect(doneCues(row).map((cue) => cue.textContent.trim())).toEqual([
      "Done",
    ]);
    expect(treatment(row)).not.toBe(standardRow);
    // Nothing has been refused, so nothing is claimed as an error either.
    expect(alerts(container)).toHaveLength(0);
    // The rest of the list is untouched and still answers its own clicks.
    expect(treatment(rowAt(container, 1))).toBe(standardRow);

    await settle();
    // Still in flight, still showing the state that was asked for.
    expect(alerts(container)).toHaveLength(0);
    expect(control.getAttribute("aria-checked")).toBe("true");

    held.resolve(savedAs(plain, true));
    await settle();
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(alerts(container)).toHaveLength(0);
  });

  it("keeps the state the PATCH response confirms", async () => {
    const plain = noteWith("a plain note", false);
    const container = await viewNotes([plain]);
    const row = onlyRow(container);
    const control = toggleOf(row);
    patchSaves(savedAs(plain, true));
    await click(control);
    await settle();
    // The service confirmed the tick, so the row keeps it.
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(tickMarks(control)).toHaveLength(1);
    expect(doneCues(row)).toHaveLength(1);
    expect(row.style["opacity"]).toBe("0.72");
  });

  it("takes the server's answer over the state the click asked for", async () => {
    const plain = noteWith("a plain note", false);
    const container = await viewNotes([plain]);
    const row = onlyRow(container);
    const control = toggleOf(row);
    const standardRow = treatment(row);
    const standardTitle = treatment(titleOf(row));
    const held = patchHeld();

    await click(control);
    // The hope is shown while the request is out ...
    expect(control.getAttribute("aria-checked")).toBe("true");
    // ... and the note that comes back has the last word: storage says not done.
    held.resolve(savedAs(plain, false));
    await settle();

    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(tickMarks(control)).toHaveLength(0);
    expect(doneCues(row)).toHaveLength(0);
    expect(treatment(row)).toBe(standardRow);
    expect(treatment(titleOf(row))).toBe(standardTitle);
    // The service was asked once, and the row is not left mid-way.
    expect(world.patches).toHaveLength(1);
    expect(alerts(container)).toHaveLength(0);
  });

  it("shows the note the PATCH response answered, not the one the click started from", async () => {
    const plain = noteWith("a plain note", false);
    const container = await viewNotes([plain]);
    const row = onlyRow(container);
    // The service answers with the note as it now holds it, which is not the
    // row the click started from.
    patchSaves({
      id: plain.id,
      label: "renamed elsewhere",
      createdAt: "2026-10-12T09:30:00Z",
      done: true,
    });
    await click(toggleOf(row));
    await settle();
    expect(titleOf(row).textContent).toBe("renamed elsewhere");
    expect(toggleOf(row).getAttribute("aria-checked")).toBe("true");
    expect(row.textContent).toContain("Done");
  });

  it("reverts the control and shows an error when the save fails", async () => {
    const plain = noteWith("a plain note", false);
    const other = noteWith("another plain note", false);
    const container = await viewNotes([plain, other]);
    const row = rowAt(container, 0);
    const control = toggleOf(row);
    const standardRow = treatment(row);
    const standardTitle = treatment(titleOf(row));
    const held = patchHeld();

    await click(control);
    expect(control.getAttribute("aria-checked")).toBe("true");
    held.reject(new Error("The notes service is unavailable"));
    await settle();

    // The row is as it was before the click, not as the click hoped.
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(tickMarks(control)).toHaveLength(0);
    expect(doneCues(row)).toHaveLength(0);
    expect(treatment(row)).toBe(standardRow);
    expect(treatment(titleOf(row))).toBe(standardTitle);
    expect(row.textContent).toBe("a plain note");
    // The person is told the state did not save.
    const shown = alerts(container);
    expect(shown).toHaveLength(1);
    expect((shown[0] as FElement).textContent).toMatch(/could not be saved/i);
    // The note that was never clicked is left exactly as it was.
    expect(treatment(rowAt(container, 1))).toBe(standardRow);
    // The control is not left dead: a save that succeeds is shown.
    patchSaves(savedAs(plain, true));
    await click(control);
    await settle();
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(tickMarks(control)).toHaveLength(1);
    expect(world.patches).toHaveLength(2);
  });

  it("leaves a done note done when the save that would untick it fails", async () => {
    const finished = noteWith("a finished note", true);
    const container = await viewNotes([finished]);
    const row = onlyRow(container);
    const control = toggleOf(row);
    const doneRow = treatment(row);
    const doneTitle = treatment(titleOf(row));
    const held = patchHeld();

    await click(control);
    expect(world.patches).toEqual([
      { id: finished.id, done: false, token: "token" },
    ]);
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(tickMarks(control)).toHaveLength(0);
    held.reject(new Error("The notes service is unavailable"));
    await settle();

    // Back to done, cue and struck title and all.
    expect(control.getAttribute("aria-checked")).toBe("true");
    expect(tickMarks(control)).toHaveLength(1);
    expect(treatment(row)).toBe(doneRow);
    expect(treatment(titleOf(row))).toBe(doneTitle);
    expect(doneCues(row).map((cue) => cue.textContent.trim())).toEqual([
      "Done",
    ]);
    const shown = alerts(container);
    expect(shown).toHaveLength(1);
    expect((shown[0] as FElement).textContent).toMatch(/could not be saved/i);
  });

  it("shows an error for a save the service refuses outright", async () => {
    const plain = noteWith("a plain note", false);
    const container = await viewNotes([plain]);
    const row = onlyRow(container);
    const control = toggleOf(row);
    patchFails();
    await click(control);
    await settle();
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(tickMarks(control)).toHaveLength(0);
    expect(alerts(container)).toHaveLength(1);
    expect(row.textContent).toBe("a plain note");
  });
});

describe("the two states the completion control accepts", () => {
  it("announces the control as a checkbox that answers checked or unchecked", async () => {
    const plain = noteWith("a plain note", false);
    const container = await viewNotes([plain]);
    const control = toggleOf(onlyRow(container));
    expect(control.tagName).toBe("BUTTON");
    expect(control.getAttribute("role")).toBe("checkbox");
    // A button that is not a submit button: the click stays with the row.
    expect(control.getAttribute("type")).toBe("button");
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(indeterminate(control)).toBe(false);
    expect(control.getAttribute("aria-label")).toMatch(
      /^Mark "a plain note" as (done|not done)$/,
    );
  });

  // Every shape the service can answer for `done`, and whether the control
  // starts ticked for it. Only two states are on offer over all of them.
  const flagShapes: {
    name: string;
    flag: boolean | null | "missing";
    ticked: boolean;
  }[] = [
    { name: "true", flag: true, ticked: true },
    { name: "false", flag: false, ticked: false },
    { name: "null", flag: null, ticked: false },
    { name: "absent", flag: "missing", ticked: false },
  ];

  it.each(flagShapes)(
    "reads a done flag that is $name as one of two states, and never a third",
    async ({ name, flag, ticked }) => {
      const label = `a note whose flag is ${name}`;
      const note =
        flag === "missing" ? flaglessNote(label) : noteWith(label, flag);
      const container = await viewNotes([note]);
      const row = onlyRow(container);
      const control = toggleOf(row);
      const title = titleOf(row);
      const seen: string[] = [];

      const record = (): void => {
        const state = control.getAttribute("aria-checked");
        // The control answers checked or unchecked: nothing else is announced.
        expect(state === "true" || state === "false").toBe(true);
        expect(indeterminate(control)).toBe(false);
        // The mark it carries is the state it announces, and no more.
        expect(tickMarks(control)).toHaveLength(state === "true" ? 1 : 0);
        expect(control.getAttribute("aria-label")).toMatch(
          /^Mark "a note whose flag is .*" as (done|not done)$/,
        );
        seen.push(state as string);
      };

      record();
      expect(seen[0]).toBe(ticked ? "true" : "false");
      const startRow = treatment(row);
      const startTitle = treatment(title);

      // One click moves the control to the other state ...
      const asked = seen[0] !== "true";
      patchSaves(savedAs(note, asked));
      await click(control);
      await settle();
      record();
      expect(seen[1]).toBe(asked ? "true" : "false");
      expect(treatment(row)).not.toBe(startRow);
      expect(treatment(title)).not.toBe(startTitle);

      // ... and a second click moves it back: there is no third state to land
      // in, and no state between the two.
      patchSaves(savedAs(note, !asked));
      await click(control);
      await settle();
      record();
      expect(seen[2]).toBe(seen[0]);
      expect(treatment(row)).toBe(startRow);
      expect(treatment(title)).toBe(startTitle);
      expect(new Set(seen).size).toBe(2);
      // Every move was a write of a boolean flag for this note.
      expect(world.patches).toEqual([
        { id: note.id, done: asked, token: "token" },
        { id: note.id, done: !asked, token: "token" },
      ]);
    },
  );
});
