package engine

import (
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// CreateLinkParams are the inputs for CreateLink. An empty Alias asks for a
// random one; Reuse then allows returning an existing equivalent link instead
// of minting a duplicate (S2S callers pass destination URLs repeatedly).
type CreateLinkParams struct {
	DestinationURL string
	Alias          string
	Description    string
	ExpiresAt      *time.Time
	MaxVisits      int64
	ForwardParams  bool
	Reuse          bool
	CreatedBy      int64
	CreatedVia     string
}

// CreateLink mints a short link. The bool result reports whether an existing
// link was reused (Reuse fast-path) instead of created.
func (e *Engine) CreateLink(p CreateLinkParams) (*model.ShortLink, bool, error) {
	if p.Alias != "" {
		if !customAliasRe.MatchString(p.Alias) {
			return nil, false, ErrAliasInvalid
		}
		var n int64
		if err := e.db.Model(&model.ShortLink{}).Where("alias = ?", p.Alias).Count(&n).Error; err != nil {
			return nil, false, err
		}
		if n > 0 {
			return nil, false, ErrAliasTaken
		}
	} else {
		// Reuse only matches a "plain" equivalent: active, never-expiring,
		// unlimited, same forward_params — anything more specific is
		// intentional enough to warrant its own link.
		if p.Reuse && p.ExpiresAt == nil && p.MaxVisits == 0 {
			var existing model.ShortLink
			err := e.db.
				Where("destination_url = ? AND status = ? AND expires_at IS NULL AND max_visits = 0 AND forward_params = ?",
					p.DestinationURL, model.StatusActive, p.ForwardParams).
				Order("id").First(&existing).Error
			if err == nil {
				return &existing, true, nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, false, err
			}
		}
		alias, err := e.freeAlias()
		if err != nil {
			return nil, false, err
		}
		p.Alias = alias
	}

	link := &model.ShortLink{
		Alias:          p.Alias,
		DestinationURL: p.DestinationURL,
		Description:    p.Description,
		Status:         model.StatusActive,
		ExpiresAt:      p.ExpiresAt,
		MaxVisits:      p.MaxVisits,
		ForwardParams:  p.ForwardParams,
		CreatedBy:      p.CreatedBy,
		CreatedVia:     p.CreatedVia,
	}
	if err := e.db.Create(link).Error; err != nil {
		return nil, false, err
	}
	return link, false, nil
}

// freeAlias draws random aliases until one is unused: five 6-char attempts,
// then five 8-char attempts (the longer length makes collision practically
// impossible even with a crowded 6-char space).
func (e *Engine) freeAlias() (string, error) {
	for _, length := range []int{6, 6, 6, 6, 6, 8, 8, 8, 8, 8} {
		candidate := randomAlias(length)
		var n int64
		if err := e.db.Model(&model.ShortLink{}).Where("alias = ?", candidate).Count(&n).Error; err != nil {
			return "", err
		}
		if n == 0 {
			return candidate, nil
		}
	}
	return "", ErrAliasExhausted
}

// Paging bounds for the link inventory. PerPage is clamped rather than
// rejected: a page size is a display preference, and answering a slightly
// different one beats failing the request.
const (
	LinkPerPageDefault = 20
	LinkPerPageMax     = 100
)

// StatusAny is the sentinel ListLinksParams.Status carries for "every status".
// It is outside the int16 status range, so it can never collide with a real
// one.
const StatusAny int16 = -1

// Sort orders the inventory can be read in. The list is the whole vocabulary:
// anything else falls back to SortRangeVisits rather than reaching SQL.
const (
	SortRangeVisits = "range"
	SortTotalVisits = "total"
	SortCreated     = "created"
	SortAlias       = "alias"
)

// ListLinksParams is one page request against the link inventory. Search,
// status and sort all resolve in SQL: the dashboard shows one page at a time,
// so filtering in the client would only ever filter the page in front of the
// reader, not the inventory.
type ListLinksParams struct {
	// Query matches alias, destination or description, case-insensitively.
	Query  string
	Status int16
	Sort   string
	Page   int
	// PerPage is clamped to [1, LinkPerPageMax]; 0 means LinkPerPageDefault.
	PerPage int
	// RangeDays scopes each row's visit aggregates, on the same 1-30 scale the
	// rest of the dashboard uses, so a row's number agrees with the tiles and
	// the chart above it.
	RangeDays int
}

// LinkRow pairs a link with its visit aggregates for the requested window.
type LinkRow struct {
	Link        model.ShortLink
	RangeVisits int64
	RangeUnique int64
}

// LinkPage is one page of the inventory plus what a pager needs to describe
// the whole of it.
type LinkPage struct {
	Rows    []LinkRow
	Total   int64
	Page    int
	PerPage int
	// RangeDays is the window the row aggregates were built over, after
	// clamping — the caller reports what it got, not what it asked for.
	RangeDays int
}

// TotalPages is the page count for Total at this page size, at least 1 so an
// empty inventory still reads as "page 1 of 1".
func (p LinkPage) TotalPages() int {
	if p.PerPage < 1 {
		return 1
	}
	pages := int((p.Total + int64(p.PerPage) - 1) / int64(p.PerPage))
	if pages < 1 {
		return 1
	}
	return pages
}

// likeEscape neutralizes the LIKE wildcards in a user's search string, so a
// query of "100%" looks for that text instead of matching everything. The
// escape character is the Postgres default backslash.
var likeEscape = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// linkOrderBy maps a sort key to its SQL. Every ordering ends on a unique
// column, so a row can never sit on two pages (or on none) because two rows
// tied on the sort key were ordered differently between requests.
func linkOrderBy(sort string) string {
	switch sort {
	case SortTotalVisits:
		return "sl.visit_count DESC, sl.id DESC"
	case SortCreated:
		return "sl.id DESC"
	case SortAlias:
		return "sl.alias ASC"
	default:
		return "range_visits DESC, sl.id DESC"
	}
}

// rangeAggregates is the per-link visit rollup for the window, as a subquery
// to join against. It reads the hourly buckets the redirect path already
// maintains, so a page of the inventory never touches the raw visit rows.
func (e *Engine) rangeAggregates(rangeStart time.Time) *gorm.DB {
	return e.db.Model(&model.ShortLinkVisitBucket{}).
		Select("short_link_id, sum(visits) as visits, sum(unique_ips) as unique_ips").
		Where("bucket_start >= ?", rangeStart).
		Group("short_link_id")
}

// filterLinks applies the search and status predicates. It is shared by the
// page query and its COUNT, so the pager can never describe a different set
// than the one it pages through.
func filterLinks(q *gorm.DB, p ListLinksParams) *gorm.DB {
	if p.Status != StatusAny {
		q = q.Where("sl.status = ?", p.Status)
	}
	if term := strings.TrimSpace(p.Query); term != "" {
		like := "%" + likeEscape.Replace(strings.ToLower(term)) + "%"
		// Parenthesized explicitly: this OR group is ANDed with the status
		// predicate, and an unparenthesized OR would silently widen the page to
		// every link matching the term in ANY status.
		q = q.Where("(lower(sl.alias) LIKE ? OR lower(sl.destination_url) LIKE ? OR lower(sl.description) LIKE ?)",
			like, like, like)
	}
	return q
}

// ListLinks returns one page of the inventory, each row carrying its in-range
// visit aggregates, plus the total the filters match.
//
// This is deliberately a server-side page rather than "fetch everything and
// let the browser sort it out": the inventory is unbounded (siblings mint
// links over S2S all day), and a client-side list silently stops at whatever
// cap the server picked — the rows past it are invisible AND unsearchable,
// with the console still showing the true total beside them.
func (e *Engine) ListLinks(p ListLinksParams) (*LinkPage, error) {
	if p.PerPage <= 0 {
		p.PerPage = LinkPerPageDefault
	}
	if p.PerPage > LinkPerPageMax {
		p.PerPage = LinkPerPageMax
	}
	if p.Page < 1 {
		p.Page = 1
	}
	p.RangeDays = clampRange(p.RangeDays)

	page := &LinkPage{Page: p.Page, PerPage: p.PerPage, RangeDays: p.RangeDays, Rows: []LinkRow{}}
	if err := filterLinks(e.db.Table("short_link AS sl"), p).
		Count(&page.Total).Error; err != nil {
		return nil, err
	}
	// A page past the end answers empty rather than clamping: the client asked
	// for a page that no longer exists (rows were deleted under it), and moving
	// it silently would hide that.
	if page.Total == 0 {
		return page, nil
	}

	var rows []linkRow
	q := filterLinks(e.db.Table("short_link AS sl"), p).
		Joins("LEFT JOIN (?) AS b ON b.short_link_id = sl.id", e.rangeAggregates(rangeStartFor(p.RangeDays))).
		Select("sl.*, coalesce(b.visits, 0) AS range_visits, coalesce(b.unique_ips, 0) AS range_unique").
		Order(linkOrderBy(p.Sort)).
		Limit(p.PerPage).
		Offset((p.Page - 1) * p.PerPage)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	page.Rows = make([]LinkRow, len(rows))
	for i := range rows {
		page.Rows[i] = LinkRow{
			Link:        rows[i].ShortLink,
			RangeVisits: rows[i].RangeVisits,
			RangeUnique: rows[i].RangeUnique,
		}
	}
	return page, nil
}

// rangeStartFor turns a clamped day window into its start instant.
func rangeStartFor(rangeDays int) time.Time {
	return time.Now().Add(-time.Duration(rangeDays) * 24 * time.Hour)
}

// linkRow is the scan target for the page query: a link plus the two joined
// aggregate columns.
type linkRow struct {
	model.ShortLink
	RangeVisits int64
	RangeUnique int64
}

// GetLinkByAlias loads one link or ErrNotFound.
func (e *Engine) GetLinkByAlias(alias string) (*model.ShortLink, error) {
	var link model.ShortLink
	err := e.db.Where("alias = ?", alias).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// UpdateLinkParams is the full editable field set — the dashboard edit form
// always submits every field, so this is a whole-row update, not a patch.
type UpdateLinkParams struct {
	DestinationURL string
	Description    string
	Status         int16
	ExpiresAt      *time.Time
	MaxVisits      int64
	ForwardParams  bool
}

// UpdateLink applies the editable fields to a link by id.
func (e *Engine) UpdateLink(id int64, p UpdateLinkParams) (*model.ShortLink, error) {
	var link model.ShortLink
	if err := e.db.First(&link, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	link.DestinationURL = p.DestinationURL
	link.Description = p.Description
	link.Status = p.Status
	link.ExpiresAt = p.ExpiresAt
	link.MaxVisits = p.MaxVisits
	link.ForwardParams = p.ForwardParams
	// Save with Select to persist zero values (cleared expiry, status 0, …).
	if err := e.db.Model(&link).Select("destination_url", "description", "status",
		"expires_at", "max_visits", "forward_params").Updates(map[string]any{
		"destination_url": link.DestinationURL,
		"description":     link.Description,
		"status":          link.Status,
		"expires_at":      link.ExpiresAt,
		"max_visits":      link.MaxVisits,
		"forward_params":  link.ForwardParams,
	}).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

// DeleteLink hard-deletes a link; visits and buckets follow via the ON DELETE
// CASCADE foreign keys.
func (e *Engine) DeleteLink(id int64) error {
	res := e.db.Delete(&model.ShortLink{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Stats is the dashboard stats payload for one link.
type Stats struct {
	Link           model.ShortLink
	RangeDays      int
	UniqueVisitors int64
	RangeVisits    int64
	RangeUnique    int64
	Buckets        []model.ShortLinkVisitBucket
	Recent         []model.ShortLinkVisit
	Referrers      []OverviewReferrer
}

// LinkStats aggregates the detail panel's data: hourly buckets over the range,
// the range totals, the referrer breakdown, the most recent visits, and the
// all-time distinct-IP count.
func (e *Engine) LinkStats(alias string, rangeDays int) (*Stats, error) {
	rangeDays = clampRange(rangeDays)
	link, err := e.GetLinkByAlias(alias)
	if err != nil {
		return nil, err
	}
	rangeStart := time.Now().Add(-time.Duration(rangeDays) * 24 * time.Hour)

	var buckets []model.ShortLinkVisitBucket
	if err := e.db.
		Where("short_link_id = ? AND bucket_start >= ?", link.ID, rangeStart).
		Order("bucket_start").Find(&buckets).Error; err != nil {
		return nil, err
	}
	var recent []model.ShortLinkVisit
	if err := e.db.
		Where("short_link_id = ?", link.ID).
		Order("id DESC").Limit(recentVisitLimit).Find(&recent).Error; err != nil {
		return nil, err
	}
	var uniqueVisitors int64
	if err := e.db.Model(&model.ShortLinkVisit{}).
		Where("short_link_id = ?", link.ID).
		Distinct("ip").Count(&uniqueVisitors).Error; err != nil {
		return nil, err
	}
	referrers, err := e.linkReferrers(link.ID, rangeStart)
	if err != nil {
		return nil, err
	}

	stats := &Stats{
		Link:           *link,
		RangeDays:      rangeDays,
		UniqueVisitors: uniqueVisitors,
		Buckets:        buckets,
		Recent:         recent,
		Referrers:      referrers,
	}
	for _, b := range buckets {
		stats.RangeVisits += b.Visits
		stats.RangeUnique += b.UniqueIPs
	}
	return stats, nil
}

// recentVisitLimit sizes the detail panel's activity feed.
const recentVisitLimit = 12

// linkReferrers is referrerBreakdown scoped to a single link.
func (e *Engine) linkReferrers(linkID int64, rangeStart time.Time) ([]OverviewReferrer, error) {
	var rows []struct {
		Referer string
		Visits  int64
	}
	if err := e.db.Model(&model.ShortLinkVisit{}).
		Select("referer, count(*) as visits").
		Where("short_link_id = ? AND created_at >= ?", linkID, rangeStart).
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
		var rest int64
		for _, r := range out[referrerLimit:] {
			rest += r.Visits
		}
		out = append(out[:referrerLimit:referrerLimit], OverviewReferrer{Host: "", Visits: rest})
	}
	return out, nil
}
