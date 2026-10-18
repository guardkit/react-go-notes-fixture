package notes

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// contractDocument loads the contract the web types are generated from and
// returns it validated, so a test can read the declaration itself and not only
// the validator's verdict on one recorded body.
func contractDocument(t *testing.T) *openapi3.T {
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

// componentSchema returns one component schema of the contract, failing the
// test when the contract stopped declaring the schema at all.
func componentSchema(t *testing.T, doc *openapi3.T, name string) *openapi3.Schema {
	t.Helper()
	if doc.Components == nil {
		t.Fatal("contract declares no components")
	}
	ref, ok := doc.Components.Schemas[name]
	if !ok || ref == nil || ref.Value == nil {
		t.Fatalf("contract declares no %s schema", name)
	}
	return ref.Value
}

// The contract is the one place the done flag is declared - the web note type
// only ever agrees with it by regeneration - so the declaration itself is
// pinned here: a boolean named done, required of every note, next to the three
// fields a note already had to carry.
func TestContractDeclaresDoneAsARequiredBooleanOnNote(t *testing.T) {
	note := componentSchema(t, contractDocument(t), "Note")

	done, ok := note.Properties["done"]
	if !ok || done == nil || done.Value == nil {
		t.Fatalf("Note schema declares no done property, only %v", slices.Sorted(maps.Keys(note.Properties)))
	}
	if done.Value.Type == nil || !done.Value.Type.Is("boolean") {
		t.Fatalf("Note.done is typed %v, want the boolean the flag carries", done.Value.Type)
	}
	if done.Value.Nullable {
		t.Fatal("Note.done is nullable, but every note answers a boolean and never a null")
	}
	for _, field := range []string{"id", "label", "createdAt", "done"} {
		if !slices.Contains(note.Required, field) {
			t.Fatalf("Note.required %v does not list %s, so a note could be answered without it", note.Required, field)
		}
	}
}

// The flag belongs to the note the API answers and to nothing else: the caller
// still sends only a label (the read slice stays read-only, ASSUM-003), and the
// list envelope keeps its single notes field rather than gaining a flag of its
// own beside the notes it carries.
func TestContractKeepsTheDoneFlagOffTheWritePathAndTheEnvelope(t *testing.T) {
	doc := contractDocument(t)

	newNote := componentSchema(t, doc, "NewNote")
	if _, ok := newNote.Properties["done"]; ok {
		t.Fatal("NewNote declares a done property, but the caller cannot set the flag")
	}
	if !slices.Equal(newNote.Required, []string{"label"}) {
		t.Fatalf("NewNote.required is %v, want the label alone", newNote.Required)
	}

	list := componentSchema(t, doc, "NoteList")
	if _, ok := list.Properties["done"]; ok {
		t.Fatal("NoteList declares a done property; the flag belongs to each note, not the envelope")
	}
	notes, ok := list.Properties["notes"]
	if !ok || notes == nil || notes.Value == nil || notes.Value.Items == nil || notes.Value.Items.Value == nil {
		t.Fatal("NoteList.notes no longer carries the notes it lists")
	}
	if _, ok := notes.Value.Items.Value.Properties["done"]; !ok {
		t.Fatal("the notes list no longer carries notes with a done flag")
	}
}

// The validator bites on the newly required flag: a note answered without done,
// or with a done that is not a boolean, is refused on both note responses - the
// same refusal the contract already made of a note without createdAt.
func TestContractRefusesNoteResponsesWithoutABooleanDone(t *testing.T) {
	router := contractRouter(t)
	jsonHeader := http.Header{"Content-Type": []string{"application/json"}}
	cases := []struct {
		name, method, path, body string
		status                   int
	}{
		{"list note without done", http.MethodGet, "/api/notes", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z"}]}`, 200},
		{"list note with a null done", http.MethodGet, "/api/notes", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":null}]}`, 200},
		{"list note with a string done", http.MethodGet, "/api/notes", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":"false"}]}`, 200},
		{"list note with a number done", http.MethodGet, "/api/notes", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":0}]}`, 200},
		{"created note without done", http.MethodPost, "/api/notes", `{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z"}`, 201},
		{"created note with a null done", http.MethodPost, "/api/notes", `{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":null}`, 201},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if err := checkResponse(router, r, tc.status, jsonHeader, []byte(tc.body)); err == nil {
			t.Fatalf("%s: contract accepted a note response without a boolean done", tc.name)
		}
	}
}

// Both states the flag carries are contract-conformant, on its own and side by
// side in one list: required means the field is always present, never that a
// completed note is filtered out of the response.
func TestContractAcceptsNotesInBothDoneStates(t *testing.T) {
	router := contractRouter(t)
	jsonHeader := http.Header{"Content-Type": []string{"application/json"}}
	cases := []struct {
		name, method, path, body string
		status                   int
	}{
		{"not-done note in a list", http.MethodGet, "/api/notes", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":false}]}`, 200},
		{"done note in a list", http.MethodGet, "/api/notes", `{"notes":[{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":true}]}`, 200},
		{"both states in one list", http.MethodGet, "/api/notes", `{"notes":[{"id":"2","label":"done","createdAt":"2026-10-12T09:30:00Z","done":true},{"id":"1","label":"open","createdAt":"2026-10-12T09:00:00Z","done":false}]}`, 200},
		{"created note not done", http.MethodPost, "/api/notes", `{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":false}`, 201},
		{"created note done", http.MethodPost, "/api/notes", `{"id":"1","label":"x","createdAt":"2026-10-12T09:30:00Z","done":true}`, 201},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if err := checkResponse(router, r, tc.status, jsonHeader, []byte(tc.body)); err != nil {
			t.Fatalf("%s: contract refused a conformant note response: %v", tc.name, err)
		}
	}
}
