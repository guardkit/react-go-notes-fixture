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
		`{}`,
		`{"label":"   "}`,
		`{"label":"ok","extra":1}`,
		`{"label":"ok"}{"label":"again"}`,
		`["ok"]`,
		`{"label":123}`,
		"{\"label\":\"\xff\"}",
		`{"label":"` + strings.Repeat("x", 81) + `"}`,
		`{"label":"` + strings.Repeat("x", 1025) + `"}`,
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
