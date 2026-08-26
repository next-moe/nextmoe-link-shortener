package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/engine"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
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

	huma.Register(api, huma.Operation{
		OperationID: "s2s-daily-stats",
		Method:      http.MethodPost,
		Path:        "/s2s/stats/daily",
		Summary:     "Batch daily visit stats for a set of aliases",
		Description: "Per-alias daily totals and deduplicated visitor counts over an inclusive JST date range (at most 92 days, at most 500 aliases). Days without traffic are omitted; an alias that does not exist yields an empty array rather than failing the batch.",
		Tags:        []string{"s2s"},
		Security:    []map[string][]string{{"apiKey": {}}},
	}, h.s2sDailyStats)
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

// ---- daily stats ----

// maxStatsRangeDays bounds one batch stats query (inclusive day count).
const maxStatsRangeDays = 92

// DailyStatsBody is the batch stats request.
type DailyStatsBody struct {
	Aliases []string `json:"aliases" minItems:"1" maxItems:"500" doc:"Short link aliases to report on"`
	From    string   `json:"from" format:"date" doc:"First JST day, inclusive (YYYY-MM-DD)"`
	To      string   `json:"to" format:"date" doc:"Last JST day, inclusive (YYYY-MM-DD)"`
}

// DailyStatDTO is one JST day of counters for one alias.
type DailyStatDTO struct {
	Date    string `json:"date" doc:"JST calendar day (YYYY-MM-DD)"`
	Total   int64  `json:"total" doc:"All hits recorded that day"`
	Uniques int64  `json:"uniques" doc:"Distinct visitor fingerprints that day"`
}

// DailyStatsResult maps each requested alias to its days with traffic.
type DailyStatsResult struct {
	Stats map[string][]DailyStatDTO `json:"stats" doc:"Alias to its days with traffic, ascending; unknown aliases map to an empty array"`
}

type s2sDailyStatsInput struct {
	Authorization string `header:"Authorization" doc:"Bearer slk_..."`
	Body          DailyStatsBody
}

type s2sDailyStatsOutput struct {
	Body DailyStatsResult
}

func (h *handlers) s2sDailyStats(_ context.Context, in *s2sDailyStatsInput) (*s2sDailyStatsOutput, error) {
	if _, err := h.requireAPIKey(in.Authorization); err != nil {
		return nil, err
	}
	// minItems does not cover an explicit null: a Go slice is nullable, so the
	// generated schema accepts null and only this check rejects it.
	if len(in.Body.Aliases) == 0 {
		return nil, huma.Error422UnprocessableEntity("aliases must hold at least one alias")
	}
	from, err := engine.ParseDate(in.Body.From)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("from must be a YYYY-MM-DD date")
	}
	to, err := engine.ParseDate(in.Body.To)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("to must be a YYYY-MM-DD date")
	}
	if to.Before(from) {
		return nil, huma.Error422UnprocessableEntity("to must not precede from")
	}
	if int(to.Sub(from).Hours()/24)+1 > maxStatsRangeDays {
		return nil, huma.Error422UnprocessableEntity(
			fmt.Sprintf("the range must span at most %d days", maxStatsRangeDays))
	}

	stats, err := h.engine.DailyStats(in.Body.Aliases, from, to)
	if err != nil {
		slog.Error("s2s daily stats", "error", err)
		return nil, huma.Error500InternalServerError("internal error")
	}
	out := &s2sDailyStatsOutput{}
	out.Body.Stats = make(map[string][]DailyStatDTO, len(stats))
	for alias, days := range stats {
		dtos := make([]DailyStatDTO, len(days))
		for i, d := range days {
			dtos[i] = DailyStatDTO{Date: d.Date, Total: d.Total, Uniques: d.Uniques}
		}
		out.Body.Stats[alias] = dtos
	}
	return out, nil
}
