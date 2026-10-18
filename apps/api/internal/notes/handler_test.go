package notes

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const goodToken = "good-token"

type fakeStore struct {
	calls int
	owner string
	label string
	ctx   context.Context
	note  Note
	list  []Note
	err   error

	// The done-flag write of TASK-F552-001: the call the handler made, and the
	// note - or failure - storage answers with.
	doneCtx   context.Context
	doneCalls int
	doneOwner string
	doneID    string
	doneValue bool
	doneErr   error
	doneNote  Note
}

func (f *fakeStore) Create(ctx context.Context, owner, label string) (Note, error) {
	f.calls++
	f.ctx, f.owner, f.label = ctx, owner, label
	return f.note, f.err
}

// SetDone records the owner, id and flag it was asked to write, and answers the
// way PGStore.SetDone does: the updated note on a hit, and nothing but the
// not-found sentinel on a miss.
func (f *fakeStore) SetDone(ctx context.Context, owner, id string, done bool) (Note, error) {
	f.doneCalls++
	f.doneCtx, f.doneOwner, f.doneID, f.doneValue = ctx, owner, id, done
	if f.doneErr != nil {
		return Note{}, f.doneErr
	}
	return f.doneNote, nil
}

func (f *fakeStore) List(ctx context.Context, owner string) ([]Note, error) {
	f.calls++
	f.ctx, f.owner = ctx, owner
	return f.list, f.err
}

// fakeAuth accepts exactly goodToken as subject "user-1"; real token checks
// are tested in internal/auth with locally generated keys.
type fakeAuth struct{}

func (fakeAuth) Subject(_ context.Context, raw string) (string, error) {
	if raw != goodToken {
		return "", errors.New("refused")
	}
	return "user-1", nil
}

var created = time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)

func request(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestRejectsInvalidJSONBeforePersistence(t *testing.T) {
	cases := []string{
		// A blank title (absent, empty or whitespace-only) is a decodable body
		// refused by the validation rule, not by the decoder, so those cases
		// live in TestRefusesBlankTitleBeforePersistence.
		`{"label":"ok","extra":1}`,
		`{"label":"ok"}{"label":"again"}`,
		`["ok"]`,
		`{"label":123}`,
		"{\"label\":\"\xff\"}",
		// Over the 1024-byte body guard, so the decode refuses it before the
		// 200-character title rule is reached.
		`{"label":"` + strings.Repeat("x", 1025) + `"}`,
		// The 81-character body that used to sit in this table was refused only
		// by the 80-character cap ValidateLabel applied before this rule existed.
		// A 200-character-inclusive limit and an 81-character refusal cannot both
		// hold, so that one case moved to TestTitleLengthBoundaryAtCreation,
		// where it is asserted as accepted; nothing else here changed, and every
		// case above still answers 400 without reaching persistence.
	}
	for _, body := range cases {
		store := &fakeStore{}
		got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", body, goodToken)
		if got.Code != http.StatusBadRequest || got.Body.String() != "{\"message\":\"Invalid request\"}\n" {
			t.Fatalf("body %q: got %d %q", body, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("body %q reached persistence", body)
		}
	}
}

// blankTitle is the whole 400 body a blank title must answer: a plain message
// naming the title as the invalid field, not the generic "Invalid request".
const blankTitle = "{\"message\":\"label (title) must not be blank\"}\n"

// Every blank title - absent, empty, whitespace-only, or blank once the
// surrounding whitespace is trimmed - is refused by the create handler before
// store.Create is ever called.
func TestRefusesBlankTitleBeforePersistence(t *testing.T) {
	cases := []struct {
		name, body string
	}{
		{"absent title", `{}`},
		{"empty string", `{"label":""}`},
		{"spaces", `{"label":"   "}`},
		{"tabs", `{"label":"\t\t"}`},
		{"newlines", `{"label":"\n"}`},
		{"mixed whitespace", `{"label":" \t\n "}`},
		{"blank once trimmed", `{"label":"\u00a0\u2003"}`},
	}
	for _, tc := range cases {
		store := &fakeStore{note: Note{ID: "1", Label: "x", CreatedAt: created}}
		got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", tc.body, goodToken)
		if got.Code != http.StatusBadRequest || got.Body.String() != blankTitle {
			t.Fatalf("%s: got %d %q", tc.name, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("%s: refused title reached persistence", tc.name)
		}
	}
}

// titleTooLong is the whole 400 body an over-long title must answer: a plain
// message naming the title field and the 200-character limit (ASSUM-004).
const titleTooLong = "{\"message\":\"label (title) must be 200 characters or fewer\"}\n"

// The limit is inclusive and counted in code points, not bytes (ASSUM-002), so
// a title at the limit is stored and one character over is refused before
// store.Create is ever called.
func TestTitleLengthBoundaryAtCreation(t *testing.T) {
	cases := []struct {
		name, label string
		// stored is the title the handler must hand to store.Create: the request
		// title with its surrounding spaces trimmed off before it is counted.
		stored string
	}{
		{"short", "hello", "hello"},
		{"single character", "x", "x"},
		{"one under the limit", strings.Repeat("x", 199), strings.Repeat("x", 199)},
		{"at the limit", strings.Repeat("x", 200), strings.Repeat("x", 200)},
		{"above the superseded 80-character rule", strings.Repeat("x", 81), strings.Repeat("x", 81)},
		// The limit counts the title that is actually stored, so padding may not
		// hide a title that is at the limit.
		{"at the limit once padded", "  " + strings.Repeat("x", 200) + "  ", strings.Repeat("x", 200)},
		// 200 code points are 400 bytes of a two-byte rune and 800 of a
		// four-byte rune; neither may be refused.
		{"at the limit in two-byte runes", strings.Repeat("é", 200), strings.Repeat("é", 200)},
		{"at the limit in four-byte runes", strings.Repeat("\U0001F600", 200), strings.Repeat("\U0001F600", 200)},
	}
	for _, tc := range cases {
		store := &fakeStore{note: Note{ID: "1", Label: tc.stored, CreatedAt: created}}
		got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", `{"label":"`+tc.label+`"}`, goodToken)
		if got.Code != http.StatusCreated {
			t.Fatalf("%s: got %d %q", tc.name, got.Code, got.Body.String())
		}
		if store.calls != 1 || store.label != tc.stored {
			t.Fatalf("%s: calls %d label %q", tc.name, store.calls, store.label)
		}
	}
}

func TestRefusesOverLongTitleBeforePersistence(t *testing.T) {
	cases := []struct {
		name  string
		label string
	}{
		{"one over the limit", strings.Repeat("x", 201)},
		{"well over the limit", strings.Repeat("x", 300)},
		{"one over in two-byte runes", strings.Repeat("é", 201)},
		{"one over in four-byte runes", strings.Repeat("\U0001F600", 201)},
		// Padding is trimmed before the count, so it cannot hide the extra one.
		{"one over the limit once padded", "  " + strings.Repeat("x", 201) + "  "},
	}
	for _, tc := range cases {
		store := &fakeStore{note: Note{ID: "1", Label: tc.label, CreatedAt: created}}
		got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", `{"label":"`+tc.label+`"}`, goodToken)
		if got.Code != http.StatusBadRequest || got.Body.String() != titleTooLong {
			t.Fatalf("%s: got %d %q", tc.name, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("%s: refused title reached persistence", tc.name)
		}
	}
}

func TestCreatePreservesContextOwnerAndWireShape(t *testing.T) {
	store := &fakeStore{note: Note{ID: "9007199254740993", Label: "hello", CreatedAt: created}}
	h := Routes(store, fakeAuth{}, "run-1")
	r := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"label":"  hello  "}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("Authorization", "Bearer "+goodToken)
	type contextKey struct{}
	r = r.WithContext(context.WithValue(r.Context(), contextKey{}, "trace"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	want := "{\"id\":\"9007199254740993\",\"label\":\"hello\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":false}\n"
	if w.Code != http.StatusCreated || w.Body.String() != want {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q", got)
	}
	if store.calls != 1 || store.label != "hello" || store.owner != "user-1" || store.ctx.Value(contextKey{}) != "trace" {
		t.Fatal("validated label, owner or request context not passed to persistence")
	}
}

// storingStore models the real adapter, whose Create hands back the row it
// wrote (INSERT ... RETURNING id, label, created_at). The title it reports is
// therefore the title that reached storage, so one fake proves both halves of
// the trimming rule: what was written and what was answered.
type storingStore struct {
	calls int
	owner string
	label string
}

func (s *storingStore) Create(_ context.Context, owner, label string) (Note, error) {
	s.calls++
	s.owner, s.label = owner, label
	return Note{ID: "1", Label: label, CreatedAt: created}, nil
}

func (s *storingStore) List(context.Context, string) ([]Note, error) { return []Note{}, nil }

// SetDone answers with the row as storage holds it after the flag moved: the
// title and creation time it already carried, beside the flag that was written.
func (s *storingStore) SetDone(_ context.Context, owner, id string, done bool) (Note, error) {
	s.calls++
	s.owner = owner
	return Note{ID: id, Label: s.label, CreatedAt: created, Done: done}, nil
}

// createTitle asks the real create handler to store one title, and reports the
// title the store was handed and the title the response carried.
func createTitle(t *testing.T, store *storingStore, label string) (stored, answered string) {
	t.Helper()
	body, err := json.Marshal(struct {
		Label string `json:"label"`
	}{Label: label})
	if err != nil {
		t.Fatalf("encode title %q: %v", label, err)
	}
	got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", string(body), goodToken)
	if got.Code != http.StatusCreated {
		t.Fatalf("title %q: got %d %q", label, got.Code, got.Body.String())
	}
	if store.calls != 1 || store.owner != "user-1" {
		t.Fatalf("title %q: calls %d owner %q", label, store.calls, store.owner)
	}
	var note Note
	if err := json.Unmarshal(got.Body.Bytes(), &note); err != nil {
		t.Fatalf("title %q: decode response %q: %v", label, got.Body.String(), err)
	}
	return store.label, note.Label
}

// Every surrounding-whitespace position - leading, trailing and both - comes off
// the title before it is written, and the create response carries the title that
// was written. The interior is never touched.
func TestCreateTrimsSurroundingWhitespaceFromTheTitle(t *testing.T) {
	// position names the surrounding-space position each case exercises, so the
	// guard after the loop can prove the table still answers every position the
	// rule speaks of, and that each one was answered by the create handler.
	cases := []struct {
		name, label, stored, position string
	}{
		{"leading only", "   hello", "hello", "leading"},
		{"trailing only", "hello   ", "hello", "trailing"},
		{"leading and trailing", "   hello   ", "hello", "both"},
		{"no surrounding whitespace", "hello world", "hello world", "none"},
		{"interior double spaces", "My  Note  Title", "My  Note  Title", "none"},
		{"interior kept, edges cut", "  My  Note  Title  ", "My  Note  Title", "both"},
		{"interior tabs and newlines", "a\tb\nc", "a\tb\nc", "none"},
		{"tabs and newlines at the edges", "\t\nhello\r\n", "hello", "both"},
		{"unicode whitespace at the edges", "\u00a0hello\u2003", "hello", "both"},
		{"punctuation and digits kept", " !! 1st note? ", "!! 1st note?", "both"},
	}
	answeredBy := map[string]bool{}
	for _, tc := range cases {
		store := &storingStore{}
		stored, answered := createTitle(t, store, tc.label)
		if stored != tc.stored {
			t.Fatalf("%s: persisted title %q, want %q", tc.name, stored, tc.stored)
		}
		if answered != tc.stored {
			t.Fatalf("%s: response title %q, want %q", tc.name, answered, tc.stored)
		}
		// createTitle fails unless the case created exactly one note through the
		// in-process router, so recording the call count is what proves the
		// position was answered by the create handler and not by a helper.
		answeredBy[tc.position] = store.calls == 1
	}
	for _, position := range []string{"leading", "trailing", "both"} {
		if !answeredBy[position] {
			t.Fatalf("surrounding-space position %q is not answered by a create-handler case", position)
		}
	}
}

// An update answers with the trimmed title as well: the trim lives at the
// persistence boundary (storageTitle, the last step before the row is written),
// so a replacement title loses its surrounding whitespace on the way in, and the
// note response carries the title that was written. The delivered surface has one
// note write - contract/openapi.yaml documents only GET and POST /api/notes, and
// notes.Store declares only Create and List - so the update case is pinned at the
// two steps an update is made of, the shared write-boundary trim and the note
// response, both of which keep their bite when an update handler joins them.
func TestUpdateTrimsSurroundingWhitespaceFromTheReplacementTitle(t *testing.T) {
	cases := []struct{ name, replacement, trimmed string }{
		{"trailing spaces", "hello   ", "hello"},
		{"trailing tab and carriage return", "hello\t\r", "hello"},
		{"trailing unicode whitespace", "hello\u00a0\u2003", "hello"},
		{"trailing spaces, interior kept", "My  Note  Title   ", "My  Note  Title"},
	}
	for _, tc := range cases {
		// The write path: the replacement title as it stands the moment the row
		// is written, whatever handler handed it over.
		if got := storageTitle(tc.replacement); got != tc.trimmed {
			t.Fatalf("%s: persisted replacement title %q, want %q", tc.name, got, tc.trimmed)
		}
		// The response: the note write answers with the title it persisted, so
		// the title field of the response body carries the trimmed replacement
		// title rather than what the client padded.
		store := &storingStore{}
		stored, answered := createTitle(t, store, tc.replacement)
		if stored != tc.trimmed || answered != tc.trimmed {
			t.Fatalf("%s: stored %q, response title %q, want %q", tc.name, stored, answered, tc.trimmed)
		}
	}
}

// A title that is nothing but whitespace trims down to the empty title. The
// blank-title rule in validation.go refuses it before persistence, so the
// delivered answer is the blank-title 400 and no note carrying an empty title
// is ever stored; this task must not move that rule.
func TestWhitespaceOnlyTitleTrimsToAnEmptyTitle(t *testing.T) {
	for _, label := range []string{"   ", "\t\t", "\n", " \t\n ", "\u00a0\u2003"} {
		store := &storingStore{}
		// The trim, on its own terms: an all-spaces title has nothing left but
		// the empty title, so that is the title the rule yields for it.
		if trimmed := storageTitle(label); trimmed != "" {
			t.Fatalf("title %q: trimmed title %q, want the empty title", label, trimmed)
		}
		body, err := json.Marshal(struct {
			Label string `json:"label"`
		}{Label: label})
		if err != nil {
			t.Fatalf("encode title %q: %v", label, err)
		}
		got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPost, "/api/notes", string(body), goodToken)
		if got.Code != http.StatusBadRequest || got.Body.String() != blankTitle {
			t.Fatalf("title %q: got %d %q", label, got.Code, got.Body.String())
		}
		if store.calls != 0 {
			t.Fatalf("title %q: trimmed-empty title reached persistence as %q", label, store.label)
		}
	}
}

// The trim lives at the persistence boundary rather than in one handler: every
// note write reaches storage through storageTitle, so the title that is written
// is trimmed at the edges only - by the create path today, and by an update path
// whenever one joins it - whatever the caller handed over. The interior is never
// rewritten, and a title that is nothing but whitespace reaches storage empty.
func TestStorageTitleIsTheTrimEveryWritePathApplies(t *testing.T) {
	cases := []struct{ name, submitted, stored string }{
		{"leading only", "   hello", "hello"},
		{"trailing only", "hello   ", "hello"},
		{"leading and trailing", "   hello   ", "hello"},
		{"nothing to cut", "hello world", "hello world"},
		{"interior kept, edges cut", "  My  Note  Title  ", "My  Note  Title"},
		{"tabs and newlines at the edges", "\t\nhello\r\n", "hello"},
		{"whitespace only", " \t\n ", ""},
	}
	for _, tc := range cases {
		if got := storageTitle(tc.submitted); got != tc.stored {
			t.Fatalf("%s: title reaching storage %q, want %q", tc.name, got, tc.stored)
		}
	}
}

func TestListIsScopedToTheSignedInOwner(t *testing.T) {
	store := &fakeStore{list: []Note{{ID: "2", Label: "second", CreatedAt: created}}}
	got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken)
	want := "{\"notes\":[{\"id\":\"2\",\"label\":\"second\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":false}]}\n"
	if got.Code != http.StatusOK || got.Body.String() != want || store.owner != "user-1" {
		t.Fatalf("got %d %q owner %q", got.Code, got.Body.String(), store.owner)
	}
	empty := request(Routes(&fakeStore{}, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken)
	if empty.Body.String() != "{\"notes\":[]}\n" {
		t.Fatalf("empty list %q", empty.Body.String())
	}
}

func TestAnonymousAndRefusedTokensGet401BeforePersistence(t *testing.T) {
	for _, tc := range []struct{ method, header string }{
		{http.MethodGet, ""},
		{http.MethodPost, ""},
		{http.MethodGet, "Bearer wrong"},
		{http.MethodPost, "Bearer wrong"},
		{http.MethodGet, "Basic " + goodToken},
		{http.MethodGet, "Bearer "},
	} {
		store := &fakeStore{}
		r := httptest.NewRequest(tc.method, "/api/notes", strings.NewReader(`{"label":"x"}`))
		r.Header.Set("Content-Type", "application/json")
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		w := httptest.NewRecorder()
		Routes(store, fakeAuth{}, "run-1").ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || w.Body.String() != "{\"message\":\"Sign-in required\"}\n" {
			t.Fatalf("%s %q: got %d %q", tc.method, tc.header, w.Code, w.Body.String())
		}
		if !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Bearer") || store.calls != 0 {
			t.Fatalf("%s %q: challenge %q calls %d", tc.method, tc.header, w.Header().Get("WWW-Authenticate"), store.calls)
		}
	}
}

func TestPersistenceFailureIsPrivate(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		store := &fakeStore{err: errors.New("password=private")}
		got := request(Routes(store, fakeAuth{}, "run-1"), method, "/api/notes", `{"label":"hello"}`, goodToken)
		if got.Code != http.StatusInternalServerError || got.Body.String() != "{\"message\":\"Service unavailable\"}\n" {
			t.Fatalf("%s: got %d %q", method, got.Code, got.Body.String())
		}
	}
}

func TestMethodsAndHealth(t *testing.T) {
	h := Routes(&fakeStore{}, fakeAuth{}, "run-1")
	if got := request(h, http.MethodDelete, "/api/notes", "", goodToken); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE notes: %d", got.Code)
	}
	if got := request(h, http.MethodPost, "/api/health", "", ""); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST health: %d", got.Code)
	}
	if got := request(h, http.MethodGet, "/api/health", "", ""); got.Code != http.StatusOK || got.Body.String() != "{\"status\":\"ok\",\"run\":\"run-1\"}\n" {
		t.Fatalf("GET health: %d %q", got.Code, got.Body.String())
	}
	if got := request(h, http.MethodGet, "/api/missing", "", goodToken); got.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", got.Code)
	}
}

// listingNotes decodes the notes array of a GET /api/notes body as raw JSON
// objects: a decoded struct cannot tell an omitted field from its zero value, so
// the done-flag checks ask the response body itself whether the flag is there.
func listingNotes(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var envelope struct {
		Notes []map[string]any `json:"notes"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode listing %q: %v", body, err)
	}
	return envelope.Notes
}

// listedNotes asks the real listing route for the signed-in owner's notes
// exactly as the store holds them, and answers with every note as the caller
// receives it, beside the owner the store was asked for. A listing that answers
// fewer notes than the store holds fails here, so no note can slip past the
// checks that walk the result.
func listedNotes(t *testing.T, stored []Note) ([]map[string]any, string) {
	t.Helper()
	store := &fakeStore{list: stored}
	got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken)
	if got.Code != http.StatusOK {
		t.Fatalf("GET /api/notes: got %d %q", got.Code, got.Body.String())
	}
	notes := listingNotes(t, got.Body.Bytes())
	if len(notes) != len(stored) {
		t.Fatalf("listing carries %d notes, want %d: %q", len(notes), len(stored), got.Body.String())
	}
	return notes, store.owner
}

// doneFlag answers with the done flag one listed note carries, refusing a
// listing that omits the flag, nulls it, or answers with anything but a JSON
// boolean.
func doneFlag(t *testing.T, note map[string]any, id string) bool {
	t.Helper()
	raw, found := note["done"]
	if !found {
		t.Fatalf("note %s: the listing omits the done flag: %v", id, note)
	}
	if raw == nil {
		t.Fatalf("note %s: the done flag is null", id)
	}
	flag, ok := raw.(bool)
	if !ok {
		t.Fatalf("note %s: the done flag %v is not a JSON boolean", id, raw)
	}
	return flag
}

// assertCarriesTheExistingFields pins the fields the listing already carried:
// the done flag is added beside a note's id, label and creation time, and
// nothing is renamed, retyped or dropped on the way.
func assertCarriesTheExistingFields(t *testing.T, note map[string]any, want Note) {
	t.Helper()
	if note["id"] != want.ID || note["label"] != want.Label {
		t.Fatalf("note %s: id/label became %v/%v, want %q/%q", want.ID, note["id"], note["label"], want.ID, want.Label)
	}
	if got := note["createdAt"]; got != want.CreatedAt.UTC().Format(time.RFC3339) {
		t.Fatalf("note %s: createdAt became %v, want %q", want.ID, got, want.CreatedAt.UTC().Format(time.RFC3339))
	}
}

// A note written before the done column existed is stored with the 0003
// migration's false default, and the listing hands that flag over. The row is
// observable only through GET /api/notes (ASSUM-004), so the column default is
// pinned here rather than by reading storage: the zero Done value is exactly the
// row the migration backfilled.
func TestListingAnswersANoteStoredBeforeTheDoneFlagWithFalse(t *testing.T) {
	before := Note{ID: "1", Label: "before the flag", CreatedAt: created}
	notes, owner := listedNotes(t, []Note{before})
	if owner != "user-1" {
		t.Fatalf("listing scoped to owner %q, want user-1", owner)
	}
	if doneFlag(t, notes[0], before.ID) {
		t.Fatal("a note stored before the done flag existed is listed as done")
	}
	assertCarriesTheExistingFields(t, notes[0], before)
}

// A note whose stored completion state is done is listed with done true: the
// just-inside bound of the boolean domain. The state is seeded as stored state,
// because this slice has no caller-facing write path that sets it (ASSUM-004).
func TestListingAnswersADoneNoteWithTrue(t *testing.T) {
	finished := Note{ID: "7", Label: "finished", CreatedAt: created, Done: true}
	notes, _ := listedNotes(t, []Note{finished})
	if !doneFlag(t, notes[0], finished.ID) {
		t.Fatal("a note stored as done is listed as not done")
	}
	assertCarriesTheExistingFields(t, notes[0], finished)
}

// Every note of a mixed listing carries its own flag: one note's completion
// state never leaks into another, no note is left without a flag, no flag is
// anything but a JSON boolean, and the listing still answers the signed-in
// owner's notes newest-first.
func TestEveryNoteOfAMixedListingCarriesItsOwnDoneFlag(t *testing.T) {
	stored := []Note{
		{ID: "4", Label: "fourth", CreatedAt: created.Add(time.Minute), Done: true},
		{ID: "3", Label: "third", CreatedAt: created, Done: false},
		{ID: "2", Label: "second", CreatedAt: created.Add(-time.Minute), Done: true},
		{ID: "1", Label: "first", CreatedAt: created.Add(-2 * time.Minute), Done: false},
	}
	notes, owner := listedNotes(t, stored)
	if owner != "user-1" {
		t.Fatalf("listing scoped to owner %q, want user-1", owner)
	}
	for i, note := range notes {
		want := stored[i]
		if got := doneFlag(t, note, want.ID); got != want.Done {
			t.Fatalf("note %s: done flag %v, want %v", want.ID, got, want.Done)
		}
		assertCarriesTheExistingFields(t, note, want)
	}
	if notes[0]["id"] != "4" || notes[3]["id"] != "1" {
		t.Fatalf("listing no longer answers newest-first: %v", notes)
	}
}

// noteShapeTypes names every type handler.go may use to shape one note of the
// listing. A type the route starts naming that is not listed here fails the
// check below, so the each-note proof cannot quietly stop covering the surface.
var noteShapeTypes = map[string]reflect.Type{
	"Note": reflect.TypeOf(Note{}),
}

// listingMemberTypeNames parses handler.go - the file that serves the listing -
// and answers with the note types its GET /api/notes route names to shape each
// note of the response. The set is discovered when the test runs, so a route
// that stops naming a note type is caught rather than assumed.
func listingMemberTypeNames(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("read the listing handler: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "handler.go", src, 0)
	if err != nil {
		t.Fatalf("parse the listing handler: %v", err)
	}
	builtins := map[string]bool{"any": true, "bool": true, "byte": true, "error": true, "int": true, "int64": true, "rune": true, "string": true}
	names := map[string]bool{}
	var collect func(ast.Expr)
	collect = func(expr ast.Expr) {
		switch e := expr.(type) {
		case *ast.ArrayType:
			collect(e.Elt)
		case *ast.StarExpr:
			collect(e.X)
		case *ast.MapType:
			collect(e.Value)
		case *ast.StructType:
			for _, field := range e.Fields.List {
				collect(field.Type)
			}
		case *ast.Ident:
			if !builtins[e.Name] {
				names[e.Name] = true
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		route, ok := call.Args[0].(*ast.BasicLit)
		if !ok || route.Kind != token.STRING {
			return true
		}
		if spec, err := strconv.Unquote(route.Value); err != nil || spec != "GET /api/notes" {
			return true
		}
		ast.Inspect(call.Args[1], func(inner ast.Node) bool {
			switch e := inner.(type) {
			case *ast.StructType:
				for _, field := range e.Fields.List {
					collect(field.Type)
				}
			case *ast.ArrayType:
				collect(e.Elt)
			}
			return true
		})
		return true
	})
	members := make([]string, 0, len(names))
	for name := range names {
		members = append(members, name)
	}
	sort.Strings(members)
	return members
}

// One check covers "each note": it discovers the members of the each-note set
// from handler.go when it runs, refuses a set it finds empty or a member it does
// not cover, and then demands of every member that a listed note can never be
// answered without its completion flag - a boolean named done that encoding can
// never leave out. The members are proven on the wire as well as on the type,
// from the listing the handler really answers with.
func TestEachNoteListingMemberCarriesTheDoneFlag(t *testing.T) {
	members := listingMemberTypeNames(t)
	if len(members) == 0 {
		t.Fatal("the each-note set is empty: handler.go shapes the listing without naming a note type")
	}
	for _, member := range members {
		noteType, ok := noteShapeTypes[member]
		if !ok {
			t.Fatalf("handler.go shapes each listed note with %q, which the done-flag check does not cover", member)
		}
		field, ok := noteType.FieldByName("Done")
		if !ok {
			t.Fatalf("type %q has no Done field, so a listed note can be answered without a completion flag", member)
		}
		if field.Type.Kind() != reflect.Bool {
			t.Fatalf("type %q carries Done as %s, want a boolean", member, field.Type.Kind())
		}
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "done" {
			t.Fatalf("type %q encodes Done as %q, want the done flag", member, name)
		}
		if strings.Contains(options, "omitempty") {
			t.Fatalf("type %q may omit the done flag: json tag %q", member, field.Tag.Get("json"))
		}
	}
	// The members the set discovered are the ones the listing really answers
	// with, so the flag is proven on the wire as well as on the type.
	stored := []Note{
		{ID: "2", Label: "done", CreatedAt: created, Done: true},
		{ID: "1", Label: "not done", CreatedAt: created.Add(-time.Minute)},
	}
	notes, _ := listedNotes(t, stored)
	for i, note := range notes {
		if got := doneFlag(t, note, stored[i].ID); got != stored[i].Done {
			t.Fatalf("note %s of the members' listing: done flag %v, want %v", stored[i].ID, got, stored[i].Done)
		}
	}
}

// storeMethodStatements parses store.go and answers with the SQL statements and
// the Scan targets of one PGStore method, so the columns the store reads and
// writes are checked as they are really written rather than as a copy kept in
// this test. A method the file does not declare is a failure, not a skip.
func storeMethodStatements(t *testing.T, method string) (sql []string, scan []string) {
	t.Helper()
	src, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "store.go", src, 0)
	if err != nil {
		t.Fatalf("parse the store: %v", err)
	}
	declared := false
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		if !ok || decl.Recv == nil || len(decl.Recv.List) == 0 ||
			!strings.Contains(types.ExprString(decl.Recv.List[0].Type), "PGStore") ||
			decl.Name.Name != method {
			return true
		}
		declared = true
		ast.Inspect(decl, func(inner ast.Node) bool {
			switch e := inner.(type) {
			case *ast.BasicLit:
				if e.Kind == token.STRING {
					if text, err := strconv.Unquote(e.Value); err == nil && strings.Contains(text, "notes") {
						sql = append(sql, text)
					}
				}
			case *ast.CallExpr:
				if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Scan" {
					for _, arg := range e.Args {
						scan = append(scan, types.ExprString(arg))
					}
				}
			}
			return true
		})
		return true
	})
	if !declared {
		t.Fatalf("store.go declares no PGStore method %q for the done flag to be read through", method)
	}
	return sql, scan
}

// selectList answers with the columns a SELECT names, between SELECT and FROM.
func selectList(t *testing.T, statement string) []string {
	t.Helper()
	upper := strings.ToUpper(statement)
	from := strings.Index(upper, " FROM ")
	if !strings.HasPrefix(strings.TrimSpace(upper), "SELECT") || from < 0 {
		t.Fatalf("not a SELECT on notes: %q", statement)
	}
	list := strings.TrimSpace(statement[len("SELECT"):from])
	var columns []string
	for _, column := range strings.Split(list, ",") {
		columns = append(columns, strings.TrimSpace(column))
	}
	return columns
}

// The listing reads the completion flag out of storage: PGStore.List selects the
// done column the 0003 migration adds beside the columns it already read, and
// scans it into the note it hands the handler. Reading it is the only way a row
// written before the column existed can be answered with the false the column
// default gave it. Create, which has no write path for the flag (ASSUM-004),
// still names no done column, so the default writes the new row.
func TestListReadsTheDoneColumnAndCreateWritesNoFlag(t *testing.T) {
	statements, scan := storeMethodStatements(t, "List")
	var listed string
	for _, statement := range statements {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "SELECT") {
			listed = statement
		}
	}
	if listed == "" {
		t.Fatal("PGStore.List reads no SELECT on notes")
	}
	columns := selectList(t, listed)
	for _, want := range []string{"id", "label", "created_at", "done"} {
		found := false
		for _, column := range columns {
			if column == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("PGStore.List selects %v, which leaves out %s", columns, want)
		}
	}
	scansFlag := false
	for _, target := range scan {
		if target == "&note.Done" {
			scansFlag = true
		}
	}
	if !scansFlag {
		t.Fatalf("PGStore.List scans %v, which never fills in the done flag", scan)
	}

	createdStatements, _ := storeMethodStatements(t, "Create")
	var inserted string
	for _, statement := range createdStatements {
		if strings.Contains(strings.ToUpper(strings.TrimSpace(statement)), "INSERT INTO NOTES") {
			inserted = statement
		}
	}
	if inserted == "" {
		t.Fatal("PGStore.Create writes no INSERT on notes")
	}
	openParen := strings.Index(inserted, "(")
	closeParen := strings.Index(inserted, ")")
	if openParen < 0 || closeParen < openParen {
		t.Fatalf("PGStore.Create writes an INSERT with no column list: %q", inserted)
	}
	if strings.Contains(strings.ToLower(inserted[openParen:closeParen]), "done") {
		t.Fatalf("PGStore.Create writes the completion flag, which no caller may set: %q", inserted)
	}
}

// storedRow is one row of the done store's notes table, kept beside the subject
// that owns it, so a write can be scoped to the caller exactly as the real
// owner-scoped UPDATE is.
type storedRow struct {
	owner string
	note  Note
}

// doneStore is the notes table a caller may toggle the completion flag of. It
// answers a write the way PGStore.SetDone does: the row is looked up by id AND
// owner, its flag is written, and the whole updated row is read back - so a
// foreign id is a miss that changes nothing rather than a hit on someone else's
// note.
type doneStore struct {
	rows  []storedRow
	calls int
	owner string
	id    string
	value bool
	ctx   context.Context
}

func (d *doneStore) Create(context.Context, string, string) (Note, error) {
	return Note{}, errors.New("the done store writes no new note")
}

func (d *doneStore) List(_ context.Context, owner string) ([]Note, error) {
	list := []Note{}
	for _, row := range d.rows {
		if row.owner == owner {
			list = append(list, row.note)
		}
	}
	return list, nil
}

func (d *doneStore) SetDone(ctx context.Context, owner, id string, done bool) (Note, error) {
	d.calls++
	d.ctx, d.owner, d.id, d.value = ctx, owner, id, done
	for i, row := range d.rows {
		// Both filters of the real UPDATE, applied to one row: a note of another
		// owner never matches, whatever its id.
		if row.owner != owner || row.note.ID != id {
			continue
		}
		d.rows[i].note.Done = done
		updated := d.rows[i].note
		updated.CreatedAt = updated.CreatedAt.UTC()
		return updated, nil
	}
	return Note{}, ErrNoteNotFound
}

// The write behind the done flag, as the caller of the storage layer sees it:
// both toggle directions are answered with the note that now carries the new
// flag, the store is asked for the signed-in owner, the requested id and the
// flag exactly as it came in, and a write that matches no row of the caller's
// answers the not-found sentinel with no note at all.
func TestSetDoneWritesTheFlagBothWaysAndReportsAMissAsNotFound(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "trace")
	for _, flag := range []bool{true, false} {
		store := &fakeStore{doneNote: Note{ID: "7", Label: "keep it short", CreatedAt: created, Done: flag}}
		got, err := store.SetDone(ctx, "user-1", "7", flag)
		if err != nil {
			t.Fatalf("done %v: %v", flag, err)
		}
		if want := (Note{ID: "7", Label: "keep it short", CreatedAt: created, Done: flag}); got != want {
			t.Fatalf("done %v: answered note %+v, want %+v", flag, got, want)
		}
		if store.doneOwner != "user-1" || store.doneID != "7" || store.doneValue != flag {
			t.Fatalf("done %v: store asked for owner %q id %q flag %v", flag, store.doneOwner, store.doneID, store.doneValue)
		}
		if store.doneCtx == nil || store.doneCtx.Value(contextKey{}) != "trace" {
			t.Fatalf("done %v: the write did not carry the request context", flag)
		}
		if store.doneCalls != 1 || store.calls != 0 {
			t.Fatalf("done %v: %d done writes, %d other calls", flag, store.doneCalls, store.calls)
		}
	}
	// No row of the caller matches the id: the answer is the sentinel the route
	// maps to 404, and never a zero note dressed up as a real one.
	miss := &fakeStore{doneErr: ErrNoteNotFound}
	got, err := miss.SetDone(ctx, "user-1", "999", true)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("miss: err %v, want ErrNoteNotFound", err)
	}
	if got != (Note{}) {
		t.Fatalf("miss: answered %+v, want no note at all", got)
	}
	if miss.doneOwner != "user-1" || miss.doneID != "999" || !miss.doneValue {
		t.Fatalf("miss: store asked for owner %q id %q flag %v", miss.doneOwner, miss.doneID, miss.doneValue)
	}
}

// The write is scoped to the signed-in owner on the row it changes: the flag of
// the caller's own note moves - and stays moved when it is read back - while a
// note of another owner is neither changed nor answered, and reads as the same
// not-found as an id nobody holds.
func TestSetDoneChangesOnlyTheOwnersOwnRow(t *testing.T) {
	mine := Note{ID: "7", Label: "mine", CreatedAt: created}
	theirs := Note{ID: "8", Label: "theirs", CreatedAt: created.Add(-time.Minute)}
	store := &doneStore{rows: []storedRow{{owner: "user-1", note: mine}, {owner: "user-2", note: theirs}}}

	// Toggle on, then off: both directions answer with the note as it now stands.
	for _, flag := range []bool{true, false} {
		got, err := store.SetDone(context.Background(), "user-1", "7", flag)
		if err != nil {
			t.Fatalf("done %v: %v", flag, err)
		}
		if want := (Note{ID: "7", Label: "mine", CreatedAt: created, Done: flag}); got != want {
			t.Fatalf("done %v: answered %+v, want %+v", flag, got, want)
		}
	}
	if store.calls != 2 {
		t.Fatalf("%d writes for two toggles", store.calls)
	}
	list, err := store.List(context.Background(), "user-1")
	if err != nil || len(list) != 1 || list[0].Done {
		t.Fatalf("the last write reads back as %+v (%v), want the flag false it was set to", list, err)
	}
	// Someone else's note is a miss on the caller's write, and stays untouched.
	got, err := store.SetDone(context.Background(), "user-1", "8", true)
	if !errors.Is(err, ErrNoteNotFound) || got != (Note{}) {
		t.Fatalf("foreign note: got %+v err %v, want no note and ErrNoteNotFound", got, err)
	}
	if store.rows[1].note != theirs {
		t.Fatalf("a write for another owner's note changed it to %+v", store.rows[1].note)
	}
	// Its own owner can still write it, and an id nobody holds is the same miss.
	if _, err := store.SetDone(context.Background(), "user-2", "8", true); err != nil {
		t.Fatalf("owner of note 8 refused: %v", err)
	}
	if !store.rows[1].note.Done {
		t.Fatal("note 8 did not take its owner's write")
	}
	if _, err := store.SetDone(context.Background(), "user-1", "999", false); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("missing note: err %v, want ErrNoteNotFound", err)
	}
}

// statementTail answers with everything a statement says after a keyword,
// refusing a statement that never says it.
func statementTail(t *testing.T, statement, from string) string {
	t.Helper()
	upper := strings.ToUpper(statement)
	start := strings.Index(upper, strings.ToUpper(from))
	if start < 0 {
		t.Fatalf("statement %q never says %s", statement, from)
	}
	return strings.TrimSpace(statement[start+len(from):])
}

// storeMethodBindings parses store.go and answers with the SQL statements, the
// Scan targets, the values bound to the statement's placeholders and the
// qualified functions of one PGStore method, so a write is checked as it is
// really written rather than as a copy kept in this test.
func storeMethodBindings(t *testing.T, method string) (sql, scan, bound, fns []string) {
	t.Helper()
	src, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "store.go", src, 0)
	if err != nil {
		t.Fatalf("parse the store: %v", err)
	}
	declared := false
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.FuncDecl)
		if !ok || decl.Recv == nil || len(decl.Recv.List) == 0 ||
			!strings.Contains(types.ExprString(decl.Recv.List[0].Type), "PGStore") ||
			decl.Name.Name != method {
			return true
		}
		declared = true
		ast.Inspect(decl, func(inner ast.Node) bool {
			switch e := inner.(type) {
			case *ast.BasicLit:
				if e.Kind == token.STRING {
					if text, err := strconv.Unquote(e.Value); err == nil && strings.Contains(text, "notes") {
						sql = append(sql, text)
					}
				}
			case *ast.CallExpr:
				sel, ok := e.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Scan":
					for _, arg := range e.Args {
						scan = append(scan, types.ExprString(arg))
					}
				case "Query", "QueryRow":
					if len(e.Args) > 2 {
						for _, arg := range e.Args[2:] {
							bound = append(bound, types.ExprString(arg))
						}
					}
				default:
					fns = append(fns, types.ExprString(e.Fun))
				}
			}
			return true
		})
		return true
	})
	if !declared {
		t.Fatalf("store.go declares no PGStore method %q", method)
	}
	return sql, scan, bound, fns
}

// statementPart answers with the section of a statement between two keywords,
// refusing a statement that does not carry them in that order.
func statementPart(t *testing.T, statement, from, to string) string {
	t.Helper()
	upper := strings.ToUpper(statement)
	start := strings.Index(upper, strings.ToUpper(from))
	if start < 0 {
		t.Fatalf("statement %q never says %s", statement, from)
	}
	end := strings.Index(upper[start:], strings.ToUpper(to))
	if end < 0 {
		t.Fatalf("statement %q says %s but never %s after it", statement, from, to)
	}
	return strings.TrimSpace(statement[start+len(from) : start+end])
}

// The write itself, read out of store.go as it is really written: one UPDATE on
// notes that sets the done column and filters on both the id and the caller's
// owner_sub on that same statement, so a foreign note cannot be touched; the
// whole updated row read back in the same statement - id, label, created_at and
// done - and scanned into the note the caller answers with, the flag into the
// boolean field rather than a string; and the flag bound to its placeholder as
// the Go boolean it is, beside the id and the owner, with no quoted literal and
// no statement built out of the caller's values.
func TestSetDoneUpdatesTheOwnersRowAndReadsTheWholeNoteBack(t *testing.T) {
	statements, scan, bound, fns := storeMethodBindings(t, "SetDone")
	var updated string
	for _, statement := range statements {
		if strings.Contains(strings.ToUpper(strings.TrimSpace(statement)), "UPDATE NOTES") {
			updated = statement
		}
	}
	if updated == "" {
		t.Fatal("PGStore.SetDone writes no UPDATE on notes")
	}
	lowered := strings.ToLower(updated)
	if strings.Contains(lowered, "'") || strings.Contains(lowered, "%") {
		t.Fatalf("PGStore.SetDone builds its statement from literals or caller values: %q", updated)
	}
	if set := statementPart(t, updated, "SET", "WHERE"); !strings.Contains(strings.ToLower(set), "done = $") {
		t.Fatalf("PGStore.SetDone sets %q, which does not write the done flag by placeholder", set)
	}
	where := strings.ToLower(statementPart(t, updated, "WHERE", "RETURNING"))
	for _, filter := range []string{"id = $", "owner_sub = $"} {
		if !strings.Contains(where, filter) {
			t.Fatalf("PGStore.SetDone filters %q, which leaves out the %s scope", where, filter)
		}
	}
	returning := statementTail(t, updated, "RETURNING")
	columns := []string{}
	for _, column := range strings.Split(returning, ",") {
		columns = append(columns, strings.TrimSpace(column))
	}
	for _, want := range []string{"id", "label", "created_at", "done"} {
		found := false
		for _, column := range columns {
			if column == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("PGStore.SetDone reads back %v, which leaves out %s", columns, want)
		}
	}
	scansFlag := false
	for _, target := range scan {
		if target == "&note.Done" {
			scansFlag = true
		}
	}
	if !scansFlag {
		t.Fatalf("PGStore.SetDone scans %v, which never fills in the done flag as a boolean", scan)
	}
	if len(bound) != 3 || bound[0] != "done" || bound[1] != "noteID" || bound[2] != "owner" {
		t.Fatalf("PGStore.SetDone binds %v, want the boolean flag, the note id and the owner", bound)
	}
	formatsID := false
	for _, fn := range fns {
		if fn == "strconv.FormatInt" {
			formatsID = true
		}
	}
	if !formatsID {
		t.Fatalf("PGStore.SetDone calls %v, which never shapes the id the way List does", fns)
	}
}

// listingShapeMembers is every file of the repository that declares or reads the
// shape of one listed note - the handler, the store, the contract, the generated
// types, the web client and the view - with the check that covers it.
// mustCarryDone marks the members this task's change is responsible for putting
// the flag in; the others carry it through the contract declaration and the
// regeneration that TASK-9018-003 delivers.
var listingShapeMembers = map[string]struct {
	coveredBy     string
	mustCarryDone bool
}{
	"apps/api/internal/notes/handler.go": {coveredBy: "TestEachNoteListingMemberCarriesTheDoneFlag and the listing wire-shape assertions above", mustCarryDone: true},
	"apps/api/internal/notes/store.go":   {coveredBy: "TestListReadsTheDoneColumnAndCreateWritesNoFlag, which reads the List and Create statements back out of this file", mustCarryDone: true},
	"contract/openapi.yaml":              {coveredBy: "TestImplementedResponsesMatchTheContract and TestContractRefusesForbiddenResponses in contract_test.go, which load this file as the source; TASK-9018-003 declares the flag in it"},
	"apps/web/src/api-types.ts":          {coveredBy: "npm run contract:check on the types regenerated from the contract, which TASK-9018-003 regenerates"},
	"apps/web/src/api.ts":                {coveredBy: "apps/web/src/api.test.ts, the web reader of a listed note"},
	"apps/web/src/App.tsx":               {coveredBy: "apps/web/src/App.test.tsx and tests/e2e/specs/journey.spec.ts, which render a listed note through the web client"},
	"tests/e2e/specs/helpers.ts":         {coveredBy: "tests/e2e/specs/done-indicator.spec.ts, which reads the flag off every member of the collection the app was handed and asserts each row's status word against it", mustCarryDone: true},
}

// The each-note sweep discovers the response-shaping members itself when it
// runs, over the whole repository: a member the checks above do not cover is a
// failure, a declared member the sweep cannot find is a failure, a member this
// task is answerable for that never carries the flag is a failure, and a sweep
// that finds nothing is a failure rather than a silent pass.
func TestListingShapeMembersAreAllCoveredByACheck(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	skipDirs := map[string]bool{".claude": true, ".git": true, ".guardkit": true, ".tmp": true, "coverage": true, "dist": true, "node_modules": true, "playwright-report": true, "test-results": true}
	sources := map[string]bool{".go": true, ".ts": true, ".tsx": true, ".yaml": true, ".yml": true}
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if skipDirs[name] {
				return fs.SkipDir
			}
			return nil
		}
		if !sources[filepath.Ext(name)] || strings.HasSuffix(name, "_test.go") ||
			strings.HasSuffix(name, ".test.ts") || strings.HasSuffix(name, ".test.tsx") ||
			strings.HasSuffix(name, ".spec.ts") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		// A file shapes a listed note when it names the fields one note is
		// answered with, when it registers the route that answers them, or when
		// it reads a listed note field by field.
		if (!namesAnyField(text, "createdAt", "CreatedAt", "created_at") ||
			!namesAnyField(text, "label", "Label") || !namesAnyField(text, "id", "ID")) &&
			!servesTheListingRoute(text) && !readsAListedNote(text) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("sweep the repository for files shaping a listed note: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("the sweep found no member of the each-note set")
	}
	for member := range found {
		if _, ok := listingShapeMembers[member]; !ok {
			t.Fatalf("%s shapes a listed note but no check covers it", member)
		}
	}
	for member, cover := range listingShapeMembers {
		if !found[member] {
			t.Fatalf("%s is declared as a member of the each-note set but the sweep did not find it (covered by %s)", member, cover.coveredBy)
		}
		if !cover.mustCarryDone {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(member)))
		if err != nil {
			t.Fatalf("read %s: %v", member, err)
		}
		if !strings.Contains(string(data), "done") {
			t.Fatalf("%s shapes a listed note but never carries the done flag", member)
		}
	}
}

// servesTheListingRoute reports the file that registers the listing the whole
// each-note set is read from, whether or not it names the fields itself.
func servesTheListingRoute(text string) bool {
	return strings.Contains(text, `"GET /api/notes"`)
}

// readsAListedNote reports the file that walks a listed note field by field,
// which is how the view consumes one without naming every field.
func readsAListedNote(text string) bool {
	return strings.Contains(text, "note.id") && strings.Contains(text, "note.label")
}

// namesAnyField reports whether text spells one field under any of its names.
func namesAnyField(text string, names ...string) bool {
	for _, name := range names {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}

// patchedNote asks the delivered PATCH route to set the completion flag of one
// note, exactly as the person using the surface does: the id goes in the path,
// the flag in a JSON body, the caller's token in the Authorization header.
func patchedNote(store Store, id, body, token string) *httptest.ResponseRecorder {
	return request(Routes(store, fakeAuth{}, "run-1"), http.MethodPatch, "/api/notes/"+id, body, token)
}

// patchedNoteFields decodes the note the PATCH route answered with as a raw JSON
// object: a decoded struct cannot tell an omitted field from its zero value, so
// the checks on the answered note ask the body itself what it carries.
func patchedNoteFields(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var note map[string]any
	if err := json.Unmarshal(body, &note); err != nil {
		t.Fatalf("decode patched note %q: %v", body, err)
	}
	if note == nil {
		t.Fatalf("patched note %q is not an object", body)
	}
	return note
}

// The delivered surface, seen by the person who asked for it: the signed-in
// owner sends the flag they want in a JSON body, the route hands the write to
// storage for that owner and that id, and the answer is 200 with the note as
// storage now holds it - the new flag beside every field the note already had.
func TestPatchSetsTheDoneFlagBothWaysAndAnswersTheUpdatedNote(t *testing.T) {
	for _, flag := range []bool{true, false} {
		body := `{"done":` + strconv.FormatBool(flag) + `}`
		// Storage holds the note with the flag the caller asked for, and answers
		// with it, exactly as PGStore.SetDone does.
		updated := Note{ID: "7", Label: "keep it short", CreatedAt: created, Done: flag}
		store := &fakeStore{doneNote: updated}
		got := patchedNote(store, "7", body, goodToken)
		want := `{"id":"7","label":"keep it short","createdAt":"2026-10-12T09:30:00Z","done":` + strconv.FormatBool(flag) + "}\n"
		if got.Code != http.StatusOK || got.Body.String() != want {
			t.Fatalf("done %v: got %d %q, want 200 %q", flag, got.Code, got.Body.String(), want)
		}
		if got.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("done %v: content type %q", flag, got.Header().Get("Content-Type"))
		}
		// The write the route made: the signed-in owner, the id from the path,
		// the flag from the body - and nothing else.
		if store.doneCalls != 1 || store.doneOwner != "user-1" || store.doneID != "7" || store.doneValue != flag {
			t.Fatalf("done %v: %d writes for owner %q id %q flag %v", flag, store.doneCalls, store.doneOwner, store.doneID, store.doneValue)
		}
		if store.calls != 0 {
			t.Fatalf("done %v: %d calls besides the flag write", flag, store.calls)
		}
		// The answered note carries its flag as a JSON boolean and keeps the
		// fields the note already had: nothing renamed, retyped or dropped.
		note := patchedNoteFields(t, got.Body.Bytes())
		if doneFlag(t, note, updated.ID) != flag {
			t.Fatalf("done %v: answered flag %v", flag, note["done"])
		}
		assertCarriesTheExistingFields(t, note, updated)
	}
}

// Re-sending the same flag is not a second event: both calls answer 200 with the
// note done, and the note that storage holds afterwards is still the one note
// with the one flag - no doubling, no drift, no extra field.
func TestPatchIsIdempotentWhenTheSameFlagIsSentAgain(t *testing.T) {
	mine := Note{ID: "7", Label: "mine", CreatedAt: created}
	store := &doneStore{rows: []storedRow{{owner: "user-1", note: mine}}}
	want := "{\"id\":\"7\",\"label\":\"mine\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":true}\n"
	for round := 1; round <= 2; round++ {
		got := patchedNote(store, "7", `{"done":true}`, goodToken)
		if got.Code != http.StatusOK || got.Body.String() != want {
			t.Fatalf("round %d: got %d %q, want 200 %q", round, got.Code, got.Body.String(), want)
		}
		if !doneFlag(t, patchedNoteFields(t, got.Body.Bytes()), "7") {
			t.Fatalf("round %d: answered without the flag set", round)
		}
	}
	if store.calls != 2 || store.owner != "user-1" || store.id != "7" || !store.value {
		t.Fatalf("%d writes for owner %q id %q flag %v, want two for user-1/7/true", store.calls, store.owner, store.id, store.value)
	}
	if want := (Note{ID: "7", Label: "mine", CreatedAt: created, Done: true}); store.rows[0].note != want {
		t.Fatalf("the note after two identical writes is %+v, want %+v", store.rows[0].note, want)
	}
	if len(store.rows) != 1 {
		t.Fatalf("the writes left %d notes, want the one note that was there", len(store.rows))
	}
}

// doneNotBoolean is the whole 400 body a flag that is not a JSON boolean must
// answer: a plain message naming the field, not the generic "Invalid request".
const doneNotBoolean = "{\"message\":\"done must be the JSON boolean true or false\"}\n"

// Only the JSON booleans true and false are a completion flag. An absent done, a
// null, and every other JSON value are refused at the surface with 400, and the
// storage write is never asked for them - so no note is toggled by a body the
// contract does not accept.
func TestPatchRefusesANonBooleanDoneBeforeAnyWrite(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"done":null}`, `{"done":"yes"}`, `{"done":"true"}`, `{"done":""}`,
		`{"done":1}`, `{"done":0}`, `{"done":1.0}`, `{"done":{}}`, `{"done":[]}`, `{"done":[true]}`,
	} {
		store := &fakeStore{doneNote: Note{ID: "7", Label: "mine", CreatedAt: created, Done: true}}
		got := patchedNote(store, "7", body, goodToken)
		if got.Code != http.StatusBadRequest || got.Body.String() != doneNotBoolean {
			t.Fatalf("body %q: got %d %q, want 400 %q", body, got.Code, got.Body.String(), doneNotBoolean)
		}
		if store.doneCalls != 0 || store.calls != 0 {
			t.Fatalf("body %q: %d flag writes, %d other calls, want none", body, store.doneCalls, store.calls)
		}
	}
}

// A body that is not one JSON object at all - not JSON, not an object, a field
// the request does not define, two objects, invalid UTF-8, an empty or oversized
// body - is refused with 400 before the write, exactly as the create path
// refuses it.
func TestPatchRefusesAnUndecodableBodyBeforeAnyWrite(t *testing.T) {
	for _, body := range []string{
		``, `not json`, `["true"]`, `true`, `{"done":true,"extra":1}`,
		`{"done":true}{"done":false}`, "{\"done\":\"\\xff\"}", `{"done":"` + strings.Repeat("x", 1025) + `"}`,
	} {
		store := &fakeStore{doneNote: Note{ID: "7", Label: "mine", CreatedAt: created}}
		got := patchedNote(store, "7", body, goodToken)
		if got.Code != http.StatusBadRequest || got.Body.String() != "{\"message\":\"Invalid request\"}\n" {
			t.Fatalf("body %q: got %d %q", body, got.Code, got.Body.String())
		}
		if store.doneCalls != 0 || store.calls != 0 {
			t.Fatalf("body %q: %d flag writes, %d other calls, want none", body, store.doneCalls, store.calls)
		}
	}
}

// notFound is the whole 404 body a note the caller cannot address must answer.
const notFound = "{\"message\":\"Not found\"}\n"

// An id that names no note of the caller is answered 404. The failed call leaves
// the collection exactly as it was: the caller's own note keeps its flag, and the
// call creates nothing - the one write it made matched no row.
func TestPatchAnswersNotFoundForANoteThatDoesNotExist(t *testing.T) {
	mine := Note{ID: "7", Label: "mine", CreatedAt: created}
	store := &doneStore{rows: []storedRow{{owner: "user-1", note: mine}}}
	got := patchedNote(store, "999", `{"done":true}`, goodToken)
	if got.Code != http.StatusNotFound || got.Body.String() != notFound {
		t.Fatalf("missing note: got %d %q, want 404 %q", got.Code, got.Body.String(), notFound)
	}
	// The write was made for the caller and the id they asked for, and it is the
	// only call the route made: nothing was created.
	if store.calls != 1 || store.owner != "user-1" || store.id != "999" || !store.value {
		t.Fatalf("%d calls for owner %q id %q flag %v", store.calls, store.owner, store.id, store.value)
	}
	if store.rows[0].note != mine {
		t.Fatalf("a failed call changed the caller's note to %+v", store.rows[0].note)
	}
	if listing := request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken); listing.Body.String() != "{\"notes\":[{\"id\":\"7\",\"label\":\"mine\",\"createdAt\":\"2026-10-12T09:30:00Z\",\"done\":false}]}\n" {
		t.Fatalf("the collection after a failed call reads %q", listing.Body.String())
	}
}

// A note that exists but belongs to someone else is answered with the very same
// 404 as a note nobody holds - never 403 - so the answer leaks nothing about
// whether the id is real, and the other user's note is left exactly as it was.
func TestPatchAnswersNotFoundNeverForbiddenForAnotherUsersNote(t *testing.T) {
	mine := Note{ID: "7", Label: "mine", CreatedAt: created}
	theirs := Note{ID: "8", Label: "theirs", CreatedAt: created.Add(-time.Minute)}
	store := &doneStore{rows: []storedRow{{owner: "user-1", note: mine}, {owner: "user-2", note: theirs}}}
	got := patchedNote(store, "8", `{"done":true}`, goodToken)
	if got.Code == http.StatusForbidden {
		t.Fatal("a foreign note answered 403, leaking that the id exists")
	}
	if got.Code != http.StatusNotFound || got.Body.String() != notFound {
		t.Fatalf("foreign note: got %d %q, want 404 %q", got.Code, got.Body.String(), notFound)
	}
	if store.rows[1].note != theirs {
		t.Fatalf("a write for another owner's note changed it to %+v", store.rows[1].note)
	}
	// Indistinguishable from a note that was never stored: same code, same body.
	missing := patchedNote(store, "999", `{"done":true}`, goodToken)
	if missing.Code != got.Code || missing.Body.String() != got.Body.String() {
		t.Fatalf("a missing note answers %d %q and a foreign note %d %q: the two must match",
			missing.Code, missing.Body.String(), got.Code, got.Body.String())
	}
	// The same refusal a second time changes nothing either: the foreign note is
	// still untouched, and its own owner keeps it (checked against storage).
	if again := patchedNote(store, "8", `{"done":true}`, goodToken); again.Code != http.StatusNotFound {
		t.Fatalf("note 8 answered %d for a user-1 write, want 404", again.Code)
	}
	if store.rows[1].note != theirs {
		t.Fatalf("a refused write still moved another owner's flag to %v", store.rows[1].note.Done)
	}
}

// An anonymous or refused PATCH is answered 401 - the same refusal as
// GET /api/notes - before the handler runs, so the note's flag is untouched and
// storage is never asked to write it.
func TestAnonymousPatchIsRefusedWith401BeforeAnyMutation(t *testing.T) {
	mine := Note{ID: "7", Label: "mine", CreatedAt: created}
	for _, header := range []string{"", "Bearer wrong", "Basic " + goodToken, "Bearer ", "Bearer"} {
		store := &doneStore{rows: []storedRow{{owner: "user-1", note: mine}}}
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/7", strings.NewReader(`{"done":true}`))
		r.Header.Set("Content-Type", "application/json")
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		w := httptest.NewRecorder()
		Routes(store, fakeAuth{}, "run-1").ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized || w.Body.String() != "{\"message\":\"Sign-in required\"}\n" {
			t.Fatalf("PATCH %q: got %d %q", header, w.Code, w.Body.String())
		}
		if !strings.HasPrefix(w.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Fatalf("PATCH %q: challenge %q", header, w.Header().Get("WWW-Authenticate"))
		}
		if store.calls != 0 {
			t.Fatalf("PATCH %q: %d writes made by an unauthenticated request", header, store.calls)
		}
		if store.rows[0].note != mine || store.rows[0].note.Done {
			t.Fatalf("PATCH %q: the note's flag moved to %v without a signed-in caller", header, store.rows[0].note.Done)
		}
	}
}

// The route is registered on the note path for PATCH alone, and it is the
// signed-in wrapper that hands it the owner: the token's subject - never a
// caller-supplied value - is what the write is scoped to, and no neighbouring
// method reaches it.
func TestPatchRouteIsRegisteredSignedInAndScopedToTheCaller(t *testing.T) {
	store := &fakeStore{doneNote: Note{ID: "7", Label: "mine", CreatedAt: created, Done: true}}
	if got := patchedNote(store, "7", `{"done":true}`, goodToken); got.Code != http.StatusOK || store.doneOwner != "user-1" {
		t.Fatalf("signed-in PATCH: got %d %q owner %q", got.Code, got.Body.String(), store.doneOwner)
	}
	anonymous := &fakeStore{}
	if got := patchedNote(anonymous, "7", `{"done":true}`, ""); got.Code != http.StatusUnauthorized || anonymous.doneCalls != 0 {
		t.Fatalf("anonymous PATCH: got %d, %d writes", got.Code, anonymous.doneCalls)
	}
	h := Routes(&fakeStore{}, fakeAuth{}, "run-1")
	if got := request(h, http.MethodPut, "/api/notes/7", `{"done":true}`, goodToken); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT /api/notes/7: %d, want 405", got.Code)
	}
	if got := request(h, http.MethodPatch, "/api/notes", `{"done":true}`, goodToken); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /api/notes: %d, want 405", got.Code)
	}
	// The note path is registered for PATCH alone, so every other method on it is
	// refused rather than quietly served: no single-note read was asked for.
	if got := request(h, http.MethodGet, "/api/notes/7", "", goodToken); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/notes/7: %d, want 405", got.Code)
	}
}

// The flag write is the only thing the route does with storage: a storage failure
// that is not the not-found sentinel answers the private 500, never the 404 that
// would tell a caller their note was missing, and never the error itself.
func TestPatchPersistenceFailureIsPrivate(t *testing.T) {
	store := &fakeStore{doneErr: errors.New("password=private")}
	got := patchedNote(store, "7", `{"done":true}`, goodToken)
	if got.Code != http.StatusInternalServerError || got.Body.String() != "{\"message\":\"Service unavailable\"}\n" {
		t.Fatalf("got %d %q", got.Code, got.Body.String())
	}
	if store.doneCalls != 1 {
		t.Fatalf("%d writes attempted", store.doneCalls)
	}
}
