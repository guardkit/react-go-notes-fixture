import { describe, expect, it } from "vitest";
import {
  NoteGoneError,
  SignInRequiredError,
  updateNoteDone,
  type Note,
} from "./api";

// The note as the PATCH route answers it: the flag the owner just set, and
// every other field exactly as it was stored.
const stored: Note = {
  id: "9007199254740993",
  label: "hello",
  createdAt: "2026-10-12T09:30:00Z",
  done: false,
};

/** The same note with its flag moved, as the service would answer. */
function flagged(done: boolean): Note {
  return { ...stored, done };
}

describe("PATCH /api/notes/{id} boundary", () => {
  it("patches the done flag with the bearer token and returns the updated note", async () => {
    let requests = 0;
    const updated = await updateNoteDone(
      stored.id,
      true,
      "tok",
      (url, init) => {
        requests += 1;
        const headers = new Headers(init?.headers);
        // The same same-origin base the rest of the client uses, never a host
        // the request did not name.
        expect(url).toBe(`/api/notes/${stored.id}`);
        expect(init?.method).toBe("PATCH");
        expect(headers.get("authorization")).toBe("Bearer tok");
        expect(headers.get("content-type")).toBe("application/json");
        expect(init?.body).toBe(JSON.stringify({ done: true }));
        return Promise.resolve(Response.json(flagged(true)));
      },
    );
    // One PATCH, and the note it answered with is what the caller gets back.
    expect(requests).toBe(1);
    expect(updated).toEqual(flagged(true));
    // The id stays a string across the round trip, and the untouched fields
    // travel back unchanged.
    expect(updated.id).toBe("9007199254740993");
    expect(updated.label).toBe("hello");
    expect(updated.createdAt).toBe("2026-10-12T09:30:00Z");
  });

  it("patches the flag back to false", async () => {
    const updated = await updateNoteDone(
      stored.id,
      false,
      "tok",
      (_url, init) => {
        expect(init?.body).toBe(JSON.stringify({ done: false }));
        return Promise.resolve(Response.json(flagged(false)));
      },
    );
    expect(updated.done).toBe(false);
  });

  it("reports a note that is not the caller's as gone, not as a failure", async () => {
    await expect(
      updateNoteDone("42", true, "tok", () =>
        Promise.resolve(
          Response.json({ message: "note not found" }, { status: 404 }),
        ),
      ),
    ).rejects.toBeInstanceOf(NoteGoneError);
  });

  it("still reports a refused token as sign-in required", async () => {
    await expect(
      updateNoteDone("42", true, "expired", () =>
        Promise.resolve(
          Response.json({ message: "Sign-in required" }, { status: 401 }),
        ),
      ),
    ).rejects.toBeInstanceOf(SignInRequiredError);
  });

  it("reports a service failure apart from a note having gone away", async () => {
    await expect(
      updateNoteDone("42", true, "tok", () =>
        Promise.resolve(Response.json({ message: "nope" }, { status: 500 })),
      ),
    ).rejects.toThrow("The notes service is unavailable");
  });

  it("rejects a body that is not a note", async () => {
    await expect(
      updateNoteDone("42", true, "tok", () =>
        Promise.resolve(Response.json({ done: true })),
      ),
    ).rejects.toThrow("Invalid API response");
    await expect(
      updateNoteDone("42", true, "tok", () =>
        Promise.resolve(Response.json({ ...stored, done: "yes" })),
      ),
    ).rejects.toThrow("Invalid API response");
  });

  it("refuses to address a note by an id the contract would not accept", async () => {
    for (const id of ["", "../1", "1/2", "1 OR 1=1"]) {
      await expect(
        updateNoteDone(id, true, "tok", () => {
          throw new Error("no request is shaped with an unusable id");
        }),
      ).rejects.toThrow("Invalid note id");
    }
  });
});
