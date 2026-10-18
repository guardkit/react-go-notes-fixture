import { describe, expect, it } from "vitest";
import {
  createNote,
  listNotes,
  parseNote,
  parseNoteList,
  SignInRequiredError,
  type Note,
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
    // done is a required boolean of the contract's note object, so a note
    // answered without one — or with anything but true or false — is not a note.
    const noteWithoutDone: Record<string, unknown> = { ...note };
    delete noteWithoutDone.done;
    expect(() => parseNote(noteWithoutDone)).toThrow("Invalid API response");
    expect(() => parseNote({ ...note, done: null })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNote({ ...note, done: "yes" })).toThrow(
      "Invalid API response",
    );
    expect(() => parseNoteList([noteWithoutDone])).toThrow(
      "Invalid API response",
    );
    expect(() => parseNoteList([note])).toThrow("Invalid API response");
  });

  it("keeps the completion flag of a well-formed note", () => {
    expect(parseNote(note)).toEqual(note);
    expect(parseNote({ ...note, done: true }).done).toBe(true);
  });

  it("types the note with a completion flag that is always present", () => {
    // The note type is generated from contract/openapi.yaml, so the keys the
    // compiler demands are the contract's `required` list: an optional done
    // would drop out of this union and fail `npm run typecheck`, which is how
    // a client built from the contract reads the flag unconditionally.
    type RequiredKeys<T> = {
      [K in keyof T]-?: undefined extends T[K] ? never : K;
    }[keyof T];
    const required: RequiredKeys<Note>[] = ["id", "label", "createdAt", "done"];
    expect(new Set(required)).toEqual(
      new Set(["id", "label", "createdAt", "done"]),
    );
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
