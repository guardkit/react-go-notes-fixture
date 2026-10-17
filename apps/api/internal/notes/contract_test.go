package notes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// The contract is the source file the web types are generated from.
var contractPath = filepath.Join("..", "..", "..", "..", "contract", "openapi.yaml")

func contractDoc(t *testing.T) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(contractPath)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("contract is not valid OpenAPI: %v", err)
	}
	return doc
}

func contractRouter(t *testing.T) routers.Router {
	t.Helper()
	router, err := legacy.NewRouter(contractDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

// checkResponse validates one recorded response against the contract's
// operation for the request; an undocumented status or body is an error.
func checkResponse(router routers.Router, r *http.Request, status int, header http.Header, body []byte) error {
	route, params, err := router.FindRoute(r)
	if err != nil {
		return err
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route},
		Status:                 status,
		Header:                 header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	return openapi3filter.ValidateResponse(context.Background(), input)
}

func TestImplementedResponsesMatchTheContract(t *testing.T) {
	router := contractRouter(t)
	note := Note{ID: "42", Label: "hello", CreatedAt: created}
	cases := []struct {
		name, method, path, body, token string
		store                           *fakeStore
		want                            int
	}{
		{"health", http.MethodGet, "/api/health", "", "", &fakeStore{}, 200},
		{"list", http.MethodGet, "/api/notes", "", goodToken, &fakeStore{list: []Note{note}}, 200},
		// A stored done note is listed with its completion flag: the contract
		// must accept the flag the endpoint answers with, beside the fields it
		// already declared.
		{"list done note", http.MethodGet, "/api/notes", "", goodToken, &fakeStore{list: []Note{note, {ID: "43", Label: "finished", CreatedAt: created, Done: true}}}, 200},
		{"empty list", http.MethodGet, "/api/notes", "", goodToken, &fakeStore{}, 200},
		{"list anonymous", http.MethodGet, "/api/notes", "", "", &fakeStore{}, 401},
		{"list refused token", http.MethodGet, "/api/notes", "", "expired", &fakeStore{}, 401},
		{"list failure", http.MethodGet, "/api/notes", "", goodToken, &fakeStore{err: errors.New("down")}, 500},
		{"create", http.MethodPost, "/api/notes", `{"label":"hello"}`, goodToken, &fakeStore{note: note}, 201},
		{"create invalid", http.MethodPost, "/api/notes", `{"label":""}`, goodToken, &fakeStore{}, 400},
		// One code point over the enforced title limit (maxLabelChars, so 201
		// characters today): the refusal must be a contract-conformant Error
		// body, not merely a 400 status.
		{"create title too long", http.MethodPost, "/api/notes", `{"label":"` + strings.Repeat("x", maxLabelChars+1) + `"}`, goodToken, &fakeStore{}, 400},
		{"create anonymous", http.MethodPost, "/api/notes", `{"label":"x"}`, "", &fakeStore{}, 401},
		{"create failure", http.MethodPost, "/api/notes", `{"label":"x"}`, goodToken, &fakeStore{err: errors.New("down")}, 500},
	}
	for _, tc := range cases {
		got := request(Routes(tc.store, fakeAuth{}, "run-1"), tc.method, tc.path, tc.body, tc.token)
		if got.Code != tc.want {
			t.Fatalf("%s: status %d, want %d", tc.name, got.Code, tc.want)
		}
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if err := checkResponse(router, r, got.Code, got.Header(), got.Body.Bytes()); err != nil {
			t.Fatalf("%s: response breaks the contract: %v", tc.name, err)
		}
	}
}

// The validator must bite: responses the contract does not allow are refused.
func TestContractRefusesForbiddenResponses(t *testing.T) {
	router := contractRouter(t)
	jsonHeader := http.Header{"Content-Type": []string{"application/json"}}
	challenge := http.Header{"Content-Type": []string{"application/json"}, "Www-Authenticate": []string{"Bearer"}}
	cases := []struct {
		name, method, path string
		status             int
		header             http.Header
		body               string
	}{
		{"numeric id", http.MethodPost, "/api/notes", 201, jsonHeader, `{"id":42,"label":"x","createdAt":"2026-10-12T09:30:00Z"}`},
		{"undocumented field", http.MethodPost, "/api/notes", 201, jsonHeader, `{"id":"42","label":"x","createdAt":"2026-10-12T09:30:00Z","owner":"u"}`},
		{"missing createdAt", http.MethodGet, "/api/notes", 200, jsonHeader, `{"notes":[{"id":"1","label":"x"}]}`},
		// done is a required boolean of the note object: the listing is refused
		// when the flag is absent, null or not a boolean at all. A completion
		// state the caller cannot read as true or false is not a flag, and a
		// note answered without one is the live response drifting from the
		// contract the web types are generated from.
		{"missing done", http.MethodGet, "/api/notes", 200, jsonHeader, `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z"}]}`},
		{"null done", http.MethodGet, "/api/notes", 200, jsonHeader, `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":null}]}`},
		{"done is not a boolean", http.MethodGet, "/api/notes", 200, jsonHeader, `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":"yes"}]}`},
		{"bare array", http.MethodGet, "/api/notes", 200, jsonHeader, `[]`},
		{"undocumented status", http.MethodGet, "/api/notes", 403, jsonHeader, `{"message":"no"}`},
		{"401 without challenge", http.MethodGet, "/api/notes", 401, jsonHeader, `{"message":"Sign-in required"}`},
		{"health without run", http.MethodGet, "/api/health", 200, jsonHeader, `{"status":"ok"}`},
		{"wrong body on 401", http.MethodGet, "/api/notes", 401, challenge, `{"error":"x"}`},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if err := checkResponse(router, r, tc.status, tc.header, []byte(tc.body)); err == nil {
			t.Fatalf("%s: contract accepted a forbidden response", tc.name)
		}
	}
}

// The done refusals must bite on the flag itself, not on some unrelated rule:
// each one is refused for the note object's done property.
func TestContractDoneRefusalsNameTheFlag(t *testing.T) {
	router := contractRouter(t)
	jsonHeader := http.Header{"Content-Type": []string{"application/json"}}
	cases := []struct{ name, body string }{
		{"missing done", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z"}]}`},
		{"null done", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":null}]}`},
		{"done is not a boolean", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":"yes"}]}`},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
		err := checkResponse(router, r, 200, jsonHeader, []byte(tc.body))
		if err == nil {
			t.Fatalf("%s: contract accepted a forbidden response", tc.name)
		}
		if !strings.Contains(err.Error(), "done") {
			t.Fatalf("%s: refusal does not name the flag: %v", tc.name, err)
		}
	}
}

// The contract is the source the web types are generated from, so the note
// object's declared fields are pinned here: done is a required boolean, and
// declaring it removed none of the fields the listing already carried.
func TestContractNoteObjectDeclaresTheDoneFlag(t *testing.T) {
	doc := contractDoc(t)
	ref, ok := doc.Components.Schemas["Note"]
	if !ok || ref == nil || ref.Value == nil {
		t.Fatal("contract declares no Note schema")
	}
	note := ref.Value
	required := map[string]bool{}
	for _, name := range note.Required {
		required[name] = true
	}
	for _, name := range []string{"id", "label", "createdAt", "done"} {
		if !required[name] {
			t.Errorf("Note does not require %q", name)
		}
	}
	done, ok := note.Properties["done"]
	if !ok || done == nil || done.Value == nil {
		t.Fatal("Note declares no done property")
	}
	if !done.Value.Type.Is("boolean") {
		t.Errorf("Note.done is %v, want boolean", done.Value.Type)
	}
	if done.Value.Nullable {
		t.Error("Note.done is nullable: a blank completion state is not a flag")
	}
	for _, want := range []struct{ name, typ string }{
		{"id", "string"},
		{"label", "string"},
		{"createdAt", "string"},
	} {
		field, ok := note.Properties[want.name]
		if !ok || field == nil || field.Value == nil {
			t.Errorf("declaring the done flag removed Note.%s", want.name)
			continue
		}
		if !field.Value.Type.Is(want.typ) {
			t.Errorf("Note.%s is %v, want %s", want.name, field.Value.Type, want.typ)
		}
	}
	// The listing's items are this same note object, so the required flag bites
	// on GET /api/notes and not only on the note the create endpoint answers with.
	list, ok := doc.Components.Schemas["NoteList"]
	if !ok || list == nil || list.Value == nil {
		t.Fatal("contract declares no NoteList schema")
	}
	notes, ok := list.Value.Properties["notes"]
	if !ok || notes == nil || notes.Value == nil || notes.Value.Items == nil {
		t.Fatal("contract declares no notes array in NoteList")
	}
	if !notes.Value.Type.Is("array") {
		t.Errorf("NoteList.notes is %v, want array", notes.Value.Type)
	}
	if notes.Value.Items.Value != note {
		t.Error("the notes listing's items are not the Note schema")
	}
}

// The drift the contract exists to catch is the live response moving away from
// the declared shape, so this starts from what the handler actually answers and
// removes only the flag: the untouched listing is accepted, the same listing
// without the flag is refused. A required boolean that nothing outside the
// hand-written tables bites on would not catch it.
func TestContractRefusesTheLiveListingOnceTheDoneFlagDriftsAway(t *testing.T) {
	router := contractRouter(t)
	store := &fakeStore{list: []Note{
		{ID: "42", Label: "hello", CreatedAt: created},
		{ID: "43", Label: "finished", CreatedAt: created, Done: true},
	}}
	got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodGet, "/api/notes", "", goodToken)
	if got.Code != http.StatusOK {
		t.Fatalf("live listing status %d, want 200", got.Code)
	}
	if err := checkResponse(router, liveRequest(), got.Code, got.Header(), got.Body.Bytes()); err != nil {
		t.Fatalf("the live listing breaks the contract: %v", err)
	}
	drifted := withoutDoneFlag(t, got.Body.Bytes())
	if err := checkResponse(router, liveRequest(), got.Code, got.Header(), drifted); err == nil {
		t.Fatal("contract accepted a live listing that dropped the done flag")
	}
}

func liveRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/api/notes", nil)
}

// withoutDoneFlag returns the listing body with every note's done key removed,
// leaving the fields the contract already demanded untouched.
func withoutDoneFlag(t *testing.T, body []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("live listing is not JSON: %v", err)
	}
	notes, ok := doc["notes"].([]any)
	if !ok || len(notes) == 0 {
		t.Fatalf("live listing carries no notes array: %s", body)
	}
	for _, entry := range notes {
		note, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("listing entry is not an object: %v", entry)
		}
		if _, ok := note["done"]; !ok {
			t.Fatalf("the live listing already omits the done flag: %s", body)
		}
		delete(note, "done")
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode drifted listing: %v", err)
	}
	return encoded
}
