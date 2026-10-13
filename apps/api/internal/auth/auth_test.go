package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	internalIssuer = "http://127.0.0.1:41001/auth/realms/notes-test"
	publicIssuer   = "https://preview.example.test:8950/auth/realms/notes-test"
	audience       = "notes-api"
)

type keyPair struct {
	private *rsa.PrivateKey
	kid     string
}

func newKey(t *testing.T, kid string) keyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return keyPair{private: key, kid: kid}
}

// jwksServer publishes only the given public keys, as Keycloak's certs endpoint does.
func jwksServer(t *testing.T, keys ...keyPair) *httptest.Server {
	t.Helper()
	set := jose.JSONWebKeySet{}
	for _, k := range keys {
		set.Keys = append(set.Keys, jose.JSONWebKey{Key: &k.private.PublicKey, KeyID: k.kid, Algorithm: "RS256", Use: "sig"})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(set); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

type claims struct {
	jwt.Claims
	Type string `json:"typ,omitempty"`
}

func sign(t *testing.T, key keyPair, c claims) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key.private, KeyID: key.kid}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(c).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func valid(issuer string) claims {
	now := time.Now()
	return claims{Claims: jwt.Claims{
		Issuer:   issuer,
		Subject:  "3f1c2a8e-user",
		Audience: jwt.Audience{"account", audience},
		IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}, Type: "Bearer"}
}

func newVerifier(t *testing.T, server *httptest.Server, issuers ...string) *Verifier {
	t.Helper()
	v, err := New(Config{Issuers: issuers, Audience: audience, JWKSURL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAcceptsEachConfiguredIssuer(t *testing.T) {
	key := newKey(t, "k1")
	v := newVerifier(t, jwksServer(t, key), internalIssuer, publicIssuer)
	for _, issuer := range []string{internalIssuer, publicIssuer} {
		sub, err := v.Subject(context.Background(), sign(t, key, valid(issuer)))
		if err != nil || sub != "3f1c2a8e-user" {
			t.Fatalf("issuer %s: sub %q err %v", issuer, sub, err)
		}
	}
}

func TestRefusesBadTokens(t *testing.T) {
	key := newKey(t, "k1")
	other := newKey(t, "k2")
	impostor := keyPair{private: newKey(t, "x").private, kid: "k1"}
	v := newVerifier(t, jwksServer(t, key), internalIssuer)

	wrongAudience := valid(internalIssuer)
	wrongAudience.Audience = jwt.Audience{"account", "notes-web"}
	expired := valid(internalIssuer)
	expired.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	noSubject := valid(internalIssuer)
	noSubject.Subject = ""

	cases := map[string]string{
		"wrong audience":       sign(t, key, wrongAudience),
		"expired":              sign(t, key, expired),
		"unlisted issuer":      sign(t, key, valid(publicIssuer)),
		"issuer prefix only":   sign(t, key, valid(internalIssuer+"x")),
		"unknown signing key":  sign(t, other, valid(internalIssuer)),
		"forged with same kid": sign(t, impostor, valid(internalIssuer)),
		"empty subject":        sign(t, key, noSubject),
		"not a JWT":            "not-a-token",
		"unsigned":             "eyJhbGciOiJub25lIn0.eyJpc3MiOiJodHRwOi8vMTI3LjAuMC4xOjQxMDAxL2F1dGgvcmVhbG1zL25vdGVzLXRlc3QiLCJzdWIiOiJ4IiwiYXVkIjoibm90ZXMtYXBpIn0.",
	}
	for name, raw := range cases {
		if sub, err := v.Subject(context.Background(), raw); err == nil || !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%s: accepted (sub %q, err %v)", name, sub, err)
		}
	}
}

func TestUnreachableKeysRefuse(t *testing.T) {
	key := newKey(t, "k1")
	server := jwksServer(t, key)
	v := newVerifier(t, server, internalIssuer)
	server.Close()
	if _, err := v.Subject(context.Background(), sign(t, key, valid(internalIssuer))); err == nil {
		t.Fatal("token accepted without reachable keys")
	}
}

func TestConfigValidation(t *testing.T) {
	if _, err := ParseIssuers("a, ,b"); err == nil {
		t.Fatal("blank issuer accepted")
	}
	if _, err := ParseIssuers("a,a"); err == nil {
		t.Fatal("duplicate issuer accepted")
	}
	got, err := ParseIssuers(" http://a/realms/r , https://b/realms/r ")
	if err != nil || len(got) != 2 || got[0] != "http://a/realms/r" || got[1] != "https://b/realms/r" {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := New(Config{Issuers: got, JWKSURL: "http://k"}, nil); err == nil {
		t.Fatal("missing audience accepted")
	}
	if _, err := New(Config{Issuers: got, Audience: "a", JWKSURL: "file:///keys"}, nil); err == nil {
		t.Fatal("non-HTTP key set accepted")
	}
}

func segment(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// Algorithm confusion: an HMAC token keyed with the published public key, and
// unsigned tokens that otherwise carry every valid claim, are refused.
func TestRefusesAlgorithmConfusion(t *testing.T) {
	key := newKey(t, "k1")
	v := newVerifier(t, jwksServer(t, key), internalIssuer)
	now := time.Now().Unix()
	payload := segment(t, map[string]any{"iss": internalIssuer, "sub": "x", "aud": []string{audience}, "exp": now + 300, "iat": now})

	der, err := x509.MarshalPKIXPublicKey(&key.private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	for name, secret := range map[string][]byte{"pem": pemKey, "der": der} {
		signing := segment(t, map[string]any{"alg": "HS256", "kid": "k1", "typ": "JWT"}) + "." + payload
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(signing))
		raw := signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		if sub, err := v.Subject(context.Background(), raw); err == nil {
			t.Fatalf("HS256 keyed with the public key (%s) accepted as %q", name, sub)
		}
	}
	for _, header := range []map[string]any{{"alg": "none"}, {"alg": "none", "kid": "k1"}, {"alg": "None", "kid": "k1"}} {
		for _, sig := range []string{"", "AAAA"} {
			raw := segment(t, header) + "." + payload + "." + sig
			if sub, err := v.Subject(context.Background(), raw); err == nil {
				t.Fatalf("alg %v with exp accepted as %q", header["alg"], sub)
			}
		}
	}
}

// countingKeys serves the key set and counts fetches.
func countingKeys(t *testing.T, keys ...keyPair) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	set := jose.JSONWebKeySet{}
	for _, k := range keys {
		set.Keys = append(set.Keys, jose.JSONWebKey{Key: &k.private.PublicKey, KeyID: k.kid, Algorithm: "RS256", Use: "sig"})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if err := json.NewEncoder(w).Encode(set); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

// Tokens naming an unknown key id do not make the API refetch the key set on
// every request: at most once per refresh interval.
func TestUnknownKeyIDRefetchIsBounded(t *testing.T) {
	key := newKey(t, "k1")
	server, hits := countingKeys(t, key)
	v := newVerifier(t, server, internalIssuer)
	if _, err := v.Subject(context.Background(), sign(t, key, valid(internalIssuer))); err != nil {
		t.Fatal(err)
	}
	stranger := newKey(t, "unknown")
	for range 20 {
		if _, err := v.Subject(context.Background(), sign(t, stranger, valid(internalIssuer))); err == nil {
			t.Fatal("unknown key accepted")
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("key set fetched %d times for 1 valid and 20 unknown-key tokens, want 1", got)
	}
}

// A rotated key is picked up once the refresh interval has passed.
func TestRotatedKeyIsFetchedAfterTheInterval(t *testing.T) {
	old := newKey(t, "k1")
	rotated := newKey(t, "k2")
	server, hits := countingKeys(t, old, rotated)
	clock := time.Now()
	keys := newKeySet(server.URL, server.Client())
	keys.now = func() time.Time { return clock }
	keys.keys = map[string]*jose.JSONWebKey{}
	keys.lastFetch = clock
	v := &Verifier{byIssuer: map[string]*oidc.IDTokenVerifier{
		internalIssuer: oidc.NewVerifier(internalIssuer, keys, &oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.RS256}}),
	}}
	if _, err := v.Subject(context.Background(), sign(t, rotated, valid(internalIssuer))); err == nil || hits.Load() != 0 {
		t.Fatalf("fetched within the interval (err %v, fetches %d)", err, hits.Load())
	}
	clock = clock.Add(minRefresh)
	if sub, err := v.Subject(context.Background(), sign(t, rotated, valid(internalIssuer))); err != nil || sub == "" || hits.Load() != 1 {
		t.Fatalf("rotated key not picked up after the interval (err %v, fetches %d)", err, hits.Load())
	}
}

// A key set endpoint that never answers is bounded by the client's timeout.
func TestSlowKeySetIsBounded(t *testing.T) {
	key := newKey(t, "k1")
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })
	v, err := New(Config{Issuers: []string{internalIssuer}, Audience: audience, JWKSURL: server.URL}, &http.Client{Timeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := v.Subject(context.Background(), sign(t, key, valid(internalIssuer))); err == nil {
		t.Fatal("accepted without keys")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("slow key set held the request for %s", elapsed)
	}
}

// gatedKeys serves the key set; while gate is set, a fetch waits on it.
type gatedKeys struct {
	set  jose.JSONWebKeySet
	hits atomic.Int64
	gate atomic.Pointer[chan struct{}]
}

func (g *gatedKeys) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.hits.Add(1)
	if gate := g.gate.Load(); gate != nil {
		select {
		case <-*gate:
		case <-r.Context().Done():
			return
		}
	}
	_ = json.NewEncoder(w).Encode(g.set)
}

func gatedVerifier(t *testing.T, keys ...keyPair) (*Verifier, *keySet, *gatedKeys, *time.Time) {
	t.Helper()
	g := &gatedKeys{}
	for _, k := range keys {
		g.set.Keys = append(g.set.Keys, jose.JSONWebKey{Key: &k.private.PublicKey, KeyID: k.kid, Algorithm: "RS256", Use: "sig"})
	}
	server := httptest.NewServer(g)
	t.Cleanup(server.Close)
	clock := time.Now()
	ks := newKeySet(server.URL, server.Client())
	ks.now = func() time.Time { return clock }
	v := &Verifier{byIssuer: map[string]*oidc.IDTokenVerifier{
		internalIssuer: oidc.NewVerifier(internalIssuer, ks, &oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.RS256}}),
	}}
	return v, ks, g, &clock
}

// A token with a cached key is checked at once, even while another request's
// key-set fetch is stuck.
func TestCachedKeyDoesNotWaitBehindARefetch(t *testing.T) {
	known := newKey(t, "k1")
	v, _, g, clock := gatedVerifier(t, known)
	if _, err := v.Subject(context.Background(), sign(t, known, valid(internalIssuer))); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	g.gate.Store(&gate)
	defer close(gate)
	*clock = clock.Add(minRefresh)
	stranger := newKey(t, "unknown")
	go func() { _, _ = v.Subject(context.Background(), sign(t, stranger, valid(internalIssuer))) }()
	deadline := time.Now().Add(2 * time.Second)
	for g.hits.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if g.hits.Load() != 2 {
		t.Fatalf("the refetch did not start (fetches %d)", g.hits.Load())
	}
	start := time.Now()
	if _, err := v.Subject(context.Background(), sign(t, known, valid(internalIssuer))); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(start); waited > 500*time.Millisecond {
		t.Fatalf("a cached key waited %s behind a refetch", waited)
	}
}

// The request that triggered the first fetch gives up, yet the fetch completes
// and later requests use its keys without fetching again.
func TestCancelledTriggerDoesNotLeaveTheAPIWithoutKeys(t *testing.T) {
	key := newKey(t, "k1")
	v, ks, g, _ := gatedVerifier(t, key)
	gate := make(chan struct{})
	g.gate.Store(&gate)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := v.Subject(ctx, sign(t, key, valid(internalIssuer))); err == nil {
		t.Fatal("accepted before any key was fetched")
	}
	close(gate)
	g.gate.Store(nil)
	deadline := time.Now().Add(2 * time.Second)
	for {
		ks.mu.Lock()
		ready := ks.inflight == nil && ks.keys != nil
		ks.mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if sub, err := v.Subject(context.Background(), sign(t, key, valid(internalIssuer))); err != nil || sub == "" {
		t.Fatalf("no keys after the cancelled trigger (err %v)", err)
	}
	if got := g.hits.Load(); got != 1 {
		t.Fatalf("key set fetched %d times, want 1", got)
	}
}
