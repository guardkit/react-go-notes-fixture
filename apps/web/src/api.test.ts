import { describe, expect, it } from "vitest";
import {
  createNote,
  listNotes,
  parseNote,
  parseNoteList,
  SignInRequiredError,
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
