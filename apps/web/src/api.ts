import type { components } from "./api-types";

// Types come from contract/openapi.yaml through the generated api-types.ts;
// the parsers below still check every response at runtime.
export type Note = components["schemas"]["Note"];
type NewNote = components["schemas"]["NewNote"];
type PatchNoteRequest = components["schemas"]["PatchNoteRequest"];

type Fetcher = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export class SignInRequiredError extends Error {
  constructor() {
    super("Sign-in required");
  }
}

// The one answer a single-note write gives for a note the caller may not touch,
// whether that note was never written or belongs to someone else. The name is
// the caller's to branch on; the message carries neither the id nor an owner,
// because the service answers both cases identically and the client must not
// turn that single answer back into a reading of whether the note exists.
export class NoteNotFoundError extends Error {
  constructor() {
    super("Note not found");
  }
}

export function parseNote(value: unknown): Note {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error("Invalid API response");
  }
  if (
    !("id" in value) ||
    typeof value.id !== "string" ||
    !/^[0-9]+$/.test(value.id) ||
    !("label" in value) ||
    typeof value.label !== "string" ||
    !("createdAt" in value) ||
    typeof value.createdAt !== "string" ||
    Number.isNaN(Date.parse(value.createdAt)) ||
    // The contract declares done a required boolean of the note object, so a
    // note answered without a readable flag is as malformed as one answered
    // without a label.
    !("done" in value) ||
    typeof value.done !== "boolean"
  ) {
    throw new Error("Invalid API response");
  }
  return {
    id: value.id,
    label: value.label,
    createdAt: value.createdAt,
    done: value.done,
  };
}

export function parseNoteList(value: unknown): Note[] {
  if (
    typeof value !== "object" ||
    value === null ||
    !("notes" in value) ||
    !Array.isArray(value.notes)
  ) {
    throw new Error("Invalid API response");
  }
  return value.notes.map(parseNote);
}

type CallOptions = {
  // Only the caller that aims at one note has a 404 to name, so the helper
  // stays silent about that status for every other path: the list and create
  // routes never answer it, and their error mapping is left exactly as it was.
  notFound?: () => Error;
};

async function call(
  path: string,
  token: string,
  init: RequestInit,
  fetcher: Fetcher,
  options: CallOptions = {},
): Promise<unknown> {
  const headers = new Headers(init.headers);
  headers.set("authorization", `Bearer ${token}`);
  const response = await fetcher(path, { ...init, headers });
  if (response.status === 401) throw new SignInRequiredError();
  // The body of the 404 is never read: whatever it carries, the client has no
  // more to say about the note than "not yours to change".
  if (response.status === 404 && options.notFound) throw options.notFound();
  if (!response.ok) throw new Error("The notes service is unavailable");
  return (await response.json()) as unknown;
}

export async function listNotes(
  token: string,
  fetcher: Fetcher = fetch,
): Promise<Note[]> {
  return parseNoteList(await call("/api/notes", token, {}, fetcher));
}

export async function createNote(
  label: string,
  token: string,
  fetcher: Fetcher = fetch,
): Promise<Note> {
  const body: NewNote = { label };
  return parseNote(
    await call(
      "/api/notes",
      token,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      },
      fetcher,
    ),
  );
}

// The whole of what the browser may change on one note: the completion flag.
// The id goes in the path - escaped, so it cannot reach past /api/notes/ - the
// flag is the only member of the body, and the note that comes back is the
// whole note as storage now holds it, so the caller refreshes its view from the
// answer rather than from what it asked for.
export async function updateNoteDone(
  id: string,
  done: boolean,
  token: string,
  fetcher: Fetcher = fetch,
): Promise<Note> {
  const body: PatchNoteRequest = { done };
  return parseNote(
    await call(
      `/api/notes/${encodeURIComponent(id)}`,
      token,
      {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      },
      fetcher,
      { notFound: () => new NoteNotFoundError() },
    ),
  );
}
