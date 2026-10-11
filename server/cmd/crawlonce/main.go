package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"gongkao/internal/config"
	"gongkao/internal/crawler"
	"gongkao/internal/model"
	"gongkao/internal/pkg"
	"gongkao/internal/service"
)

// One-shot crawl for local verification: go run ./cmd/crawlonce
func main() {
	config.LoadDotEnv(".env", ".env.example")
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := pkg.OpenDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := pkg.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}
	ann := service.NewAnnouncementService(db)
	aff := service.NewAffairsService(db)
	crawler.NewRunner(db, ann, aff, cfg).RunOnce(context.Background())

	var annN, affN, failN int64
	db.Model(&model.Announcement{}).Count(&annN)
	db.Model(&model.AffairsArticle{}).Count(&affN)
	db.Model(&model.CrawlFailure{}).Count(&failN)
	slog.Info("crawlonce done", "announcements", annN, "affairs", affN, "failures", failN)
}
