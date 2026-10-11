package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gongkao/internal/config"
	"gongkao/internal/model"
	"gongkao/internal/service"

	"gorm.io/gorm"
)

// Source is a pluggable crawl unit.
type Source interface {
	Name() string
	Fetch(ctx context.Context) error
}

// announcementWriter stores crawled announcements. The API service and the Excel collector both implement it.
type announcementWriter interface {
	UpsertFromCrawl(a *model.Announcement) error
}

type Runner struct {
	db      *gorm.DB
	ann     announcementWriter
	aff     *service.AffairsService
	sources []Source
	client  *http.Client
}

func NewRunner(db *gorm.DB, ann *service.AnnouncementService, aff *service.AffairsService, cfg *config.Config) *Runner {
	r := &Runner{
		db:     db,
		ann:    ann,
		aff:    aff,
		client: defaultHTTPClient(),
	}
	r.sources = buildSources(r, cfg)
	return r
}

func buildSources(r *Runner, cfg *config.Config) []Source {
	sources := []Source{}
	seedFallback := true
	if cfg != nil {
		seedFallback = cfg.CrawlerSeedFallback
	}

	// Live sources (public, no API key).
	sources = append(sources,
		&cptaSource{runner: r, url: "http://www.cpta.com.cn/", client: r.client},
		&govZhengceSource{
			runner: r,
			url:    "https://www.gov.cn/zhengce/zuixin/ZUIXINZHENGCE.json",
			client: r.client,
			limit:  40,
		},
		&rssAffairsSource{
			runner: r, name: "xinhua-politics", tag: "时政,新华网",
			url:    "http://www.news.cn/politics/news_politics.xml",
			client: r.client, limit: 40,
		},
		&rssAffairsSource{
			runner: r, name: "people-politics", tag: "时政,人民网",
			url:    "http://www.people.com.cn/rss/politics.xml",
			client: r.client, limit: 40,
		},
		&rssAffairsSource{
			runner: r, name: "xinhua-legal", tag: "法治,新华网",
			url:    "http://www.xinhuanet.com/legal/news_legal.xml",
			client: r.client, limit: 20,
		},
	)

	// Extra affairs RSS from env (comma-separated), optional override additions.
	if cfg != nil {
		extra := cfg.CrawlerAffairsRSSURL
		if extra == "" {
			extra = cfg.CrawlerAffairsRSSURLs
		}
		for _, u := range splitCSV(extra) {
			sources = append(sources, &rssAffairsSource{
				runner: r, name: "rss-extra", tag: "时政",
				url: u, client: r.client, limit: 30,
			})
		}
	}

	if seedFallback {
		sources = append(sources,
			&seedAnnouncementSource{runner: r},
			&seedAffairsSource{runner: r},
		)
	}
	return sources
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (r *Runner) RunOnce(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, src := range r.sources {
		if err := src.Fetch(ctx); err != nil {
			slog.Error("crawl source failed", "source", src.Name(), "err", err)
			_ = r.recordFailure(src.Name(), "", err)
		}
	}
}

func (r *Runner) recordFailure(source, url string, err error) error {
	if r.db == nil {
		return nil
	}
	return r.db.Create(&model.CrawlFailure{
		Source: source, URL: url, Error: err.Error(), CreatedAt: time.Now(),
	}).Error
}

// ---- seed fallback (demo data when live sources are empty / offline) ----

type seedAnnouncementSource struct{ runner *Runner }

func (s *seedAnnouncementSource) Name() string { return "seed-announcement" }

func (s *seedAnnouncementSource) Fetch(ctx context.Context) error {
	_ = ctx
	var n int64
	s.runner.db.Model(&model.Announcement{}).Where("source <> ?", "seed").Count(&n)
	if n > 0 {
		slog.Info("seed announcement skipped: live data present", "live", n)
		return nil
	}
	now := time.Now()
	items := []model.Announcement{
		{
			Type: model.AnnouncementCivil, Title: "2026年度中央机关及其直属机构公务员招考公告（示例）",
			Region: "全国", Source: "seed", SourceID: "civil-2026-central",
			SourceURL: "https://example.com/civil/2026", Summary: "演示数据：中央机关公务员招考示例公告。",
			RegisterStart: ptrTime(now.Add(24 * time.Hour)), RegisterEnd: ptrTime(now.Add(72 * time.Hour)),
			ConfirmStart: ptrTime(now.Add(96 * time.Hour)), ConfirmEnd: ptrTime(now.Add(120 * time.Hour)),
			ExamAt: ptrTime(now.Add(240 * time.Hour)), PublishedAt: now, Status: model.StatusPublished,
		},
		{
			Type: model.AnnouncementInstitution, Title: "某省事业单位公开招聘工作人员公告（示例）",
			Region: "示例省", Source: "seed", SourceID: "inst-demo-001",
			SourceURL: "https://example.com/inst/001", Summary: "演示数据：事业单位招聘示例公告。",
			RegisterStart: ptrTime(now.Add(12 * time.Hour)), RegisterEnd: ptrTime(now.Add(48 * time.Hour)),
			ExamAt: ptrTime(now.Add(168 * time.Hour)), PublishedAt: now.Add(-2 * time.Hour), Status: model.StatusPublished,
		},
	}
	for i := range items {
		if err := s.runner.ann.UpsertFromCrawl(&items[i]); err != nil {
			return err
		}
	}
	slog.Info("seed announcement upserted", "count", len(items))
	return nil
}

type seedAffairsSource struct{ runner *Runner }

func (s *seedAffairsSource) Name() string { return "seed-affairs" }

func (s *seedAffairsSource) Fetch(ctx context.Context) error {
	_ = ctx
	var n int64
	s.runner.db.Model(&model.AffairsArticle{}).Where("source NOT LIKE ?", "seed%").Count(&n)
	if n > 0 {
		slog.Info("seed affairs skipped: live data present", "live", n)
		return nil
	}
	now := time.Now()
	items := []model.AffairsArticle{
		{
			Title: "示例时政：高质量发展专题学习要点", Source: "seed", SourceURL: "https://example.com/affairs/hq",
			Summary: "演示数据：高质量发展学习要点。", Content: "坚持创新驱动，推动产业升级与绿色发展。",
			Tags: "学习,经济", PublishedAt: now, Status: model.StatusPublished,
		},
	}
	for i := range items {
		if err := s.runner.aff.UpsertFromCrawl(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func ptrTime(t time.Time) *time.Time { return &t }
