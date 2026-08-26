package engine

import (
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// referrerLimit caps the referrer breakdown. Past a handful of slices the
// reader is better served by a table than by more categories.
const referrerLimit = 8

// OverviewTotals are the dashboard's headline counters. The Prev* fields cover
// the equally-long window immediately before the range, which is what turns a
// bare number into a trend.
type OverviewTotals struct {
	Links         int64
	ActiveLinks   int64
	DisabledLinks int64
	ArchivedLinks int64
	AllTimeVisits int64
	RangeVisits   int64
	RangeUnique   int64
	PrevVisits    int64
	PrevUnique    int64
	Keys          int64
	ActiveKeys    int64
}

// OverviewLink pairs a link with its in-range visit aggregates.
type OverviewLink struct {
	Link        model.ShortLink
	RangeVisits int64
	RangeUnique int64
}

// OverviewSource is the per-origin rollup ("dashboard" vs each S2S key name).
type OverviewSource struct {
	CreatedVia  string
	Links       int64
	RangeVisits int64
}

// OverviewReferrer is one row of the referrer breakdown, folded to the host so
// a thousand distinct deep links collapse into the handful of sites that
// actually send traffic.
type OverviewReferrer struct {
	Host   string
	Visits int64
}

// Overview is the dashboard's read model: every number the console renders in
// one round trip.
//
// Series carries the summed hourly buckets, sparse (only hours with traffic).
// Rolling them up to days is deliberately left to the client so the axis lands
// on the *viewer's* midnight rather than the server's.
type Overview struct {
	RangeDays  int
	RangeStart time.Time
	Totals     OverviewTotals
	Series     []Bucket
	Links      []OverviewLink
	Sources    []OverviewSource
	Referrers  []OverviewReferrer
}

// Bucket is one aggregated time bucket (summed across links).
type Bucket struct {
	BucketStart time.Time
	Visits      int64
	UniqueIPs   int64
}

// clampRange normalizes the requested window to the supported 1-30 days.
func clampRange(rangeDays int) int {
	if rangeDays < 1 || rangeDays > 30 {
		return 7
	}
	return rangeDays
}

// Overview aggregates the whole dashboard in one pass. Each query is a plain
// GROUP BY over the hourly bucket table — the redirect path already paid the
// aggregation cost, so nothing here scans the raw visit rows except the
// referrer breakdown (which needs a column the buckets do not carry).
func (e *Engine) Overview(rangeDays int) (*Overview, error) {
	rangeDays = clampRange(rangeDays)
	window := time.Duration(rangeDays) * 24 * time.Hour
	rangeStart := time.Now().Add(-window)
	prevStart := rangeStart.Add(-window)

	out := &Overview{RangeDays: rangeDays, RangeStart: rangeStart}

	// ---- link counts by status + all-time visits ----
	var statusRows []struct {
		Status int16
		Links  int64
		Visits int64
	}
	if err := e.db.Model(&model.ShortLink{}).
		Select("status, count(*) as links, coalesce(sum(visit_count), 0) as visits").
		Group("status").Scan(&statusRows).Error; err != nil {
		return nil, err
	}
	for _, r := range statusRows {
		out.Totals.Links += r.Links
		out.Totals.AllTimeVisits += r.Visits
		switch r.Status {
		case model.StatusActive:
			out.Totals.ActiveLinks = r.Links
		case model.StatusDisabled:
			out.Totals.DisabledLinks = r.Links
		case model.StatusArchived:
			out.Totals.ArchivedLinks = r.Links
		}
	}

	// ---- range + previous-range visit totals ----
	current, err := e.bucketTotals(rangeStart, time.Time{})
	if err != nil {
		return nil, err
	}
	out.Totals.RangeVisits, out.Totals.RangeUnique = current.Visits, current.UniqueIPs

	previous, err := e.bucketTotals(prevStart, rangeStart)
	if err != nil {
		return nil, err
	}
	out.Totals.PrevVisits, out.Totals.PrevUnique = previous.Visits, previous.UniqueIPs

	// ---- API key counters ----
	if err := e.db.Model(&model.APIKey{}).Count(&out.Totals.Keys).Error; err != nil {
		return nil, err
	}
	if err := e.db.Model(&model.APIKey{}).Where("disabled = ?", false).
		Count(&out.Totals.ActiveKeys).Error; err != nil {
		return nil, err
	}

	// ---- the traffic series (summed hourly buckets) ----
	if err := e.db.Model(&model.ShortLinkVisitBucket{}).
		Select("bucket_start, sum(visits) as visits, sum(unique_ips) as unique_ips").
		Where("bucket_start >= ?", rangeStart).
		Group("bucket_start").Order("bucket_start").
		Scan(&out.Series).Error; err != nil {
		return nil, err
	}

	// ---- per-link range aggregates ----
	var perLink []struct {
		ShortLinkID int64
		Visits      int64
		UniqueIPs   int64
	}
	if err := e.db.Model(&model.ShortLinkVisitBucket{}).
		Select("short_link_id, sum(visits) as visits, sum(unique_ips) as unique_ips").
		Where("bucket_start >= ?", rangeStart).
		Group("short_link_id").Scan(&perLink).Error; err != nil {
		return nil, err
	}
	rangeByLink := make(map[int64]struct{ Visits, Unique int64 }, len(perLink))
	for _, r := range perLink {
		rangeByLink[r.ShortLinkID] = struct{ Visits, Unique int64 }{r.Visits, r.UniqueIPs}
	}

	links, err := e.ListLinks()
	if err != nil {
		return nil, err
	}
	out.Links = make([]OverviewLink, len(links))
	sources := map[string]*OverviewSource{}
	for i := range links {
		agg := rangeByLink[links[i].ID]
		out.Links[i] = OverviewLink{Link: links[i], RangeVisits: agg.Visits, RangeUnique: agg.Unique}

		via := links[i].CreatedVia
		if via == "" {
			via = "unknown"
		}
		if sources[via] == nil {
			sources[via] = &OverviewSource{CreatedVia: via}
		}
		sources[via].Links++
		sources[via].RangeVisits += agg.Visits
	}
	out.Sources = make([]OverviewSource, 0, len(sources))
	for _, s := range sources {
		out.Sources = append(out.Sources, *s)
	}
	// Busiest source first, then by name so equal rows keep a stable order.
	sort.Slice(out.Sources, func(a, b int) bool {
		if out.Sources[a].RangeVisits != out.Sources[b].RangeVisits {
			return out.Sources[a].RangeVisits > out.Sources[b].RangeVisits
		}
		return out.Sources[a].CreatedVia < out.Sources[b].CreatedVia
	})

	out.Referrers, err = e.referrerBreakdown(rangeStart)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// bucketTotals sums the hourly buckets in [from, until). A zero `until` means
// "no upper bound".
func (e *Engine) bucketTotals(from, until time.Time) (Bucket, error) {
	var row Bucket
	q := e.db.Model(&model.ShortLinkVisitBucket{}).
		Select("coalesce(sum(visits), 0) as visits, coalesce(sum(unique_ips), 0) as unique_ips").
		Where("bucket_start >= ?", from)
	if !until.IsZero() {
		q = q.Where("bucket_start < ?", until)
	}
	err := q.Scan(&row).Error
	return row, err
}

// referrerBreakdown groups the range's visits by referrer host. Grouping in SQL
// on the raw referrer would scatter one site across every deep link it links
// from, so the fold to host happens here.
func (e *Engine) referrerBreakdown(rangeStart time.Time) ([]OverviewReferrer, error) {
	var rows []struct {
		Referer string
		Visits  int64
	}
	if err := e.db.Model(&model.ShortLinkVisit{}).
		Select("referer, count(*) as visits").
		Where("created_at >= ?", rangeStart).
		Group("referer").Scan(&rows).Error; err != nil {
		return nil, err
	}

	byHost := map[string]int64{}
	for _, r := range rows {
		byHost[refererHost(r.Referer)] += r.Visits
	}
	out := make([]OverviewReferrer, 0, len(byHost))
	for host, visits := range byHost {
		out = append(out, OverviewReferrer{Host: host, Visits: visits})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Visits != out[b].Visits {
			return out[a].Visits > out[b].Visits
		}
		return out[a].Host < out[b].Host
	})
	if len(out) > referrerLimit {
		// Fold the tail rather than growing the category count — the reader
		// cannot tell twenty slices apart anyway.
		var rest int64
		for _, r := range out[referrerLimit:] {
			rest += r.Visits
		}
		out = append(out[:referrerLimit:referrerLimit], OverviewReferrer{Host: "", Visits: rest})
	}
	return out, nil
}

// refererHost reduces a Referer header to its host. An empty or unparseable
// value becomes "" — the caller renders that as "direct".
func refererHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
}
