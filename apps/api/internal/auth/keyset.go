package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// minRefresh bounds how often the key set is fetched. A token naming an
// unknown key id triggers at most one fetch per interval, so unauthenticated
// requests cannot make the API hammer Keycloak.
const minRefresh = 10 * time.Second

// fetchTimeout bounds one key-set fetch. Fetches run on a background context,
// so a cancelled request never cancels the fetch other requests wait on.
const fetchTimeout = 10 * time.Second

// keySet is the oidc.KeySet the verifiers share: RS256 only, keys cached by
// key id, refreshed lazily and at most once per minRefresh. One fetch runs at
// a time, outside the lock, so a cached key never waits behind a fetch.
type keySet struct {
	url      string
	client   *http.Client
	interval time.Duration
	now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*jose.JSONWebKey
	lastFetch time.Time
	lastErr   error
	inflight  chan struct{} // closed when the running fetch ends; nil when none runs
	fetches   int
}

func newKeySet(url string, client *http.Client) *keySet {
	return &keySet{url: url, client: client, interval: minRefresh, now: time.Now}
}

// VerifySignature implements oidc.KeySet: it returns the payload only when
// the single RS256 signature verifies against a known key.
func (s *keySet) VerifySignature(ctx context.Context, raw string) ([]byte, error) {
	jws, err := jose.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if len(jws.Signatures) != 1 || jws.Signatures[0].Header.KeyID == "" {
		return nil, errors.New("token must carry exactly one signature with a key id")
	}
	key, err := s.key(ctx, jws.Signatures[0].Header.KeyID)
	if err != nil {
		return nil, err
	}
	return jws.Verify(key)
}

func (s *keySet) key(ctx context.Context, kid string) (*jose.JSONWebKey, error) {
	for waited := false; ; waited = true {
		s.mu.Lock()
		if key, ok := s.keys[kid]; ok {
			s.mu.Unlock()
			return key, nil
		}
		done := s.inflight
		if done == nil {
			if waited || (!s.lastFetch.IsZero() && s.now().Sub(s.lastFetch) < s.interval) {
				err := s.lastErr
				s.mu.Unlock()
				if err != nil {
					return nil, fmt.Errorf("unknown signing key %q (last key-set fetch: %w)", kid, err)
				}
				return nil, fmt.Errorf("unknown signing key %q", kid)
			}
			s.lastFetch = s.now()
			s.fetches++
			done = make(chan struct{})
			s.inflight = done
			go s.refresh(done)
		}
		s.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// refresh fetches the key set on a background context and wakes the waiters.
func (s *keySet) refresh(done chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	keys, err := s.fetch(ctx)
	s.mu.Lock()
	if err == nil {
		s.keys = keys
	}
	s.lastErr = err
	s.inflight = nil
	s.mu.Unlock()
	close(done)
}

func (s *keySet) fetch(ctx context.Context) (map[string]*jose.JSONWebKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("key set request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch key set: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch key set: status %d", resp.StatusCode)
	}
	var set jose.JSONWebKeySet
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode key set: %w", err)
	}
	keys := map[string]*jose.JSONWebKey{}
	for i := range set.Keys {
		key := set.Keys[i]
		if key.KeyID == "" || !key.IsPublic() || (key.Use != "" && key.Use != "sig") {
			continue
		}
		keys[key.KeyID] = &key
	}
	return keys, nil
}
