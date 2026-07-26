package auth

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// clockSkew is the tolerance applied to exp/nbf so a small clock difference
// between the OP and this service doesn't spuriously reject a just-issued
// token.
const clockSkew = 30 * time.Second

// Claims is the subset of the OP access-token JWT (RFC 9068 style) this RP
// reads. UserID is the integer `id`; Sub is the stable UUID. Roles are the
// cross-site ecosystem roles; SiteRoles are the OP's roles scoped to this
// site. ClientID (RFC 9068 §2.2) is the audience binding the Verifier checks.
type Claims struct {
	UserID    int64    `json:"id"`
	Sub       string   `json:"sub"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	SiteRoles []string `json:"site_roles"`
	ClientID  string   `json:"client_id"`
	jwt.RegisteredClaims
}

// KeyResolver resolves a JWS `kid` to the public key that verifies tokens
// signed with it. *JWKSResolver is the production implementation.
type KeyResolver interface {
	Key(ctx context.Context, kid string) (crypto.PublicKey, error)
}

// Verifier validates OP access tokens by their asymmetric signature (the OP's
// ES256 — RS256 tolerated — JWKS keys) plus iss / exp / nbf / audience. It
// accepts ES256 and RS256 only and rejects HS256 and `none`: this RP holds no
// shared HMAC secret, so an HS256 token here could only be "verified" against
// nothing — always a hard rejection, never a silent accept.
type Verifier struct {
	resolver KeyResolver
	issuer   string // expected `iss` — the OP issuer URL
	audience string // this site's own OAuth client_id (expected `client_id`)
}

// NewVerifier builds a Verifier.
func NewVerifier(resolver KeyResolver, issuer, audience string) *Verifier {
	return &Verifier{resolver: resolver, issuer: issuer, audience: audience}
}

// Verify validates the token's signature (via the JWKS public key selected by
// `kid`) and its iss / exp / nbf / audience, returning the parsed claims. It
// fails closed: any failure returns an error and NO claims.
func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	keyfunc := func(t *jwt.Token) (any, error) {
		switch t.Method.(type) {
		case *jwt.SigningMethodECDSA, *jwt.SigningMethodRSA:
			kid, _ := t.Header["kid"].(string)
			if kid == "" {
				return nil, fmt.Errorf("auth: token missing kid")
			}
			return v.resolver.Key(ctx, kid)
		default:
			return nil, fmt.Errorf("auth: unexpected signing method %v (asymmetric only)", t.Header["alg"])
		}
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, keyfunc,
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithLeeway(clockSkew),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(v.issuer),
	)
	if err != nil {
		return nil, err
	}
	if claims.ClientID != v.audience {
		return nil, fmt.Errorf("auth: token client_id %q is not this client", claims.ClientID)
	}
	return claims, nil
}
