package notes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Note uses a decimal string because JavaScript numbers cannot safely hold all BIGINT values.
// The done flag is the completion state the contract declares required on every
// note (contract/openapi.yaml, Note.done); the column behind it is added by
// apps/api/migrations/0003_add_done.sql as BOOLEAN NOT NULL DEFAULT false, so a
// note the store has not marked complete is answered not-done, never omitted.
type Note struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
	Done      bool      `json:"done"`
}

// Store is owned by the HTTP consumer; the PostgreSQL adapter implements it.
// Every call is scoped to the signed-in owner (the token's `sub`).
type Store interface {
	Create(ctx context.Context, owner, label string) (Note, error)
	List(ctx context.Context, owner string) ([]Note, error)
	// UpdateDone sets the done flag of owner's note id and returns the note as
	// storage now holds it. The update is scoped to the owner in the same way
	// Create and List are, so a note of another owner answers ErrNoteNotFound -
	// the same signal a note that does not exist gives, and never a different
	// one. Only the flag is written; the label and the creation time are the
	// note's own, and they come back as the row holds them.
	UpdateDone(ctx context.Context, owner, id string, done bool) (Note, error)
}

// Authenticator turns a bearer token into the owner's subject, or refuses it.
type Authenticator interface {
	Subject(ctx context.Context, rawToken string) (string, error)
}

// errInvalidDone refuses a body that is not a JSON object carrying done as a
// boolean. The flag is the only property this path accepts, so a body offering
// anything else - no flag, a flag that is not a boolean, a label trying to ride
// along - is refused before storage is reached.
var errInvalidDone = errors.New("invalid done flag")

// Routes returns the notes API. Health reports process liveness and the run
// identity, so a check can confirm it reached its own instance.
func Routes(store Store, authn Authenticator, runID string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
			Run    string `json:"run"`
		}{Status: "ok", Run: runID})
	})
	mux.Handle("GET /api/notes", signedIn(authn, func(w http.ResponseWriter, r *http.Request, owner string) {
		// The read path is read-only (ASSUM-003): the owner comes from the token
		// and nothing else. The body, the query and the headers are never consulted,
		// so a caller cannot hand over a done value - each note answers with the
		// flag its row holds, which the store read from the NOT NULL column.
		list, err := store.List(r.Context(), owner)
		if err != nil {
			slog.ErrorContext(r.Context(), "list notes failed")
			writeError(w, http.StatusInternalServerError, "Service unavailable")
			return
		}
		if list == nil {
			list = []Note{}
		}
		writeJSON(w, http.StatusOK, struct {
			Notes []Note `json:"notes"`
		}{Notes: list})
	}))
	mux.Handle("POST /api/notes", signedIn(authn, func(w http.ResponseWriter, r *http.Request, owner string) {
		label, err := decodeLabel(w, r)
		if err != nil {
			// A blank or over-long title answers with the validation message
			// naming the field; every other failure stays "Invalid request".
			if errors.Is(err, errLabelBlank) || errors.Is(err, errLabelTooLong) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		// The title is trimmed at the hand-off to persistence, so whatever the
		// store writes - and so whatever the response echoes back - is the
		// trimmed title, whatever the client padded it with.
		note, err := store.Create(r.Context(), owner, storageTitle(label))
		if err != nil {
			slog.ErrorContext(r.Context(), "create note failed")
			writeError(w, http.StatusInternalServerError, "Service unavailable")
			return
		}
		writeJSON(w, http.StatusCreated, note)
	}))
	mux.Handle("PATCH /api/notes/{id}", signedIn(authn, func(w http.ResponseWriter, r *http.Request, owner string) {
		// The flag is the only thing this body may carry, so every refusal -
		// undecodable, a flag that is not a boolean, a label riding along -
		// answers the one invalid-request message the create path already uses.
		done, err := decodeDone(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		// The owner comes from the token and the id from the path, and the store
		// alone decides whether the row is the caller's to change: the handler
		// never asks first whether the note exists, so a note belonging to someone
		// else answers the same thing a note that was never written answers.
		note, err := store.UpdateDone(r.Context(), owner, r.PathValue("id"), done)
		if err != nil {
			if errors.Is(err, ErrNoteNotFound) {
				// One message for both cases, carrying neither the id nor the owner,
				// so the answer cannot be read as a probe of another user's notes.
				writeError(w, http.StatusNotFound, "Not found")
				return
			}
			slog.ErrorContext(r.Context(), "update note done flag failed")
			writeError(w, http.StatusInternalServerError, "Service unavailable")
			return
		}
		// The note as storage now holds it: the flag this call set, and the label
		// and creation time the note already had, read back by the same statement.
		writeJSON(w, http.StatusOK, note)
	}))
	return mux
}

// signedIn refuses anonymous and invalid tokens with 401 before the handler runs.
func signedIn(authn Authenticator, next func(http.ResponseWriter, *http.Request, string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="notes"`)
			writeError(w, http.StatusUnauthorized, "Sign-in required")
			return
		}
		owner, err := authn.Subject(r.Context(), strings.TrimSpace(token))
		if err != nil {
			// The reason (never the token) is logged, e.g. a wrong audience.
			slog.InfoContext(r.Context(), "token refused", "reason", err.Error())
			w.Header().Set("WWW-Authenticate", `Bearer realm="notes", error="invalid_token"`)
			writeError(w, http.StatusUnauthorized, "Sign-in required")
			return
		}
		next(w, r, owner)
	})
}

func decodeLabel(w http.ResponseWriter, r *http.Request) (string, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return "", errInvalidLabel
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(body) {
		return "", errInvalidLabel
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return "", errInvalidLabel
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var input struct {
		Label string `json:"label"`
	}
	if err := decoder.Decode(&input); err != nil {
		return "", err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", errInvalidLabel
	}
	return ValidateLabel(input.Label)
}

// decodeDone reads the one property the done update accepts: done, as a boolean.
// The body is read as its members, so the key must be exactly done - a "Done" is
// an unknown member here, though a Go field would take it - and any second member
// is refused rather than ignored, so a label has no way onto this path. A flag
// that is absent or not a boolean - a string, a number, a null - is refused too.
// Nothing is handed to storage until the flag decodes.
func decodeDone(w http.ResponseWriter, r *http.Request) (bool, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false, errInvalidDone
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	if err != nil {
		return false, err
	}
	if !utf8.Valid(body) {
		return false, errInvalidDone
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false, errInvalidDone
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	var members map[string]json.RawMessage
	if err := decoder.Decode(&members); err != nil {
		return false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return false, errInvalidDone
	}
	raw, ok := members["done"]
	if !ok || len(members) != 1 {
		return false, errInvalidDone
	}
	// A null decodes to no flag at all, so it answers with the rest of the
	// refusals: the caller must hand over true or false, never a stand-in.
	var flag *bool
	if err := json.Unmarshal(raw, &flag); err != nil || flag == nil {
		return false, errInvalidDone
	}
	return *flag, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Message string `json:"message"`
	}{Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		slog.Error("encode response failed")
		http.Error(w, "Service unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(append(data, '\n')); err != nil {
		slog.Warn("write response failed")
	}
}
