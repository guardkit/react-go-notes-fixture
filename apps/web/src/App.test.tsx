import { afterEach, describe, expect, it, vi } from "vitest";
import type { User, UserManager } from "oidc-client-ts";
import { NoteGoneError } from "./api";
import type { Note } from "./api";
import type { RuntimeConfig } from "./config";

// The surface under test is the "N notes" summary the notes list view renders
// above the list, so every case here mounts the real component over a
// collection and reads what the browser would show. Notes reach the view only
// through `listNotes`, which is stubbed with a fixed collection, so the count is
// deterministic and is never seeded into the component through a back door.
const world = vi.hoisted(() => ({
  collection: [] as Note[],
  nextId: 0,
  // Every request the view makes, counted, so a case can say the toggle cost
  // exactly one PATCH and no second read of the list.
  listCalls: 0,
  patches: [] as { id: string; done: boolean }[],
  // The id the stubbed service answers 404 for: the note that went away.
  goneId: null as string | null,
}));

// One id space for every note, so a saved note can never reuse a key that the
// rendered list already holds.
function nextNoteId(): string {
  world.nextId += 1;
  return String(world.nextId);
}

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    listNotes: (): Promise<Note[]> => {
      world.listCalls += 1;
      return Promise.resolve(world.collection.map((note) => ({ ...note })));
    },
    createNote: (label: string): Promise<Note> => {
      const saved: Note = {
        id: nextNoteId(),
        label,
        createdAt: "2026-10-13T09:00:00Z",
        done: false,
      };
      world.collection.push(saved);
      return Promise.resolve(saved);
    },
    // The stubbed PATCH route: it answers with the note carrying the flag the
    // view asked for, and 404s for the id the case marked as gone.
    updateNoteDone: (id: string, done: boolean): Promise<Note> => {
      world.patches.push({ id, done });
      if (world.goneId === id) return Promise.reject(new NoteGoneError());
      const stored = world.collection.find((note) => note.id === id);
      if (!stored) return Promise.reject(new NoteGoneError());
      stored.done = done;
      return Promise.resolve({ ...stored });
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

/** A collection of `count` distinct notes, labelled as a note list would be. */
function collectionOf(count: number): Note[] {
  return Array.from({ length: count }, (_unused, index) => ({
    id: nextNoteId(),
    label: `Note ${index + 1}`,
    createdAt: "2026-10-12T09:30:00Z",
    done: false,
  }));
}

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
  world.listCalls = 0;
  world.patches = [];
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

/** Types a label and saves it: the only way the view itself adds a note. */
async function saveNote(container: FElement, label: string): Promise<void> {
  const input = findBy(container, (element) => element.tagName === "INPUT");
  const form = findBy(container, (element) => element.tagName === "FORM");
  if (!input || !form) throw new Error("the note form is missing");
  input.value = label;
  await flush(() => {
    fire(input, "input");
  });
  await flush(() => {
    fire(form, "submit");
  });
}

/**
 * The notes list, found by the accessible name the view gives it rather than by
 * a test-only hook, so the assertions survive a change of attribute.
 */
function notesList(container: FElement): FElement {
  const list = findBy(
    container,
    (element) => element.getAttribute("aria-label") === "Your notes",
  );
  if (!list) throw new Error("the notes list is missing from the view");
  return list;
}

/** Every element whose whole text is a total, e.g. "12 notes". */
function totals(container: FElement): FElement[] {
  return documentOrder(container).filter(
    (node) => node instanceof FElement && /^\d+ notes$/.test(node.textContent),
  ) as FElement[];
}

/** The rows the list renders, in the order the list renders them. */
function noteRows(container: FElement): FElement[] {
  return documentOrder(notesList(container)).filter(
    (node) => node instanceof FElement && node.tagName === "LI",
  ) as FElement[];
}

/** One field a row shows about its note, read apart from the row's controls. */
function noteField(row: FElement, testId: string): string {
  const field = findBy(
    row,
    (element) => element.getAttribute("data-testid") === testId,
  );
  if (!field) throw new Error(`a note row shows no ${testId}`);
  return field.textContent;
}

/** The label a row shows for its note. */
function noteLabel(row: FElement): string {
  return noteField(row, "note-label");
}

/** The flag a row shows for its note: "done" or "not done". */
function noteFlag(row: FElement): string {
  return noteField(row, "note-done");
}

/**
 * The toggle of a note: reached as a person reaches it — a button, named for
 * the note it belongs to — never through a hook the view only exists for.
 */
function toggleFor(container: FElement, label: string): FElement {
  const button = findBy(
    container,
    (element) =>
      element.tagName === "BUTTON" &&
      (element.getAttribute("aria-label") ?? "").includes(`"${label}"`),
  );
  if (!button) throw new Error(`no toggle button names the note "${label}"`);
  return button;
}

/** What the view reports as an alert, if it reports anything at all. */
function alerts(container: FElement): string[] {
  return documentOrder(container)
    .filter(
      (node) =>
        node instanceof FElement && node.getAttribute("role") === "alert",
    )
    .map((node) => node.textContent);
}

/** Presses a control and lets everything that press schedules settle. */
async function press(control: FElement): Promise<void> {
  await flush(() => {
    fire(control, "click");
  });
}

/**
 * The one total shown above the list. A total that is missing, repeated, or
 * placed inside the list is not the summary this feature specifies, so the
 * checks that hold it to that shape live here, once.
 */
function totalAboveTheList(container: FElement): FElement {
  const shown = totals(container);
  expect(shown).toHaveLength(1);
  const total = shown[0];
  if (!total) throw new Error("no total is displayed above the notes list");
  const list = notesList(container);
  const order = documentOrder(container);
  expect(order.indexOf(total)).toBeLessThan(order.indexOf(list));
  expect(list.contains(total)).toBe(false);
  return total;
}

/** The browser tab title, read where a person sees it: `document.title`. */
function tabTitle(): string {
  return doc.title;
}

/** The number in the tab title's "Notes (N)". NaN if it is in no such format. */
function tabTitleCount(): number {
  const match = /^Notes \((\d+)\)$/.exec(tabTitle());
  return match ? Number(match[1]) : Number.NaN;
}

afterEach(async () => {
  await flush(closeView);
  world.collection = [];
  world.goneId = null;
  // A title is never inherited from the case before it, so no assertion here
  // can pass on a stale tab title.
  doc.title = "";
});

describe("total note count above the notes list", () => {
  it('displays the total as "12 notes" for twelve notes', async () => {
    const container = await viewNotes(collectionOf(12));
    expect(totalAboveTheList(container).textContent).toBe("12 notes");
    expect(noteRows(container)).toHaveLength(12);
  });

  it("places the total above the list, outside it", async () => {
    const container = await viewNotes(collectionOf(12));
    const total = totalAboveTheList(container);
    const list = notesList(container);
    expect(list.contains(total)).toBe(false);
    expect(total.textContent).toBe("12 notes");
  });

  it.each([0, 1, 5, 12, 37])(
    "shows the size of the collection it renders (%i notes)",
    async (size) => {
      const notes = collectionOf(size);
      const container = await viewNotes(notes);
      const rows = noteRows(container);
      expect(totalAboveTheList(container).textContent).toBe(
        `${notes.length} notes`,
      );
      // The total and the rows come off the one collection: same length, and
      // one row per note in it, so the two can never disagree.
      expect(rows).toHaveLength(notes.length);
      // The rows show the notes of the collection they were rendered from.
      expect(rows.map((row) => noteLabel(row))).toEqual(
        notes.map((note) => note.label),
      );
    },
  );

  it("follows the collection when the view itself adds a note", async () => {
    const notes = collectionOf(12);
    const container = await viewNotes(notes);
    expect(totalAboveTheList(container).textContent).toBe("12 notes");
    await saveNote(container, "Note 13");
    expect(notes).toHaveLength(13);
    expect(totalAboveTheList(container).textContent).toBe("13 notes");
    expect(noteRows(container)).toHaveLength(13);
  });

  it("does not display the total as part of any individual note", async () => {
    const container = await viewNotes(collectionOf(12));
    const rows = noteRows(container);
    expect(rows).toHaveLength(12);
    for (const row of rows) {
      // A row says which note it is, never how many notes there are.
      expect(row.textContent).not.toMatch(/\d+\s+notes/);
      expect(totals(row)).toHaveLength(0);
    }
    // The total is one node in the whole view, so it is not repeated per note.
    expect(totals(container)).toHaveLength(1);
    expect(notesList(container).contains(totalAboveTheList(container))).toBe(
      false,
    );
  });
});

// The tab title is the same count seen at a different surface, so every case
// here reads `document.title` and compares it with what the list actually
// renders, never with a number the test computed for the component.
describe("note count in the browser tab title", () => {
  it.each([0, 1, 3, 100])(
    'titles the tab "Notes (%i)" and holds no other text',
    async (size) => {
      const notes = collectionOf(size);
      const container = await viewNotes(notes);
      expect(tabTitle()).toBe(`Notes (${size})`);
      // The literal prefix, the count, and nothing else in the tab.
      expect(tabTitle()).toMatch(/^Notes \(\d+\)$/);
      // The title counts the very notes the list renders.
      expect(noteRows(container)).toHaveLength(notes.length);
      expect(tabTitleCount()).toBe(noteRows(container).length);
    },
  );

  it("carries the count the moment the list is first shown", async () => {
    const container = await viewNotes(collectionOf(3));
    expect(tabTitle()).toBe("Notes (3)");
    // The two surfaces of the one collection agree on load.
    expect(totalAboveTheList(container).textContent).toBe("3 notes");
    expect(tabTitleCount()).toBe(
      Number(totalAboveTheList(container).textContent.replace(" notes", "")),
    );
  });

  it("titles an empty collection Notes (0)", async () => {
    const container = await viewNotes([]);
    expect(tabTitle()).toBe("Notes (0)");
    expect(noteRows(container)).toHaveLength(0);
  });

  it("counts the note the view itself adds, without a reload", async () => {
    const container = await viewNotes(collectionOf(2));
    expect(tabTitle()).toBe("Notes (2)");
    await saveNote(container, "Note 3");
    // The same view, never reloaded: the title moved with the row it added.
    expect(tabTitle()).toBe("Notes (3)");
    expect(noteRows(container)).toHaveLength(3);
    expect(tabTitleCount()).toBe(noteRows(container).length);
  });

  it("drops the count when the last note is gone from the collection", async () => {
    const notes = collectionOf(1);
    await viewNotes(notes);
    expect(tabTitle()).toBe("Notes (1)");
    // The view offers no delete control, so a note leaves the collection
    // between one showing of the list and the next. The title is derived from
    // that collection again, so it cannot keep the count it last showed.
    notes.length = 0;
    const container = await viewNotes(notes);
    expect(tabTitle()).toBe("Notes (0)");
    expect(tabTitle()).not.toBe("Notes (1)");
    expect(noteRows(container)).toHaveLength(0);
  });

  it("settles on the list's count after an add and a removal", async () => {
    const notes = collectionOf(2);
    const container = await viewNotes(notes);
    expect(tabTitle()).toBe("Notes (2)");
    await saveNote(container, "Note 3");
    expect(tabTitle()).toBe("Notes (3)");
    // The added note is then the one that leaves the collection the list is
    // next shown over: the title lands on that list's count, not between.
    notes.pop();
    const shown = await viewNotes(notes);
    expect(tabTitle()).toBe("Notes (2)");
    expect(tabTitleCount()).toBe(noteRows(shown).length);
  });

  it("titles the tab from the collection the list shows, not from a copy", async () => {
    const notes = collectionOf(4);
    const container = await viewNotes(notes);
    expect(tabTitle()).toBe("Notes (4)");
    await saveNote(container, "Note 5");
    // Five notes shown, five notes titled: the title and the list are one
    // collection, so neither can be ahead of the other.
    expect(noteRows(container)).toHaveLength(5);
    expect(tabTitle()).toBe("Notes (5)");
    expect(totalAboveTheList(container).textContent).toBe("5 notes");
  });
});

// The done flag is the one field a person may change on a note they own, and
// the only answer the view takes it from is the note the PATCH route returns.
// So every case here presses the row's own control and reads the row back,
// while counting the requests the view made to prove the flag was never
// re-read from the list.
describe("done-flag toggle on a note row", () => {
  it("marks a note done from the PATCH answer, with no follow-up read", async () => {
    const notes = collectionOf(2);
    const container = await viewNotes(notes);
    // The list was read once, on opening, and nothing has been patched yet.
    expect(world.listCalls).toBe(1);
    expect(world.patches).toEqual([]);

    await press(toggleFor(container, "Note 1"));

    // Exactly one PATCH, and it asked for the flag to be set on that note.
    expect(world.patches).toEqual([{ id: notes[0]!.id, done: true }]);
    // No second GET: the row below carries the flag the PATCH answered with.
    expect(world.listCalls).toBe(1);

    const rows = noteRows(container);
    expect(rows).toHaveLength(2);
    expect(noteFlag(rows[0]!)).toBe("done");
    // Every other field of the toggled note is shown exactly as it was.
    expect(noteLabel(rows[0]!)).toBe("Note 1");
    // And the note that was not touched is untouched.
    expect(noteLabel(rows[1]!)).toBe("Note 2");
    expect(noteFlag(rows[1]!)).toBe("not done");
    // The flag is the only thing that moved: same collection, same count.
    expect(totalAboveTheList(container).textContent).toBe("2 notes");
    expect(tabTitle()).toBe("Notes (2)");
  });

  it("marks a done note not done again", async () => {
    const notes = collectionOf(1);
    notes[0]!.done = true;
    const container = await viewNotes(notes);
    expect(noteFlag(noteRows(container)[0]!)).toBe("done");

    await press(toggleFor(container, "Note 1"));

    expect(world.patches).toEqual([{ id: notes[0]!.id, done: false }]);
    expect(world.listCalls).toBe(1);
    expect(noteFlag(noteRows(container)[0]!)).toBe("not done");
  });

  it("toggles the same note back and forth without re-reading the list", async () => {
    const notes = collectionOf(2);
    const container = await viewNotes(notes);
    await press(toggleFor(container, "Note 2"));
    expect(noteFlag(noteRows(container)[1]!)).toBe("done");
    await press(toggleFor(container, "Note 2"));
    expect(noteFlag(noteRows(container)[1]!)).toBe("not done");
    // Two presses, two PATCHes, and still the one read the list opened with.
    expect(world.patches).toHaveLength(2);
    expect(world.patches.map((patch) => patch.done)).toEqual([true, false]);
    expect(world.listCalls).toBe(1);
    expect(noteRows(container)).toHaveLength(2);
  });

  it("offers the toggle as a button named for its note, not a bare link", async () => {
    const container = await viewNotes(collectionOf(2));
    const rows = noteRows(container);
    for (const [index, row] of rows.entries()) {
      const label = `Note ${index + 1}`;
      const toggle = toggleFor(container, label);
      // Reachable by role: a real button, carrying its own accessible name.
      expect(toggle.tagName).toBe("BUTTON");
      expect(toggle.getAttribute("aria-label")).toContain(`"${label}"`);
      expect(row.contains(toggle)).toBe(true);
      // The row holds no anonymous link standing in for the control.
      const links = documentOrder(row).filter(
        (node) => node instanceof FElement && node.tagName === "A",
      );
      expect(links).toHaveLength(0);
    }
  });

  it("reports a note that has gone away and drops its row", async () => {
    const notes = collectionOf(2);
    world.goneId = notes[1]!.id;
    const container = await viewNotes(notes);

    await press(toggleFor(container, "Note 2"));

    // One PATCH, refused with 404, and no read of the list behind it.
    expect(world.patches).toHaveLength(1);
    expect(world.listCalls).toBe(1);
    // The note is said to be gone, the way a refused token is said out loud.
    expect(alerts(container)).toContain("That note is gone.");
    // The row does not stay behind holding the flag it last showed.
    const rows = noteRows(container);
    expect(rows).toHaveLength(1);
    expect(rows.map((row) => noteLabel(row))).toEqual(["Note 1"]);
    expect(totalAboveTheList(container).textContent).toBe("1 notes");
    expect(tabTitle()).toBe("Notes (1)");
  });

  it("keeps the view standing when the note it was told about is gone", async () => {
    const notes = collectionOf(1);
    world.goneId = notes[0]!.id;
    const container = await viewNotes(notes);

    await press(toggleFor(container, "Note 1"));

    // No crash: the view is still mounted, still shows its heading and its
    // form, and still lets the person add a note.
    expect(
      documentOrder(container).some((node) => node.textContent === "Notes"),
    ).toBe(true);
    expect(
      findBy(container, (element) => element.tagName === "INPUT"),
    ).toBeDefined();
    expect(noteRows(container)).toHaveLength(0);
    expect(alerts(container)).toContain("That note is gone.");
  });
});
