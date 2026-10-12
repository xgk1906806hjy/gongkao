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
	subSvc := service.NewSubscribeService(db)
	remindSvc := service.NewReminderService(db, rdb, wxCli, cfg, subSvc)
	affPushSvc := service.NewAffairsPushService(db, wxCli, cfg, subSvc)
	runner := crawler.NewRunner(db, annSvc, affSvc, cfg)

	ctx := context.Background()
	_ = examSvc.EnqueuePendingReminders(ctx)

	c := cron.New()
	_, _ = c.AddFunc("@every 1m", func() {
		remindSvc.ProcessDue(context.Background())
	})
	pracSvc := service.NewPracticeService(db)
	refreshAnnouncements := func(ctx context.Context) {
		if _, err := crawler.WriteAnnouncementWorkbook(ctx, cfg); err != nil {
			slog.Error("announcement workbook", "err", err)
		}
		n, err := annSvc.ImportFromXLSX(cfg.AnnouncementXLSX)
		if err != nil {
			slog.Error("announcement xlsx import", "err", err)
			return
		}
		slog.Info("announcements imported from xlsx", "count", n, "path", cfg.AnnouncementXLSX)
	}
	refreshQuestionBank := func(ctx context.Context) {
		items, err := crawler.CollectQuestions(ctx, db, cfg)
		if err != nil {
			slog.Error("question bank collect", "err", err)
			return
		}
		if _, err := crawler.SaveQuestionBankJSON(cfg, items); err != nil {
			slog.Error("question bank write", "err", err)
		}
		n, err := pracSvc.ImportQuestionsSkipDup(items)
		if err != nil {
			slog.Error("question bank import", "err", err)
			return
		}
		slog.Info("question bank imported", "collected", len(items), "imported_new", n)
	}
	if cfg.CrawlerEnabled {
		_, _ = c.AddFunc("@every 30m", func() {
			bg := context.Background()
			refreshAnnouncements(bg)
			runner.RunOnce(bg)
			if cfg.CrawlerQuestionBankEnabled {
				refreshQuestionBank(bg)
			}
		})
		refreshAnnouncements(ctx)
		runner.RunOnce(ctx)
		if cfg.CrawlerQuestionBankEnabled {
			refreshQuestionBank(ctx)
		}
	}
	if cfg.AffairsDailyPushEnabled {
		_, _ = c.AddFunc("0 8 * * *", func() {
			affPushSvc.PushDaily(context.Background())
		})
	}
	c.Start()
	slog.Info("worker started", "crawler", cfg.CrawlerEnabled, "wx_mock", cfg.WXMock, "affairs_push", cfg.AffairsDailyPushEnabled)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctxStop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Stop()
	<-ctxStop.Done()
	slog.Info("worker stopped")
}
