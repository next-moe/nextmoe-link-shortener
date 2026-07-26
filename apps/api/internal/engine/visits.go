package engine

import (
	"errors"
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

	if err := e.recordVisit(link, meta); err != nil {
		return "", err
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

// recordVisit writes the three visit facts in one transaction: the counter
// bump on the link, the visit row, and the hourly bucket upsert. is_unique
// marks the first hit from this IP within the current window.
func (e *Engine) recordVisit(link *model.ShortLink, meta VisitMeta) error {
	now := time.Now()
	bucket := bucketStart(now)
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
			IsUnique:    unique,
		}).Error; err != nil {
			return err
		}
		uniqueInc := int64(0)
		if unique {
			uniqueInc = 1
		}
		return tx.Clauses(clause.OnConflict{
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
		}).Error
	})
}

// IsGone reports whether err is the redirect 410 condition.
func IsGone(err error) bool { return errors.Is(err, ErrLinkGone) }
