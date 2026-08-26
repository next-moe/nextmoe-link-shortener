package engine

import (
	"os"
	"testing"
	"time"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/db"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// testEngine opens the throwaway integration database named by
// SHORTLINK_DB_DSN, applies the schema, and empties it. The DSN is never
// discovered from a .env: an unset variable skips instead of touching whatever
// database happens to be configured for development.
func testEngine(t *testing.T) *Engine {
	t.Helper()
	dsn := os.Getenv("SHORTLINK_DB_DSN")
	if dsn == "" {
		t.Skip("SHORTLINK_DB_DSN is not set; skipping the database integration test")
	}
	handle, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := handle.Gorm.Exec(`TRUNCATE short_link, short_link_visit, short_link_visit_bucket,
		short_link_visitor_days, short_link_visit_days RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate test database: %v", err)
	}
	return New(handle.Gorm)
}

// atClock pins the visit clock for the duration of the test.
func atClock(t *testing.T, at time.Time) {
	t.Helper()
	timeNow = func() time.Time { return at }
	t.Cleanup(func() { timeNow = time.Now })
}

func mustLink(t *testing.T, e *Engine, alias string) *model.ShortLink {
	t.Helper()
	link, _, err := e.CreateLink(CreateLinkParams{
		DestinationURL: "https://www.dlsite.com/maniax/work/=/product_id/RJ01234567.html",
		Alias:          alias,
		CreatedVia:     "test",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	return link
}

func mustResolve(t *testing.T, e *Engine, alias string, meta VisitMeta) {
	t.Helper()
	if _, err := e.ResolveRedirect(alias, "", meta); err != nil {
		t.Fatalf("resolve %s: %v", alias, err)
	}
}

func dayRow(t *testing.T, e *Engine, linkID int64, day string) model.ShortLinkVisitDay {
	t.Helper()
	var row model.ShortLinkVisitDay
	err := e.db.Where("short_link_id = ? AND day = ?", linkID, day).First(&row).Error
	if err != nil {
		t.Fatalf("load day %s: %v", day, err)
	}
	return row
}

func TestRecordVisitDedupesFingerprintWithinTheSameJSTDay(t *testing.T) {
	e := testEngine(t)
	link := mustLink(t, e, "dedupe01")
	atClock(t, time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC))

	meta := VisitMeta{IP: "203.0.113.7", UserAgent: "Mozilla/5.0 (settlement probe)"}
	mustResolve(t, e, link.Alias, meta)
	mustResolve(t, e, link.Alias, meta)

	row := dayRow(t, e, link.ID, "2026-08-25")
	if row.Total != 2 || row.Uniques != 1 {
		t.Fatalf("want total=2 uniques=1, got total=%d uniques=%d", row.Total, row.Uniques)
	}

	var claims int64
	if err := e.db.Model(&model.ShortLinkVisitorDay{}).
		Where("short_link_id = ?", link.ID).Count(&claims).Error; err != nil {
		t.Fatalf("count visitor days: %v", err)
	}
	if claims != 1 {
		t.Fatalf("want 1 visitor-day claim, got %d", claims)
	}
}

func TestRecordVisitSplitsAtJSTMidnight(t *testing.T) {
	e := testEngine(t)
	link := mustLink(t, e, "midnight01")
	meta := VisitMeta{IP: "203.0.113.7", UserAgent: "Mozilla/5.0 (settlement probe)"}

	// 14:59:59Z is 23:59:59 JST on the 25th; one second later is the 26th.
	atClock(t, time.Date(2026, 8, 25, 14, 59, 59, 0, time.UTC))
	mustResolve(t, e, link.Alias, meta)
	atClock(t, time.Date(2026, 8, 25, 15, 0, 0, 0, time.UTC))
	mustResolve(t, e, link.Alias, meta)

	for _, day := range []string{"2026-08-25", "2026-08-26"} {
		row := dayRow(t, e, link.ID, day)
		if row.Total != 1 || row.Uniques != 1 {
			t.Fatalf("%s: want total=1 uniques=1, got total=%d uniques=%d", day, row.Total, row.Uniques)
		}
	}
}

func TestRecordVisitCountsDistinctFingerprintsSeparately(t *testing.T) {
	e := testEngine(t)
	link := mustLink(t, e, "distinct01")
	atClock(t, time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC))

	mustResolve(t, e, link.Alias, VisitMeta{IP: "203.0.113.7", UserAgent: "agent-a"})
	mustResolve(t, e, link.Alias, VisitMeta{IP: "203.0.113.8", UserAgent: "agent-a"})

	row := dayRow(t, e, link.ID, "2026-08-25")
	if row.Total != 2 || row.Uniques != 2 {
		t.Fatalf("want total=2 uniques=2, got total=%d uniques=%d", row.Total, row.Uniques)
	}
}

func TestResolveRedirectSurvivesAFailedStatsWrite(t *testing.T) {
	e := testEngine(t)
	link := mustLink(t, e, "resilient01")

	if err := e.db.Exec(`ALTER TABLE short_link_visit_days RENAME TO short_link_visit_days_gone`).Error; err != nil {
		t.Fatalf("hide the aggregate table: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(`ALTER TABLE short_link_visit_days_gone RENAME TO short_link_visit_days`)
	})

	dest, err := e.ResolveRedirect(link.Alias, "", VisitMeta{IP: "203.0.113.7", UserAgent: "agent-a"})
	if err != nil {
		t.Fatalf("the redirect must survive a stats failure, got: %v", err)
	}
	if dest != link.DestinationURL {
		t.Fatalf("want destination %q, got %q", link.DestinationURL, dest)
	}
}

func TestDailyStatsCoversRequestedAliasesAndRange(t *testing.T) {
	e := testEngine(t)
	first := mustLink(t, e, "range01")
	second := mustLink(t, e, "range02")

	atClock(t, time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC))
	mustResolve(t, e, first.Alias, VisitMeta{IP: "203.0.113.7", UserAgent: "agent-a"})
	atClock(t, time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC))
	mustResolve(t, e, first.Alias, VisitMeta{IP: "203.0.113.7", UserAgent: "agent-a"})
	mustResolve(t, e, second.Alias, VisitMeta{IP: "203.0.113.9", UserAgent: "agent-b"})

	from, _ := ParseDate("2026-08-25")
	to, _ := ParseDate("2026-08-26")
	stats, err := e.DailyStats([]string{first.Alias, second.Alias, "nosuchalias"}, from, to)
	if err != nil {
		t.Fatalf("daily stats: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("want a key per requested alias, got %d", len(stats))
	}
	if got := stats[first.Alias]; len(got) != 1 || got[0].Date != "2026-08-25" || got[0].Total != 1 {
		t.Fatalf("unexpected days for %s: %+v", first.Alias, got)
	}
	if got := stats[second.Alias]; len(got) != 0 {
		t.Fatalf("%s has no traffic in range, got %+v", second.Alias, got)
	}
	if got := stats["nosuchalias"]; got == nil || len(got) != 0 {
		t.Fatalf("an unknown alias must map to an empty array, got %+v", got)
	}
}
