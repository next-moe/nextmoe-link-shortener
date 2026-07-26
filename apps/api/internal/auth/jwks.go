package auth

import (
	"context"
	"crypto"
	"crypto/ecdh"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// ErrKeyUnavailable marks a verification failure caused by the JWKS being
// unreachable (the fetch failed) — the RP's problem, not the token's. Callers
// fail closed (401) but can treat it as transient rather than as a bad token.
var ErrKeyUnavailable = errors.New("auth: signing keys unavailable")

// publicKeyFromJWK reconstructs a crypto.PublicKey from a public JWK. EC keys
// go via crypto/ecdh + a PKIX round-trip (validates the point is on-curve,
// avoids the deprecated ecdsa.X/Y fields); RSA keys are built directly.
func publicKeyFromJWK(jwk map[string]any) (crypto.PublicKey, error) {
	kty, _ := jwk["kty"].(string)
	switch kty {
	case "EC":
		if crv, _ := jwk["crv"].(string); crv != "P-256" {
			return nil, fmt.Errorf("auth: unsupported EC crv %q", crv)
		}
		xs, _ := jwk["x"].(string)
		ys, _ := jwk["y"].(string)
		x, err := base64.RawURLEncoding.DecodeString(xs)
		if err != nil {
			return nil, fmt.Errorf("auth: bad EC x: %w", err)
		}
		y, err := base64.RawURLEncoding.DecodeString(ys)
		if err != nil {
			return nil, fmt.Errorf("auth: bad EC y: %w", err)
		}
		point := make([]byte, 0, 1+len(x)+len(y))
		point = append(point, 0x04) // uncompressed point marker
		point = append(point, x...)
		point = append(point, y...)
		ecdhPub, err := ecdh.P256().NewPublicKey(point) // validates on-curve
		if err != nil {
			return nil, fmt.Errorf("auth: bad EC point: %w", err)
		}
		der, err := x509.MarshalPKIXPublicKey(ecdhPub)
		if err != nil {
			return nil, err
		}
		return x509.ParsePKIXPublicKey(der) // -> *ecdsa.PublicKey
	case "RSA":
		ns, _ := jwk["n"].(string)
		es, _ := jwk["e"].(string)
		n, err := base64.RawURLEncoding.DecodeString(ns)
		if err != nil {
			return nil, fmt.Errorf("auth: bad RSA n: %w", err)
		}
		e, err := base64.RawURLEncoding.DecodeString(es)
		if err != nil {
			return nil, fmt.Errorf("auth: bad RSA e: %w", err)
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}, nil
	default:
		return nil, fmt.Errorf("auth: unsupported kty %q", kty)
	}
}

// parseJWKSet parses a JWK Set ({"keys":[...]}) into a kid -> public key map.
// Malformed/unsupported entries are skipped (best-effort) so one bad key can't
// void the whole set.
func parseJWKSet(raw []byte) (map[string]crypto.PublicKey, error) {
	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, err
	}
	out := make(map[string]crypto.PublicKey, len(set.Keys))
	for _, jwk := range set.Keys {
		kid, _ := jwk["kid"].(string)
		if kid == "" {
			continue
		}
		pub, err := publicKeyFromJWK(jwk)
		if err != nil {
			continue
		}
		out[kid] = pub
	}
	return out, nil
}

// JWKSResolver fetches + caches the OP's JWK Set over HTTP and resolves a JWS
// `kid` to its public key. On a cache miss (an unknown kid — the fingerprint
// of a key rotation) it refetches ONCE, throttled to minRefresh so a burst of
// unknown-kid tokens can't hammer the OP.
type JWKSResolver struct {
	url        string
	httpc      *http.Client
	minRefresh time.Duration

	mu        sync.RWMutex
	keys      map[string]crypto.PublicKey
	lastFetch time.Time
}

// NewJWKSResolver creates a resolver for the discovered JWKS URI.
func NewJWKSResolver(url string) *JWKSResolver {
	return &JWKSResolver{
		url:        url,
		httpc:      &http.Client{Timeout: 5 * time.Second},
		minRefresh: 30 * time.Second,
	}
}

// Key returns the public key for a kid, refetching the JWK Set once on a miss.
// A fetch failure is wrapped in ErrKeyUnavailable so the caller fails closed
// but treats it as transient, distinct from an unknown kid.
func (r *JWKSResolver) Key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	r.mu.RLock()
	k := r.keys[kid]
	r.mu.RUnlock()
	if k != nil {
		return k, nil
	}
	if err := r.refetch(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyUnavailable, err)
	}
	r.mu.RLock()
	k = r.keys[kid]
	r.mu.RUnlock()
	if k == nil {
		return nil, fmt.Errorf("auth: unknown kid %q", kid)
	}
	return k, nil
}

func (r *JWKSResolver) refetch(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Another goroutine may have refreshed while we waited for the lock, or a
	// recent refetch may have already run — throttle to minRefresh. The very
	// first fetch (keys == nil) always proceeds.
	if r.keys != nil && time.Since(r.lastFetch) < r.minRefresh {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	resp, err := r.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: jwks fetch status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	keys, err := parseJWKSet(body)
	if err != nil {
		return err
	}
	r.keys = keys
	r.lastFetch = time.Now()
	return nil
}
