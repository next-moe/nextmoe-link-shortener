// Package auth is the identity seam, ported from the ecosystem's standard-RP
// shape (kun-softmoe). A Huma middleware resolves each request into an
// *Identity and stashes it in the request context; dashboard handlers require
// it plus an admin role. The seam point: swap the Resolver, never touch the
// engine — dev uses a header backdoor, prod uses the cookie→Redis→IdP session.
package auth

import (
	"context"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Identity is the resolved caller. UserID is the global IdP user id; Name is
// the display name from the verified claims; Roles are the ecosystem role
// claims (global ∪ site roles).
type Identity struct {
	UserID int64
	Name   string
	Roles  []string
}

// HasAnyRole reports whether the identity holds at least one of the wanted
// roles.
func (id *Identity) HasAnyRole(wanted []string) bool {
	for _, r := range id.Roles {
		for _, w := range wanted {
			if r == w {
				return true
			}
		}
	}
	return false
}

// Request is the minimal read-only view of an inbound request a Resolver
// needs: one header value and one named cookie. Middleware adapts huma.Context
// to it, so resolvers stay adapter-agnostic.
type Request interface {
	Header(name string) string
	Cookie(name string) string
}

// Resolver turns an inbound request into an Identity. A nil identity with a
// nil error means "anonymous"; a non-nil error means the credentials were
// present but invalid.
type Resolver interface {
	Resolve(ctx context.Context, req Request) (*Identity, error)
}

type ctxKey struct{}

// Middleware resolves identity once per request and, when present, stores it
// in the context the handler receives. Resolution failures are swallowed here:
// they surface as 401 at Require, keeping "no identity" and "bad identity" on
// one path.
func Middleware(resolver Resolver) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if id, err := resolver.Resolve(ctx.Context(), humaRequest{ctx}); err == nil && id != nil {
			ctx = huma.WithValue(ctx, ctxKey{}, id)
		}
		next(ctx)
	}
}

// humaRequest adapts a huma.Context to the Request view a Resolver sees.
type humaRequest struct{ ctx huma.Context }

func (r humaRequest) Header(name string) string { return r.ctx.Header(name) }

func (r humaRequest) Cookie(name string) string {
	if c, err := huma.ReadCookie(r.ctx, name); err == nil && c != nil {
		return c.Value
	}
	return ""
}

// FromContext returns the resolved identity, if any.
func FromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(*Identity)
	return id, ok
}

// Require returns the caller's identity or a Huma 401 error.
func Require(ctx context.Context) (*Identity, error) {
	if id, ok := FromContext(ctx); ok {
		return id, nil
	}
	return nil, huma.Error401Unauthorized("authentication required")
}

// DevResolver accepts "Authorization: Dev <user_id> [role,role]" and trusts it
// blindly (roles default to "admin" when omitted — this is an admin tool, and
// the backdoor exists to exercise the dashboard endpoints). It is constructed
// ONLY when SHORTLINK_MODE=dev; it must never be wired in prod.
type DevResolver struct{}

// Resolve parses the dev header. A malformed value is an error (→ 401), not
// silently anonymous, so a typo'd dev token fails loud rather than degrading.
func (DevResolver) Resolve(_ context.Context, req Request) (*Identity, error) {
	const prefix = "Dev "
	authorization := req.Header("Authorization")
	if authorization == "" {
		return nil, nil // anonymous
	}
	if !strings.HasPrefix(authorization, prefix) {
		return nil, nil // not the dev scheme (Bearer rides through to S2S auth)
	}
	idPart, rolePart, _ := strings.Cut(strings.TrimSpace(authorization[len(prefix):]), " ")
	uid, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil || uid <= 0 {
		return nil, errBadDevToken
	}
	roles := []string{"admin"}
	if rolePart != "" {
		roles = strings.Split(rolePart, ",")
	}
	return &Identity{UserID: uid, Name: "dev", Roles: roles}, nil
}

// DenyResolver resolves every request to anonymous. It is the safe default in
// spec generation and any context without a real resolver.
type DenyResolver struct{}

// Resolve always returns anonymous.
func (DenyResolver) Resolve(context.Context, Request) (*Identity, error) {
	return nil, nil
}

// ChainResolver tries each resolver in order and returns the first identity
// found; a resolver that yields anonymous (nil, nil) falls through to the
// next. Dev wiring is Chain{SessionResolver, DevResolver}; prod wires the
// SessionResolver alone, so the dev backdoor cannot exist there.
type ChainResolver struct{ Resolvers []Resolver }

// Resolve walks the chain.
func (c ChainResolver) Resolve(ctx context.Context, req Request) (*Identity, error) {
	for _, r := range c.Resolvers {
		id, err := r.Resolve(ctx, req)
		if id != nil {
			return id, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

var errBadDevToken = huma.Error401Unauthorized("malformed dev authorization header")
