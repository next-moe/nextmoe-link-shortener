package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/engine"
	"github.com/kungal/kungal-link-shortener/apps/api/internal/model"
)

// registerS2S wires the sibling-product surface: create/resolve short links
// with a Bearer API key. It deliberately does NOT ride the session identity —
// products authenticate as products, not as users.
func (h *handlers) registerS2S(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "s2s-create-link",
		Method:      http.MethodPost,
		Path:        "/s2s/links",
		Summary:     "Create (or reuse) a short link",
		Description: "Authenticated by a Bearer API key minted in the dashboard. When alias is empty and reuse is not disabled, an existing active plain link for the same destination is returned instead of minting a duplicate.",
		Tags:        []string{"s2s"},
		Security:    []map[string][]string{{"apiKey": {}}},
	}, h.s2sCreateLink)

	huma.Register(api, huma.Operation{
		OperationID: "s2s-get-link",
		Method:      http.MethodGet,
		Path:        "/s2s/links/{alias}",
		Summary:     "Look up a short link by alias",
		Tags:        []string{"s2s"},
		Security:    []map[string][]string{{"apiKey": {}}},
	}, h.s2sGetLink)
}

// requireAPIKey authenticates the Bearer token against the api_key table.
// Fail-closed: missing/unknown/disabled keys all answer 401 without
// distinguishing.
func (h *handlers) requireAPIKey(authorization string) (*model.APIKey, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return nil, huma.Error401Unauthorized("an S2S Bearer API key is required")
	}
	key, err := h.engine.VerifyKey(strings.TrimSpace(authorization[len(prefix):]))
	if err != nil {
		if errors.Is(err, engine.ErrNotFound) {
			return nil, huma.Error401Unauthorized("invalid API key")
		}
		slog.Error("verify api key", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}
	return key, nil
}

// ---- create ----

type s2sCreateLinkInput struct {
	Authorization string `header:"Authorization" doc:"Bearer slk_..."`
	Body          struct {
		CreateLinkBody
		// Reuse defaults to true: repeated S2S calls for the same destination
		// return the same link instead of minting duplicates. Explicit false
		// forces a fresh link. Only applies when alias is empty and the link
		// carries no expiry/limit.
		Reuse *bool `json:"reuse,omitempty" doc:"Default true. False forces a new link even when an equivalent exists."`
	}
}

type s2sLinkOutput struct {
	Body struct {
		LinkDTO
		Reused bool `json:"reused" doc:"True when an existing equivalent link was returned instead of a new one"`
	}
}

func (h *handlers) s2sCreateLink(_ context.Context, in *s2sCreateLinkInput) (*s2sLinkOutput, error) {
	key, err := h.requireAPIKey(in.Authorization)
	if err != nil {
		return nil, err
	}
	if err := validateDestination(in.Body.DestinationURL); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	reuse := in.Body.Reuse == nil || *in.Body.Reuse
	link, reused, err := h.engine.CreateLink(engine.CreateLinkParams{
		DestinationURL: in.Body.DestinationURL,
		Alias:          in.Body.Alias,
		Description:    in.Body.Description,
		ExpiresAt:      in.Body.ExpiresAt,
		MaxVisits:      in.Body.MaxVisits,
		ForwardParams:  in.Body.ForwardParams,
		Reuse:          reuse,
		CreatedVia:     key.Name,
	})
	if err != nil {
		return nil, mapLinkErr("s2s create link", err)
	}
	out := &s2sLinkOutput{}
	out.Body.LinkDTO = h.toLinkDTO(link)
	out.Body.Reused = reused
	return out, nil
}

// ---- get ----

type s2sGetLinkInput struct {
	Authorization string `header:"Authorization" doc:"Bearer slk_..."`
	Alias         string `path:"alias"`
}

func (h *handlers) s2sGetLink(_ context.Context, in *s2sGetLinkInput) (*linkOutput, error) {
	if _, err := h.requireAPIKey(in.Authorization); err != nil {
		return nil, err
	}
	link, err := h.engine.GetLinkByAlias(in.Alias)
	if err != nil {
		return nil, mapLinkErr("s2s get link", err)
	}
	return &linkOutput{Body: h.toLinkDTO(link)}, nil
}
