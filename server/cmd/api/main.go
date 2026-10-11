package main

import (
	"log"
	"log/slog"
	"os"

	"gongkao/internal/config"
	"gongkao/internal/crawler"
	"gongkao/internal/handler"
	"gongkao/internal/pkg"
	"gongkao/internal/service"
	"gongkao/internal/wx"

	"github.com/gin-gonic/gin"
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
		slog.Warn("redis unavailable, reminders fall back to DB scan", "err", err)
		rdb = nil
	}

	wxCli := wx.NewClient(cfg.WXAppID, cfg.WXSecret, cfg.WXMock, rdb)
	jwtm := pkg.NewJWTManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	authSvc := service.NewAuthService(db, wxCli, jwtm, cfg)
	annSvc := service.NewAnnouncementService(db)
	examSvc := service.NewExamTodoService(db, rdb)
	pracSvc := service.NewPracticeService(db)
	affSvc := service.NewAffairsService(db)
	subSvc := service.NewSubscribeService(db)

	if err := authSvc.EnsureAdmin(); err != nil {
		log.Fatalf("admin bootstrap: %v", err)
	}
	_ = pracSvc.SeedIfEmpty()
	_ = affSvc.SeedIfEmpty()

	if n, err := annSvc.ImportFromXLSX(cfg.AnnouncementXLSX); err != nil {
		slog.Warn("announcement xlsx import", "err", err, "path", cfg.AnnouncementXLSX)
	} else if n > 0 {
		slog.Info("announcements imported from xlsx", "count", n, "path", cfg.AnnouncementXLSX)
	}

	// affairs crawl, plus announcement seed only when no live rows exist
	crawler.NewRunner(db, annSvc, affSvc, cfg).RunOnce(nil)

	h := &handler.Handlers{
		Auth: authSvc, Ann: annSvc, Exam: examSvc, Prac: pracSvc, Aff: affSvc, Sub: subSvc,
		JWT: jwtm, DB: db,
		ClientCfg: func() gin.H {
			todoTpl := cfg.WXSubscribeTemplateTodo
			affTpl := cfg.WXSubscribeTemplateAff
			if todoTpl == "" && cfg.WXMock {
				todoTpl = "MOCK_TODO_TEMPLATE"
			}
			if affTpl == "" && cfg.WXMock {
				affTpl = "MOCK_AFFAIRS_TEMPLATE"
			}
			return gin.H{
				"todo_template_id":    todoTpl,
				"affairs_template_id": affTpl,
				"wx_mock":             cfg.WXMock,
			}
		},
	}

	if cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	handler.Register(r, h)

	slog.Info("api listening", "addr", cfg.HTTPAddr, "wx_mock", cfg.WXMock)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
