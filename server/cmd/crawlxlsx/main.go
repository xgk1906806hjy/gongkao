package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"gongkao/internal/config"
	"gongkao/internal/crawler"
)

// Crawl public 公考 announcements into an Excel workbook.
// The API and worker parse that workbook into the announcement module.
//
//	go run ./cmd/crawlxlsx
func main() {
	config.LoadDotEnv(".env", ".env.example")
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	n, err := crawler.WriteAnnouncementWorkbook(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("crawlxlsx done", "rows", n, "path", cfg.AnnouncementXLSX)
}
