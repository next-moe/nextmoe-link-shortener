package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/engine"
)

// registerLinks wires the dashboard link CRUD + stats. Every handler is
// admin-gated (requireAdmin) — the dashboard has no non-admin surface.
func (h *handlers) registerLinks(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-links",
		Method:      http.MethodGet,
		Path:        "/links",
		Summary:     "List short links (newest first)",
		Tags:        []string{"links"},
	}, h.listLinks)

	huma.Register(api, huma.Operation{
		OperationID: "create-link",
		Method:      http.MethodPost,
		Path:        "/links",
		Summary:     "Create a short link",
		Tags:        []string{"links"},
	}, h.createLink)

	huma.Register(api, huma.Operation{
		OperationID: "link-stats",
		Method:      http.MethodGet,
		Path:        "/links/{alias}/stats",
		Summary:     "Visit stats for one link (hourly buckets + recent visits)",
		Tags:        []string{"links"},
	}, h.linkStats)

	huma.Register(api, huma.Operation{
		OperationID: "update-link",
		Method:      http.MethodPut,
		Path:        "/links/{id}",
		Summary:     "Update a short link (full editable field set)",
		Tags:        []string{"links"},
	}, h.updateLink)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-link",
		Method:        http.MethodDelete,
		Path:          "/links/{id}",
		Summary:       "Delete a short link (visits and buckets cascade)",
		Tags:          []string{"links"},
		DefaultStatus: http.StatusNoContent,
	}, h.deleteLink)
}

// ---- list ----

type linkListOutput struct {
	Body struct {
		Links []LinkDTO `json:"links"`
	}
}

func (h *handlers) listLinks(ctx context.Context, _ *struct{}) (*linkListOutput, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	links, err := h.engine.ListLinks()
	if err != nil {
		slog.Error("list links", "error", err)
		return nil, huma.Error500InternalServerError("could not list links")
	}
	out := &linkListOutput{}
	out.Body.Links = make([]LinkDTO, len(links))
	for i := range links {
		out.Body.Links[i] = h.toLinkDTO(&links[i])
	}
	return out, nil
}

// ---- create ----

// CreateLinkBody is shared by the dashboard create form and the S2S create.
type CreateLinkBody struct {
	DestinationURL string     `json:"destination_url" doc:"Absolute http(s) URL to redirect to"`
	Alias          string     `json:"alias,omitempty" doc:"Custom alias (4-32 chars of [A-Za-z0-9_-]); empty = random"`
	Description    string     `json:"description,omitempty" maxLength:"500"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty" doc:"RFC3339; null/absent = never expires"`
	MaxVisits      int64      `json:"max_visits,omitempty" minimum:"0" doc:"0 = unlimited"`
	ForwardParams  bool       `json:"forward_params,omitempty" doc:"Append the incoming query string to the destination"`
}

// validateDestination enforces an absolute http(s) destination.
func validateDestination(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("destination_url must be an absolute http(s) URL")
	}
	return nil
}

type createLinkInput struct {
	Body CreateLinkBody
}

type linkOutput struct {
	Body LinkDTO
}

func (h *handlers) createLink(ctx context.Context, in *createLinkInput) (*linkOutput, error) {
	id, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateDestination(in.Body.DestinationURL); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	link, _, err := h.engine.CreateLink(engine.CreateLinkParams{
		DestinationURL: in.Body.DestinationURL,
		Alias:          in.Body.Alias,
		Description:    in.Body.Description,
		ExpiresAt:      in.Body.ExpiresAt,
		MaxVisits:      in.Body.MaxVisits,
		ForwardParams:  in.Body.ForwardParams,
		CreatedBy:      id.UserID,
		CreatedVia:     "dashboard",
	})
	if err != nil {
		return nil, mapLinkErr("create link", err)
	}
	return &linkOutput{Body: h.toLinkDTO(link)}, nil
}

// ---- stats ----

type linkStatsInput struct {
	Alias string `path:"alias"`
	Range int    `query:"range" doc:"days of buckets to return (1-30, default 7)"`
}

// BucketDTO is one hourly aggregation row.
type BucketDTO struct {
	BucketStart time.Time `json:"bucket_start"`
	Visits      int64     `json:"visits"`
	UniqueIPs   int64     `json:"unique_ips"`
}

// VisitDTO is one recent-visit row.
type VisitDTO struct {
	ID        int64     `json:"id"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	IsUnique  bool      `json:"is_unique"`
	CreatedAt time.Time `json:"created_at"`
}

type linkStatsOutput struct {
	Body struct {
		Link           LinkDTO     `json:"link"`
		RangeDays      int         `json:"range_days"`
		UniqueVisitors int64       `json:"unique_visitors" doc:"all-time distinct IPs"`
		Buckets        []BucketDTO `json:"buckets"`
		Recent         []VisitDTO  `json:"recent"`
	}
}

func (h *handlers) linkStats(ctx context.Context, in *linkStatsInput) (*linkStatsOutput, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	stats, err := h.engine.LinkStats(in.Alias, in.Range)
	if err != nil {
		return nil, mapLinkErr("link stats", err)
	}
	out := &linkStatsOutput{}
	out.Body.Link = h.toLinkDTO(&stats.Link)
	out.Body.RangeDays = stats.RangeDays
	out.Body.UniqueVisitors = stats.UniqueVisitors
	out.Body.Buckets = make([]BucketDTO, len(stats.Buckets))
	for i, b := range stats.Buckets {
		out.Body.Buckets[i] = BucketDTO{BucketStart: b.BucketStart, Visits: b.Visits, UniqueIPs: b.UniqueIPs}
	}
	out.Body.Recent = make([]VisitDTO, len(stats.Recent))
	for i, v := range stats.Recent {
		out.Body.Recent[i] = VisitDTO{
			ID: v.ID, IP: v.IP, UserAgent: v.UserAgent,
			Referer: v.Referer, IsUnique: v.IsUnique, CreatedAt: v.CreatedAt,
		}
	}
	return out, nil
}

// ---- update ----

type updateLinkInput struct {
	ID   int64 `path:"id"`
	Body struct {
		DestinationURL string     `json:"destination_url"`
		Description    string     `json:"description" maxLength:"500"`
		Status         int16      `json:"status" minimum:"0" maximum:"2" doc:"0=active 1=disabled 2=archived"`
		ExpiresAt      *time.Time `json:"expires_at" doc:"null clears the expiry"`
		MaxVisits      int64      `json:"max_visits" minimum:"0"`
		ForwardParams  bool       `json:"forward_params"`
	}
}

func (h *handlers) updateLink(ctx context.Context, in *updateLinkInput) (*linkOutput, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := validateDestination(in.Body.DestinationURL); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	link, err := h.engine.UpdateLink(in.ID, engine.UpdateLinkParams{
		DestinationURL: in.Body.DestinationURL,
		Description:    in.Body.Description,
		Status:         in.Body.Status,
		ExpiresAt:      in.Body.ExpiresAt,
		MaxVisits:      in.Body.MaxVisits,
		ForwardParams:  in.Body.ForwardParams,
	})
	if err != nil {
		return nil, mapLinkErr("update link", err)
	}
	return &linkOutput{Body: h.toLinkDTO(link)}, nil
}

// ---- delete ----

type deleteLinkInput struct {
	ID int64 `path:"id"`
}

func (h *handlers) deleteLink(ctx context.Context, in *deleteLinkInput) (*struct{}, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.engine.DeleteLink(in.ID); err != nil {
		return nil, mapLinkErr("delete link", err)
	}
	return &struct{}{}, nil
}

// mapLinkErr translates engine sentinels into HTTP errors; anything else is a
// logged 500.
func mapLinkErr(op string, err error) error {
	switch {
	case errors.Is(err, engine.ErrNotFound):
		return huma.Error404NotFound("link not found")
	case errors.Is(err, engine.ErrAliasTaken):
		return huma.Error409Conflict("alias already taken")
	case errors.Is(err, engine.ErrAliasInvalid):
		return huma.Error422UnprocessableEntity("alias must be 4-32 chars of [A-Za-z0-9_-]")
	case errors.Is(err, engine.ErrAliasExhausted):
		return huma.Error500InternalServerError("could not generate a free alias")
	default:
		slog.Error(op, "error", err)
		return huma.Error500InternalServerError("internal error")
	}
}
