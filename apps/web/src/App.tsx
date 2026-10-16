import { useEffect, useState, type FormEvent } from "react";
import type { User, UserManager } from "oidc-client-ts";
import { createNote, listNotes, SignInRequiredError, type Note } from "./api";
import type { RuntimeConfig } from "./config";

type Props = {
  config: RuntimeConfig;
  manager: UserManager;
  initialUser: User | null;
  initialError: string;
};

export function App({ config, manager, initialUser, initialError }: Props) {
  const [user, setUser] = useState<User | null>(initialUser);
  const [notes, setNotes] = useState<Note[]>([]);
  const [label, setLabel] = useState("");
  const [error, setError] = useState(initialError);
  const [busy, setBusy] = useState(false);
  const token = user?.access_token ?? "";

  useEffect(() => {
    if (!token) return;
    let current = true;
    listNotes(token).then(
      (list) => {
        if (current) setNotes(list);
      },
      (failure: unknown) => {
        if (!current) return;
        if (failure instanceof SignInRequiredError) setUser(null);
        setError("Your notes could not be loaded.");
      },
    );
    return () => {
      current = false;
    };
  }, [token]);

  // The tab title carries the size of the very collection the list below
  // renders, so it is re-derived in the same commit as the list: an add or a
  // removal moves the title and the rows together, and the title can never be
  // left counting notes the person can no longer see.
  useEffect(() => {
    document.title = `Notes (${notes.length})`;
  }, [notes]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setBusy(true);
    try {
      const saved = await createNote(label, token);
      setNotes((existing) => [saved, ...existing]);
      setLabel("");
    } catch (failure) {
      if (failure instanceof SignInRequiredError) setUser(null);
      setError("The note could not be saved. Please try again.");
    } finally {
      setBusy(false);
    }
  }

  function signIn() {
    setError("");
    manager.signinRedirect().catch(() => {
      setError("Sign-in is unavailable. Please try again.");
    });
  }

  function signOut() {
    manager.signoutRedirect().catch(() => {
      setError("Sign-out is unavailable. Please try again.");
    });
  }

  return (
    <main>
      <h1>Notes</h1>
      {error && <p role="alert">{error}</p>}
      {!user ? (
        <section>
          <p data-testid="signin-hint">{config.signInHint}</p>
          <button type="button" onClick={signIn}>
            Sign in
          </button>
        </section>
      ) : (
        <section>
          <p>
            Signed in as{" "}
            <strong data-testid="signed-in-user">
              {user.profile.preferred_username ?? user.profile.sub}
            </strong>
          </p>
          <button type="button" onClick={signOut}>
            Sign out
          </button>
          <form
            onSubmit={(event) => {
              void submit(event);
            }}
          >
            <label htmlFor="label">New note</label>
            <input
              id="label"
              value={label}
              maxLength={80}
              onChange={(event) => {
                setLabel(event.target.value);
              }}
            />
            <button type="submit" disabled={busy || !label.trim()}>
              {busy ? "Saving…" : "Save"}
            </button>
          </form>
          <p data-testid="notes-count">{notes.length} notes</p>
          <ul aria-label="Your notes">
            {notes.map((note) => (
              <li key={note.id}>{note.label}</li>
            ))}
          </ul>
        </section>
      )}
    </main>
  );
}
