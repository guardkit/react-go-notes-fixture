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
	"unicode/utf8"
)

// Store is owned by the HTTP consumer; the PostgreSQL adapter implements it.
// Every call is scoped to the signed-in owner (the token's `sub`).
type Store interface {
	Create(ctx context.Context, owner, label string) (Note, error)
	List(ctx context.Context, owner string) ([]Note, error)
	UpdateDone(ctx context.Context, owner, id string, done bool) (Note, error)
}

// Authenticator turns a bearer token into the owner's subject, or refuses it.
type Authenticator interface {
	Subject(ctx context.Context, rawToken string) (string, error)
}

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
	// The signed-in owner's notes, newest first, each answered with its own
	// done flag beside its id, label and creation time.
	mux.Handle("GET /api/notes", signedIn(authn, func(w http.ResponseWriter, r *http.Request, owner string) {
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
	// The signed-in owner completes or re-opens one of their own notes. The
	// done flag is the one field a caller may set, so the note comes back with
	// its id, label and creation time exactly as they were stored (ASSUM-002),
	// in the same note shape the listing answers with.
	mux.Handle("PATCH /api/notes/{id}", signedIn(authn, func(w http.ResponseWriter, r *http.Request, owner string) {
		done, err := decodeDone(w, r)
		if err != nil {
			// A flag that is absent, null or not a boolean answers with the
			// message naming the flag; every other failure stays
			// "Invalid request".
			if errors.Is(err, errDoneMissing) || errors.Is(err, errDoneNotBoolean) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		// The owner is the token's subject and the id is the path's: neither is
		// ever read out of the body, so a caller cannot name whose note it
		// updates. The store scopes the write to both.
		note, err := store.UpdateDone(r.Context(), owner, r.PathValue("id"), done)
		if err != nil {
			// One call answers both a note that was never written and a note
			// owned by someone else, so the two are byte-identical and the
			// answer discloses neither (ASSUM-003).
			if errors.Is(err, ErrNoteNotFound) {
				writeError(w, http.StatusNotFound, "Note not found")
				return
			}
			slog.ErrorContext(r.Context(), "update note failed")
			writeError(w, http.StatusInternalServerError, "Service unavailable")
			return
		}
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

// jsonBody reads a request body under the guard every note write shares: the
// media type must be JSON, the body must fit the size limit, must be valid
// UTF-8, and must be a single JSON object. Nothing looks at a field before
// these hold.
func jsonBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, errInvalidBody
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(body) {
		return nil, errInvalidBody
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errInvalidBody
	}
	return trimmed, nil
}

func decodeLabel(w http.ResponseWriter, r *http.Request) (string, error) {
	trimmed, err := jsonBody(w, r)
	if err != nil {
		return "", err
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
		return "", errInvalidBody
	}
	return ValidateLabel(input.Label)
}

// decodeDone reads the one field a note update may carry. The flag is decoded
// as the raw JSON value the caller sent, so an absent flag, a null flag and a
// flag of some other type stay three distinguishable refusals rather than one
// decode failure; DisallowUnknownFields keeps the flag the sole field a client
// may set, so a body that also tries to write the label is refused outright.
func decodeDone(w http.ResponseWriter, r *http.Request) (bool, error) {
	trimmed, err := jsonBody(w, r)
	if err != nil {
		return false, err
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var input struct {
		Done *json.RawMessage `json:"done"`
	}
	if err := decoder.Decode(&input); err != nil {
		return false, errInvalidBody
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return false, errInvalidBody
	}
	return ValidateDone(input.Done)
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
