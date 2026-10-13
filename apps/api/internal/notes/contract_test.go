package notes

import (
	"bytes"
	"context"
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

func contractRouter(t *testing.T) routers.Router {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(contractPath)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("contract is not valid OpenAPI: %v", err)
	}
	router, err := legacy.NewRouter(doc)
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
		{"empty list", http.MethodGet, "/api/notes", "", goodToken, &fakeStore{}, 200},
		{"list anonymous", http.MethodGet, "/api/notes", "", "", &fakeStore{}, 401},
		{"list refused token", http.MethodGet, "/api/notes", "", "expired", &fakeStore{}, 401},
		{"list failure", http.MethodGet, "/api/notes", "", goodToken, &fakeStore{err: errors.New("down")}, 500},
		{"create", http.MethodPost, "/api/notes", `{"label":"hello"}`, goodToken, &fakeStore{note: note}, 201},
		{"create invalid", http.MethodPost, "/api/notes", `{"label":""}`, goodToken, &fakeStore{}, 400},
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
