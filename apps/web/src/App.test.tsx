import { readdirSync, readFileSync } from "node:fs";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { User, UserManager } from "oidc-client-ts";
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
    listNotes: (): Promise<Note[]> =>
      Promise.resolve(world.collection.map((note) => ({ ...note }))),
    createNote: (label: string): Promise<Note> => {
      const saved: Note = {
        id: nextNoteId(),
        label,
        createdAt: "2026-10-13T09:00:00Z",
        // A note that has never been marked complete still carries the flag.
        done: false,
      };
      world.collection.push(saved);
      return Promise.resolve(saved);
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

/**
 * A collection whose notes carry the completion flags `flags`, in the order the
 * list shows them: the done state a row is checked against comes from the
 * payload the list was shown over, never from a copy kept in this test.
 */
function flaggedCollection(flags: boolean[]): Note[] {
  return flags.map((done, index) => ({
    id: nextNoteId(),
    label: `Note ${index + 1}`,
    createdAt: "2026-10-12T09:30:00Z",
    done,
  }));
}

/** `count` completion flags, alternating done and open from the first note. */
function alternatingFlags(count: number): boolean[] {
  return Array.from({ length: count }, (_unused, index) => index % 2 === 0);
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

/**
 * The cells one row renders, in the order the row renders them. The row is a
 * status cell and a title cell, so the order here is the order a person's eye
 * meets them in.
 */
function rowCells(row: FElement): FElement[] {
  return row.childNodes.filter(
    (node): node is FElement => node instanceof FElement,
  );
}

function at<T>(items: T[], index: number, what: string): T {
  const item = items[index];
  if (item === undefined)
    throw new Error(`the list renders no ${what} at ${index}`);
  return item;
}

/** The status cell a row leads with: nothing else may come before it. */
function statusCell(row: FElement): FElement {
  return at(rowCells(row), 0, "status cell");
}

/** The cell the row gives the note's title, read beside the status cell. */
function titleCell(row: FElement): FElement {
  return at(rowCells(row), 1, "title cell");
}

function statusText(row: FElement): string {
  return statusCell(row).textContent;
}

function noteTitleText(row: FElement): string {
  return titleCell(row).textContent;
}

/** The style a committed element really carries, read as the browser sees it. */
function styleOf(element: FElement): Record<string, string> {
  return element.style;
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
      // The label is read from the title cell the row gives the note: the row
      // also leads with a status cell, which is the done flag's own surface.
      expect(rows.map(noteTitleText)).toEqual(notes.map((note) => note.label));
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

// What a row shows of its note's completion flag, read off the row itself: the
// done state is a check mark with the word "Done", the not-done state the word
// "Not done". Every case below views the real list over a collection, so the
// flag is always the one the payload carried into the view.
const DONE_STATE = "✓ Done";
const OPEN_STATE = "○ Not done";

// ASSUM-006: the narrowest supported viewport is a single mobile-class width.
const NARROWEST_VIEWPORT_PX = 320;

type ContractSchema = {
  name: string;
  properties: string[];
  types: Record<string, string>;
  required: string[];
  refs: string[];
};

/**
 * The objects contract/openapi.yaml declares, read from the contract while the
 * check runs: the properties each answers with, the type each property is
 * declared as, its `required` list, and the schemas it answers through.
 */
function contractSchemas(): ContractSchema[] {
  const text = readFileSync(
    new URL("../../../contract/openapi.yaml", import.meta.url),
    "utf8",
  );
  const schemas: ContractSchema[] = [];
  let current: ContractSchema | null = null;
  let inSchemas = false;
  let inProperties = false;
  let property: string | null = null;
  for (const raw of text.split("\n")) {
    const line = raw.replace(/\s+$/, "");
    if (!inSchemas) {
      if (line === "  schemas:") inSchemas = true;
      continue;
    }
    // The next key at the `components:` level ends the schemas block.
    if (/^  \w/.test(line)) break;
    const schema = /^    (\w+):$/.exec(line);
    if (schema?.[1]) {
      current = {
        name: schema[1],
        properties: [],
        types: {},
        required: [],
        refs: [],
      };
      schemas.push(current);
      inProperties = false;
      property = null;
      continue;
    }
    if (!current) continue;
    const ref = /\$ref: "#\/components\/schemas\/(\w+)"/.exec(line)?.[1];
    if (ref) {
      current.refs.push(ref);
      continue;
    }
    const required = /^      required: \[(.*)\]$/.exec(line)?.[1];
    if (required !== undefined) {
      current.required = required
        .split(",")
        .map((name) => name.trim())
        .filter((name) => name !== "");
      inProperties = false;
      continue;
    }
    if (line === "      properties:") {
      inProperties = true;
      continue;
    }
    if (/^      \w/.test(line)) {
      inProperties = false;
      continue;
    }
    const declared = /^        (\w+):$/.exec(line)?.[1];
    if (inProperties && declared) {
      property = declared;
      current.properties.push(declared);
      continue;
    }
    const type = /^          type: (\w+)$/.exec(line)?.[1];
    if (type && property) current.types[property] = type;
  }
  return schemas;
}

/**
 * The members of the each-note set: every contract object that answers with a
 * listed note, whether it shapes the note itself or answers through one. The
 * set is discovered from the contract, never listed in this test, so a note
 * object added to the contract is covered without editing this file.
 */
function eachNoteMembers(): ContractSchema[] {
  const schemas = contractSchemas();
  const noteObjects = schemas.filter((schema) =>
    ["id", "label", "createdAt"].every((property) =>
      schema.properties.includes(property),
    ),
  );
  const answersANote = (schema: ContractSchema): boolean =>
    noteObjects.some((note) => note.name === schema.name) ||
    schema.refs.some((ref) => noteObjects.some((note) => note.name === ref));
  return schemas.filter(answersANote);
}

/** The note object one member of the set answers with. */
function noteObjectBehind(member: ContractSchema): ContractSchema {
  const schemas = contractSchemas();
  const behind = schemas.find(
    (schema) =>
      schema.properties.includes("done") &&
      (schema.name === member.name || member.refs.includes(schema.name)),
  );
  if (!behind) {
    throw new Error(
      `${member.name} answers a listed note with no object carrying a done flag`,
    );
  }
  return behind;
}

/** A file on the web read path shapes a listed note when it says so. */
function shapesAListedNote(text: string): boolean {
  return (
    text.includes("listNotes") ||
    (text.includes("createdAt") && text.includes("label"))
  );
}

describe("done flag on every notes-list row", () => {
  it("presents every note's done status on its own row, unopened", async () => {
    const notes = flaggedCollection(alternatingFlags(12));
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    expect(rows).toHaveLength(notes.length);
    notes.forEach((note, index) => {
      const row = at(rows, index, "row");
      // The row says which note it is and whether it is done, in one glance.
      expect(noteTitleText(row)).toBe(note.label);
      expect(statusText(row)).toBe(note.done ? DONE_STATE : OPEN_STATE);
    });
    // Nothing has to be opened, expanded or clicked for the status to read.
    for (const row of rows) {
      const controls = documentOrder(row).filter(
        (node) =>
          node instanceof FElement &&
          ["BUTTON", "INPUT", "A", "SUMMARY", "DETAILS"].includes(node.tagName),
      );
      expect(controls).toHaveLength(0);
    }
  });

  it("shows the done state for a done note, the not-done state for an open one", async () => {
    const container = await viewNotes(flaggedCollection([true, false]));
    const rows = noteRows(container);
    expect(statusText(at(rows, 0, "row"))).toBe(DONE_STATE);
    expect(statusText(at(rows, 1, "row"))).toBe(OPEN_STATE);
    // The states are the two the flag can be in, and no third thing.
    expect(statusText(at(rows, 0, "row"))).not.toBe(
      statusText(at(rows, 1, "row")),
    );
  });

  it("marks a done row by more than one channel", async () => {
    const container = await viewNotes(flaggedCollection([true, false]));
    const rows = noteRows(container);
    const done = at(rows, 0, "row");
    const open = at(rows, 1, "row");
    // Channel one: the check mark and the written "Done" label.
    expect(statusText(done)).toContain("Done");
    expect(statusText(done)).toContain("✓");
    expect(statusText(open)).not.toContain("✓");
    expect(statusText(open)).toContain("Not done");
    // Channel two: the title itself is treated differently from an open title.
    expect(styleOf(titleCell(done)).textDecoration).toBe("line-through");
    expect(styleOf(titleCell(open)).textDecoration).not.toBe("line-through");
    expect(styleOf(titleCell(done)).fontWeight).not.toBe(
      styleOf(titleCell(open)).fontWeight,
    );
  });

  it("keeps the status cell in the same leading position on every row", async () => {
    const notes = flaggedCollection(alternatingFlags(6));
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    expect(rows.length).toBeGreaterThan(1);
    const widths = new Set<string>();
    rows.forEach((row, index) => {
      // The status cell leads the row: no text or other cell comes before it.
      expect(row.childNodes[0]).toBe(statusCell(row));
      expect(rowCells(row)).toHaveLength(2);
      expect(statusText(row)).toMatch(/^(✓ Done|○ Not done)$/);
      // The title follows it, in the second slot, on every row.
      expect(noteTitleText(row)).toBe(at(notes, index, "note").label);
      widths.add(styleOf(statusCell(row)).minWidth ?? "");
    });
    // One width for the status column, so every title starts at the same place.
    expect(widths.size).toBe(1);
  });

  it("treats a note answered without a done value as not done", async () => {
    const noFlag = {
      id: nextNoteId(),
      label: "No flag",
      createdAt: "2026-10-12T09:30:00Z",
    } as Note;
    const container = await viewNotes([noFlag]);
    const row = at(noteRows(container), 0, "row");
    expect(statusText(row)).toBe(OPEN_STATE);
    // No unknown state leaks into the row as text.
    expect(row.textContent).toBe(`${OPEN_STATE}No flag`);
    expect(row.textContent).not.toMatch(/undefined|null|✓/);
  });

  it("follows the stored flag when the list is shown again", async () => {
    const notes = flaggedCollection([true, false, true]);
    const first = await viewNotes(notes);
    const firstStatuses = noteRows(first).map(statusText);
    expect(firstStatuses).toEqual([DONE_STATE, OPEN_STATE, DONE_STATE]);
    // The flags are changed elsewhere while the list sits closed, then the list
    // is loaded again: the rows answer with what the store holds now.
    for (const note of notes) note.done = !note.done;
    const second = await viewNotes(notes);
    const secondStatuses = noteRows(second).map(statusText);
    expect(secondStatuses).toEqual([OPEN_STATE, DONE_STATE, OPEN_STATE]);
    expect(secondStatuses).not.toEqual(firstStatuses);
  });

  it("carries the done status on every page the list is loaded over", async () => {
    // The list is loaded a page at a time through `listNotes`, so each showing
    // is one page: the flag has to ride on the rows of every one of them.
    const pageSize = 5;
    const pages = [
      alternatingFlags(pageSize),
      alternatingFlags(pageSize).map((flag) => !flag),
      Array.from({ length: pageSize }, (_unused, index) => index % 3 !== 0),
    ];
    for (const [pageIndex, flags] of pages.entries()) {
      const container = await viewNotes(flaggedCollection(flags));
      const rows = noteRows(container);
      expect(rows).toHaveLength(pageSize);
      flags.forEach((flag, index) => {
        expect(
          statusText(at(rows, index, `row of page ${pageIndex + 1}`)),
        ).toBe(flag ? DONE_STATE : OPEN_STATE);
      });
    }
    // A page as full as the contract allows still carries the flag on its last
    // row, not just on the rows a short list would have shown.
    const full = await viewNotes(flaggedCollection(alternatingFlags(100)));
    const fullRows = noteRows(full);
    expect(fullRows).toHaveLength(100);
    expect(statusText(at(fullRows, 98, "second-to-last row"))).toBe(DONE_STATE);
    expect(statusText(at(fullRows, 99, "last row"))).toBe(OPEN_STATE);
  });

  it("renders done and open rows side by side without ambiguity", async () => {
    const container = await viewNotes(
      flaggedCollection([true, false, true, false]),
    );
    const rows = noteRows(container);
    expect(rows.map(statusText)).toEqual([
      DONE_STATE,
      OPEN_STATE,
      DONE_STATE,
      OPEN_STATE,
    ]);
    // The rows read as done are exactly the notes whose flag is set.
    expect(
      rows.filter((row) => statusText(row) === DONE_STATE).map(noteTitleText),
    ).toEqual(["Note 1", "Note 3"]);
    expect(
      rows.filter((row) => statusText(row) === OPEN_STATE).map(noteTitleText),
    ).toEqual(["Note 2", "Note 4"]);
    for (const row of rows) {
      const state = statusText(row);
      if (state !== DONE_STATE && state !== OPEN_STATE) {
        throw new Error(`a row reads as neither done nor open: "${state}"`);
      }
    }
  });

  it("leaves every note's done flag untouched while the list is viewed", async () => {
    const flags = alternatingFlags(6);
    const notes = flaggedCollection(flags);
    const stored = (): boolean[] => notes.map((note) => note.done);
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    expect(stored()).toEqual(flags);
    expect(world.collection.map((note) => note.done)).toEqual(flags);
    for (const row of rows) {
      // The row offers no control that could write the flag back.
      const controls = documentOrder(row).filter(
        (node) =>
          node instanceof FElement &&
          ["BUTTON", "INPUT", "A", "SELECT", "TEXTAREA"].includes(node.tagName),
      );
      expect(controls).toHaveLength(0);
      expect(statusCell(row).hasAttribute("aria-checked")).toBe(false);
    }
    // Operating the row where the status is read changes nothing either.
    await flush(() => {
      fire(statusCell(at(rows, 0, "row")), "click");
    });
    expect(stored()).toEqual(flags);
    expect(world.collection.map((note) => note.done)).toEqual(flags);
    expect(statusText(at(rows, 0, "row"))).toBe(DONE_STATE);
  });

  it("does not displace or truncate the row's existing content", async () => {
    const long =
      "A note label long enough that it cannot fit a phone screen on one line".slice(
        0,
        80,
      );
    const notes = flaggedCollection([true, false]).map((note, index) =>
      index === 0 ? { ...note, label: long } : note,
    );
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    // The title is still there in full, beside the status it now shares a row
    // with: the indicator is added to the row, never traded for the title.
    expect(noteTitleText(at(rows, 0, "row"))).toBe(long);
    expect(at(rows, 0, "row").textContent).toContain(long);
    expect(noteTitleText(at(rows, 1, "row"))).toBe("Note 2");
    for (const row of rows) {
      for (const cell of [statusCell(row), titleCell(row)]) {
        const style = styleOf(cell);
        expect(style.overflow).not.toBe("hidden");
        expect(style.textOverflow).not.toBe("ellipsis");
        expect(style.maxWidth).not.toBe("0px");
      }
      expect(styleOf(row).overflow).not.toBe("hidden");
    }
  });

  it(`holds the row together at the narrowest supported viewport (${String(NARROWEST_VIEWPORT_PX)}px)`, async () => {
    const notes = flaggedCollection(alternatingFlags(4));
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    for (const row of rows) {
      // At one mobile-class width the row gives up a line rather than clipping
      // anything: it wraps, while the status keeps its shape and the title
      // takes the width that is left and wraps inside it.
      expect(styleOf(row).flexWrap).toBe("wrap");
      const status = styleOf(statusCell(row));
      expect(status.flexShrink).toBe("0");
      expect(status.whiteSpace).toBe("nowrap");
      expect(status.minWidth).toBe("5.5em");
      const title = styleOf(titleCell(row));
      expect(title.minWidth).toBe("0");
      expect(title.overflowWrap).toBe("anywhere");
      expect(title.whiteSpace).not.toBe("nowrap");
    }
    // Nothing is lost from a row at that width: both surfaces read whole.
    rows.forEach((row, index) => {
      expect(statusText(row)).toBe(
        at(notes, index, "note").done ? DONE_STATE : OPEN_STATE,
      );
      expect(noteTitleText(row)).toBe(at(notes, index, "note").label);
    });
  });

  it("covers each note the contract answers the listing with", async () => {
    const members = eachNoteMembers();
    if (members.length === 0) {
      throw new Error(
        "the each-note set is empty: contract/openapi.yaml answers the listing without a note object",
      );
    }
    expect(members.length).toBeGreaterThan(0);
    for (const member of members) {
      // The member answers every listed note with a completion flag the row can
      // read: a required boolean, never an optional or blank state.
      const noteObject = noteObjectBehind(member);
      expect(noteObject.properties).toContain("done");
      expect(noteObject.types["done"]).toBe("boolean");
      expect(noteObject.required).toContain("done");
      // And the row presents that flag for every note of a list shaped like
      // this member, in both states the flag can be in.
      const flags = [true, false, true];
      const container = await viewNotes(flaggedCollection(flags));
      const rows = noteRows(container);
      expect(rows).toHaveLength(flags.length);
      flags.forEach((flag, index) => {
        expect(statusText(at(rows, index, `row of ${member.name}`))).toBe(
          flag ? DONE_STATE : OPEN_STATE,
        );
      });
    }
    // The rest of the set lives outside the contract: every web file that
    // shapes, fetches or renders a listed note is a member of "each note" too.
    // The sweep finds them on disk as they really are, not from a list kept in
    // this test, and refuses any member that never carries the flag.
    const webRoot = new URL("./", import.meta.url);
    const swept = readdirSync(webRoot, { withFileTypes: true })
      .filter((entry) => entry.isFile() && /\.tsx?$/.test(entry.name))
      .map((entry) => ({
        name: entry.name,
        text: readFileSync(new URL(entry.name, webRoot), "utf8"),
      }))
      .filter((file) => shapesAListedNote(file.text));
    if (swept.length === 0) {
      throw new Error(
        "the sweep found no member of the each-note set on the web read path",
      );
    }
    for (const member of swept) {
      if (!member.text.includes("done")) {
        throw new Error(
          `${member.name} shapes a listed note but never carries the done flag`,
        );
      }
    }
  });
});
