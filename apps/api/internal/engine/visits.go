package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/model"
)

// bucketSize is the visit-aggregation window (hourly buckets; also the
// unique-visitor window — one IP counts once per hour, matching the
// pre-migration behavior).
const bucketSize = time.Hour

// bucketStart floors t to its hour.
func bucketStart(t time.Time) time.Time {
	return t.Truncate(bucketSize)
}

// settlementZone is the day boundary for the daily aggregates: monthly
// reconciliation is against DLsite's JST calendar month. JST has no DST, so a
// fixed zone is exact — and it keeps the distroless image working, where
// time.LoadLocation fails for want of tzdata.
var settlementZone = time.FixedZone("JST", 9*3600)

// settlementDay is the JST calendar day of t, as a date at UTC midnight (the
// date column carries no zone).
func settlementDay(t time.Time) time.Time {
	y, m, d := t.In(settlementZone).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// timeNow is the clock the visit path stamps with; tests replace it to drive
// visits across the JST day boundary.
var timeNow = time.Now

// fingerprint is the published dedup identity: SHA-256(ip + "\n" + user_agent),
// hex-encoded. Its exact shape is a promise to the settlement counterparty, not
// an implementation detail to tune.
func fingerprint(ip, userAgent string) string {
	sum := sha256.Sum256([]byte(ip + "\n" + userAgent))
	return hex.EncodeToString(sum[:])
}

// VisitMeta is the request context recorded with a redirect hit.
type VisitMeta struct {
	IP        string
	UserAgent string
	Referer   string
}

// ResolveRedirect looks up an alias, enforces the gate conditions
// (status/expiry/visit limit), records the visit, and returns the final
// destination URL. rawQuery is the incoming query string without the leading
// "?" — appended to the destination when the link forwards params.
//
// Errors: ErrNotFound (404) or ErrLinkGone (410).
func (e *Engine) ResolveRedirect(alias, rawQuery string, meta VisitMeta) (string, error) {
	link, err := e.GetLinkByAlias(alias)
	if err != nil {
		return "", err
	}
	if link.Status != model.StatusActive {
		return "", ErrLinkGone
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return "", ErrLinkGone
	}
	if link.MaxVisits > 0 && link.VisitCount >= link.MaxVisits {
		return "", ErrLinkGone
	}

	// The redirect is the product and the counters are a fairness signal, so a
	// failed stats write is logged and dropped rather than turned into a 500.
	if err := e.recordVisit(link, meta); err != nil {
		slog.Error("record visit", "alias", alias, "error", err)
	}

	dest := link.DestinationURL
	if link.ForwardParams && rawQuery != "" {
		glue := "?"
		if strings.Contains(dest, "?") {
			glue = "&"
		}
		dest += glue + rawQuery
	}
	return dest, nil
}

// recordVisit writes the visit facts in one transaction: the counter bump on
// the link, the visit row, the hourly bucket upsert, and the JST-day
// settlement pair (visitor-day claim + daily aggregate). is_unique marks the
// first hit from this IP within the current hourly window.
func (e *Engine) recordVisit(link *model.ShortLink, meta VisitMeta) error {
	now := timeNow()
	bucket := bucketStart(now)
	day := settlementDay(now)
	fpHash := fingerprint(meta.IP, meta.UserAgent)
	ip := truncate(meta.IP, 45)

	unique := false
	if ip != "" {
		var prior int64
		err := e.db.Model(&model.ShortLinkVisit{}).
			Where("short_link_id = ? AND ip = ? AND created_at >= ?", link.ID, ip, now.Add(-bucketSize)).
			Count(&prior).Error
		if err != nil {
			return err
		}
		unique = prior == 0
	}

	return e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ShortLink{}).Where("id = ?", link.ID).Updates(map[string]any{
			"visit_count":     gorm.Expr("visit_count + 1"),
			"last_visited_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.ShortLinkVisit{
			ShortLinkID: link.ID,
			IP:          ip,
			UserAgent:   truncate(meta.UserAgent, 500),
			Referer:     truncate(meta.Referer, 500),
			FpHash:      fpHash,
			IsUnique:    unique,
		}).Error; err != nil {
			return err
		}
		uniqueInc := int64(0)
		if unique {
			uniqueInc = 1
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "short_link_id"}, {Name: "bucket_start"}},
			DoUpdates: clause.Assignments(map[string]any{
				"visits":     gorm.Expr("short_link_visit_bucket.visits + 1"),
				"unique_ips": gorm.Expr("short_link_visit_bucket.unique_ips + ?", uniqueInc),
				"updated_at": now,
			}),
		}).Create(&model.ShortLinkVisitBucket{
			ShortLinkID: link.ID,
			BucketStart: bucket,
			Visits:      1,
			UniqueIPs:   uniqueInc,
		}).Error; err != nil {
			return err
		}

		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ShortLinkVisitorDay{
			ShortLinkID: link.ID,
			Day:         day,
			FpHash:      fpHash,
		})
		if claim.Error != nil {
			return claim.Error
		}
		dayUniqueInc := int64(0)
		if claim.RowsAffected > 0 {
			dayUniqueInc = 1
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "short_link_id"}, {Name: "day"}},
			DoUpdates: clause.Assignments(map[string]any{
				"total":      gorm.Expr("short_link_visit_days.total + 1"),
				"uniques":    gorm.Expr("short_link_visit_days.uniques + ?", dayUniqueInc),
				"updated_at": now,
			}),
		}).Create(&model.ShortLinkVisitDay{
			ShortLinkID: link.ID,
			Day:         day,
			Total:       1,
			Uniques:     dayUniqueInc,
		}).Error
	})
}

// IsGone reports whether err is the redirect 410 condition.
func IsGone(err error) bool { return errors.Is(err, ErrLinkGone) }

// DateLayout is the wire format of a settlement day (a JST calendar date).
const DateLayout = "2006-01-02"

// ParseDate reads a settlement day off the wire.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// DailyStat is one JST day of a link's settlement counters.
type DailyStat struct {
	Date    string
	Total   int64
	Uniques int64
}

// DailyStats returns the JST-day counters of each alias over the inclusive
// [from, to] range, days without data omitted. Every requested alias gets a
// key; an unknown one gets an empty slice, because the caller pulls in batches
// and one bad member must not sink the batch.
func (e *Engine) DailyStats(aliases []string, from, to time.Time) (map[string][]DailyStat, error) {
	out := make(map[string][]DailyStat, len(aliases))
	wanted := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if _, seen := out[alias]; seen {
			continue
		}
		out[alias] = []DailyStat{}
		wanted = append(wanted, alias)
	}
	if len(wanted) == 0 {
		return out, nil
	}

	var rows []struct {
		Alias   string
		Day     time.Time
		Total   int64
		Uniques int64
	}
	err := e.db.Table("short_link_visit_days AS d").
		Select("l.alias AS alias, d.day AS day, d.total AS total, d.uniques AS uniques").
		Joins("JOIN short_link AS l ON l.id = d.short_link_id").
		Where("l.alias IN ? AND d.day >= ? AND d.day <= ?", wanted, from, to).
		Order("d.day").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Alias] = append(out[r.Alias], DailyStat{
			Date:    r.Day.Format(DateLayout),
			Total:   r.Total,
			Uniques: r.Uniques,
		})
	}
	return out, nil
}
