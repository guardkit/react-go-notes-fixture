package notes

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

// The contract is the one place the done-flag write is declared: the web types
// only ever agree with it by regeneration, and the handler is held to it by the
// validator. These tests read that declaration and exercise the delivered
// handler against it, so the document records the surface as it was delivered -
// the flag alone going in, the whole note coming back, and one 404 that cannot
// be read as a probe of another user's notes.

// patchItem returns the contract's /api/notes/{id} path item: the one place a
// caller may aim a write at a single note.
func patchItem(t *testing.T, doc *openapi3.T) *openapi3.PathItem {
	t.Helper()
	item := doc.Paths.Find("/api/notes/{id}")
	if item == nil {
		t.Fatalf("contract declares no /api/notes/{id} path, only %v", doc.Paths.Keys())
	}
	return item
}

// patchOperation returns the patch operation of that path item.
func patchOperation(t *testing.T, doc *openapi3.T) *openapi3.Operation {
	t.Helper()
	item := patchItem(t, doc)
	if item.Patch == nil {
		t.Fatal("contract declares no patch operation on /api/notes/{id}")
	}
	return item.Patch
}

// jsonContentSchema returns the schema one request body or response carries as
// application/json, failing the test when it documents no readable body at all:
// a note or an error the generated client cannot parse is no answer.
func jsonContentSchema(t *testing.T, content openapi3.Content) *openapi3.SchemaRef {
	t.Helper()
	media, ok := content["application/json"]
	if !ok || media == nil || media.Schema == nil {
		t.Fatalf("documented no application/json body, only %v", slices.Sorted(maps.Keys(content)))
	}
	if media.Schema.Value == nil {
		t.Fatalf("the %s body does not resolve to a schema", media.Schema.Ref)
	}
	return media.Schema
}

// mustResponse returns one documented response of an operation, failing the test
// when the operation stopped documenting that code at all.
func mustResponse(t *testing.T, op *openapi3.Operation, status int) *openapi3.Response {
	t.Helper()
	if op.Responses == nil {
		t.Fatalf("operation %s declares no responses", op.OperationID)
	}
	ref := op.Responses.Status(status)
	if ref == nil || ref.Value == nil {
		t.Fatalf("operation %s declares no %d response", op.OperationID, status)
	}
	return ref.Value
}

// checkPatchRequest asks the contract validator whether a patch body is one the
// operation accepts. The write slice is what this bites on: the flag is the only
// thing a caller may hand over. The security check is answered by the bearer
// scheme the operation names, so a body is judged on its members alone.
func checkPatchRequest(router routers.Router, id, body string) error {
	r := httptest.NewRequest(http.MethodPatch, "/api/notes/"+id, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+goodToken)
	route, params, err := router.FindRoute(r)
	if err != nil {
		return err
	}
	return openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: params,
		Route:      route,
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
			MultiError:            true,
			AuthenticationFunc: func(_ context.Context, input *openapi3filter.AuthenticationInput) error {
				if input.SecuritySchemeName != "bearer" {
					return fmt.Errorf("the update asked for %q, want the bearer scheme every note write carries", input.SecuritySchemeName)
				}
				return nil
			},
		},
	})
}

// The write is declared as delivered: a bearer-only operation on the note named
// by id, handing over the flag and nothing else, answering either the whole note
// or the one 404 - and the sign-in refusal every note operation already gives.
func TestContractDeclaresTheDoneFlagUpdateOperation(t *testing.T) {
	doc := contractDocument(t)
	op := patchOperation(t, doc)

	if op.OperationID != "updateNoteDone" {
		t.Fatalf("patch operationId %q, want updateNoteDone, the name the generated web types index the operation under", op.OperationID)
	}
	if op.Security == nil || len(*op.Security) != 1 {
		t.Fatalf("patch security %v, want the single bearer requirement every note write carries", op.Security)
	}
	if _, ok := (*op.Security)[0]["bearer"]; !ok {
		t.Fatalf("patch security %v does not name bearer, so the write is documented as open to anonymous callers", *op.Security)
	}

	var id *openapi3.Parameter
	for _, ref := range op.Parameters {
		if ref != nil && ref.Value != nil && ref.Value.Name == "id" {
			id = ref.Value
		}
	}
	if id == nil || id.In != "path" || !id.Required {
		t.Fatalf("patch does not declare id as a required path parameter: %v", op.Parameters)
	}

	if op.RequestBody == nil || op.RequestBody.Value == nil || !op.RequestBody.Value.Required {
		t.Fatal("patch declares no required request body, so the flag could be set by a caller who sends none")
	}
	if ref := jsonContentSchema(t, op.RequestBody.Value.Content).Ref; ref != "#/components/schemas/PatchNoteRequest" {
		t.Fatalf("patch body schema %q, want the shared PatchNoteRequest component the web types import", ref)
	}

	returned := jsonContentSchema(t, mustResponse(t, op, http.StatusOK).Content)
	if returned.Ref != "#/components/schemas/Note" {
		t.Fatalf("patch 200 body %q, want the whole Note the read path already answers", returned.Ref)
	}
	note := componentSchema(t, doc, "Note")
	if got, want := slices.Sorted(maps.Keys(returned.Value.Properties)), slices.Sorted(maps.Keys(note.Properties)); !slices.Equal(got, want) {
		t.Fatalf("patch 200 answers %v, want the whole note the client already knows: %v", got, want)
	}
	for _, field := range []string{"id", "label", "createdAt", "done"} {
		if !slices.Contains(returned.Value.Required, field) {
			t.Fatalf("patch 200 requires %v, which does not list %s: the updated note the client refreshes its view from must carry every field", returned.Value.Required, field)
		}
	}
}

// The flag is the whole of the write: the body carries done and nothing beside
// it, so the label stays the create path's property (NewNote, still label-only)
// and the id stays a path value. A body offering anything else is refused by the
// contract itself, before any handler runs.
func TestContractKeepsThePatchWriteSliceToTheDoneFlag(t *testing.T) {
	doc := contractDocument(t)
	op := patchOperation(t, doc)

	body := componentSchema(t, doc, "PatchNoteRequest")
	done, ok := body.Properties["done"]
	if !ok || done == nil || done.Value == nil {
		t.Fatalf("PatchNoteRequest declares no done property, only %v", slices.Sorted(maps.Keys(body.Properties)))
	}
	if done.Value.Type == nil || !done.Value.Type.Is("boolean") {
		t.Fatalf("PatchNoteRequest.done is typed %v, want the boolean the flag carries", done.Value.Type)
	}
	if done.Value.Nullable {
		t.Fatal("PatchNoteRequest.done is nullable, but the flag is a boolean or the request is refused")
	}
	if !slices.Equal(body.Required, []string{"done"}) {
		t.Fatalf("PatchNoteRequest.required is %v, want the done flag alone, so a caller cannot leave it out", body.Required)
	}
	if len(body.Properties) != 1 {
		t.Fatalf("PatchNoteRequest declares %v, want the done flag alone", slices.Sorted(maps.Keys(body.Properties)))
	}
	if !bodyAdditionalPropertiesForbidden(body) {
		t.Fatal("PatchNoteRequest accepts undeclared members, so a label could ride along on the update")
	}

	// The two write slices stay distinct: the collection still takes a label
	// alone, and the single note takes the flag alone.
	createBody := jsonContentSchema(t, mustRequestBody(t, mustOperation(t, doc, "/api/notes", http.MethodPost)).Content)
	if createBody.Ref != "#/components/schemas/NewNote" {
		t.Fatalf("POST /api/notes takes %q, want the label-only NewNote the caller still sends", createBody.Ref)
	}
	if ref := jsonContentSchema(t, op.RequestBody.Value.Content).Ref; ref == createBody.Ref {
		t.Fatal("the note update takes the create body, so the write slices are no longer distinct")
	}
	if patchItem(t, doc).Get != nil || patchItem(t, doc).Post != nil {
		t.Fatal("/api/notes/{id} gained a read or a create; only the update is documented there")
	}
	if item := doc.Paths.Find("/api/notes"); item == nil || item.Patch != nil {
		t.Fatal("the collection gained a patch operation; the flag is written to one note named by id")
	}
}

// The 404 is one answer, not two: the contract gives the missing note and the
// foreign note the same status and the same body, so the document itself cannot
// express which of the two happened.
func TestContractAnswersOneFourOhFourForBothCases(t *testing.T) {
	doc := contractDocument(t)
	op := patchOperation(t, doc)

	notFound := op.Responses.Status(http.StatusNotFound)
	if notFound == nil || notFound.Value == nil {
		t.Fatal("patch declares no 404 response, so a note the caller cannot see has no documented answer")
	}
	if notFound.Ref != "#/components/responses/NotFound" {
		t.Fatalf("patch 404 %q, want the shared NotFound response component", notFound.Ref)
	}
	if _, ok := doc.Components.Responses["NotFound"]; !ok {
		t.Fatal("contract declares no NotFound response component for the unified 404")
	}
	failed := jsonContentSchema(t, notFound.Value.Content)
	if failed.Ref != "#/components/schemas/Error" {
		t.Fatalf("patch 404 body %q, want the plain Error the rest of the API refuses with, so the answer cannot distinguish a foreign note from a missing one", failed.Ref)
	}
	errSchema := componentSchema(t, doc, "Error")
	if !slices.Equal(slices.Sorted(maps.Keys(errSchema.Properties)), []string{"message"}) {
		t.Fatalf("Error declares %v, want the message alone: any other member is a channel for saying more about the note than that the caller cannot see it", slices.Sorted(maps.Keys(errSchema.Properties)))
	}
	if !bodyAdditionalPropertiesForbidden(errSchema) {
		t.Fatal("Error accepts undeclared members, so a 404 could carry an existence hint the contract never forbade")
	}

	// And the handler's two 404s are that one answer, byte for byte.
	router := contractRouter(t)
	bodies := map[string]string{}
	for _, id := range []string{"42", "missing-1", "foreign-1"} {
		store := &fakeStore{err: ErrNoteNotFound}
		got := request(Routes(store, fakeAuth{}, "run-1"), http.MethodPatch, "/api/notes/"+id, `{"done":true}`, goodToken)
		if got.Code != http.StatusNotFound {
			t.Fatalf("note %s: status %d, want 404", id, got.Code)
		}
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/"+id, strings.NewReader(`{"done":true}`))
		if err := checkResponse(router, r, got.Code, got.Header(), got.Body.Bytes()); err != nil {
			t.Fatalf("note %s: the not-found answer breaks the contract: %v", id, err)
		}
		bodies[id] = got.Body.String()
	}
	if bodies["42"] != bodies["missing-1"] || bodies["missing-1"] != bodies["foreign-1"] {
		t.Fatalf("the 404 answers differ per id - %q, %q, %q - so the answer distinguishes the cases", bodies["42"], bodies["missing-1"], bodies["foreign-1"])
	}
	if strings.Contains(bodies["42"], "42") || strings.Contains(bodies["42"], "user-1") {
		t.Fatalf("the 404 body %q names the id or the owner", bodies["42"])
	}
}

// The sign-in refusal is not a new one: the update documents the same response
// component the collection's two operations already answer, and the handler's
// anonymous answer validates against it - challenge header and all.
func TestContractDeclaresTheSignInRequiredResponseLikeTheOtherOperations(t *testing.T) {
	doc := contractDocument(t)
	patchRef := patchOperation(t, doc).Responses.Status(http.StatusUnauthorized).Ref
	if patchRef != "#/components/responses/SignInRequired" {
		t.Fatalf("patch 401 %q, want the shared SignInRequired component", patchRef)
	}
	for _, tc := range []struct {
		path, method string
		status       int
	}{
		{"/api/notes", http.MethodGet, http.StatusOK},
		{"/api/notes", http.MethodPost, http.StatusCreated},
	} {
		op := mustOperation(t, doc, tc.path, tc.method)
		if ref := op.Responses.Status(http.StatusUnauthorized).Ref; ref != patchRef {
			t.Fatalf("%s %s answers 401 with %q, want the same component the update answers: %q", tc.method, tc.path, ref, patchRef)
		}
	}

	router := contractRouter(t)
	for _, tc := range []struct {
		name, token string
	}{
		{"no token", ""},
		{"a refused token", "expired"},
	} {
		got := request(Routes(&fakeStore{}, fakeAuth{}, "run-1"), http.MethodPatch, "/api/notes/42", `{"done":true}`, tc.token)
		if got.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status %d, want 401", tc.name, got.Code)
		}
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/42", strings.NewReader(`{"done":true}`))
		if err := checkResponse(router, r, got.Code, got.Header(), got.Body.Bytes()); err != nil {
			t.Fatalf("%s: the sign-in refusal breaks the contract: %v", tc.name, err)
		}
	}
}

// Every code the handler answers for the write is a code the contract documents,
// with a body that validates: the document records the surface as delivered, so
// an undocumented status or a malformed note fails here rather than surprising
// the generated client later.
func TestImplementedPatchResponsesMatchTheContract(t *testing.T) {
	router := contractRouter(t)
	note := Note{ID: "42", Label: "Buy milk", CreatedAt: created, Done: true}
	cases := []struct {
		name, body, token, id string
		store                 *fakeStore
		want                  int
	}{
		{"flag set", `{"done":true}`, goodToken, "42", &fakeStore{update: note}, 200},
		{"flag cleared", `{"done":false}`, goodToken, "42", &fakeStore{update: Note{ID: "42", Label: "Buy milk", CreatedAt: created}}, 200},
		{"flag set again on a note already in that state", `{"done":true}`, goodToken, "42", &fakeStore{update: note}, 200},
		{"flag absent", `{}`, goodToken, "42", &fakeStore{}, 400},
		{"flag not a boolean", `{"done":"yes"}`, goodToken, "42", &fakeStore{}, 400},
		{"a label riding along", `{"done":true,"label":"x"}`, goodToken, "42", &fakeStore{}, 400},
		{"note the caller cannot see", `{"done":true}`, goodToken, "missing-1", &fakeStore{err: ErrNoteNotFound}, 404},
		{"note of someone else", `{"done":true}`, goodToken, "foreign-1", &fakeStore{err: ErrNoteNotFound}, 404},
		{"anonymous", `{"done":true}`, "", "42", &fakeStore{}, 401},
		{"refused token", `{"done":true}`, "expired", "42", &fakeStore{}, 401},
		{"persistence failure", `{"done":true}`, goodToken, "42", &fakeStore{err: errors.New("down")}, 500},
	}
	for _, tc := range cases {
		path := "/api/notes/" + tc.id
		got := request(Routes(tc.store, fakeAuth{}, "run-1"), http.MethodPatch, path, tc.body, tc.token)
		if got.Code != tc.want {
			t.Fatalf("%s: status %d, want %d", tc.name, got.Code, tc.want)
		}
		r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(tc.body))
		if err := checkResponse(router, r, got.Code, got.Header(), got.Body.Bytes()); err != nil {
			t.Fatalf("%s: patch response breaks the contract: %v", tc.name, err)
		}
	}
}

// The validator bites on the new operation as it does on the two the collection
// already had: the updated note is a note, so a 200 without the flag, or with a
// flag that is not a boolean, or carrying a field a note never has, is refused -
// and so is a 404 that says more than that no note of the caller's carries the id.
func TestContractRefusesBrokenPatchResponses(t *testing.T) {
	router := contractRouter(t)
	jsonHeader := http.Header{"Content-Type": []string{"application/json"}}
	challenge := http.Header{"Content-Type": []string{"application/json"}, "Www-Authenticate": []string{"Bearer"}}
	cases := []struct {
		name, body string
		status     int
		header     http.Header
	}{
		{"updated note without done", `{"id":"42","label":"x","createdAt":"2026-10-12T09:30:00Z"}`, 200, jsonHeader},
		{"updated note with a null done", `{"id":"42","label":"x","createdAt":"2026-10-12T09:30:00Z","done":null}`, 200, jsonHeader},
		{"updated note with a string done", `{"id":"42","label":"x","createdAt":"2026-10-12T09:30:00Z","done":"true"}`, 200, jsonHeader},
		{"updated note with a numeric id", `{"id":42,"label":"x","createdAt":"2026-10-12T09:30:00Z","done":true}`, 200, jsonHeader},
		{"updated note without its label", `{"id":"42","createdAt":"2026-10-12T09:30:00Z","done":true}`, 200, jsonHeader},
		{"updated note carrying the owner", `{"id":"42","label":"x","createdAt":"2026-10-12T09:30:00Z","done":true,"owner":"user-1"}`, 200, jsonHeader},
		{"404 that reveals whether the note exists", `{"message":"Not found","exists":true}`, 404, jsonHeader},
		{"404 that says whose note it is", `{"message":"Not found","owner":"user-2"}`, 404, jsonHeader},
		{"404 with no body shape at all", `{"error":"gone"}`, 404, jsonHeader},
		{"404 without a message", `{}`, 404, jsonHeader},
		{"undocumented status", `{"message":"no"}`, 403, jsonHeader},
		{"401 without the challenge", `{"message":"Sign-in required"}`, 401, jsonHeader},
		{"401 with the challenge but the wrong body", `{"error":"nope"}`, 401, challenge},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/42", nil)
		if err := checkResponse(router, r, tc.status, tc.header, []byte(tc.body)); err == nil {
			t.Fatalf("%s: contract accepted a broken patch response", tc.name)
		}
	}
}

// The contract refuses the bodies the handler also refuses: the flag alone is a
// valid request, and everything else - no flag, a flag of the wrong type, a label
// trying to ride along, a body that is not an object - is refused by the
// document, not only by the handler.
func TestContractRefusesAPatchBodyThatIsNotTheFlagAlone(t *testing.T) {
	router := contractRouter(t)
	for _, body := range []string{`{"done":true}`, `{"done":false}`} {
		if err := checkPatchRequest(router, "42", body); err != nil {
			t.Fatalf("body %s: contract refused the flag the update is made of: %v", body, err)
		}
	}
	cases := []struct {
		name, body string
	}{
		{"no flag at all", `{}`},
		{"the label instead", `{"label":"x"}`},
		{"a flag that is a string", `{"done":"true"}`},
		{"a flag that is a number", `{"done":1}`},
		{"a flag that is null", `{"done":null}`},
		{"a label riding along with the flag", `{"done":true,"label":"x"}`},
		{"an id riding along with the flag", `{"done":true,"id":"7"}`},
		{"a flag under another name", `{"Done":true}`},
	}
	for _, tc := range cases {
		if err := checkPatchRequest(router, "42", tc.body); err == nil {
			t.Fatalf("%s: contract accepted a body that is not the done flag alone", tc.name)
		}
	}
}

// bodyAdditionalPropertiesForbidden reports whether a schema closes itself to
// members it does not declare.
func bodyAdditionalPropertiesForbidden(schema *openapi3.Schema) bool {
	return schema.AdditionalProperties.Has != nil && !*schema.AdditionalProperties.Has
}

// mustRequestBody returns an operation's required request body.
func mustRequestBody(t *testing.T, op *openapi3.Operation) *openapi3.RequestBody {
	t.Helper()
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		t.Fatalf("operation %s declares no request body", op.OperationID)
	}
	return op.RequestBody.Value
}

// mustOperation returns one operation of the document by path and method, so a
// test can compare the update against the operations beside it.
func mustOperation(t *testing.T, doc *openapi3.T, path, method string) *openapi3.Operation {
	t.Helper()
	item := doc.Paths.Find(path)
	if item == nil {
		t.Fatalf("contract declares no %s path", path)
	}
	var op *openapi3.Operation
	switch method {
	case http.MethodGet:
		op = item.Get
	case http.MethodPost:
		op = item.Post
	case http.MethodPatch:
		op = item.Patch
	default:
		t.Fatalf("no contract reader for method %s", method)
	}
	if op == nil {
		t.Fatalf("contract declares no %s operation on %s", method, path)
	}
	return op
}
