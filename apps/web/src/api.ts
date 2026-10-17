import type { components, operations } from "./api-types";

// Types come from contract/openapi.yaml through the generated api-types.ts;
// the parsers below still check every response at runtime.
export type Note = components["schemas"]["Note"];
type NewNote = components["schemas"]["NewNote"];

// The PATCH /api/notes/{id} operation's own request and response shapes, read
// off the generated contract types so the client cannot drift from the
// contract without failing to typecheck.
type UpdateNoteDone = operations["updateNoteDone"];
type NoteId = UpdateNoteDone["parameters"]["path"]["id"];
type DoneUpdate = UpdateNoteDone["requestBody"]["content"]["application/json"];
type UpdatedNote =
  UpdateNoteDone["responses"][200]["content"]["application/json"];

type Fetcher = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export class SignInRequiredError extends Error {
  constructor() {
    super("Sign-in required");
  }
}

// The single-note route answers 404 for a note that was never written and for
// one owned by someone else alike, so the only thing the client can say is
// that the note the person was looking at is no longer theirs to see.
export class NoteGoneError extends Error {
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
    // The contract declares done a required boolean, so a note answered
    // without a readable flag is as malformed as one without a label.
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

async function call(
  path: string,
  token: string,
  init: RequestInit,
  fetcher: Fetcher,
): Promise<unknown> {
  const headers = new Headers(init.headers);
  headers.set("authorization", `Bearer ${token}`);
  const response = await fetcher(path, { ...init, headers });
  if (response.status === 401) throw new SignInRequiredError();
  // Only the single-note route answers 404 in the contract, and it means the
  // note is not the caller's to touch: said apart from a service failure so
  // the view never leaves the flag it last showed standing.
  if (response.status === 404) throw new NoteGoneError();
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

export async function updateNoteDone(
  id: NoteId,
  done: boolean,
  token: string,
  fetcher: Fetcher = fetch,
): Promise<UpdatedNote> {
  // The id goes into the path, so it is held to the contract's own pattern
  // before a request is ever shaped with it.
  if (!/^[0-9]+$/.test(id)) throw new Error("Invalid note id");
  const body: DoneUpdate = { done };
  return parseNote(
    await call(
      `/api/notes/${id}`,
      token,
      {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      },
      fetcher,
    ),
  );
}
