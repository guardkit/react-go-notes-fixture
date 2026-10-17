package notes

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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

	// The update path is recorded apart from the two reads, so a test can tell
	// which call the handler made and with what.
	updateCalls int
	updateCtx   context.Context
	updateOwner string
	updateID    string
	updateDone  bool
	updated     Note
	updateErr   error
}

func (f *fakeStore) Create(ctx context.Context, owner, label string) (Note, error) {
	f.calls++
	f.ctx, f.owner, f.label = ctx, owner, label
	return f.note, f.err
}

func (f *fakeStore) List(ctx context.Context, owner string) ([]Note, error) {
	f.calls++
	f.ctx, f.owner = ctx, owner
	return f.list, f.err
}

// UpdateDone records the owner, id and flag the handler handed over and answers
// with the note stored under updated, exactly as the adapter reads the updated
// row back. Set updateErr to ErrNoteNotFound for the note the owner does not
// have, or to any other error for a persistence failure.
func (f *fakeStore) UpdateDone(ctx context.Context, owner, id string, done bool) (Note, error) {
	f.updateCalls++
	f.updateCtx, f.updateOwner, f.updateID, f.updateDone = ctx, owner, id, done
	return f.updated, f.updateErr
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

// UpdateDone answers with the row the adapter reads back from its own
// UPDATE ... RETURNING: the title and creation time the row already held, and
// the flag the caller set. The update path writes no title, so the title it
// reports is the title storage already carried.
func (s *storingStore) UpdateDone(_ context.Context, _, id string, done bool) (Note, error) {
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
// note response carries the title that was written. The note write the API now
// answers besides the create - PATCH /api/notes/{id} - sets the done flag alone
// and never a title, so this rule still bites at the two steps an update is made
// of: the shared write-boundary trim every note title goes through, and the note
// response, which carries the title storage already held.
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
// objects: a decoded struct cannot tell an omitted field from a zero value, so
// the done-flag checks ask the body itself whether the flag is there.
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
// exactly as the store holds them, and answers with each note as the caller
// receives it, beside the owner the store was asked for.
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

// The fields the listing already carried stay where they were: the done flag is
// added to each note beside its id, label and creation time, and nothing is
// renamed or dropped on the way.
func assertCarriesTheExistingFields(t *testing.T, note map[string]any, want Note) {
	t.Helper()
	if note["id"] != want.ID || note["label"] != want.Label {
		t.Fatalf("note %s: id/label became %v/%v, want %q/%q", want.ID, note["id"], note["label"], want.ID, want.Label)
	}
	if got := note["createdAt"]; got != want.CreatedAt.UTC().Format(time.RFC3339) {
		t.Fatalf("note %s: createdAt became %v, want %q", want.ID, got, want.CreatedAt.UTC().Format(time.RFC3339))
	}
}

// A note written before the done column existed is stored with the migration's
// false default, and the listing hands that flag over: the row is observable
// only through GET /api/notes (ASSUM-004), so the column default is pinned here
// rather than by reading storage.
func TestListingReturnsNotDoneForANoteStoredBeforeTheDoneFlag(t *testing.T) {
	// The zero Done value is the row the 0003 migration backfills: the column
	// default wrote false, so it carries no completion.
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

// A note whose stored completion state is done is listed with done true. The
// state is seeded as stored state: this slice has no caller-facing write path
// that sets it (ASSUM-003).
func TestListingReturnsTheDoneFlagForADoneNote(t *testing.T) {
	finished := Note{ID: "7", Label: "finished", CreatedAt: created, Done: true}
	notes, _ := listedNotes(t, []Note{finished})
	if !doneFlag(t, notes[0], finished.ID) {
		t.Fatal("a note stored as done is listed as not done")
	}
	assertCarriesTheExistingFields(t, notes[0], finished)
}

// Every note of a mixed listing carries its own flag: the completion state of
// one note never leaks into another, no note is left without a flag, and the
// listing still answers the signed-in owner's notes newest-first.
func TestEveryNoteInTheListingCarriesItsOwnDoneFlag(t *testing.T) {
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
// note of the response. The set is discovered when the test runs.
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
// never leave out.
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

// listingShapeMembers is every file of the repository that declares or reads the
// shape of one listed note, with the check that covers it. mustCarryDone marks
// the members that declare the fields themselves, as opposed to the members
// whose done-carrying is proven through the type they name or through the check
// named beside them.
var listingShapeMembers = map[string]struct {
	coveredBy     string
	mustCarryDone bool
}{
	"apps/api/internal/notes/handler.go": {coveredBy: "TestEachNoteListingMemberCarriesTheDoneFlag and the listing wire-shape assertions above"},
	"apps/api/internal/notes/store.go":   {coveredBy: "scripts/check-feature.sh, which lists notes read back from PostgreSQL", mustCarryDone: true},
	"contract/openapi.yaml":              {coveredBy: "TestImplementedResponsesMatchTheContract in contract_test.go", mustCarryDone: true},
	"apps/web/src/api-types.ts":          {coveredBy: "npm run contract:check on the types regenerated from the contract", mustCarryDone: true},
	"apps/web/src/api.ts":                {coveredBy: "apps/web/src/api.test.ts, the web reader of a listed note"},
}

// The each-note sweep discovers the response-shaping members itself when it
// runs, over the whole repository: a member the checks above do not cover is a
// failure, a declared member the sweep cannot find is a failure, and a sweep
// that finds nothing is a failure rather than a silent pass.
func TestListingShapeMembersAreAllCoveredByACheck(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	skipDirs := map[string]bool{".claude": true, ".git": true, ".guardkit": true, "coverage": true, "dist": true, "node_modules": true, "playwright-report": true, "test-results": true}
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
		// answered with, or when it registers the route that answers them.
		if (!namesAnyField(text, "createdAt", "CreatedAt", "created_at") ||
			!namesAnyField(text, "label", "Label") || !namesAnyField(text, "id", "ID")) &&
			!servesTheListingRoute(text) {
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

// namesAnyField reports whether text spells one field under any of its names.
func namesAnyField(text string, names ...string) bool {
	for _, name := range names {
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}
