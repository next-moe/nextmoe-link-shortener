// Package db owns the Postgres connection and schema migration. The schema is
// applied with GORM AutoMigrate at startup (the ecosystem's image/artifact
// service precedent for small services) — there is no separate migrate step.
package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// DB wraps the GORM handle.
type DB struct {
	Gorm *gorm.DB
}

// Open connects to Postgres, configures the pool, and applies the schema.
func Open(dsn string) (*DB, error) {
	g, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := g.AutoMigrate(
		&model.ShortLink{},
		&model.ShortLinkVisit{},
		&model.ShortLinkVisitBucket{},
		&model.ShortLinkVisitorDay{},
		&model.ShortLinkVisitDay{},
		&model.APIKey{},
	); err != nil {
		return nil, fmt.Errorf("db automigrate: %w", err)
	}
	return &DB{Gorm: g}, nil
}

// Close releases the underlying connection pool.
func (d *DB) Close() error {
	sqlDB, err := d.Gorm.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
