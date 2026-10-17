import type { components } from "./api-types";

// Types come from contract/openapi.yaml through the generated api-types.ts;
// the parsers below still check every response at runtime.
export type Note = components["schemas"]["Note"];
type NewNote = components["schemas"]["NewNote"];

type Fetcher = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export class SignInRequiredError extends Error {
  constructor() {
    super("Sign-in required");
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
