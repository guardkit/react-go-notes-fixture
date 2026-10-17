package notes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// storedNote is one note as the update tests hold it in storage: the note
// object the API answers with, and the subject the row belongs to. A Note
// carries no owner - ownership lives in the store, exactly as the owner_sub
// column lives in PostgreSQL - so the fake keeps the two apart and can tell a
// note of the caller's from someone else's.
type storedNote struct {
	owner string
	note  Note
}

// patchStore is the owner-scoped store the PATCH tests run against. Its
// UpdateDone mirrors the adapter rather than echoing whatever it was handed:
// the row is reachable only when the id and the owner both match, the flag is
// the only thing written, and the note is answered back as it now stands - so a
// test cannot pass by having the handler answer for a row it never reached.
type patchStore struct {
	stored  []storedNote
	calls   int
	owner   string
	id      string
	done    bool
	failure error
}

func (p *patchStore) Create(context.Context, string, string) (Note, error) {
	return Note{}, errors.New("create is not part of the update path")
}

func (p *patchStore) List(context.Context, string) ([]Note, error) {
	return nil, errors.New("list is not part of the update path")
}

func (p *patchStore) UpdateDone(_ context.Context, owner, id string, done bool) (Note, error) {
	p.calls++
	p.owner, p.id, p.done = owner, id, done
	if p.failure != nil {
		return Note{}, p.failure
	}
	for i, entry := range p.stored {
		// One miss answers both halves of "the owner does not have this note",
		// exactly as the single scoped UPDATE does.
		if entry.note.ID != id || entry.owner != owner {
			continue
		}
		p.stored[i].note.Done = done
		return p.stored[i].note, nil
	}
	return Note{}, ErrNoteNotFound
}

// ownNote is one note of the signed-in subject ("user-1", the only subject
// fakeAuth accepts).
func ownNote(id, label string, done bool) storedNote {
	return storedNote{owner: "user-1", note: Note{ID: id, Label: label, CreatedAt: created, Done: done}}
}

// patchNote sends the update a caller sends for one note, under the same
// content-type and bearer handling the other routes are tested with.
func patchNote(store *patchStore, id, body, token string) *httptest.ResponseRecorder {
	return request(Routes(store, fakeAuth{}, "run-1"), http.MethodPatch, "/api/notes/"+id, body, token)
}

// answeredNote decodes the note the update answered with as raw JSON: a decoded
// struct cannot tell an omitted field from a zero value, so the shape checks ask
// the body itself which fields it carries.
func answeredNote(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var note map[string]any
	if err := json.Unmarshal(body, &note); err != nil {
		t.Fatalf("update answer is not a JSON object: %v (%s)", err, body)
	}
	if note == nil {
		t.Fatalf("update answer is not an object: %s", body)
	}
	return note
}

// The owner completes a not-done note and re-opens a done one, and each time
// the answer is the note itself: its own id, label and creation time beside the
// flag the owner just set. This is the Scenario Outline - both accepted boolean
// values are accepted, and each is answered with the value that was sent.
func TestOwnerSetsTheDoneFlagAndReceivesTheUpdatedNote(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stored    bool
		requested bool
	}{
		{"a not-done note is completed", false, true},
		{"a done note is re-opened", true, false},
	} {
		note := Note{ID: "7", Label: "Buy groceries", CreatedAt: created, Done: tc.stored}
		store := &patchStore{stored: []storedNote{{owner: "user-1", note: note}}}
		got := patchNote(store, note.ID, fmt.Sprintf(`{"done":%t}`, tc.requested), goodToken)
		if got.Code != http.StatusOK {
			t.Fatalf("%s: got %d %q", tc.name, got.Code, got.Body.String())
		}
		answered := answeredNote(t, got.Body.Bytes())
		if len(answered) != 4 {
			t.Fatalf("%s: the updated note carries %v, want id, label, createdAt and done", tc.name, answered)
		}
		if got := doneFlag(t, answered, note.ID); got != tc.requested {
			t.Fatalf("%s: answered done %v, want %v", tc.name, got, tc.requested)
		}
		assertCarriesTheExistingFields(t, answered, note)
		// The flag the store was asked to write is the one the caller sent.
		if store.calls != 1 || store.owner != "user-1" || store.id != note.ID || store.done != tc.requested {
			t.Fatalf("%s: store saw %d calls, owner %q, id %q, done %v", tc.name, store.calls, store.owner, store.id, store.done)
		}
	}
}

// The done flag is the sole mutable field, so the rest of the note survives the
// update: the note stored with the label "Buy groceries" comes back carrying
// that label and its original creation time (ASSUM-002), and the stored row is
// left holding them too.
func TestSettingTheDoneFlagLeavesTheNotesOtherFieldsAsTheyWere(t *testing.T) {
	stored := Note{ID: "3", Label: "Buy groceries", CreatedAt: created}
	store := &patchStore{stored: []storedNote{{owner: "user-1", note: stored}}}
	got := patchNote(store, "3", `{"done":true}`, goodToken)
	if got.Code != http.StatusOK {
		t.Fatalf("got %d %q", got.Code, got.Body.String())
	}
	answered := answeredNote(t, got.Body.Bytes())
	assertCarriesTheExistingFields(t, answered, stored)
	if !doneFlag(t, answered, "3") {
		t.Fatal("the updated note is not done")
	}
	// Nothing outside the flag moved: the row still holds the title and the
	// creation time it was written with.
	if keep := store.stored[0].note; keep.Label != "Buy groceries" || !keep.CreatedAt.Equal(created) || keep.ID != "3" {
		t.Fatalf("the update rewrote stored fields: %+v", keep)
	}
}

// The flag is a boolean or the request is refused: a done value that is absent,
// null or not a boolean answers 400 and never reaches the store. The two
// flag-shaped refusals name the flag, as the title refusals name the title.
func TestRefusesAnAbsentNullOrNonBooleanDoneFlag(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		namesFlag bool
	}{
		{"absent", `{}`, true},
		{"null", `{"done":null}`, true},
		{"a word", `{"done":"yes"}`, true},
		{"a quoted boolean", `{"done":"true"}`, true},
		{"a number", `{"done":1}`, true},
		{"an object", `{"done":{}}`, true},
		{"an array", `{"done":[]}`, true},
		{"not JSON", `done=true`, false},
		{"an empty body", ``, false},
		{"a JSON array", `[]`, false},
	} {
		store := &patchStore{stored: []storedNote{ownNote("7", "Buy groceries", false)}}
		got := patchNote(store, "7", tc.body, goodToken)
		if got.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d %q", tc.name, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("%s: body %q reached persistence", tc.name, tc.body)
		}
		var refusal struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &refusal); err != nil || refusal.Message == "" {
			t.Fatalf("%s: refusal %q is not an Error carrying a message", tc.name, got.Body.String())
		}
		if tc.namesFlag && !strings.Contains(refusal.Message, "done") {
			t.Fatalf("%s: refusal %q does not name the flag", tc.name, refusal.Message)
		}
	}
}

// The flag is the only field a client may set: any other field in the body -
// the title, the id, an owner - is refused outright rather than ignored, so no
// caller can smuggle a second write through the update.
func TestRefusesAnyFieldBesidesTheDoneFlag(t *testing.T) {
	for _, body := range []string{
		`{"done":true,"label":"Rewritten"}`,
		`{"done":true,"id":"9"}`,
		`{"done":true,"owner":"user-2"}`,
		`{"done":true,"createdAt":"2026-10-12T09:30:00Z"}`,
		`{"label":"Buy groceries"}`,
	} {
		store := &patchStore{stored: []storedNote{ownNote("7", "Buy groceries", false)}}
		got := patchNote(store, "7", body, goodToken)
		if got.Code != http.StatusBadRequest {
			t.Fatalf("body %q: got %d %q", body, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("body %q reached persistence", body)
		}
	}
}

// A note the owner does not have is refused with 404, and the two ways of not
// having it - the id was never written, and the id belongs to someone else -
// answer byte-identically, so the refusal reveals nothing about whether the
// note exists at all (ASSUM-003).
func TestAnswersTheSame404ForAMissingNoteAndForSomeoneElsesNote(t *testing.T) {
	store := &patchStore{stored: []storedNote{
		{owner: "user-2", note: Note{ID: "9", Label: "not mine", CreatedAt: created}},
		ownNote("7", "Buy groceries", false),
	}}
	missing := patchNote(store, "1234567890", `{"done":true}`, goodToken)
	foreign := patchNote(store, "9", `{"done":true}`, goodToken)
	if missing.Code != http.StatusNotFound || foreign.Code != http.StatusNotFound {
		t.Fatalf("missing answered %d %q, foreign answered %d %q",
			missing.Code, missing.Body.String(), foreign.Code, foreign.Body.String())
	}
	if missing.Body.String() != foreign.Body.String() {
		t.Fatalf("the two refusals differ and so disclose whether the note exists: %q vs %q",
			missing.Body.String(), foreign.Body.String())
	}
	if missing.Header().Get("Content-Type") != foreign.Header().Get("Content-Type") {
		t.Fatalf("the two refusals are answered as different kinds of body: %q vs %q",
			missing.Header().Get("Content-Type"), foreign.Header().Get("Content-Type"))
	}
	body := missing.Body.String()
	if body != "{\"message\":\"Note not found\"}\n" {
		t.Fatalf("the shared refusal is %q", body)
	}
	// The one body names neither id, so it cannot say which of the two happened.
	for _, id := range []string{"1234567890", "9"} {
		if strings.Contains(body, id) {
			t.Fatalf("the refusal discloses the id %q: %q", id, body)
		}
	}
	// The untouched note of the caller is still there, unchanged.
	if store.stored[1].note.Done {
		t.Fatal("a refused update still wrote the flag")
	}
}

// The route sits behind the same guard as the rest of the API: without a token
// the answer is 401 with the existing challenge, and no note is written.
func TestUpdateRequiresTheSignedInOwner(t *testing.T) {
	for _, tc := range []struct{ name, header string }{
		{"anonymous", ""},
		{"refused token", "Bearer wrong"},
		{"wrong scheme", "Basic " + goodToken},
		{"empty bearer", "Bearer "},
	} {
		store := &patchStore{stored: []storedNote{ownNote("7", "Buy groceries", false)}}
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/7", strings.NewReader(`{"done":true}`))
		r.Header.Set("Content-Type", "application/json")
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		w := httptest.NewRecorder()
		Routes(store, fakeAuth{}, "run-1").ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || w.Body.String() != "{\"message\":\"Sign-in required\"}\n" {
			t.Fatalf("%s: got %d %q", tc.name, w.Code, w.Body.String())
		}
		if !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Fatalf("%s: challenge %q", tc.name, w.Header().Get("WWW-Authenticate"))
		}
		if store.calls != 0 {
			t.Fatalf("%s: an anonymous update reached persistence", tc.name)
		}
	}
}

// The caller names neither the owner nor the note: the owner is the token's
// subject and the id is the path's, and a body that tries to name either is
// refused rather than obeyed.
func TestUpdateTakesTheOwnerFromTheTokenAndTheIDFromThePath(t *testing.T) {
	store := &patchStore{stored: []storedNote{
		ownNote("7", "Buy groceries", false),
		{owner: "user-2", note: Note{ID: "8", Label: "theirs", CreatedAt: created}},
	}}
	// Naming another owner or another note is refused, and nothing is written.
	for _, body := range []string{
		`{"done":true,"owner":"user-2"}`,
		`{"done":true,"sub":"user-2"}`,
		`{"done":true,"id":"8"}`,
	} {
		got := patchNote(store, "7", body, goodToken)
		if got.Code != http.StatusBadRequest || store.calls != 0 {
			t.Fatalf("body %q: got %d %q after %d store calls", body, got.Code, got.Body.String(), store.calls)
		}
	}
	// The accepted update asks for the token's subject and the path's id.
	if got := patchNote(store, "7", `{"done":true}`, goodToken); got.Code != http.StatusOK {
		t.Fatalf("got %d %q", got.Code, got.Body.String())
	}
	if store.owner != "user-1" || store.id != "7" || !store.done {
		t.Fatalf("store was asked for owner %q, id %q, done %v", store.owner, store.id, store.done)
	}
	// Naming another subject's id reaches nothing: the row is scoped to the
	// token's subject, so the caller is answered as for a note it never had.
	if got := patchNote(store, "8", `{"done":true}`, goodToken); got.Code != http.StatusNotFound {
		t.Fatalf("another subject's note answered %d %q", got.Code, got.Body.String())
	}
	if store.stored[1].note.Done {
		t.Fatal("a caller reached a note belonging to another subject")
	}
}

// A persistence failure is answered privately, as the other routes answer it.
func TestUpdateFailureStaysPrivate(t *testing.T) {
	store := &patchStore{failure: errors.New("password=private")}
	got := patchNote(store, "7", `{"done":true}`, goodToken)
	if got.Code != http.StatusInternalServerError || got.Body.String() != "{\"message\":\"Service unavailable\"}\n" {
		t.Fatalf("got %d %q", got.Code, got.Body.String())
	}
	if strings.Contains(got.Body.String(), "password") {
		t.Fatalf("the refusal leaked the failure: %q", got.Body.String())
	}
}

// The adapter must scope the write to the row's owner as well as to its id, so
// a note belonging to another subject is unreachable rather than merely
// unanswered, and no caller value may be spelled into the SQL. No PostgreSQL is
// reachable from the unit suite, so this reads the statement UpdateDone carries.
func TestTheUpdateStatementIsScopedToTheRowOwner(t *testing.T) {
	src, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("read the adapter: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "store.go", src, 0)
	if err != nil {
		t.Fatalf("parse the adapter: %v", err)
	}
	var statement string
	bound := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "UpdateDone" {
			return true
		}
		ast.Inspect(fn, func(inner ast.Node) bool {
			switch e := inner.(type) {
			case *ast.BasicLit:
				if e.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(e.Value)
				if err != nil {
					return true
				}
				if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(value)), "UPDATE NOTES") {
					statement = value
				}
			case *ast.CallExpr:
				sel, ok := e.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "QueryRow" {
					return true
				}
				for _, arg := range e.Args {
					if ident, ok := arg.(*ast.Ident); ok {
						bound[ident.Name] = true
					}
				}
			}
			return true
		})
		return true
	})
	if statement == "" {
		t.Fatal("UpdateDone carries no UPDATE statement against the notes table")
	}
	upper := strings.ToUpper(statement)
	for _, want := range []string{"SET DONE =", "WHERE ID = $", "AND OWNER_SUB = $", "RETURNING ID, LABEL, CREATED_AT, DONE"} {
		if !strings.Contains(upper, want) {
			t.Errorf("the update statement %q does not carry %q", statement, want)
		}
	}
	// The flag is the only column the statement writes.
	for _, written := range []string{"LABEL =", "CREATED_AT ="} {
		if strings.Contains(upper, written) {
			t.Errorf("the update statement %q writes %s, which the caller may not set", statement, written)
		}
	}
	if strings.Contains(statement, "'") {
		t.Errorf("the update statement %q spells a value into the SQL instead of binding it", statement)
	}
	// The owner and the flag reach PostgreSQL as bound arguments of the call.
	for _, name := range []string{"owner", "done"} {
		if !bound[name] {
			t.Errorf("the update statement does not bind %q", name)
		}
	}
}
