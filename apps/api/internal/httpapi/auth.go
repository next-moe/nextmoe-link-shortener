package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/auth"
)

// AuthBackend carries the OIDC runtime dependencies the /auth endpoints need.
// It is nil in Deps for spec generation and in dev when OIDC is not
// configured: the endpoints still register (so the OpenAPI document is
// complete) but return 503 at runtime. It is never nil in prod (config
// enforces it).
type AuthBackend struct {
	OAuth        *auth.OAuthClient
	Store        *auth.SessionStore
	Provider     *auth.Provider
	ClientID     string
	RedirectURI  string
	Scopes       string // space-delimited, e.g. "openid profile"
	CookieSecure bool   // Secure cookie flag (true in prod / https)
}

var errAuthUnconfigured = huma.Error503ServiceUnavailable("authentication is not configured")

// registerAuth wires the session BFF endpoints: begin login, exchange a code
// for a session, read the current identity, and log out.
func (h *handlers) registerAuth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodGet,
		Path:        "/auth/login",
		Summary:     "Begin OIDC login",
		Description: "Returns the discovered authorization endpoint and public client parameters the browser needs to start the PKCE authorization-code flow. Contains no secrets.",
		Tags:        []string{"auth"},
	}, h.authLogin)

	huma.Register(api, huma.Operation{
		OperationID: "create-session",
		Method:      http.MethodPost,
		Path:        "/auth/session",
		Summary:     "Exchange an authorization code for a session",
		Description: "Exchanges {code, code_verifier} at the OP token endpoint, verifies the access token, stores the tokens in a server-side session, and sets the httpOnly session cookie. The browser never receives tokens.",
		Tags:        []string{"auth"},
	}, h.createSession)

	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/auth/me",
		Summary:     "Current identity",
		Description: "Returns the authenticated identity resolved from the session cookie, or 401 when anonymous.",
		Tags:        []string{"auth"},
	}, h.getMe)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-session",
		Method:        http.MethodDelete,
		Path:          "/auth/session",
		Summary:       "Log out",
		Description:   "Deletes the server-side session and clears the session cookie. Idempotent.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusNoContent,
	}, h.deleteSession)
}

// ---- begin login ----

type authLoginOutput struct {
	Body struct {
		AuthorizationEndpoint string `json:"authorization_endpoint" doc:"OP authorization endpoint (from discovery)"`
		ClientID              string `json:"client_id"`
		RedirectURI           string `json:"redirect_uri"`
		Scope                 string `json:"scope" doc:"space-delimited scopes to request"`
	}
}

func (h *handlers) authLogin(_ context.Context, _ *struct{}) (*authLoginOutput, error) {
	if h.auth == nil {
		return nil, errAuthUnconfigured
	}
	out := &authLoginOutput{}
	out.Body.AuthorizationEndpoint = h.auth.Provider.AuthorizationEndpoint
	out.Body.ClientID = h.auth.ClientID
	out.Body.RedirectURI = h.auth.RedirectURI
	out.Body.Scope = h.auth.Scopes
	return out, nil
}

// ---- create session (login) ----

type createSessionInput struct {
	Body struct {
		Code         string `json:"code" doc:"authorization code from the OP redirect"`
		CodeVerifier string `json:"code_verifier" doc:"PKCE verifier matching the sent challenge"`
	}
}

type createSessionOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      struct {
		UserID  int64    `json:"user_id"`
		Name    string   `json:"name"`
		Roles   []string `json:"roles"`
		IsAdmin bool     `json:"is_admin"`
	}
}

func (h *handlers) createSession(ctx context.Context, in *createSessionInput) (*createSessionOutput, error) {
	if h.auth == nil {
		return nil, errAuthUnconfigured
	}
	if in.Body.Code == "" || in.Body.CodeVerifier == "" {
		return nil, huma.Error400BadRequest("code and code_verifier are required")
	}

	// Errors below carry no token material — safe to log.
	tok, err := h.auth.OAuth.Exchange(ctx, in.Body.Code, in.Body.CodeVerifier)
	if err != nil {
		slog.Warn("oidc code exchange failed", "error", err)
		return nil, huma.Error401Unauthorized("login failed")
	}
	claims, err := h.auth.Store.Claims(ctx, tok.AccessToken)
	if err != nil {
		slog.Warn("oidc access-token verification failed", "error", err)
		return nil, huma.Error401Unauthorized("login failed")
	}

	sess := &auth.Session{
		UserID:       claims.UserID,
		Sub:          claims.Sub,
		Name:         claims.Name,
		Email:        claims.Email,
		Roles:        claims.Roles,
		SiteRoles:    claims.SiteRoles,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
	}
	sid, err := h.auth.Store.Create(ctx, sess)
	if err != nil {
		slog.Error("session create failed", "error", err)
		return nil, huma.Error500InternalServerError("could not create session")
	}

	id := auth.Identity{UserID: sess.UserID, Name: sess.Name, Roles: sess.EffectiveRoles()}
	out := &createSessionOutput{SetCookie: h.sessionCookie(sid)}
	out.Body.UserID = id.UserID
	out.Body.Name = id.Name
	out.Body.Roles = id.Roles
	out.Body.IsAdmin = id.HasAnyRole(h.adminRoles)
	return out, nil
}

// ---- current identity ----

type meOutput struct {
	Body struct {
		UserID  int64    `json:"user_id"`
		Name    string   `json:"name"`
		Roles   []string `json:"roles"`
		IsAdmin bool     `json:"is_admin"`
	}
}

// getMe reads the Identity the resolver already seated from the session
// cookie — it does not touch the auth backend, so it works under any
// resolver (including the dev header).
func (h *handlers) getMe(ctx context.Context, _ *struct{}) (*meOutput, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	out := &meOutput{}
	out.Body.UserID = id.UserID
	out.Body.Name = id.Name
	out.Body.Roles = id.Roles
	out.Body.IsAdmin = id.HasAnyRole(h.adminRoles)
	return out, nil
}

// ---- logout ----

type deleteSessionInput struct {
	Cookie string `cookie:"shortlink_session"`
}

type deleteSessionOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

// deleteSession deletes the server-side session and clears the cookie. It
// always clears the cookie (idempotent logout), even when the backend is
// unconfigured.
func (h *handlers) deleteSession(ctx context.Context, in *deleteSessionInput) (*deleteSessionOutput, error) {
	if h.auth != nil && in.Cookie != "" {
		if err := h.auth.Store.Delete(ctx, in.Cookie); err != nil {
			slog.Warn("session delete failed", "error", err)
		}
	}
	return &deleteSessionOutput{SetCookie: h.clearCookie()}, nil
}

// ---- cookie helpers ----

// sessionCookie builds the httpOnly session cookie. SameSite=Lax is the CSRF
// posture: the cookie is not sent on cross-site subrequests, while top-level
// navigations back from the OP still carry it. Secure is set in prod.
func (h *handlers) sessionCookie(sid string) http.Cookie {
	return http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sid,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *handlers) clearCookie() http.Cookie {
	secure := false
	if h.auth != nil {
		secure = h.auth.CookieSecure
	}
	return http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}
