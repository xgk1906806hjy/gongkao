package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gongkao/internal/config"
	"gongkao/internal/crawler"
	"gongkao/internal/pkg"
	"gongkao/internal/service"
	"gongkao/internal/wx"

	"github.com/robfig/cron/v3"
)

func main() {
	config.LoadDotEnv(".env", ".env.example")
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := pkg.OpenDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := pkg.AutoMigrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	rdb, err := pkg.OpenRedis(cfg.RedisURL)
	if err != nil {
		slog.Warn("redis unavailable", "err", err)
		rdb = nil
	}

	wxCli := wx.NewClient(cfg.WXAppID, cfg.WXSecret, cfg.WXMock, rdb)
	annSvc := service.NewAnnouncementService(db)
	affSvc := service.NewAffairsService(db)
	examSvc := service.NewExamTodoService(db, rdb)
	remindSvc := service.NewReminderService(db, rdb, wxCli, cfg)
	runner := crawler.NewRunner(db, annSvc, affSvc)

	ctx := context.Background()
	_ = examSvc.EnqueuePendingReminders(ctx)

	c := cron.New()
	_, _ = c.AddFunc("@every 1m", func() {
		remindSvc.ProcessDue(context.Background())
	})
	if cfg.CrawlerEnabled {
		_, _ = c.AddFunc("@every 30m", func() {
			runner.RunOnce(context.Background())
		})
		runner.RunOnce(ctx)
	}
	c.Start()
	slog.Info("worker started", "crawler", cfg.CrawlerEnabled, "wx_mock", cfg.WXMock)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctxStop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Stop()
	<-ctxStop.Done()
	slog.Info("worker stopped")
}