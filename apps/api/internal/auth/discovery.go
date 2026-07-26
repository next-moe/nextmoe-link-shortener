package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider is the subset of the OpenID Provider metadata this RP consumes
// (OIDC Discovery §3). Every endpoint the RP calls is taken from here — the RP
// hardcodes NO IdP path, so a path change on the OP is picked up by re-running
// discovery, not by editing this repo.
type Provider struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// Discover fetches {issuer}/.well-known/openid-configuration and returns the
// provider metadata. It is called once at startup and the result cached for
// the process lifetime (endpoints are stable; JWKS rotation is handled
// separately by JWKSResolver). It fails loud: a discovery failure or a missing
// required endpoint returns an error so the caller refuses to start rather
// than running a half-configured RP.
func Discover(ctx context.Context, httpc *http.Client, issuer string) (*Provider, error) {
	if httpc == nil {
		httpc = &http.Client{Timeout: 10 * time.Second}
	}
	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery status %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var p Provider
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("oidc discovery decode: %w", err)
	}
	// OIDC Discovery §4.3: the issuer in the document MUST match the requested
	// issuer, or a malicious server could point us at attacker endpoints.
	if strings.TrimRight(p.Issuer, "/") != strings.TrimRight(issuer, "/") {
		return nil, fmt.Errorf("oidc discovery issuer mismatch: got %q, want %q", p.Issuer, issuer)
	}
	if p.AuthorizationEndpoint == "" || p.TokenEndpoint == "" || p.JWKSURI == "" {
		return nil, fmt.Errorf("oidc discovery missing required endpoints: %+v", p)
	}
	return &p, nil
}
