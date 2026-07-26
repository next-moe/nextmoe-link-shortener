// Package httpapi builds the shortlink HTTP API: a Huma (code-first OpenAPI
// 3.1) layer over Fiber v3. One builder feeds both the running server and the
// cmd/spec OpenAPI export, so the committed spec can never drift from the
// code. The /s/{alias} redirect is plain Fiber (RegisterRedirect) — it is a
// browser-facing 302, not a JSON operation.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/auth"
	"github.com/kungal/kungal-link-shortener/apps/api/internal/engine"
	"github.com/kungal/kungal-link-shortener/apps/api/internal/model"
)

// Version is the build version, overridable at build time via -ldflags.
var Version = "dev"

// Deps are the runtime dependencies handlers close over. All may be nil/zero
// for spec generation (cmd/spec), where no handler runs.
type Deps struct {
	Engine   *engine.Engine
	Resolver auth.Resolver
	Auth     *AuthBackend
	// AdminRoles gate every dashboard endpoint (JWT roles ∩ AdminRoles).
	AdminRoles []string
	// PublicBaseURL is the advertised origin for short URLs.
	PublicBaseURL string
}

// handlers holds the dependencies the operation handlers delegate to.
type handlers struct {
	engine        *engine.Engine
	auth          *AuthBackend
	adminRoles    []string
	publicBaseURL string
}

// NewAPI mounts the Huma API on the Fiber app, installs the identity
// middleware, and registers every operation. It returns the huma.API so
// callers (cmd/spec) can export the OpenAPI document.
func NewAPI(app *fiber.App, deps Deps) huma.API {
	cfg := huma.DefaultConfig("KunGal Link Shortener API", "1.0.0")
	api := humafiber.New(app, cfg)

	resolver := deps.Resolver
	if resolver == nil {
		resolver = auth.DenyResolver{}
	}
	api.UseMiddleware(auth.Middleware(resolver))

	registerSystem(api)
	h := &handlers{
		engine:        deps.Engine,
		auth:          deps.Auth,
		adminRoles:    deps.AdminRoles,
		publicBaseURL: deps.PublicBaseURL,
	}
	h.registerAuth(api)
	h.registerLinks(api)
	h.registerKeys(api)
	h.registerS2S(api)
	return api
}

// requireAdmin is the dashboard gate every /links and /keys handler calls.
func (h *handlers) requireAdmin(ctx context.Context) (*auth.Identity, error) {
	return auth.RequireRole(ctx, h.adminRoles)
}

// shortURL builds the advertised short URL for an alias.
func (h *handlers) shortURL(alias string) string {
	return h.publicBaseURL + "/s/" + alias
}

func registerSystem(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "healthz",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Liveness probe",
		Description: "Reports that the process is up. Does not touch the database.",
		Tags:        []string{"system"},
	}, healthz)
}

// HealthOutput is the /healthz response body.
type HealthOutput struct {
	Body struct {
		Status  string `json:"status" doc:"Liveness status" example:"ok"`
		Version string `json:"version" doc:"Build version" example:"dev"`
	}
}

// healthz is a pure liveness handler: it must never touch the database, so a
// transient dependency blip cannot fail the process's own probe.
func healthz(_ context.Context, _ *struct{}) (*HealthOutput, error) {
	out := &HealthOutput{}
	out.Body.Status = "ok"
	out.Body.Version = Version
	return out, nil
}

// LinkDTO is the wire shape of a short link (dashboard + S2S read).
type LinkDTO struct {
	ID             int64      `json:"id"`
	Alias          string     `json:"alias"`
	ShortURL       string     `json:"short_url" doc:"Advertised short URL (public base + /s/alias)"`
	DestinationURL string     `json:"destination_url"`
	Description    string     `json:"description"`
	Status         int16      `json:"status" doc:"0=active 1=disabled 2=archived"`
	ExpiresAt      *time.Time `json:"expires_at"`
	MaxVisits      int64      `json:"max_visits" doc:"0 = unlimited"`
	VisitCount     int64      `json:"visit_count"`
	LastVisitedAt  *time.Time `json:"last_visited_at"`
	ForwardParams  bool       `json:"forward_params"`
	CreatedBy      int64      `json:"created_by" doc:"IdP user id; 0 for S2S-created links"`
	CreatedVia     string     `json:"created_via" doc:"\"dashboard\" or the S2S API key name"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (h *handlers) toLinkDTO(l *model.ShortLink) LinkDTO {
	return LinkDTO{
		ID:             l.ID,
		Alias:          l.Alias,
		ShortURL:       h.shortURL(l.Alias),
		DestinationURL: l.DestinationURL,
		Description:    l.Description,
		Status:         l.Status,
		ExpiresAt:      l.ExpiresAt,
		MaxVisits:      l.MaxVisits,
		VisitCount:     l.VisitCount,
		LastVisitedAt:  l.LastVisitedAt,
		ForwardParams:  l.ForwardParams,
		CreatedBy:      l.CreatedBy,
		CreatedVia:     l.CreatedVia,
		CreatedAt:      l.CreatedAt,
	}
}
