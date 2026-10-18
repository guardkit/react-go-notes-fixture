package notes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

// patchPath is the contract path the done-flag toggle is published at: the same
// route TASK-F552-002 wired in Routes, spelled the OpenAPI way.
const patchPath = "/api/notes/{id}"

// patchOperation answers with the contract's toggle operation.
func patchOperation(t *testing.T, doc *openapi3.T) *openapi3.Operation {
	t.Helper()
	item := doc.Paths.Value(patchPath)
	if item == nil {
		t.Fatalf("contract declares no %s path", patchPath)
	}
	if item.Patch == nil {
		t.Fatalf("contract declares no patch operation on %s", patchPath)
	}
	return item.Patch
}

// documentedResponse answers with the contract's response for one status of an
// operation, failing when that status is undocumented. The status is looked up
// by its exact key, so a wildcard 4XX never stands in for the 404 the toggle
// must publish.
func documentedResponse(t *testing.T, op *openapi3.Operation, status string) *openapi3.Response {
	t.Helper()
	ref := op.Responses.Value(status)
	if ref == nil || ref.Value == nil {
		t.Fatalf("contract documents no %s response for %s", status, op.OperationID)
	}
	return ref.Value
}

// responseSchema answers with the JSON body schema one documented response
// carries.
func responseSchema(t *testing.T, op *openapi3.Operation, status string) *openapi3.Schema {
	t.Helper()
	media, ok := documentedResponse(t, op, status).Content["application/json"]
	if !ok || media == nil || media.Schema == nil || media.Schema.Value == nil {
		t.Fatalf("the contract's %s response for %s carries no application/json body", status, op.OperationID)
	}
	return media.Schema.Value
}

// sharedResponse answers with a component response the contract reuses, so an
// operation can be pinned to the very object its siblings point at rather than a
// copy of it.
func sharedResponse(t *testing.T, doc *openapi3.T, name string) *openapi3.Response {
	t.Helper()
	ref, ok := doc.Components.Responses[name]
	if !ok || ref == nil || ref.Value == nil {
		t.Fatalf("contract declares no %s component response", name)
	}
	return ref.Value
}

// The published contract describes the toggle end to end: a bearer-guarded patch
// on the note path, a body that is just the boolean flag, the updated note back,
// and the not-found, sign-in and unavailable answers.
func TestContractDeclaresTheDoneFlagToggle(t *testing.T) {
	doc := contractDoc(t)
	op := patchOperation(t, doc)

	if op.Security == nil || len(*op.Security) != 1 {
		t.Fatalf("the toggle declares %v security requirements, want one bearer requirement", op.Security)
	}
	requirement := (*op.Security)[0]
	if _, ok := requirement["bearer"]; !ok {
		t.Fatalf("the toggle's security requirement is %v, want the bearer scheme", requirement)
	}
	scheme, ok := doc.Components.SecuritySchemes["bearer"]
	if !ok || scheme == nil || scheme.Value == nil {
		t.Fatal("contract declares no bearer security scheme to require")
	}
	if scheme.Value.Type != "http" || scheme.Value.Scheme != "bearer" {
		t.Errorf("the bearer scheme is %s/%s, want http/bearer", scheme.Value.Type, scheme.Value.Scheme)
	}

	id := pathParameter(t, op, "id")
	if id.In != "path" || !id.Required {
		t.Errorf("the {id} parameter is in %q and required=%v, want a required path parameter", id.In, id.Required)
	}
	if id.Schema == nil || id.Schema.Value == nil || !id.Schema.Value.Type.Is("string") {
		t.Errorf("the {id} parameter is not a string: %v", id.Schema)
	}

	if op.RequestBody == nil || op.RequestBody.Value == nil {
		t.Fatal("the toggle declares no request body")
	}
	if !op.RequestBody.Value.Required {
		t.Error("the toggle's request body is optional: a bodyless toggle would be accepted")
	}
	body, ok := op.RequestBody.Value.Content["application/json"]
	if !ok || body == nil || body.Schema == nil || body.Schema.Value == nil {
		t.Fatal("the toggle declares no application/json request body")
	}
	doneUpdate, ok := doc.Components.Schemas["DoneUpdate"]
	if !ok || doneUpdate == nil || doneUpdate.Value == nil {
		t.Fatal("contract declares no DoneUpdate request schema")
	}
	if body.Schema.Value != doneUpdate.Value {
		t.Error("the toggle's body is not the DoneUpdate schema the contract declares")
	}
	if !schemaRequires(doneUpdate.Value, "done") {
		t.Error("the toggle's body does not require the done flag")
	}
	done, ok := doneUpdate.Value.Properties["done"]
	if !ok || done == nil || done.Value == nil {
		t.Fatal("the toggle's body declares no done property")
	}
	if !done.Value.Type.Is("boolean") {
		t.Errorf("the toggle's done flag is %v, want a boolean", done.Value.Type)
	}
	if done.Value.Nullable {
		t.Error("the toggle's done flag is nullable: a blank completion state is not a flag")
	}
	if has := doneUpdate.Value.AdditionalProperties.Has; has == nil || *has {
		t.Error("the toggle's body accepts fields beside the flag")
	}

	if updated := responseSchema(t, op, "200"); updated != noteSchema(t, doc) {
		t.Error("the toggle does not answer with the contract's note object")
	}
	if got := documentedResponse(t, op, "401"); got != sharedResponse(t, doc, "SignInRequired") {
		t.Error("the toggle's 401 is not the SignInRequired response its siblings reuse")
	}
	if got := documentedResponse(t, op, "500"); got != sharedResponse(t, doc, "Unavailable") {
		t.Error("the toggle's 500 is not the Unavailable response its siblings reuse")
	}
	if missed := responseSchema(t, op, "404"); missed != errorSchema(t, doc) {
		t.Error("the toggle's 404 does not answer with the contract's error object")
	}
}

// pathParameter answers with one of the operation's declared parameters.
func pathParameter(t *testing.T, op *openapi3.Operation, name string) *openapi3.Parameter {
	t.Helper()
	for _, ref := range op.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		if ref.Value.Name == name {
			return ref.Value
		}
	}
	t.Fatalf("the toggle declares no %s parameter", name)
	return nil
}

// errorSchema answers with the contract's error object, the body every refusal
// carries.
func errorSchema(t *testing.T, doc *openapi3.T) *openapi3.Schema {
	t.Helper()
	ref, ok := doc.Components.Schemas["Error"]
	if !ok || ref == nil || ref.Value == nil {
		t.Fatal("contract declares no Error schema")
	}
	return ref.Value
}

// The contract must describe what the wired route actually answers, status for
// status: the toggle's whole documented surface is driven through the handler and
// validated, so a response the API serves that the published contract does not
// describe fails here.
func TestLiveToggleResponsesMatchTheContract(t *testing.T) {
	router := contractRouter(t)
	doneNote := Note{ID: "42", Label: "hello", CreatedAt: created, Done: true}
	notDoneNote := Note{ID: "42", Label: "hello", CreatedAt: created, Done: false}
	cases := []struct {
		name, body, token string
		store             *fakeStore
		want              int
	}{
		{"done true", `{"done":true}`, goodToken, &fakeStore{doneNote: doneNote}, 200},
		{"done false", `{"done":false}`, goodToken, &fakeStore{doneNote: notDoneNote}, 200},
		{"anonymous", `{"done":true}`, "", &fakeStore{doneNote: doneNote}, 401},
		{"refused token", `{"done":true}`, "expired", &fakeStore{doneNote: doneNote}, 401},
		// A note that was never stored and a note held by another subject are
		// the same answer, and both are the one the contract publishes.
		{"missing note", `{"done":true}`, goodToken, &fakeStore{doneErr: ErrNoteNotFound}, 404},
		{"foreign note", `{"done":false}`, goodToken, &fakeStore{doneErr: ErrNoteNotFound}, 404},
		{"persistence failure", `{"done":true}`, goodToken, &fakeStore{doneErr: errors.New("down")}, 500},
	}
	for _, tc := range cases {
		got := request(Routes(tc.store, fakeAuth{}, "run-1"), http.MethodPatch, "/api/notes/42", tc.body, tc.token)
		if got.Code != tc.want {
			t.Fatalf("%s: status %d, want %d", tc.name, got.Code, tc.want)
		}
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/42", strings.NewReader(tc.body))
		if err := checkResponse(router, r, got.Code, got.Header(), got.Body.Bytes()); err != nil {
			t.Fatalf("%s: the toggle's response breaks the contract: %v", tc.name, err)
		}
	}
}

// The published toggle must bite, not merely describe: bodies and statuses the
// contract does not allow are refused.
func TestContractRefusesForbiddenToggleResponses(t *testing.T) {
	router := contractRouter(t)
	jsonHeader := http.Header{"Content-Type": []string{"application/json"}}
	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"note without a flag", 200, jsonHeader, `{"id":"42","label":"hello","createdAt":"2026-10-12T09:30:00Z"}`},
		{"note with a non-boolean flag", 200, jsonHeader, `{"id":"42","label":"hello","createdAt":"2026-10-12T09:30:00Z","done":"yes"}`},
		{"note with a numeric id", 200, jsonHeader, `{"id":42,"label":"hello","createdAt":"2026-10-12T09:30:00Z","done":true}`},
		{"note with an undocumented field", 200, jsonHeader, `{"id":"42","label":"hello","createdAt":"2026-10-12T09:30:00Z","done":true,"owner":"u"}`},
		{"flag-only body", 200, jsonHeader, `{"done":true}`},
		{"forbidden instead of not found", 403, jsonHeader, `{"message":"no"}`},
		{"undocumented status", 204, jsonHeader, ``},
		{"not found without an error body", 404, jsonHeader, `{"error":"nope"}`},
		{"not found as a note", 404, jsonHeader, `{"id":"42","label":"hello","createdAt":"2026-10-12T09:30:00Z","done":true}`},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodPatch, "/api/notes/42", nil)
		if err := checkResponse(router, r, tc.status, tc.header, []byte(tc.body)); err == nil {
			t.Fatalf("%s: contract accepted a forbidden toggle response", tc.name)
		}
	}
}

// The body the contract publishes is the body the route accepts: the flag alone,
// either way, from a signed-in caller, and nothing else.
func TestContractAcceptsOnlyTheBooleanFlagBody(t *testing.T) {
	router := contractRouter(t)
	accepted := []string{`{"done":true}`, `{"done":false}`}
	for _, body := range accepted {
		if err := checkPatchRequest(router, body, goodToken); err != nil {
			t.Fatalf("contract refused the toggle body %s: %v", body, err)
		}
	}
	refused := []string{
		`{}`,
		`{"label":"hello"}`,
		`{"done":"yes"}`,
		`{"done":1}`,
		`{"done":null}`,
		`{"done":true,"label":"hello"}`,
		`true`,
	}
	for _, body := range refused {
		if err := checkPatchRequest(router, body, goodToken); err == nil {
			t.Fatalf("contract accepted the toggle body %s", body)
		}
	}
	// The toggle is the signed-in owner's: the contract refuses an anonymous one
	// before its body is ever considered.
	if err := checkPatchRequest(router, `{"done":true}`, ""); err == nil {
		t.Fatal("contract accepted an anonymous toggle")
	}
}

// requireBearer accepts only the scheme the contract names, on a request that
// actually carries a bearer token - so a toggle published without the guard could
// not pass the request checks.
func requireBearer(_ context.Context, input *openapi3filter.AuthenticationInput) error {
	if input.SecuritySchemeName != "bearer" {
		return errors.New("the toggle demands a scheme other than bearer")
	}
	if input.SecurityScheme == nil || input.SecurityScheme.Type != "http" || input.SecurityScheme.Scheme != "bearer" {
		return errors.New("the bearer scheme is not an HTTP bearer scheme")
	}
	raw := input.RequestValidationInput.Request.Header.Get("Authorization")
	scheme, token, found := strings.Cut(raw, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return errors.New("no bearer token on the toggle request")
	}
	return nil
}

func checkPatchRequest(router routers.Router, body, token string) error {
	r := httptest.NewRequest(http.MethodPatch, "/api/notes/42", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	route, params, err := router.FindRoute(r)
	if err != nil {
		return err
	}
	return openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: params,
		Route:      route,
		Options:    &openapi3filter.Options{MultiError: true, AuthenticationFunc: requireBearer},
	})
}

// The toggle is additive: publishing it left every operation that was already in
// the contract standing as it was - the same paths, the same bodies, the same
// responses, the same guards.
func TestPublishingTheToggleLeftTheOtherOperationsAsTheyWere(t *testing.T) {
	doc := contractDoc(t)
	note := noteSchema(t, doc)
	noteList, ok := doc.Components.Schemas["NoteList"]
	if !ok || noteList == nil || noteList.Value == nil {
		t.Fatal("contract declares no NoteList schema")
	}
	newNote, ok := doc.Components.Schemas["NewNote"]
	if !ok || newNote == nil || newNote.Value == nil {
		t.Fatal("contract declares no NewNote schema")
	}
	health, ok := doc.Components.Schemas["Health"]
	if !ok || health == nil || health.Value == nil {
		t.Fatal("contract declares no Health schema")
	}
	signIn := sharedResponse(t, doc, "SignInRequired")
	unavailable := sharedResponse(t, doc, "Unavailable")

	healthItem := doc.Paths.Value("/api/health")
	if healthItem == nil || healthItem.Get == nil {
		t.Fatal("contract no longer declares GET /api/health")
	}
	if healthItem.Get.Security != nil {
		t.Error("GET /api/health gained a security requirement")
	}
	if got := responseSchema(t, healthItem.Get, "200"); got != health.Value {
		t.Error("GET /api/health no longer answers with the Health object")
	}

	notesItem := doc.Paths.Value("/api/notes")
	if notesItem == nil || notesItem.Get == nil || notesItem.Post == nil {
		t.Fatal("contract no longer declares both operations on /api/notes")
	}
	if got := responseSchema(t, notesItem.Get, "200"); got != noteList.Value {
		t.Error("GET /api/notes no longer answers with the notes listing")
	}
	if notesItem.Get.RequestBody != nil {
		t.Error("GET /api/notes gained a request body")
	}
	if notesItem.Post.RequestBody == nil || notesItem.Post.RequestBody.Value == nil {
		t.Fatal("POST /api/notes lost its request body")
	}
	if sent := jsonBodySchema(t, notesItem.Post.RequestBody.Value); sent != newNote.Value {
		t.Error("POST /api/notes no longer accepts the NewNote body")
	}
	if got := responseSchema(t, notesItem.Post, "201"); got != note {
		t.Error("POST /api/notes no longer answers with the note object")
	}
	if got := responseSchema(t, notesItem.Post, "400"); got != errorSchema(t, doc) {
		t.Error("POST /api/notes no longer answers a refusal with the error object")
	}
	for _, op := range []*openapi3.Operation{notesItem.Get, notesItem.Post} {
		if got := documentedResponse(t, op, "401"); got != signIn {
			t.Errorf("%s no longer reuses the SignInRequired response", op.OperationID)
		}
		if got := documentedResponse(t, op, "500"); got != unavailable {
			t.Errorf("%s no longer reuses the Unavailable response", op.OperationID)
		}
		if op.Security == nil || len(*op.Security) != 1 {
			t.Errorf("%s lost its bearer security requirement", op.OperationID)
		}
	}
}

// jsonBodySchema answers with the JSON schema a request body declares.
func jsonBodySchema(t *testing.T, body *openapi3.RequestBody) *openapi3.Schema {
	t.Helper()
	media, ok := body.Content["application/json"]
	if !ok || media == nil || media.Schema == nil || media.Schema.Value == nil {
		t.Fatal("the request body declares no application/json schema")
	}
	return media.Schema.Value
}
