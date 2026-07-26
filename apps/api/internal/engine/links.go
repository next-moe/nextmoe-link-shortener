package engine

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/model"
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

// ListLinks returns every link, newest first. The dashboard is an internal
// admin tool with a bounded link population; 500 is a sanity cap, not
// pagination.
func (e *Engine) ListLinks() ([]model.ShortLink, error) {
	var links []model.ShortLink
	err := e.db.Order("id DESC").Limit(500).Find(&links).Error
	return links, err
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
	Buckets        []model.ShortLinkVisitBucket
	Recent         []model.ShortLinkVisit
}

// LinkStats aggregates the stats panel data: hourly buckets over the range,
// the 10 most recent visits, and the all-time distinct-IP count.
func (e *Engine) LinkStats(alias string, rangeDays int) (*Stats, error) {
	if rangeDays < 1 || rangeDays > 30 {
		rangeDays = 7
	}
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
		Order("id DESC").Limit(10).Find(&recent).Error; err != nil {
		return nil, err
	}
	var uniqueVisitors int64
	if err := e.db.Model(&model.ShortLinkVisit{}).
		Where("short_link_id = ?", link.ID).
		Distinct("ip").Count(&uniqueVisitors).Error; err != nil {
		return nil, err
	}
	return &Stats{
		Link:           *link,
		RangeDays:      rangeDays,
		UniqueVisitors: uniqueVisitors,
		Buckets:        buckets,
		Recent:         recent,
	}, nil
}
