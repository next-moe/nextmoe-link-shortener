package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// seedInventory creates n links named alias-000, alias-001, … in that order,
// so a test can reason about ids without capturing every return value.
func seedInventory(t *testing.T, e *Engine, n int) []*model.ShortLink {
	t.Helper()
	links := make([]*model.ShortLink, n)
	for i := range links {
		links[i] = mustLink(t, e, fmt.Sprintf("alias-%03d", i))
	}
	return links
}

// aliases is the readable form of a page for a failure message.
func aliases(rows []LinkRow) []string {
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = rows[i].Link.Alias
	}
	return out
}

func mustPage(t *testing.T, e *Engine, p ListLinksParams) *LinkPage {
	t.Helper()
	page, err := e.ListLinks(p)
	if err != nil {
		t.Fatalf("list links %+v: %v", p, err)
	}
	return page
}

// The whole point of the change: every row is reachable. A client-side list
// stops at whatever cap the server picked and the rows past it are invisible
// AND unsearchable, so walk the pages and check the set is complete.
func TestListLinksPagesCoverEveryRowExactlyOnce(t *testing.T) {
	e := testEngine(t)
	const total = 25
	seedInventory(t, e, total)

	seen := map[string]int{}
	pages := 0
	for page := 1; ; page++ {
		got := mustPage(t, e, ListLinksParams{Page: page, PerPage: 10, Status: StatusAny, Sort: SortCreated})
		if got.Total != total {
			t.Fatalf("page %d: want total=%d, got %d", page, total, got.Total)
		}
		if len(got.Rows) == 0 {
			break
		}
		pages++
		for _, row := range got.Rows {
			seen[row.Link.Alias]++
		}
		if page > total {
			t.Fatal("paging did not terminate")
		}
	}
	if pages != 3 {
		t.Fatalf("want 3 pages of 10 over %d rows, got %d", total, pages)
	}
	if len(seen) != total {
		t.Fatalf("want %d distinct rows across the pages, got %d", total, len(seen))
	}
	for alias, times := range seen {
		if times != 1 {
			t.Fatalf("%s appeared %d times across the pages", alias, times)
		}
	}
}

func TestListLinksReportsTotalPagesIndependentlyOfThePage(t *testing.T) {
	e := testEngine(t)
	seedInventory(t, e, 21)

	page := mustPage(t, e, ListLinksParams{Page: 1, PerPage: 10, Status: StatusAny})
	if page.TotalPages() != 3 {
		t.Fatalf("want 3 pages over 21 rows at 10/page, got %d", page.TotalPages())
	}

	// An empty inventory still reads as "page 1 of 1" rather than "of 0".
	empty := &LinkPage{PerPage: 10}
	if empty.TotalPages() != 1 {
		t.Fatalf("want 1 page for an empty inventory, got %d", empty.TotalPages())
	}
}

// Search must reach the whole inventory, not the page the client is holding.
func TestListLinksSearchesTheWholeInventory(t *testing.T) {
	e := testEngine(t)
	seedInventory(t, e, 30)
	// The needle sorts last by alias and is the oldest by nothing — it lives
	// well past the first page under every ordering.
	mustLink(t, e, "zz-needle")

	page := mustPage(t, e, ListLinksParams{Page: 1, PerPage: 10, Status: StatusAny, Query: "NEEDLE"})
	if page.Total != 1 || len(page.Rows) != 1 || page.Rows[0].Link.Alias != "zz-needle" {
		t.Fatalf("want the single needle row, got total=%d rows=%v", page.Total, aliases(page.Rows))
	}
}

// A search term is data, not pattern: "%" must match a literal percent sign.
func TestListLinksTreatsWildcardsInTheQueryAsLiteralText(t *testing.T) {
	e := testEngine(t)
	seedInventory(t, e, 3)
	if err := e.db.Model(&model.ShortLink{}).Where("alias = ?", "alias-000").
		Update("description", "50% off").Error; err != nil {
		t.Fatalf("set description: %v", err)
	}

	page := mustPage(t, e, ListLinksParams{Page: 1, PerPage: 10, Status: StatusAny, Query: "%"})
	if page.Total != 1 {
		t.Fatalf("want only the row whose text contains a percent sign, got total=%d rows=%v",
			page.Total, aliases(page.Rows))
	}
}

// The status filter and the search are ANDed. An unparenthesized OR group
// would quietly return matching links in every status.
func TestListLinksAndsTheStatusFilterWithTheSearch(t *testing.T) {
	e := testEngine(t)
	seedInventory(t, e, 4)
	if err := e.db.Model(&model.ShortLink{}).Where("alias IN ?", []string{"alias-000", "alias-001"}).
		Update("status", model.StatusDisabled).Error; err != nil {
		t.Fatalf("disable links: %v", err)
	}

	page := mustPage(t, e, ListLinksParams{Page: 1, PerPage: 10, Status: model.StatusActive, Query: "alias-"})
	if page.Total != 2 {
		t.Fatalf("want the 2 active matches, got total=%d rows=%v", page.Total, aliases(page.Rows))
	}
	for _, row := range page.Rows {
		if row.Link.Status != model.StatusActive {
			t.Fatalf("%s leaked through with status %d", row.Link.Alias, row.Link.Status)
		}
	}
}

// Sorting by in-window traffic has to happen in SQL: ordering the page the
// client holds would only ever rank that page.
func TestListLinksSortsByInWindowVisitsAcrossTheInventory(t *testing.T) {
	e := testEngine(t)
	seedInventory(t, e, 12)
	atClock(t, time.Now())

	// The busiest link is the OLDEST one, so it sits on the last page under the
	// default newest-first order.
	for i := 0; i < 3; i++ {
		mustResolve(t, e, "alias-000", VisitMeta{IP: fmt.Sprintf("203.0.113.%d", i)})
	}
	mustResolve(t, e, "alias-005", VisitMeta{IP: "203.0.113.99"})

	page := mustPage(t, e, ListLinksParams{Page: 1, PerPage: 5, Status: StatusAny, Sort: SortRangeVisits, RangeDays: 7})
	if len(page.Rows) != 5 {
		t.Fatalf("want a full page of 5, got %v", aliases(page.Rows))
	}
	if page.Rows[0].Link.Alias != "alias-000" || page.Rows[0].RangeVisits != 3 {
		t.Fatalf("want alias-000 with 3 in-window visits on top, got %s with %d",
			page.Rows[0].Link.Alias, page.Rows[0].RangeVisits)
	}
	if page.Rows[1].Link.Alias != "alias-005" || page.Rows[1].RangeVisits != 1 {
		t.Fatalf("want alias-005 with 1 in-window visit second, got %s with %d",
			page.Rows[1].Link.Alias, page.Rows[1].RangeVisits)
	}
	// Everything else is untouched and must report zero rather than drop out of
	// the join.
	for _, row := range page.Rows[2:] {
		if row.RangeVisits != 0 || row.RangeUnique != 0 {
			t.Fatalf("%s: want 0/0 for an unvisited link, got %d/%d",
				row.Link.Alias, row.RangeVisits, row.RangeUnique)
		}
	}
}

// The source rollup counts the whole table. It used to be folded in Go from
// the capped list the dashboard was about to render, which made it a rollup of
// the newest N links while the totals beside it counted every link.
func TestSourceBreakdownCountsEveryLinkNotJustTheFirstPage(t *testing.T) {
	e := testEngine(t)
	seedInventory(t, e, 40)

	sources, err := e.sourceBreakdown(rangeStartFor(7))
	if err != nil {
		t.Fatalf("source breakdown: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("want one source, got %+v", sources)
	}
	if sources[0].CreatedVia != "test" || sources[0].Links != 40 {
		t.Fatalf("want test=40 links, got %s=%d", sources[0].CreatedVia, sources[0].Links)
	}
}
