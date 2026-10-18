import { describe, expect, it } from "vitest";
import {
  createNote,
  listNotes,
  NoteNotFoundError,
  parseNote,
  parseNoteList,
  SignInRequiredError,
  updateNoteDone,
} from "./api";

const note = {
  id: "9007199254740993",
  label: "hello",
  createdAt: "2026-10-12T09:30:00Z",
  done: false,
};

describe("API boundary", () => {
  it("rejects malformed network data", () => {
    expect(() => parseNote({ ...note, id: 12 })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNote({ ...note, label: null })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNote({ ...note, createdAt: "yesterday" })).toThrow(
      "Invalid API response",
    );
    // The contract declares done a required boolean of the note object, so the
    // runtime parser refuses a note answered without a readable flag.
    const { id, label, createdAt } = note;
    expect(() => parseNote({ id, label, createdAt })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNote({ ...note, done: null })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNote({ ...note, done: "yes" })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNoteList([note])).toThrow("Invalid API response");
  });

  it("posts JSON with the bearer token and keeps the string ID", async () => {
    const saved = await createNote("hello", "tok", (_url, init) => {
      const headers = new Headers(init?.headers);
      expect(init?.method).toBe("POST");
      expect(headers.get("authorization")).toBe("Bearer tok");
      expect(headers.get("content-type")).toBe("application/json");
      expect(init?.body).toBe(JSON.stringify({ label: "hello" }));
      return Promise.resolve(Response.json(note, { status: 201 }));
    });
    expect(saved.id).toBe("9007199254740993");
  });

  it("lists the signed-in owner's notes", async () => {
    const list = await listNotes("tok", (url, init) => {
      expect(url).toBe("/api/notes");
      expect(new Headers(init?.headers).get("authorization")).toBe(
        "Bearer tok",
      );
      return Promise.resolve(Response.json({ notes: [note] }));
    });
    expect(list).toEqual([note]);
  });

  // The toggle is the browser's only write to a single note, and what it shows
  // afterwards is the note the PATCH answered - not the flag the caller asked
  // for. The service answers the same note whether or not the flag moved
  // (contract/openapi.yaml, /api/notes/{id}), so both directions are checked
  // against the very URL, method, body and answer of the request.
  it("patches the done flag to true and returns the answered note", async () => {
    const updated = await updateNoteDone(
      "9007199254740993",
      true,
      "tok",
      (url, init) => {
        const headers = new Headers(init?.headers);
        expect(url).toBe("/api/notes/9007199254740993");
        expect(init?.method).toBe("PATCH");
        expect(headers.get("authorization")).toBe("Bearer tok");
        expect(headers.get("content-type")).toBe("application/json");
        // The flag is the whole body - no label, no id, nothing riding along:
        // the exact string below is the only body this call may send.
        expect(init?.body).toBe(JSON.stringify({ done: true }));
        return Promise.resolve(Response.json({ ...note, done: true }));
      },
    );
    expect(updated).toEqual({
      id: "9007199254740993",
      label: "hello",
      createdAt: "2026-10-12T09:30:00Z",
      done: true,
    });
  });

  it("patches the done flag to false and returns the answered note", async () => {
    const updated = await updateNoteDone(
      "9007199254740993",
      false,
      "tok",
      (url, init) => {
        const headers = new Headers(init?.headers);
        expect(url).toBe("/api/notes/9007199254740993");
        expect(init?.method).toBe("PATCH");
        expect(headers.get("authorization")).toBe("Bearer tok");
        expect(headers.get("content-type")).toBe("application/json");
        expect(init?.body).toBe(JSON.stringify({ done: false }));
        return Promise.resolve(Response.json({ ...note, done: false }));
      },
    );
    // The note the caller started from was done; what comes back is what the
    // service said, flag included, so the view refreshes from the answer.
    expect(updated).toEqual(note);
  });

  it("escapes the note id into the path", async () => {
    // The id is data from storage, not a path segment of its own: it cannot be
    // allowed to reach past /api/notes/.
    await updateNoteDone("1/../2", true, "tok", (url) => {
      expect(url).toBe("/api/notes/1%2F..%2F2");
      return Promise.resolve(Response.json({ ...note, done: true }));
    });
  });

  it("refuses an answered note it cannot read", async () => {
    // The flag the caller set is never taken on faith: the answer goes through
    // the same parser as every other note, so a note answered without a flag -
    // or with one that is not a boolean - is a malformed response, not a
    // toggled row the view should show.
    const withoutFlag = {
      id: note.id,
      label: note.label,
      createdAt: note.createdAt,
    };
    await expect(
      updateNoteDone("9007199254740993", true, "tok", () =>
        Promise.resolve(Response.json(withoutFlag)),
      ),
    ).rejects.toThrow("Invalid API response");
    await expect(
      updateNoteDone("9007199254740993", true, "tok", () =>
        Promise.resolve(Response.json({ ...note, done: "yes" })),
      ),
    ).rejects.toThrow("Invalid API response");
  });

  // A 404 is the caller's to branch on - not a note invented locally to keep
  // the list looking toggled. The id and the token stay out of the error, so
  // the answer cannot be read back as a probe of another user's notes.
  it("names a missing note as not found without repeating the request", async () => {
    const failure = await updateNoteDone("1337", true, "tok", () =>
      Promise.resolve(Response.json({ message: "Not found" }, { status: 404 })),
    ).catch((error: unknown) => error);
    expect(failure).toBeInstanceOf(NoteNotFoundError);
    expect((failure as Error).message).toBe("Note not found");
    expect((failure as Error).message).not.toContain("1337");
    expect((failure as Error).message).not.toContain("tok");
  });

  // The service gives one answer for both cases, so the client must not turn
  // that answer back into a probe: whatever the 404 body carries - the contract's
  // bare message, a body naming the id and the owner, or nothing readable at all
  // - the caller is handed the same NoteNotFoundError and nothing from the body.
  it("answers a foreign note exactly as a missing one", async () => {
    async function failureFor(body: string | null): Promise<unknown> {
      return updateNoteDone("1337", false, "tok", () =>
        Promise.resolve(new Response(body, { status: 404 })),
      ).catch((error: unknown) => error);
    }
    const missing = await failureFor('{"message":"Not found"}');
    const foreign = await failureFor(
      '{"message":"Not found","id":"1337","owner":"someone-else"}',
    );
    const unreadable = await failureFor(null);
    for (const failure of [missing, foreign, unreadable]) {
      expect(failure).toBeInstanceOf(NoteNotFoundError);
      expect((failure as Error).message).toBe("Note not found");
    }
  });

  it("reports a refused token on the toggle as sign-in required", async () => {
    await expect(
      updateNoteDone("9007199254740993", true, "expired", () =>
        Promise.resolve(
          Response.json({ message: "Sign-in required" }, { status: 401 }),
        ),
      ),
    ).rejects.toBeInstanceOf(SignInRequiredError);
  });

  // The 404 is named for the single-note write alone. The list and create
  // routes carry no 404 in the contract (contract/openapi.yaml gives them
  // 400/401/500 and 401/500), and no task of this feature hands them one, so
  // their error mapping - the one the toggle shares - stays untouched.
  it("keeps the list and create error mapping as it was", async () => {
    const notFound = Response.json({ message: "Not found" }, { status: 404 });
    await expect(
      listNotes("tok", () => Promise.resolve(notFound)),
    ).rejects.toThrow("The notes service is unavailable");
    await expect(
      createNote("hello", "tok", () =>
        Promise.resolve(
          Response.json({ message: "Not found" }, { status: 404 }),
        ),
      ),
    ).rejects.toThrow("The notes service is unavailable");
    await expect(
      createNote("hello", "tok", () =>
        Promise.resolve(
          Response.json({ message: "Service unavailable" }, { status: 500 }),
        ),
      ),
    ).rejects.toThrow("The notes service is unavailable");
  });

  it("reports a refused token as sign-in required", async () => {
    await expect(
      listNotes("expired", () =>
        Promise.resolve(
          Response.json({ message: "Sign-in required" }, { status: 401 }),
        ),
      ),
    ).rejects.toBeInstanceOf(SignInRequiredError);
  });
});
