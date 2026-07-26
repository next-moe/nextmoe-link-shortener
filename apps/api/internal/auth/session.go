package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// SessionCookieName is shortlink-unique on purpose: in local dev several
	// sibling ecosystem sites share 127.0.0.1 + Redis, and cookies are not
	// scoped by port — a shared name would bleed sessions across sites.
	SessionCookieName = "shortlink_session"
	keyPrefix         = "shortlink:session:"
	lockPrefix        = "shortlink:lock:refresh:"
	// SessionTTL is how long a login survives in Redis without activity. The
	// httpOnly cookie's Max-Age is set to match.
	SessionTTL = 30 * 24 * time.Hour
)

// refreshSkew triggers a transparent refresh a little before the access
// token's actual expiry, so an in-flight request doesn't race the boundary.
const refreshSkew = 30 * time.Second

// ErrPermanent means the OP permanently rejected the refresh (revoked/expired
// grant) — the caller must drop the session and re-login the user.
var ErrPermanent = errors.New("auth: oauth permanently rejected refresh")

// Session is the server-side login state. It holds the OP tokens (BFF-only:
// never sent to the browser, never persisted to the business DB, never
// logged) plus the identity claims decoded from the verified access token.
type Session struct {
	UserID    int64    `json:"user_id"`
	Sub       string   `json:"sub"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	SiteRoles []string `json:"site_roles"`
	// Tokens live only here (Redis, server-side). Roles are re-read on every
	// refresh so grants/revokes take effect mid-session.
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// EffectiveRoles is the identity's role set: the union of the cross-site
// ecosystem roles and the site-scoped roles, de-duplicated.
func (s *Session) EffectiveRoles() []string {
	seen := make(map[string]struct{}, len(s.Roles)+len(s.SiteRoles))
	out := make([]string, 0, len(s.Roles)+len(s.SiteRoles))
	for _, r := range append(append([]string{}, s.Roles...), s.SiteRoles...) {
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out
}

// SessionStore persists sessions in Redis and turns access tokens into
// trusted claims via the Verifier. It is the single point where a token
// becomes a trusted session, so the create (login) and refresh paths share
// one verification policy.
type SessionStore struct {
	rdb      *redis.Client
	verifier *Verifier
}

// NewSessionStore builds a store. The verifier is required for Claims to
// work; pass nil only for storage-only tests that never call Claims.
func NewSessionStore(rdb *redis.Client, verifier *Verifier) *SessionStore {
	return &SessionStore{rdb: rdb, verifier: verifier}
}

// Claims verifies an access token (JWKS signature + iss/aud/exp) and returns
// its claims. Fail-closed: a verify error returns no claims — callers MUST
// NOT fall back to unverified decoding.
func (s *SessionStore) Claims(ctx context.Context, token string) (*Claims, error) {
	if s.verifier == nil {
		return nil, errors.New("auth: session store has no verifier configured")
	}
	return s.verifier.Verify(ctx, token)
}

func newSID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Create stores a new session under a fresh random id and returns the id.
func (s *SessionStore) Create(ctx context.Context, sess *Session) (string, error) {
	sid := newSID()
	return sid, s.Save(ctx, sid, sess)
}

// Save writes a session under sid with the standard TTL.
func (s *SessionStore) Save(ctx context.Context, sid string, sess *Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyPrefix+sid, data, SessionTTL).Err()
}

// Get returns the session for a sid, or (nil, nil) when it does not exist.
func (s *SessionStore) Get(ctx context.Context, sid string) (*Session, error) {
	data, err := s.rdb.Get(ctx, keyPrefix+sid).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// Delete removes a session.
func (s *SessionStore) Delete(ctx context.Context, sid string) error {
	return s.rdb.Del(ctx, keyPrefix+sid).Err()
}

// RefreshSession rotates the access token. Concurrent requests can hit expiry
// together; only one wins the refresh lock — the losers WAIT for the winner
// to write the refreshed session instead of racing (rotating refresh tokens
// make a double refresh invalidate each other; see the ecosystem OAuth guide
// §4.4: lock losers must poll for the winner, never clear the cookie).
func RefreshSession(ctx context.Context, store *SessionStore, oauth *OAuthClient, sid string, sess *Session) (*Session, error) {
	got, err := store.rdb.SetNX(ctx, lockPrefix+sid, "1", 10*time.Second).Result()
	if err != nil {
		return nil, err
	}
	if !got {
		// Lock contention — poll for the winner to write the refreshed session.
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
			fresh, err := store.Get(ctx, sid)
			if err != nil {
				return nil, err
			}
			if fresh == nil {
				return nil, ErrPermanent // winner deleted it (permanent reject)
			}
			if fresh.ExpiresAt.After(sess.ExpiresAt) {
				return fresh, nil // winner refreshed
			}
		}
		return sess, nil // timed out; keep current, let the next request retry
	}
	defer store.rdb.Del(ctx, lockPrefix+sid)

	tok, err := oauth.Refresh(ctx, sess.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrOAuthRejected) {
			return nil, fmt.Errorf("%w: %v", ErrPermanent, err)
		}
		// Transient (OP unreachable) → keep the session so the next request
		// retries; this refresh just failed.
		return nil, err
	}
	// Verify the refreshed token before trusting its claims. A verify failure
	// here is transient (the refresh_token itself is still valid).
	claims, err := store.Claims(ctx, tok.AccessToken)
	if err != nil {
		return nil, err
	}
	sess.AccessToken = tok.AccessToken
	sess.RefreshToken = tok.RefreshToken
	sess.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	sess.UserID = claims.UserID
	sess.Name = claims.Name
	sess.Email = claims.Email
	sess.Roles = claims.Roles
	sess.SiteRoles = claims.SiteRoles // re-read every refresh so grants take effect
	if err := store.Save(ctx, sid, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// SessionResolver is the production Resolver: it reads the httpOnly session
// cookie, loads the server-side session from Redis, transparently refreshes
// an expired access token, and returns the Identity. Tokens never leave this
// path — the browser only ever holds the opaque session id.
type SessionResolver struct {
	store *SessionStore
	oauth *OAuthClient
}

// NewSessionResolver builds the resolver over a session store and OAuth
// client.
func NewSessionResolver(store *SessionStore, oauth *OAuthClient) *SessionResolver {
	return &SessionResolver{store: store, oauth: oauth}
}

// Resolve maps the session cookie to an Identity. No cookie, an
// unknown/expired session, or a revoked grant all resolve to anonymous
// (nil, nil) — the caller simply appears logged out.
func (r *SessionResolver) Resolve(ctx context.Context, req Request) (*Identity, error) {
	sid := req.Cookie(SessionCookieName)
	if sid == "" {
		return nil, nil // anonymous
	}
	sess, err := r.store.Get(ctx, sid)
	if err != nil {
		return nil, err // Redis blip → treated as anonymous, fail-closed
	}
	if sess == nil {
		return nil, nil // logged out / expired session
	}
	if time.Now().Before(sess.ExpiresAt.Add(-refreshSkew)) {
		return identityFrom(sess), nil // access token still fresh
	}

	// Access token at/near expiry → transparent refresh (also re-reads roles).
	refreshed, err := RefreshSession(ctx, r.store, r.oauth, sid, sess)
	if err != nil {
		if errors.Is(err, ErrPermanent) {
			_ = r.store.Delete(ctx, sid)
			return nil, nil // OP revoked the grant → logged out everywhere
		}
		// Transient (OP unreachable). The engine does not need the access
		// token — the cached identity is still valid — so keep serving it
		// rather than logging everyone out on an OP blip.
		return identityFrom(sess), nil
	}
	return identityFrom(refreshed), nil
}

func identityFrom(sess *Session) *Identity {
	return &Identity{UserID: sess.UserID, Name: sess.Name, Roles: sess.EffectiveRoles()}
}
