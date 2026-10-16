package notes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	want := "{\"id\":\"9007199254740993\",\"label\":\"hello\",\"createdAt\":\"2026-10-12T09:30:00Z\"}\n"
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

func TestListIsScopedToTheSignedInOwner(t *testing.T) {
	store := &fakeStore{list: []Note{{ID: "2", Label: "second", CreatedAt: created}}}
	got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken)
	want := "{\"notes\":[{\"id\":\"2\",\"label\":\"second\",\"createdAt\":\"2026-10-12T09:30:00Z\"}]}\n"
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
