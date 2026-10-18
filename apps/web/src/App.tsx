import { useEffect, useState, type CSSProperties, type FormEvent } from "react";
import type { User, UserManager } from "oidc-client-ts";
import { createNote, listNotes, SignInRequiredError, type Note } from "./api";
import type { RuntimeConfig } from "./config";

type Props = {
  config: RuntimeConfig;
  manager: UserManager;
  initialUser: User | null;
  initialError: string;
};

// The done flag arrives on the list payload itself, so the row reads it and
// never writes it back: the list is read-only with respect to the flag. A note
// answered without one reads as not done, never as an unknown state.
function isDone(note: Note): boolean {
  return note.done === true;
}

// One status cell per row, in the same leading slot on every row: the flag of
// one note can never be read where another note's title is read, and a column
// of statuses is scannable. It cannot shrink or wrap, so the wording survives
// whole at the narrowest supported viewport; the row wraps instead.
const statusCellStyle: CSSProperties = {
  minWidth: "5.5em",
  flexShrink: 0,
  whiteSpace: "nowrap",
  fontWeight: 700,
};

// The title keeps its own line box and wraps rather than being clipped, so the
// indicator sits beside it instead of displacing or truncating it.
const openTitleStyle: CSSProperties = {
  flex: "1 1 auto",
  minWidth: 0,
  overflowWrap: "anywhere",
  fontWeight: 600,
};

const doneTitleStyle: CSSProperties = {
  ...openTitleStyle,
  // Two channels beyond colour: the title is struck through and its weight is
  // muted, so a done row reads differently from an open one.
  fontWeight: 400,
  color: "#595959",
  textDecoration: "line-through",
};

const rowStyle: CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  alignItems: "baseline",
  gap: "0.5em",
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
            {notes.map((note) => {
              const done = isDone(note);
              return (
                <li key={note.id} style={rowStyle}>
                  <span style={statusCellStyle}>
                    <span aria-hidden="true">{done ? "✓" : "○"}</span>{" "}
                    {done ? "Done" : "Not done"}
                  </span>
                  <span style={done ? doneTitleStyle : openTitleStyle}>
                    {note.label}
                  </span>
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </main>
  );
}
