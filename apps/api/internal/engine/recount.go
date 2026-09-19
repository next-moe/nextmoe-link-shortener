package engine

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// ErrRecountRange rejects a recount window the raw visits cannot rebuild.
var ErrRecountRange = errors.New("recount range invalid")

// RecountResult reports what a settlement recount rewrote.
type RecountResult struct {
	From      string
	To        string
	Visits    int64
	BotVisits int64
	LinkDays  int64
}

// jstDaySQL is the JST calendar day of a visit. AT TIME ZONE '+09' would not
// do: Postgres reads a numeric zone the POSIX way, as nine hours west of UTC.
const jstDaySQL = `((created_at AT TIME ZONE 'UTC') + interval '9 hours')::date`

// RecountSettlement rebuilds the settlement tables for the closed JST-day range
// [from, to] from the raw visit rows, classifying every visit with the current
// IsBot rule. to must precede today, because live visits keep writing today's
// rows. When from is zero it defaults to the first settlement day on record.
func (e *Engine) RecountSettlement(from, to time.Time) (RecountResult, error) {
	today := settlementDay(timeNow())
	if !to.Before(today) {
		return RecountResult{}, fmt.Errorf("%w: to must precede today (%s)", ErrRecountRange, today.Format(DateLayout))
	}

	var first sql.NullTime
	if err := e.db.Raw(`SELECT min(day) FROM short_link_visit_days`).Row().Scan(&first); err != nil {
		return RecountResult{}, err
	}
	if !first.Valid {
		return RecountResult{}, fmt.Errorf("%w: no settlement day on record yet", ErrRecountRange)
	}
	// Visits before the settlement tables existed carry no fingerprint, so a
	// rebuild would count them as hits with no visitors.
	y, m, d := first.Time.UTC().Date()
	firstDay := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if from.IsZero() {
		from = firstDay
	}
	if from.Before(firstDay) {
		return RecountResult{}, fmt.Errorf("%w: settlement counting starts on %s", ErrRecountRange, firstDay.Format(DateLayout))
	}
	if to.Before(from) {
		return RecountResult{}, fmt.Errorf("%w: to must not precede from", ErrRecountRange)
	}

	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, settlementZone)
	end := time.Date(to.Year(), to.Month(), to.Day()+1, 0, 0, 0, 0, settlementZone)
	res := RecountResult{From: from.Format(DateLayout), To: to.Format(DateLayout)}

	var agents []string
	if err := e.db.Model(&model.ShortLinkVisit{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Distinct().Pluck("user_agent", &agents).Error; err != nil {
		return res, err
	}
	bots := []string{}
	for _, ua := range agents {
		if IsBot(ua) {
			bots = append(bots, ua)
		}
	}

	err := e.db.Transaction(func(tx *gorm.DB) error {
		// GORM renders an empty slice as IN (NULL), which is NULL, not false.
		if err := tx.Exec(`UPDATE short_link_visit SET is_bot = COALESCE(user_agent IN ?, false)
			WHERE created_at >= ? AND created_at < ?`, bots, start, end).Error; err != nil {
			return err
		}
		if err := tx.Raw(`SELECT count(*) AS visits, count(*) FILTER (WHERE is_bot) AS bot_visits
			FROM short_link_visit WHERE created_at >= ? AND created_at < ?`, start, end).
			Row().Scan(&res.Visits, &res.BotVisits); err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM short_link_visitor_days WHERE day >= ? AND day <= ?`,
			res.From, res.To).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO short_link_visitor_days (short_link_id, day, fp_hash, created_at)
			SELECT short_link_id, `+jstDaySQL+`, fp_hash, min(created_at)
			  FROM short_link_visit
			 WHERE created_at >= ? AND created_at < ? AND NOT is_bot AND fp_hash <> ''
			 GROUP BY 1, 2, 3`, start, end).Error; err != nil {
			return err
		}

		if err := tx.Exec(`DELETE FROM short_link_visit_days WHERE day >= ? AND day <= ?`,
			res.From, res.To).Error; err != nil {
			return err
		}
		days := tx.Exec(`INSERT INTO short_link_visit_days (short_link_id, day, total, uniques, bots, created_at, updated_at)
			SELECT short_link_id, `+jstDaySQL+`, count(*),
			       count(DISTINCT fp_hash) FILTER (WHERE NOT is_bot AND fp_hash <> ''),
			       count(*) FILTER (WHERE is_bot), now(), now()
			  FROM short_link_visit
			 WHERE created_at >= ? AND created_at < ?
			 GROUP BY 1, 2`, start, end)
		res.LinkDays = days.RowsAffected
		return days.Error
	})
	return res, err
}

// Yesterday is the last JST day a recount may rebuild.
func Yesterday() time.Time { return settlementDay(timeNow()).AddDate(0, 0, -1) }
