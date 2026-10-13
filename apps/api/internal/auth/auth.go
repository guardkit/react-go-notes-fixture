// Package auth checks OIDC access tokens issued by the fixture's Keycloak realm.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// ErrInvalidToken covers every refusal; callers answer 401 without detail.
var ErrInvalidToken = errors.New("invalid access token")

// Config names the accepted issuers (exact strings), the audience the token
// must carry and the internal URL the signing keys are fetched from.
type Config struct {
	Issuers  []string
	Audience string
	JWKSURL  string
}

// Verifier checks signature, issuer, audience and expiry, and returns `sub`.
type Verifier struct {
	byIssuer map[string]*oidc.IDTokenVerifier
}

// ParseIssuers splits a comma-separated list, refusing blanks and duplicates.
func ParseIssuers(value string) ([]string, error) {
	var issuers []string
	seen := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		issuer := strings.TrimSpace(part)
		if issuer == "" {
			return nil, errors.New("OIDC_ISSUERS has an empty entry")
		}
		if seen[issuer] {
			return nil, fmt.Errorf("OIDC_ISSUERS repeats %q", issuer)
		}
		seen[issuer] = true
		issuers = append(issuers, issuer)
	}
	return issuers, nil
}

// New builds one verifier per accepted issuer, all sharing one key set.
// Keys are fetched lazily, so the API starts before Keycloak is ready, and
// refreshed at most once per minRefresh (see keyset.go).
func New(cfg Config, client *http.Client) (*Verifier, error) {
	if len(cfg.Issuers) == 0 {
		return nil, errors.New("at least one issuer is required")
	}
	if cfg.Audience == "" {
		return nil, errors.New("an audience is required")
	}
	if !strings.HasPrefix(cfg.JWKSURL, "http://") && !strings.HasPrefix(cfg.JWKSURL, "https://") {
		return nil, errors.New("the key set URL must be http or https")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	keys := newKeySet(cfg.JWKSURL, client)
	v := &Verifier{byIssuer: map[string]*oidc.IDTokenVerifier{}}
	for _, issuer := range cfg.Issuers {
		v.byIssuer[issuer] = oidc.NewVerifier(issuer, keys, &oidc.Config{
			ClientID:             cfg.Audience,
			SupportedSigningAlgs: []string{oidc.RS256},
		})
	}
	return v, nil
}

// Subject verifies a raw bearer token and returns its non-empty `sub`.
func (v *Verifier) Subject(ctx context.Context, raw string) (string, error) {
	issuer, err := unverifiedIssuer(raw)
	if err != nil {
		return "", ErrInvalidToken
	}
	verifier, ok := v.byIssuer[issuer]
	if !ok {
		return "", ErrInvalidToken
	}
	token, err := verifier.Verify(ctx, raw)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if token.Subject == "" || len(token.Subject) > 255 {
		return "", ErrInvalidToken
	}
	return token.Subject, nil
}

// unverifiedIssuer only selects which verifier to use; that verifier then
// checks the signature and the issuer again.
func unverifiedIssuer(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidToken
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Issuer == "" {
		return "", ErrInvalidToken
	}
	return claims.Issuer, nil
}
