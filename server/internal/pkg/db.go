package pkg

import (
	"log/slog"
	"strings"
	"time"

	"gongkao/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func OpenDB(dsn string) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)}
	if strings.HasPrefix(dsn, "sqlite:") {
		path := strings.TrimPrefix(dsn, "sqlite:")
		db, err = gorm.Open(sqlite.Open(path), cfg)
	} else {
		db, err = gorm.Open(postgres.Open(dsn), cfg)
	}
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return err
	}
	// Best-effort unique indexes (Postgres partial index; SQLite ignores WHERE gracefully via plain unique fallback).
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_announcement_source ON announcements(source, source_id)`).Error; err != nil {
		slog.Warn("announcement unique index", "err", err)
	}
	slog.Info("migrate done")
	return nil
}