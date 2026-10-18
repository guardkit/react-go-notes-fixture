import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { User, UserManager } from "oidc-client-ts";
import type { Note } from "./api";
import type { RuntimeConfig } from "./config";

// The surface under test is the row the notes list shows for a note: a done
// note carries a readable "Done" cue and a row treatment no not-done row
// shares, and a not-done note keeps the standard row with no such cue. Notes
// reach the view only through `listNotes`, so what a row shows is what the
// service answered for that note - the same proof the API gives at
// apps/api/internal/notes/done_test.go:58 and the browser gives at
// tests/e2e/specs/notes-done.spec.ts:15. Nothing here changes the API, the
// contract or the store: the flag already arrives on every note.
//
// The set "each note" is not named member by member in this file. The sweep in
// "every note the contract's list response can return" finds its members when
// it runs, by reading contract/openapi.yaml, so a note shape added later is
// covered without editing this check, and a sweep that finds no member fails.
const world = vi.hoisted(() => ({
  collection: [] as Note[],
  // When set, the raw body of a GET /api/notes response, answered verbatim.
  response: undefined as { notes?: unknown } | undefined,
  nextId: 0,
}));

function nextNoteId(): string {
  world.nextId += 1;
  return String(world.nextId);
}

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    listNotes: (token: string): Promise<Note[]> => {
      const body = world.response;
      if (body !== undefined) {
        // The real `listNotes`, over the very body a GET /api/notes answered,
        // so the row shows the flag the response carried - field checks and
        // all - rather than a fixture handed to the component.
        return actual.listNotes(token, () =>
          Promise.resolve(
            new Response(JSON.stringify(body), {
              status: 200,
              headers: { "content-type": "application/json" },
            }),
          ),
        );
      }
      return Promise.resolve(world.collection.map((note) => ({ ...note })));
    },
  };
});

// This workspace installs are locked (`npm ci`) and ship no DOM library, so the
// component tests carry their own minimal DOM: just enough of Node, Element and
// Document for React 19's client renderer to commit and re-render.
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
  getAttributeNames(): string[] {
    return [...this.attributes.keys()];
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

/** Mounts the notes list view once, over whatever `world` currently holds. */
async function mount(): Promise<FElement> {
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

/** Renders the notes list view over `notes`, as if a user opened the list. */
async function viewNotes(notes: Note[]): Promise<FElement> {
  world.response = undefined;
  world.collection = notes;
  return mount();
}

/** Renders the list over the raw body of a GET /api/notes response. */
async function viewResponse(body: { notes?: unknown }): Promise<FElement> {
  world.collection = [];
  world.response = body;
  return mount();
}

/** The notes list, found by the accessible name the view gives it. */
function notesList(container: FElement): FElement {
  const list = findBy(
    container,
    (element) => element.getAttribute("aria-label") === "Your notes",
  );
  if (!list) throw new Error("the notes list is missing from the view");
  return list;
}

/** The rows the list renders, in the order the list renders them. */
function noteRows(container: FElement): FElement[] {
  return documentOrder(notesList(container)).filter(
    (node) => node instanceof FElement && node.tagName === "LI",
  ) as FElement[];
}

/** Every element whose whole text is the "Done" cue. */
function doneCues(scope: FElement): FElement[] {
  return documentOrder(scope).filter(
    (node) => node instanceof FElement && node.textContent.trim() === "Done",
  ) as FElement[];
}

/** What a person sees as the row's presentation: its class and its styling. */
function rowTreatment(row: FElement): string {
  const style = Object.entries(row.style)
    .filter(([, value]) => value !== "" && value !== undefined)
    .map(([key, value]) => `${key}:${value}`)
    .sort()
    .join(";");
  return `class=${row.getAttribute("class") ?? ""}|style=${style}`;
}

/** The one total shown above the list, e.g. "4 notes". */
function totalAboveTheList(container: FElement): FElement {
  const shown = documentOrder(container).filter(
    (node) => node instanceof FElement && /^\d+ notes$/.test(node.textContent),
  ) as FElement[];
  if (shown.length !== 1) {
    throw new Error(`expected one total above the list, found ${shown.length}`);
  }
  const total = shown[0];
  if (!total) throw new Error("no total is displayed above the notes list");
  const list = notesList(container);
  if (list.contains(total)) {
    throw new Error("the total is shown inside the notes list");
  }
  if (
    documentOrder(container).indexOf(total) >=
    documentOrder(container).indexOf(list)
  ) {
    throw new Error("the total is not shown above the notes list");
  }
  return total;
}

// The contract is the source of the set "each note": the `listNotes` operation
// says what the list is answered with, and the `Note` schema says what every
// one of those notes carries. These readers take the members from that file at
// run time, so the sweep below covers whatever the contract declares today and
// any note shape added to it later.
const contractSource = readFileSync(
  fileURLToPath(new URL("../../../contract/openapi.yaml", import.meta.url)),
  "utf8",
);

function indentOf(line: string): number {
  return line.length - line.trimStart().length;
}

function refName(line: string): string | undefined {
  const match = /#\/components\/schemas\/([A-Za-z0-9_]+)/.exec(line);
  return match ? match[1] : undefined;
}

/** The lines of one schema's block, `name:` at the schema indentation. */
function schemaBlock(source: string, name: string): string[] | undefined {
  const lines = source.split(/\r?\n/);
  const start = lines.findIndex((line) => line.trim() === `${name}:`);
  if (start < 0) return undefined;
  const own = indentOf(lines[start] ?? "");
  const block: string[] = [];
  for (const line of lines.slice(start + 1)) {
    if (line.trim() === "") continue;
    if (indentOf(line) <= own) break;
    block.push(line);
  }
  return block.length > 0 ? block : undefined;
}

/** The body of the `listNotes` operation: its keys, up to the next operation. */
function listNotesBlock(source: string): string[] {
  const lines = source.split(/\r?\n/);
  const start = lines.findIndex(
    (line) => line.trim() === "operationId: listNotes",
  );
  if (start < 0) return [];
  const own = indentOf(lines[start] ?? "");
  const block: string[] = [];
  for (const line of lines.slice(start + 1)) {
    if (line.trim() === "") continue;
    if (indentOf(line) < own) break;
    if (/^operationId:/.test(line.trim())) break;
    block.push(line);
  }
  return block;
}

/** Schema names a block points at with `$ref`. */
function refsIn(block: string[]): string[] {
  const refs: string[] = [];
  for (const line of block) {
    const name = refName(line);
    if (name) refs.push(name);
  }
  return refs;
}

/** Schema names a block names as the members of one of its arrays. */
function itemRefsIn(block: string[]): string[] {
  const refs: string[] = [];
  let itemsIndent = -1;
  for (const line of block) {
    const indent = indentOf(line);
    if (itemsIndent >= 0) {
      if (indent > itemsIndent) {
        const name = refName(line);
        if (name) refs.push(name);
        continue;
      }
      itemsIndent = -1;
    }
    if (line.trim() === "items:") itemsIndent = indent;
  }
  return refs;
}

/** The `required: [..]` / `- x` list a schema block declares. */
function requiredFields(block: string[]): string[] {
  const index = block.findIndex((line) => line.trim().startsWith("required:"));
  if (index < 0) return [];
  const inline = /required:\s*\[([^\]]*)\]/.exec(block[index] ?? "");
  if (inline?.[1]) {
    return inline[1]
      .split(",")
      .map((entry) => entry.trim())
      .filter((entry) => entry !== "");
  }
  const own = indentOf(block[index] ?? "");
  const fields: string[] = [];
  for (const line of block.slice(index + 1)) {
    if (line.trim() === "") continue;
    if (indentOf(line) <= own) break;
    const entry = /^-\s*([A-Za-z0-9_]+)\s*$/.exec(line.trim());
    if (entry?.[1]) fields.push(entry[1]);
  }
  return fields;
}

/** The declared `type:` of each property a schema block lists. */
function fieldTypes(block: string[]): Record<string, string> {
  const types: Record<string, string> = {};
  const start = block.findIndex((line) => line.trim() === "properties:");
  if (start < 0) return types;
  const propertiesIndent = indentOf(block[start] ?? "");
  let fieldIndent = -1;
  let field: string | undefined;
  for (const line of block.slice(start + 1)) {
    if (line.trim() === "") continue;
    const indent = indentOf(line);
    if (indent <= propertiesIndent) break;
    if (fieldIndent < 0) fieldIndent = indent;
    if (indent === fieldIndent) {
      field = /^([A-Za-z][A-Za-z0-9_]*):/.exec(line.trim())?.[1];
      continue;
    }
    if (field === undefined) continue;
    const type = /^type:\s*([A-Za-z-]+)/.exec(line.trim());
    if (type?.[1] && types[field] === undefined) types[field] = type[1];
  }
  return types;
}

/** A note shape the `listNotes` response can answer, as the contract declares it. */
type NoteMember = {
  name: string;
  required: string[];
  types: Record<string, string>;
};

/**
 * Every note shape the list can be answered with, found in the contract now:
 * the schemas the `listNotes` response holds as the members of its collection,
 * and anything they in turn hold as members.
 */
function noteMembers(source: string): NoteMember[] {
  const operation = listNotesBlock(source);
  const queue: string[] = [];
  for (const envelope of refsIn(operation)) {
    const block = schemaBlock(source, envelope);
    if (!block) continue;
    const members = itemRefsIn(block);
    queue.push(...(members.length > 0 ? members : [envelope]));
  }
  const seen = new Set<string>();
  const found: NoteMember[] = [];
  while (queue.length > 0) {
    const name = queue.shift();
    if (name === undefined || seen.has(name)) continue;
    seen.add(name);
    const block = schemaBlock(source, name);
    if (!block) continue;
    found.push({
      name,
      required: requiredFields(block),
      types: fieldTypes(block),
    });
    queue.push(...itemRefsIn(block));
  }
  return found;
}

afterEach(async () => {
  await flush(closeView);
  world.collection = [];
  world.response = undefined;
  doc.title = "";
});

describe("done flag on the rows of the notes list", () => {
  it("marks a done note with a Done label and a row treatment no not-done row has", async () => {
    const container = await viewNotes([
      noteWith("a done note", true),
      noteWith("a plain note", false),
    ]);
    const rows = noteRows(container);
    expect(rows).toHaveLength(2);
    const [done, plain] = rows as [FElement, FElement];
    if (!done || !plain) throw new Error("the two rows are missing");

    // The cue reads as a word, so it survives without colour.
    expect(doneCues(done).map((cue) => cue.textContent.trim())).toEqual([
      "Done",
    ]);
    expect(done.textContent).toContain("Done");
    // And the row itself is set apart from the standard one.
    expect(rowTreatment(done)).not.toBe(rowTreatment(plain));
    expect(done.getAttribute("class")).not.toBe(plain.getAttribute("class"));
    // The title it had before the flag was shown is the title it has now.
    expect(done.textContent).toContain("a done note");
    expect(plain.textContent).toBe("a plain note");
  });

  it("leaves a not-done note in the standard row with no Done label", async () => {
    const container = await viewNotes([
      noteWith("an open note", false),
      noteWith("another open note", false),
    ]);
    const rows = noteRows(container);
    expect(rows).toHaveLength(2);
    const row = rows[0];
    const other = rows[1];
    if (!row || !other) throw new Error("the not-done rows are missing");
    expect(doneCues(row)).toHaveLength(0);
    expect(row.textContent).toBe("an open note");
    // The standard row: no class or styling set apart for done notes, and the
    // same treatment every other not-done row gets.
    expect(rowTreatment(row)).toBe(rowTreatment(other));
    expect(row.getAttribute("class") ?? "").not.toMatch(/done/i);
    expect(row.getAttribute("style")).toBeNull();
  });

  it.each([
    ["absent", "missing"],
    ["null", "null"],
  ])(
    "treats a done flag that is %s as not-done and still shows the row",
    async (_case, shape) => {
      const note =
        shape === "missing"
          ? ({
              id: nextNoteId(),
              label: "a flagless note",
              createdAt: "2026-10-12T09:30:00Z",
            } as unknown as Note)
          : noteWith("a flagless note", null);
      const container = await viewNotes([
        note,
        noteWith("an open note", false),
      ]);
      // Nothing is hidden or omitted for want of a readable flag.
      const rows = noteRows(container);
      expect(rows).toHaveLength(2);
      const row = rows[0];
      const standard = rows[1];
      if (!row || !standard) throw new Error("a row is missing");
      expect(row.textContent).toBe("a flagless note");
      expect(doneCues(row)).toHaveLength(0);
      // It is shown exactly as any other not-done note is shown.
      expect(rowTreatment(row)).toBe(rowTreatment(standard));
    },
  );

  it("never lets the done treatment reach a not-done row", async () => {
    const notes = [
      noteWith("open one", false),
      noteWith("finished one", true),
      noteWith("open two", false),
      noteWith("finished two", true),
      noteWith("open three", false),
    ];
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    expect(rows).toHaveLength(notes.length);

    const treatments = rows.map((row) => rowTreatment(row));
    const openTreatments = notes
      .map((note, index) => ({ note, treatment: treatments[index] }))
      .filter(({ note }) => note.done !== true)
      .map(({ treatment }) => treatment);
    const doneTreatments = notes
      .map((note, index) => ({ note, treatment: treatments[index] }))
      .filter(({ note }) => note.done === true)
      .map(({ treatment }) => treatment);

    // Every not-done row is shown as every other not-done row is shown, and
    // none of them shares the done row's presentation.
    expect(new Set(openTreatments).size).toBe(1);
    expect(new Set(doneTreatments).size).toBe(1);
    expect(openTreatments[0]).not.toBe(doneTreatments[0]);
    // And no not-done row carries the cue.
    for (const [index, note] of notes.entries()) {
      const row = rows[index];
      if (!row) throw new Error(`row ${index + 1} is missing`);
      expect(
        doneCues(row).length,
        `row ${index + 1} ("${note.label}") carries the Done cue`,
      ).toBe(note.done === true ? 1 : 0);
    }
  });

  it("shows the Done cue on every done row and on no not-done row", async () => {
    const notes = [
      noteWith("a", true),
      noteWith("b", false),
      noteWith("c", true),
      noteWith("d", false),
      noteWith("e", true),
      noteWith("f", false),
      noteWith("g", true),
    ];
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    expect(rows).toHaveLength(notes.length);
    // The two states stay distinguishable without colour: the word is there,
    // or it is not, for every row the list holds.
    expect(rows.filter((row) => doneCues(row).length === 0)).toHaveLength(3);
    expect(rows.filter((row) => doneCues(row).length === 1)).toHaveLength(4);
    for (const [index, row] of rows.entries()) {
      const note = notes[index];
      if (!note || !row) throw new Error(`row ${index + 1} is missing`);
      expect(doneCues(row).length === 1).toBe(note.done === true);
    }
  });

  it("shows every row of an all not-done list in the not-done presentation", async () => {
    const notes = [
      noteWith("one", false),
      noteWith("two", false),
      noteWith("three", false),
      noteWith("four", false),
      noteWith("five", false),
    ];
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    // None hidden, none omitted, none set apart.
    expect(rows).toHaveLength(notes.length);
    expect(doneCues(notesList(container))).toHaveLength(0);
    expect(new Set(rows.map((row) => rowTreatment(row))).size).toBe(1);
    expect(rows.map((row) => row.textContent)).toEqual(
      notes.map((note) => note.label),
    );
    expect(totalAboveTheList(container).textContent).toBe("5 notes");
  });

  it("keeps the titles, the order and the total while showing the flag", async () => {
    const notes = [
      noteWith("Buy milk", true),
      noteWith("Write spec", false),
      noteWith("Call the plumber", true),
      noteWith("Ship the release", false),
    ];
    const container = await viewNotes(notes);
    const rows = noteRows(container);
    // Same rows, same order, same titles as before the flag was shown.
    expect(rows).toHaveLength(notes.length);
    expect(rows.map((row) => row.textContent.replace(/Done$/, ""))).toEqual(
      notes.map((note) => note.label),
    );
    // The total above the list still counts the whole collection.
    expect(totalAboveTheList(container).textContent).toBe("4 notes");
    expect(doc.title).toBe("Notes (4)");
  });
});

describe("the done flag the notes service answers for each note", () => {
  it("shows the flag the GET /api/notes response carries, on every row", async () => {
    const answered = [
      {
        id: "1",
        label: "answered done",
        createdAt: "2026-10-12T09:30:00Z",
        done: true,
      },
      {
        id: "2",
        label: "answered not-done",
        createdAt: "2026-10-12T09:31:00Z",
        done: false,
      },
      {
        id: "3",
        label: "answered done too",
        createdAt: "2026-10-12T09:32:00Z",
        done: true,
      },
    ];
    const container = await viewResponse({ notes: answered });
    // The response the browser is answered with carries the flag on each note,
    // which is the proof apps/api/internal/notes/done_test.go:58 holds.
    for (const note of answered) {
      expect(
        Object.prototype.hasOwnProperty.call(note, "done"),
        `note ${note.id} carries a done field at all`,
      ).toBe(true);
      expect(typeof note.done, `note ${note.id} answers a boolean`).toBe(
        "boolean",
      );
    }
    const rows = noteRows(container);
    expect(rows).toHaveLength(answered.length);
    for (const [index, note] of answered.entries()) {
      const row = rows[index];
      if (!row) throw new Error(`row ${index + 1} is missing`);
      expect(row.textContent).toContain(note.label);
      expect(
        doneCues(row).length,
        `row ${index + 1} ("${note.label}") shows the flag the response answered`,
      ).toBe(note.done ? 1 : 0);
    }
    expect(totalAboveTheList(container).textContent).toBe("3 notes");
  });
});

describe("each note the contract says the list returns", () => {
  it("covers every note the listNotes response can return, found in the contract", async () => {
    const members = noteMembers(contractSource);
    // The sweep is only a sweep if it found anything to sweep.
    expect(
      members.length,
      "contract/openapi.yaml declares no note for the listNotes response",
    ).toBeGreaterThan(0);

    for (const member of members) {
      // Every member of the list carries the flag the list shows.
      expect(
        member.required,
        `${member.name} does not require the done flag the list renders`,
      ).toContain("done");
      expect(
        member.types["done"],
        `${member.name}.done is not declared a boolean`,
      ).toBe("boolean");

      // Both states of each member, shown in the list the person sees.
      const notes = [
        noteWith(`${member.name} done`, true),
        noteWith(`${member.name} not-done`, false),
      ];
      const container = await viewNotes(notes);
      const rows = noteRows(container);
      expect(rows).toHaveLength(notes.length);
      const [doneRow, openRow] = rows as [FElement, FElement];
      if (!doneRow || !openRow) throw new Error(`${member.name} rows missing`);
      expect(doneCues(doneRow).length).toBe(1);
      expect(doneCues(openRow)).toHaveLength(0);
      expect(rowTreatment(doneRow)).not.toBe(rowTreatment(openRow));
      expect(totalAboveTheList(container).textContent).toBe("2 notes");
    }
  });
});
