package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// registerOverview wires the dashboard's aggregate read model. It is one
// admin-gated call that answers every AGGREGATE the console renders, so the
// dashboard does not fan out one request per link. The inventory itself is
// paged separately (GET /links) — it is the one part of the page that grows
// without bound, and shipping it inside this payload capped what the console
// could ever see.
func (h *handlers) registerOverview(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "stats-overview",
		Method:      http.MethodGet,
		Path:        "/stats/overview",
		Summary:     "Dashboard aggregates (totals, traffic series, source and referrer breakdowns)",
		Tags:        []string{"stats"},
	}, h.statsOverview)
}

type overviewInput struct {
	Range int `query:"range" doc:"days covered by the window (1-30, default 7)"`
}

// OverviewTotalsDTO carries the headline counters. Prev* cover the equally-long
// window immediately before the range, so the dashboard can render a delta
// instead of a context-free number.
type OverviewTotalsDTO struct {
	Links         int64 `json:"links"`
	ActiveLinks   int64 `json:"active_links"`
	DisabledLinks int64 `json:"disabled_links"`
	ArchivedLinks int64 `json:"archived_links"`
	AllTimeVisits int64 `json:"all_time_visits"`
	RangeVisits   int64 `json:"range_visits"`
	RangeUnique   int64 `json:"range_unique"`
	PrevVisits    int64 `json:"prev_visits" doc:"visits in the preceding window of equal length"`
	PrevUnique    int64 `json:"prev_unique" doc:"unique IPs in the preceding window of equal length"`
	Keys          int64 `json:"keys"`
	ActiveKeys    int64 `json:"active_keys"`
}

// OverviewSourceDTO rolls links up by origin ("dashboard" or an S2S key name).
type OverviewSourceDTO struct {
	CreatedVia  string `json:"created_via"`
	Links       int64  `json:"links"`
	RangeVisits int64  `json:"range_visits"`
}

// ReferrerDTO is one row of a referrer breakdown, folded to the host. An empty
// host means the visit carried no Referer (direct traffic).
type ReferrerDTO struct {
	Host   string `json:"host" doc:"referrer host, \"\" for direct traffic"`
	Visits int64  `json:"visits"`
}

type overviewOutput struct {
	Body struct {
		RangeDays  int                 `json:"range_days"`
		RangeStart time.Time           `json:"range_start"`
		Totals     OverviewTotalsDTO   `json:"totals"`
		Series     []BucketDTO         `json:"series" doc:"hourly buckets summed across links; sparse (empty hours are omitted)"`
		Sources    []OverviewSourceDTO `json:"sources"`
		Referrers  []ReferrerDTO       `json:"referrers"`
	}
}

func (h *handlers) statsOverview(ctx context.Context, in *overviewInput) (*overviewOutput, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	ov, err := h.engine.Overview(in.Range)
	if err != nil {
		slog.Error("stats overview", "error", err)
		return nil, huma.Error500InternalServerError("could not build the overview")
	}

	out := &overviewOutput{}
	out.Body.RangeDays = ov.RangeDays
	out.Body.RangeStart = ov.RangeStart
	out.Body.Totals = OverviewTotalsDTO{
		Links:         ov.Totals.Links,
		ActiveLinks:   ov.Totals.ActiveLinks,
		DisabledLinks: ov.Totals.DisabledLinks,
		ArchivedLinks: ov.Totals.ArchivedLinks,
		AllTimeVisits: ov.Totals.AllTimeVisits,
		RangeVisits:   ov.Totals.RangeVisits,
		RangeUnique:   ov.Totals.RangeUnique,
		PrevVisits:    ov.Totals.PrevVisits,
		PrevUnique:    ov.Totals.PrevUnique,
		Keys:          ov.Totals.Keys,
		ActiveKeys:    ov.Totals.ActiveKeys,
	}
	out.Body.Series = make([]BucketDTO, len(ov.Series))
	for i, b := range ov.Series {
		out.Body.Series[i] = BucketDTO{BucketStart: b.BucketStart, Visits: b.Visits, UniqueIPs: b.UniqueIPs}
	}
	out.Body.Sources = make([]OverviewSourceDTO, len(ov.Sources))
	for i, s := range ov.Sources {
		out.Body.Sources[i] = OverviewSourceDTO{CreatedVia: s.CreatedVia, Links: s.Links, RangeVisits: s.RangeVisits}
	}
	out.Body.Referrers = toReferrerDTOs(ov.Referrers)
	return out, nil
}
