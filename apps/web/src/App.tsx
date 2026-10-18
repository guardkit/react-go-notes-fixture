import {
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type FormEvent,
} from "react";
import type { User, UserManager } from "oidc-client-ts";
import {
  createNote,
  listNotes,
  SignInRequiredError,
  updateNoteDone,
  type Note,
} from "./api";
import type { RuntimeConfig } from "./config";

type Props = {
  config: RuntimeConfig;
  manager: UserManager;
  initialUser: User | null;
  initialError: string;
};

// The row treatment a done note gets and a not-done note never does. The word
// below is the cue that carries the state on its own - a class and a style only
// set the row apart for the eye - so the two states stay distinguishable to a
// screen reader and without colour. A note whose flag is missing or null is
// answered as not-done, never as a third state, and a done note keeps the
// position the list gave it.
const doneRowStyle: CSSProperties = {
  color: "#5f6368",
  fontStyle: "italic",
  opacity: 0.72,
};
const doneTitleStyle: CSSProperties = { textDecorationLine: "line-through" };
const doneFlagStyle: CSSProperties = {
  color: "#1a7f37",
  fontWeight: 700,
  marginLeft: "0.5em",
};

// The tick the person clicks: a box that reads as a control whether or not the
// note is done, filled and marked when it is. The mark is drawn rather than
// written, so the only words a row carries stay the note's title and the "Done"
// cue that already answers the state without colour.
const doneToggleStyle: CSSProperties = {
  marginRight: "0.5em",
  width: "1.2em",
  height: "1.2em",
  padding: 0,
  borderWidth: "1px",
  borderStyle: "solid",
  borderColor: "#5f6368",
  borderRadius: "0.15em",
  background: "#ffffff",
  color: "#ffffff",
  cursor: "pointer",
  lineHeight: 1,
};
const doneToggleOnStyle: CSSProperties = {
  ...doneToggleStyle,
  borderColor: "#1a7f37",
  background: "#1a7f37",
};
const doneTickStyle: CSSProperties = { display: "block", margin: "0 auto" };

function isDone(note: Note): boolean {
  return note.done === true;
}

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

  // The click is answered at once - the row shows the state the person asked
  // for while the PATCH is still out - and then answered again by the service,
  // which has the last word: the note that comes back replaces the row, and a
  // PATCH that fails or a note that has been clicked again meanwhile leaves the
  // row as the last answer says it is, not as the click hoped.
  const attempts = useRef<Map<string, number>>(new Map());

  async function toggleDone(note: Note) {
    const id = note.id;
    const before = isDone(note);
    const wanted = !before;
    const attempt = (attempts.current.get(id) ?? 0) + 1;
    attempts.current.set(id, attempt);
    const mine = () => attempts.current.get(id) === attempt;
    setError("");
    setNotes((existing) =>
      existing.map((candidate) =>
        candidate.id === id ? { ...candidate, done: wanted } : candidate,
      ),
    );
    try {
      const saved = await updateNoteDone(id, wanted, token);
      if (mine()) {
        setNotes((existing) =>
          existing.map((candidate) =>
            candidate.id === id ? saved : candidate,
          ),
        );
      }
    } catch (failure) {
      if (mine()) {
        setNotes((existing) =>
          existing.map((candidate) =>
            candidate.id === id ? { ...candidate, done: before } : candidate,
          ),
        );
      }
      if (failure instanceof SignInRequiredError) setUser(null);
      setError("The completion state could not be saved. Please try again.");
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
              <li
                key={note.id}
                className={isDone(note) ? "note-row note-row-done" : "note-row"}
                style={isDone(note) ? doneRowStyle : undefined}
              >
                <button
                  type="button"
                  role="checkbox"
                  aria-checked={isDone(note)}
                  aria-label={
                    isDone(note)
                      ? `Mark "${note.label}" as not done`
                      : `Mark "${note.label}" as done`
                  }
                  data-testid="done-toggle"
                  className={
                    isDone(note) ? "done-toggle done-toggle-on" : "done-toggle"
                  }
                  style={isDone(note) ? doneToggleOnStyle : doneToggleStyle}
                  onClick={() => {
                    void toggleDone(note);
                  }}
                >
                  {isDone(note) ? (
                    <svg
                      aria-hidden="true"
                      focusable="false"
                      viewBox="0 0 12 12"
                      width="12"
                      height="12"
                      style={doneTickStyle}
                    >
                      <path
                        d="M2.2 6.4 4.8 9 9.8 3.2"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="1.8"
                      />
                    </svg>
                  ) : null}
                </button>
                <span
                  className="note-row-title"
                  style={isDone(note) ? doneTitleStyle : undefined}
                >
                  {note.label}
                </span>
                {isDone(note) ? (
                  <span className="note-row-done-flag" style={doneFlagStyle}>
                    Done
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
        </section>
      )}
    </main>
  );
}
