package notes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The delivered write surface for the done flag is PATCH /api/notes/{id}: the
// signed-in owner hands over the flag and nothing else, and the note that comes
// back is the row as storage now holds it. These tests sit at that handler
// boundary and cover every scenario of the feature file - both directions of the
// flag, the label that must survive untouched, the one indistinguishable 404, the
// idempotent re-application, the flag the later read answers - plus the auth
// challenge the route inherits and the single owner-scoped call it may make.

// patchNote asks the delivered write surface to set the flag of one note and
// reports the recorded response.
func patchNote(store Store, id, body string) *httptest.ResponseRecorder {
	return request(Routes(store, fakeAuth{}, "run-1"), http.MethodPatch, "/api/notes/"+id, body, goodToken)
}

// decodeWireNote reads one note response back as a raw JSON object, so a test can
// tell a field that is absent from one that is present and null - neither of which
// a Go struct can report.
func decodeWireNote(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var note map[string]any
	if err := json.Unmarshal(body, &note); err != nil {
		t.Fatalf("decode note response %q: %v", body, err)
	}
	return note
}

// The route is registered and it is a signed-in one: without a token, or with a
// token the authenticator refuses, the answer is the existing 401 challenge and
// storage is never reached - not even to read the body, which is why an
// undecodable body still answers 401 rather than 400.
func TestPatchRouteIsRegisteredAndRequiresTheBearerToken(t *testing.T) {
	for _, tc := range []struct {
		name, header, body, challenge string
	}{
		{"no token at all", "", `{"done":true}`, `Bearer realm="notes"`},
		{"a refused token", "Bearer expired", `{"done":true}`, `Bearer realm="notes", error="invalid_token"`},
		{"no token and an undecodable body", "", `{"done":"yes"}`, `Bearer realm="notes"`},
		{"an empty bearer value", "Bearer ", `{"done":true}`, `Bearer realm="notes"`},
		{"another auth scheme", "Basic " + goodToken, `{"done":true}`, `Bearer realm="notes"`},
	} {
		store := &fakeStore{}
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/note-1", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		w := httptest.NewRecorder()
		Routes(store, fakeAuth{}, "run-1").ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || w.Body.String() != "{\"message\":\"Sign-in required\"}\n" {
			t.Fatalf("%s: got %d %q, want 401 the sign-in message", tc.name, w.Code, w.Body.String())
		}
		if got := w.Header().Get("WWW-Authenticate"); got != tc.challenge {
			t.Fatalf("%s: challenge %q, want %q", tc.name, got, tc.challenge)
		}
		if store.calls != 0 {
			t.Fatalf("%s: an unsigned request reached storage %d times", tc.name, store.calls)
		}
	}

	// The route really is registered: the same call with a good token is answered
	// by the handler (200), not by the mux (404 or 405).
	store := &fakeStore{update: Note{ID: "note-1", Label: "Buy milk", CreatedAt: created, Done: true}}
	if got := patchNote(store, "note-1", `{"done":true}`); got.Code != http.StatusOK {
		t.Fatalf("signed-in patch: got %d %q, want 200 from the handler", got.Code, got.Body.String())
	}
}

// Both @smoke states of the toggle: the flag the caller asks for is the flag the
// note answers, in each direction, and the body is the whole updated note - id,
// label, createdAt and done - with nothing missing beside the flag.
func TestPatchOwnerSetsTheDoneFlagInBothDirections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stored bool
		set    bool
	}{
		{"a note that is not done is marked done", false, true},
		{"a note that is done is marked not done", true, false},
	} {
		store := &fakeStore{update: Note{ID: "note-1", Label: "Buy milk", CreatedAt: created, Done: tc.set}}
		got := patchNote(store, "note-1", `{"done":`+strconv.FormatBool(tc.set)+`}`)
		want := `{"id":"note-1","label":"Buy milk","createdAt":"2026-10-12T09:30:00Z","done":` +
			strconv.FormatBool(tc.set) + "}\n"
		if got.Code != http.StatusOK || got.Body.String() != want {
			t.Fatalf("%s: got %d %s, want 200 %s", tc.name, got.Code, got.Body.String(), want)
		}

		// The full note, as the contract declares it: every one of the four fields
		// present, the flag a boolean and never a null.
		note := decodeWireNote(t, got.Body.Bytes())
		fields := []string{}
		for field := range note {
			fields = append(fields, field)
		}
		slices.Sort(fields)
		if !slices.Equal(fields, []string{"createdAt", "done", "id", "label"}) {
			t.Fatalf("%s: note answered fields %v, want id, label, createdAt and done", tc.name, fields)
		}
		if note["id"] != "note-1" || note["label"] != "Buy milk" {
			t.Fatalf("%s: note answered %v, want the note's own id and label", tc.name, note)
		}
		flag, isBool := note["done"].(bool)
		if !isBool || flag != tc.set {
			t.Fatalf("%s: note answered done %v, want the boolean %v", tc.name, note["done"], tc.set)
		}

		// The update reached storage once, scoped to the signed-in owner, for the
		// id in the path, carrying the flag the body asked for.
		if store.calls != 1 || store.owner != "user-1" || store.id != "note-1" || store.done != tc.set {
			t.Fatalf("%s: storage reached %d times as owner %q for id %q with flag %v",
				tc.name, store.calls, store.owner, store.id, store.done)
		}
	}
}

// Only the flag is mutated: the label the note already had comes back untouched
// even though the caller never named it, and a caller that tries to hand one over
// is refused before storage is reached - the write path of this endpoint has no
// label in it at all.
func TestPatchMutatesOnlyTheDoneFlag(t *testing.T) {
	store := &fakeStore{update: Note{ID: "note-1", Label: "Buy milk", CreatedAt: created, Done: true}}
	got := patchNote(store, "note-1", `{"done":true}`)
	if got.Code != http.StatusOK {
		t.Fatalf("patch: got %d %s", got.Code, got.Body.String())
	}
	if note := decodeWireNote(t, got.Body.Bytes()); note["label"] != "Buy milk" {
		t.Fatalf("the updated note answered label %v, want the note's own %q", note["label"], "Buy milk")
	}
	if store.label != "" {
		t.Fatalf("the done update handed storage the label %q, so this path writes a label", store.label)
	}

	// A label on this path is not ignored, it is refused: the body carries the
	// flag or it is not accepted, and nothing reaches storage either way.
	for _, body := range []string{
		`{"done":true,"label":"Buy milk"}`,
		`{"label":"Buy milk"}`,
		`{"done":true,"createdAt":"2026-10-12T09:30:00Z"}`,
		`{"done":true,"id":"note-2"}`,
	} {
		store := &fakeStore{}
		refused := patchNote(store, "note-1", body)
		if refused.Code != http.StatusBadRequest || refused.Body.String() != "{\"message\":\"Invalid request\"}\n" {
			t.Fatalf("body %s: got %d %q, want 400 the invalid-request message", body, refused.Code, refused.Body.String())
		}
		if store.calls != 0 || store.label != "" {
			t.Fatalf("body %s: storage reached %d times with label %q", body, store.calls, store.label)
		}
	}
}

// One absent note, one answer: the 404 comes from the store's single not-found
// signal, so the note that was never written and the note belonging to someone
// else cannot diverge - the two responses are byte-for-byte the same, and neither
// discloses which case it was.
func TestPatchAnswersOneNotFoundForAMissingNoteAndAForeignOne(t *testing.T) {
	missing := &fakeStore{err: ErrNoteNotFound}
	missingGot := patchNote(missing, "missing-1", `{"done":true}`)
	foreign := &fakeStore{err: ErrNoteNotFound}
	foreignGot := patchNote(foreign, "foreign-1", `{"done":true}`)

	for _, tc := range []struct {
		name string
		got  *httptest.ResponseRecorder
	}{
		{"a note that does not exist", missingGot},
		{"a note of another user", foreignGot},
	} {
		if tc.got.Code != http.StatusNotFound {
			t.Fatalf("%s: got %d %q, want 404", tc.name, tc.got.Code, tc.got.Body.String())
		}
		body := tc.got.Body.String()
		// The answer says nothing but "not found": no id, no owner, no reason that
		// could be read as a probe of another user's notes.
		for _, leak := range []string{"missing-1", "foreign-1", "note-1", "user-1", "Buy milk", "owner", "exist"} {
			if strings.Contains(body, leak) {
				t.Fatalf("%s: the 404 body %q discloses %q", tc.name, body, leak)
			}
		}
	}
	if missingGot.Body.String() != foreignGot.Body.String() {
		t.Fatalf("the missing note answered %q and the foreign note %q, want one indistinguishable answer",
			missingGot.Body.String(), foreignGot.Body.String())
	}
	if missingGot.Header().Get("Content-Type") != foreignGot.Header().Get("Content-Type") {
		t.Fatal("the two 404s differ in their content type, so they are distinguishable on the wire")
	}

	// Both were the store's answer, not the handler's own look-up: one call each,
	// the owner-scoped update, for the id in the path.
	for _, tc := range []struct {
		name  string
		store *fakeStore
		id    string
	}{
		{"missing", missing, "missing-1"},
		{"foreign", foreign, "foreign-1"},
	} {
		if tc.store.calls != 1 || tc.store.owner != "user-1" || tc.store.id != tc.id || !tc.store.done {
			t.Fatalf("%s note: storage reached %d times as owner %q for id %q with flag %v",
				tc.name, tc.store.calls, tc.store.owner, tc.store.id, tc.store.done)
		}
	}

	// A real storage failure is never read as the not-found signal: an outage
	// answers 500, not a 404 that could be mistaken for an absent note.
	broken := &fakeStore{err: errors.New("connection exhausted")}
	if got := patchNote(broken, "note-1", `{"done":true}`); got.Code != http.StatusInternalServerError ||
		got.Body.String() != "{\"message\":\"Service unavailable\"}\n" {
		t.Fatalf("storage failure: got %d %q, want 500 the private message", got.Code, got.Body.String())
	}
}

// Re-applying the flag the note already carries is not an error and changes
// nothing: the note answers 200 with the flag it already had.
func TestPatchSettingTheFlagToWhatItAlreadyIsSucceedsAgain(t *testing.T) {
	for _, flag := range []bool{true, false} {
		store := &fakeStore{update: Note{ID: "note-1", Label: "Buy milk", CreatedAt: created, Done: flag}}
		got := patchNote(store, "note-1", `{"done":`+strconv.FormatBool(flag)+`}`)
		want := `{"id":"note-1","label":"Buy milk","createdAt":"2026-10-12T09:30:00Z","done":` +
			strconv.FormatBool(flag) + "}\n"
		if got.Code != http.StatusOK || got.Body.String() != want {
			t.Fatalf("re-applying done=%v: got %d %s, want 200 %s", flag, got.Code, got.Body.String(), want)
		}
		if store.calls != 1 || store.done != flag {
			t.Fatalf("re-applying done=%v: storage reached %d times with flag %v", flag, store.calls, store.done)
		}
	}
}

// The toggled flag persists for a later read: the PATCH writes the row through the
// store's owner-scoped update, and the read that follows answers the row as the
// update left it - the same store, the same note, the flag now set.
func TestPatchTogglesTheFlagTheLaterReadAnswers(t *testing.T) {
	store := &rowStore{rows: []Note{{ID: "note-1", Label: "Buy milk", CreatedAt: created, Done: false}}}
	patched := patchNote(store, "note-1", `{"done":true}`)
	if patched.Code != http.StatusOK || patched.Body.String() !=
		"{\"id\":\"note-1\",\"label\":\"Buy milk\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":true}\n" {
		t.Fatalf("patch: got %d %s", patched.Code, patched.Body.String())
	}

	listed := request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken)
	if listed.Code != http.StatusOK {
		t.Fatalf("list: got %d %s", listed.Code, listed.Body.String())
	}
	notes := decodeWireNotes(t, listed.Body.Bytes())
	if len(notes) != 1 {
		t.Fatalf("the read answered %d notes, want the one it was given: %s", len(notes), listed.Body.String())
	}
	if notes[0]["id"] != "note-1" || notes[0]["label"] != "Buy milk" {
		t.Fatalf("the read answered %v, want the note the patch updated", notes[0])
	}
	if flag, _ := notes[0]["done"].(bool); !flag {
		t.Fatalf("the toggled flag did not survive the read: %s", listed.Body.String())
	}
	// One update wrote the flag, one read answered it, and nothing else.
	if store.calls != 2 {
		t.Fatalf("the patch and the read together reached storage %d times, want one update and one read", store.calls)
	}
}

// The handler asks storage exactly once per request, and always through the
// owner-scoped update: never a read to check existence first, never a second call
// to read the note back. That single call is what makes the two not-found cases
// one case, so it is checked on the success path and on the 404 alike.
func TestPatchReachesStorageOnceThroughTheOwnerScopedUpdate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		store *updateCalls
		want  int
	}{
		{"an updated note", &updateCalls{fakeStore: fakeStore{update: Note{ID: "note-1", Label: "Buy milk", CreatedAt: created, Done: true}}}, http.StatusOK},
		{"a missing note", &updateCalls{fakeStore: fakeStore{err: ErrNoteNotFound}}, http.StatusNotFound},
		{"a foreign note", &updateCalls{fakeStore: fakeStore{err: ErrNoteNotFound}}, http.StatusNotFound},
		{"a storage failure", &updateCalls{fakeStore: fakeStore{err: errors.New("down")}}, http.StatusInternalServerError},
	} {
		got := patchNote(tc.store, "note-1", `{"done":true}`)
		if got.Code != tc.want {
			t.Fatalf("%s: got %d %q, want %d", tc.name, got.Code, got.Body.String(), tc.want)
		}
		if !slices.Equal(tc.store.methods, []string{"update"}) {
			t.Fatalf("%s: the request reached storage as %v, want the owner-scoped update alone, exactly once",
				tc.name, tc.store.methods)
		}
		if tc.store.owner != "user-1" || tc.store.id != "note-1" || !tc.store.done {
			t.Fatalf("%s: the update went as owner %q for id %q with flag %v",
				tc.name, tc.store.owner, tc.store.id, tc.store.done)
		}
	}
}

// The body carries the flag and nothing else: anything that is not a JSON object
// holding done as a boolean is refused with the existing invalid-request shape,
// and the note is left untouched - storage is never reached with a half-read body.
func TestPatchRefusesABodyThatIsNotABooleanDone(t *testing.T) {
	for _, body := range []string{
		``,
		`not json`,
		`[]`,
		`"done"`,
		`{}`,
		`{"done":null}`,
		`{"done":"true"}`,
		`{"done":1}`,
		`{"done":0}`,
		`{"done":"1"}`,
		`{"done":true}{"done":false}`,
		`{"done":tru}`,
		`{"":true}`,
		`{"Done":true}`,
		strings.Repeat("x", 1025),
	} {
		store := &fakeStore{}
		got := patchNote(store, "note-1", body)
		if got.Code != http.StatusBadRequest || got.Body.String() != "{\"message\":\"Invalid request\"}\n" {
			t.Fatalf("body %q: got %d %q, want 400 the invalid-request message", body, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("body %q: storage reached %d times, want the refusal to happen first", body, store.calls)
		}
	}

	// Both boolean literals are accepted, so the refusal above is about the shape
	// of the body and not about one of the two flags.
	for _, body := range []string{`{"done":true}`, `{"done":false}`} {
		store := &fakeStore{update: Note{ID: "note-1", Label: "Buy milk", CreatedAt: created}}
		if got := patchNote(store, "note-1", body); got.Code != http.StatusOK {
			t.Fatalf("body %s: got %d %q, want 200", body, got.Code, got.Body.String())
		}
	}
}

// rowStore keeps one note as a row: the update writes the flag to the row and the
// read answers what the row then holds, so a test can follow a toggled flag from
// the PATCH that set it to the GET that reads it back, the way the store's own
// statement does it.
type rowStore struct {
	fakeStore
	rows []Note
}

func (s *rowStore) UpdateDone(ctx context.Context, owner, id string, done bool) (Note, error) {
	// The call is recorded by the fake beside it, so the owner, the id and the
	// flag the handler handed over stay checkable.
	s.fakeStore.UpdateDone(ctx, owner, id, done)
	for index, row := range s.rows {
		// The row is the caller's only when the id and the signed-in owner both
		// match, which is how the store scopes its own statement.
		if row.ID == id && owner == "user-1" {
			s.rows[index].Done = done
			return s.rows[index], nil
		}
	}
	return Note{}, ErrNoteNotFound
}

func (s *rowStore) List(ctx context.Context, owner string) ([]Note, error) {
	s.fakeStore.List(ctx, owner)
	// A copy, so the caller of the read cannot write to the row it was answered.
	return append([]Note{}, s.rows...), nil
}

// updateCalls records which store method each call was, so a PATCH can be held to
// the one owner-scoped update and to no read of any kind - a handler that checked
// existence first would show up here as a list or create call.
type updateCalls struct {
	fakeStore
	methods []string
}

func (s *updateCalls) Create(ctx context.Context, owner, label string) (Note, error) {
	s.methods = append(s.methods, "create")
	return s.fakeStore.Create(ctx, owner, label)
}

func (s *updateCalls) List(ctx context.Context, owner string) ([]Note, error) {
	s.methods = append(s.methods, "list")
	return s.fakeStore.List(ctx, owner)
}

func (s *updateCalls) UpdateDone(ctx context.Context, owner, id string, done bool) (Note, error) {
	s.methods = append(s.methods, "update")
	return s.fakeStore.UpdateDone(ctx, owner, id, done)
}
