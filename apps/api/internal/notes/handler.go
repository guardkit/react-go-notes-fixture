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
type Note struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store is owned by the HTTP consumer; the PostgreSQL adapter implements it.
// Every call is scoped to the signed-in owner (the token's `sub`).
type Store interface {
	Create(ctx context.Context, owner, label string) (Note, error)
	List(ctx context.Context, owner string) ([]Note, error)
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
			// An over-long title answers with the validation message naming the
			// field and the limit; every other failure stays "Invalid request".
			if errors.Is(err, errLabelTooLong) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		note, err := store.Create(r.Context(), owner, label)
		if err != nil {
			slog.ErrorContext(r.Context(), "create note failed")
			writeError(w, http.StatusInternalServerError, "Service unavailable")
			return
		}
		writeJSON(w, http.StatusCreated, note)
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
