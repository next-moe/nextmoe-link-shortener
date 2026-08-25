// Package model defines the kun_shortlink schema. Table and column names keep
// the shapes of the pre-migration Prisma schema (short_link / short_link_visit
// / short_link_visit_bucket) so an optional legacy data import stays a plain
// INSERT ... SELECT; the api_key table is new (S2S credentials).
package model

import "time"

// Link statuses. Only StatusActive redirects; the others answer 410.
const (
	StatusActive   int16 = 0
	StatusDisabled int16 = 1
	StatusArchived int16 = 2
)

// ShortLink is one alias → destination mapping plus its lifetime controls and
// denormalized visit counters.
type ShortLink struct {
	ID             int64      `gorm:"primaryKey"`
	Alias          string     `gorm:"uniqueIndex;size:32;not null"`
	DestinationURL string     `gorm:"type:text;not null"`
	Description    string     `gorm:"size:500;not null;default:''"`
	Status         int16      `gorm:"not null;default:0;index"`
	ExpiresAt      *time.Time // nil = never expires
	MaxVisits      int64      `gorm:"not null;default:0"` // 0 = unlimited
	VisitCount     int64      `gorm:"not null;default:0"`
	LastVisitedAt  *time.Time
	// ForwardParams appends the incoming query string to the destination URL.
	ForwardParams bool `gorm:"not null;default:false"`
	// CreatedBy is the ecosystem (IdP) user id for dashboard-created links, 0
	// for S2S-created ones. CreatedVia records the source: "dashboard" or the
	// S2S API key name.
	CreatedBy  int64  `gorm:"not null;default:0;index"`
	CreatedVia string `gorm:"size:100;not null;default:''"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName keeps the legacy singular name.
func (ShortLink) TableName() string { return "short_link" }

// ShortLinkVisit is one recorded hit on a short link.
type ShortLinkVisit struct {
	ID          int64  `gorm:"primaryKey"`
	ShortLinkID int64  `gorm:"not null;index;index:idx_visit_link_created;index:idx_visit_link_ip"`
	IP          string `gorm:"size:45;not null;default:'';index:idx_visit_link_ip"`
	UserAgent   string `gorm:"size:500;not null;default:''"`
	Referer     string `gorm:"size:500;not null;default:''"`
	// FpHash is hex(sha256(ip + "\n" + user_agent)).
	FpHash string `gorm:"size:64;not null;default:''"`
	// IsUnique marks the first visit from this IP within the bucket window.
	IsUnique  bool      `gorm:"not null;default:false"`
	CreatedAt time.Time `gorm:"index:idx_visit_link_created"`

	ShortLink ShortLink `gorm:"constraint:OnDelete:CASCADE"`
}

// TableName keeps the legacy singular name.
func (ShortLinkVisit) TableName() string { return "short_link_visit" }

// ShortLinkVisitorDay records that a fingerprint was seen on a link on a JST
// day. All three columns form the primary key: the insert either lands (first
// sighting) or conflicts, so the day's unique count needs no read-then-write.
type ShortLinkVisitorDay struct {
	ShortLinkID int64     `gorm:"primaryKey"`
	Day         time.Time `gorm:"type:date;primaryKey"`
	FpHash      string    `gorm:"size:64;primaryKey"`
	CreatedAt   time.Time

	ShortLink ShortLink `gorm:"constraint:OnDelete:CASCADE"`
}

// TableName names the settlement-grade visitor-day table.
func (ShortLinkVisitorDay) TableName() string { return "short_link_visitor_days" }

// ShortLinkVisitDay is the per-JST-day aggregate the settlement surface reads:
// total hits and deduplicated visitors.
type ShortLinkVisitDay struct {
	ShortLinkID int64     `gorm:"primaryKey"`
	Day         time.Time `gorm:"type:date;primaryKey"`
	Total       int64     `gorm:"not null;default:0"`
	Uniques     int64     `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	ShortLink ShortLink `gorm:"constraint:OnDelete:CASCADE"`
}

// TableName names the settlement-grade daily aggregate table.
func (ShortLinkVisitDay) TableName() string { return "short_link_visit_days" }

// ShortLinkVisitBucket is the hourly aggregate used by the stats charts.
type ShortLinkVisitBucket struct {
	ID          int64     `gorm:"primaryKey"`
	ShortLinkID int64     `gorm:"not null;uniqueIndex:uniq_bucket_link_start;index"`
	BucketStart time.Time `gorm:"not null;uniqueIndex:uniq_bucket_link_start;index"`
	Visits      int64     `gorm:"not null;default:0"`
	UniqueIPs   int64     `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	ShortLink ShortLink `gorm:"constraint:OnDelete:CASCADE"`
}

// TableName keeps the legacy singular name.
func (ShortLinkVisitBucket) TableName() string { return "short_link_visit_bucket" }

// APIKey is an S2S credential for a sibling product (kungal / moyu / …). The
// secret is stored as a SHA-256 hash; the plaintext (slk_ prefixed) is shown
// exactly once at creation. Prefix keeps the first characters for display.
type APIKey struct {
	ID         int64  `gorm:"primaryKey"`
	Name       string `gorm:"size:100;not null;uniqueIndex"`
	KeyHash    string `gorm:"size:64;not null;uniqueIndex"`
	KeyPrefix  string `gorm:"size:16;not null"`
	Disabled   bool   `gorm:"not null;default:false"`
	LastUsedAt *time.Time
	CreatedBy  int64 `gorm:"not null;default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName keeps the singular convention of the schema.
func (APIKey) TableName() string { return "api_key" }
