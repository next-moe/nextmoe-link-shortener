package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/db"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/engine"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// testStack is one throwaway API under test: the real Fiber+Huma stack over
// the integration database, plus a freshly minted S2S key.
type testStack struct {
	app *fiber.App
	eng *engine.Engine
	gdb *gorm.DB
	key string
}

// newTestStack opens the throwaway integration database named by
// SHORTLINK_DB_DSN, applies the schema, and empties it. The DSN is never
// discovered from a .env: an unset variable skips instead of touching whatever
// database happens to be configured for development.
func newTestStack(t *testing.T) testStack {
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
		short_link_visitor_days, short_link_visit_days, api_key RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate test database: %v", err)
	}

	eng := engine.New(handle.Gorm)
	key, _, err := eng.MintKey("stats-test", 0)
	if err != nil {
		t.Fatalf("mint api key: %v", err)
	}
	app := fiber.New()
	NewAPI(app, Deps{Engine: eng, PublicBaseURL: "http://127.0.0.1:7844"})
	return testStack{app: app, eng: eng, gdb: handle.Gorm, key: key}
}

// postStats sends one batch stats request. An empty key omits the header.
func (s testStack) postStats(t *testing.T, key string, body map[string]any) (int, DailyStatsResult) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/s2s/stats/daily", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := s.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	var out DailyStatsResult
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp.StatusCode, out
}

// seedLink creates a link and writes its day counters straight to the
// aggregate, so the HTTP tests do not depend on the redirect clock.
func (s testStack) seedLink(t *testing.T, alias string, days map[string][2]int64) {
	t.Helper()
	link, _, err := s.eng.CreateLink(engine.CreateLinkParams{
		DestinationURL: "https://www.dlsite.com/maniax/work/=/product_id/RJ01234567.html",
		Alias:          alias,
		CreatedVia:     "test",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	for day, counts := range days {
		parsed, err := engine.ParseDate(day)
		if err != nil {
			t.Fatalf("parse day: %v", err)
		}
		if err := s.gdb.Create(&model.ShortLinkVisitDay{
			ShortLinkID: link.ID, Day: parsed, Total: counts[0], Uniques: counts[1],
		}).Error; err != nil {
			t.Fatalf("seed day: %v", err)
		}
	}
}

func TestDailyStatsReturnsCountersPerAlias(t *testing.T) {
	s := newTestStack(t)
	s.seedLink(t, "statsone", map[string][2]int64{"2026-08-25": {7, 3}, "2026-08-27": {2, 2}})
	s.seedLink(t, "statstwo", map[string][2]int64{"2026-08-26": {5, 4}})

	status, out := s.postStats(t, s.key, map[string]any{
		"aliases": []string{"statsone", "statstwo"},
		"from":    "2026-08-25",
		"to":      "2026-08-26",
	})
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}
	one := out.Stats["statsone"]
	if len(one) != 1 || one[0].Date != "2026-08-25" || one[0].Total != 7 || one[0].Uniques != 3 {
		t.Fatalf("unexpected statsone days: %+v", one)
	}
	two := out.Stats["statstwo"]
	if len(two) != 1 || two[0].Date != "2026-08-26" || two[0].Total != 5 || two[0].Uniques != 4 {
		t.Fatalf("unexpected statstwo days: %+v", two)
	}
}

func TestDailyStatsMapsAnUnknownAliasToAnEmptyArray(t *testing.T) {
	s := newTestStack(t)
	s.seedLink(t, "statsone", map[string][2]int64{"2026-08-25": {7, 3}})

	status, out := s.postStats(t, s.key, map[string]any{
		"aliases": []string{"statsone", "nosuchalias"},
		"from":    "2026-08-25",
		"to":      "2026-08-25",
	})
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}
	if len(out.Stats) != 2 {
		t.Fatalf("want a key per requested alias, got %d", len(out.Stats))
	}
	days, present := out.Stats["nosuchalias"]
	if !present {
		t.Fatal("an unknown alias must still be a key in the response")
	}
	if days == nil {
		t.Fatal("an unknown alias must map to an empty array, not null")
	}
	if len(days) != 0 {
		t.Fatalf("an unknown alias must map to an empty array, got %+v", days)
	}
}

func TestDailyStatsRejectsANullBatch(t *testing.T) {
	s := newTestStack(t)
	status, _ := s.postStats(t, s.key, map[string]any{
		"aliases": nil,
		"from":    "2026-08-25",
		"to":      "2026-08-25",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", status)
	}
}

func TestDailyStatsRejectsAnOversizedBatch(t *testing.T) {
	s := newTestStack(t)
	aliases := make([]string, 501)
	for i := range aliases {
		aliases[i] = fmt.Sprintf("alias%04d", i)
	}
	status, _ := s.postStats(t, s.key, map[string]any{
		"aliases": aliases,
		"from":    "2026-08-25",
		"to":      "2026-08-25",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", status)
	}
}

func TestDailyStatsRejectsAnEmptyBatch(t *testing.T) {
	s := newTestStack(t)
	status, _ := s.postStats(t, s.key, map[string]any{
		"aliases": []string{},
		"from":    "2026-08-25",
		"to":      "2026-08-25",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", status)
	}
}

func TestDailyStatsRejectsARangeOver92Days(t *testing.T) {
	s := newTestStack(t)
	status, _ := s.postStats(t, s.key, map[string]any{
		"aliases": []string{"statsone"},
		"from":    "2026-06-01",
		"to":      "2026-09-01", // 93 inclusive days
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", status)
	}
}

func TestDailyStatsAcceptsExactly92Days(t *testing.T) {
	s := newTestStack(t)
	status, _ := s.postStats(t, s.key, map[string]any{
		"aliases": []string{"statsone"},
		"from":    "2026-06-01",
		"to":      "2026-08-31", // 92 inclusive days
	})
	if status != http.StatusOK {
		t.Fatalf("want 200, got %d", status)
	}
}

func TestDailyStatsRejectsAnInvertedRange(t *testing.T) {
	s := newTestStack(t)
	status, _ := s.postStats(t, s.key, map[string]any{
		"aliases": []string{"statsone"},
		"from":    "2026-08-26",
		"to":      "2026-08-25",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", status)
	}
}

func TestDailyStatsRequiresAnAPIKey(t *testing.T) {
	s := newTestStack(t)
	status, _ := s.postStats(t, "", map[string]any{
		"aliases": []string{"statsone"},
		"from":    "2026-08-25",
		"to":      "2026-08-25",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", status)
	}
}
