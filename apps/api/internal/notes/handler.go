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
	// SetDone writes the completion flag of one of owner's notes and answers with
	// the note as storage holds it afterwards. A note the caller does not own is
	// reported with ErrNoteNotFound, and changes nothing.
	SetDone(ctx context.Context, owner, id string, done bool) (Note, error)
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
	// The signed-in owner's notes, newest first, each answered with its own done
	// flag beside its id, label and creation time.
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
	// The signed-in owner sets one of their own notes done or not done. The
	// {id} is the caller's path segment and the flag is the whole body, so the
	// write is scoped to the caller on the statement that makes it: a note that
	// was never stored and a note of someone else answer the same 404, and
	// nothing about either is written or leaked.
	mux.Handle("PATCH /api/notes/{id}", signedIn(authn, func(w http.ResponseWriter, r *http.Request, owner string) {
		id := r.PathValue("id")
		done, err := decodeDone(w, r)
		if err != nil {
			// A flag that is not the JSON boolean true or false answers with the
			// message naming it; every other undecodable body stays
			// "Invalid request". Neither reaches storage.
			if errors.Is(err, errDoneNotBoolean) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		note, err := store.SetDone(r.Context(), owner, id, done)
		if err != nil {
			// One answer for both negatives - no such note, or one held by
			// another subject - so a caller cannot tell the two apart.
			if errors.Is(err, ErrNoteNotFound) {
				writeError(w, http.StatusNotFound, "Not found")
				return
			}
			slog.ErrorContext(r.Context(), "set note done flag failed")
			writeError(w, http.StatusInternalServerError, "Service unavailable")
			return
		}
		// The note as storage holds it now: the flag this caller just wrote, and
		// every other field of the note exactly as it was.
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
