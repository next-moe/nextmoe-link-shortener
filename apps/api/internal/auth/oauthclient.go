package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrOAuthRejected means the token endpoint PERMANENTLY rejected the request
// (an OAuth 4xx error such as invalid_grant — a revoked/expired code or
// refresh token). It is distinct from a transient transport error (timeout /
// OP 5xx), so the refresh path can drop the session only on permanent
// rejection.
var ErrOAuthRejected = errors.New("auth: oauth token endpoint rejected the request")

// OAuthClient is the RP's confidential-client caller for the OP token
// endpoint. The endpoint URL comes from discovery — never hardcoded. Requests
// are standard OAuth 2.0: form-encoded bodies with client_secret_basic
// authentication. The client secret is held here and sent only in the
// Authorization header to the OP over TLS; it is never logged.
type OAuthClient struct {
	http          *http.Client
	tokenEndpoint string
	clientID      string
	clientSecret  string
	redirectURI   string
}

// NewOAuthClient builds a client for the discovered token endpoint.
func NewOAuthClient(tokenEndpoint, clientID, clientSecret, redirectURI string) *OAuthClient {
	return &OAuthClient{
		http:          &http.Client{Timeout: 10 * time.Second},
		tokenEndpoint: tokenEndpoint,
		clientID:      clientID,
		clientSecret:  clientSecret,
		redirectURI:   redirectURI,
	}
}

// TokenResponse is the standard OAuth 2.0 token endpoint success body
// (RFC 6749 §5.1). The id_token is ignored — identity comes from the verified
// access token (RFC 9068), not the id_token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// tokenError is the standard OAuth 2.0 error body (RFC 6749 §5.2). Neither
// field carries a token or secret, so it is safe to surface in an error.
type tokenError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

// Exchange swaps an authorization code (+ PKCE verifier) for tokens.
func (c *OAuthClient) Exchange(ctx context.Context, code, codeVerifier string) (*TokenResponse, error) {
	return c.postToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.redirectURI},
		"code_verifier": {codeVerifier},
	})
}

// Refresh rotates the refresh token for a fresh access token. The OP rotates
// refresh tokens, so the old one is invalidated — callers must persist the
// returned RefreshToken.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	return c.postToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

// postToken performs a form-encoded, client_secret_basic-authenticated POST to
// the token endpoint. It classifies failures: an OAuth 4xx error body is a
// permanent rejection (ErrOAuthRejected); a 5xx or transport failure is
// transient. No token or secret is ever included in a returned error.
func (c *OAuthClient) postToken(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(url.QueryEscape(c.clientID), url.QueryEscape(c.clientSecret)) // client_secret_basic

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: token endpoint request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		var tok TokenResponse
		if err := json.Unmarshal(body, &tok); err != nil {
			return nil, fmt.Errorf("auth: token response decode: %w", err)
		}
		if tok.AccessToken == "" {
			return nil, errors.New("auth: token response missing access_token")
		}
		return &tok, nil
	}

	// A 4xx is a permanent OAuth rejection; a 5xx is transient (OP trouble).
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		var te tokenError
		_ = json.Unmarshal(body, &te)
		return nil, fmt.Errorf("%w: %s %q (%s)", ErrOAuthRejected, te.Code, te.Description, resp.Status)
	}
	return nil, fmt.Errorf("auth: token endpoint status %d", resp.StatusCode)
}
