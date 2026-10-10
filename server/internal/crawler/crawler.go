package crawler

import (
	"context"
	"encoding/xml"
	"io"
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

type Runner struct {
	db      *gorm.DB
	ann     *service.AnnouncementService
	aff     *service.AffairsService
	sources []Source
}

func NewRunner(db *gorm.DB, ann *service.AnnouncementService, aff *service.AffairsService, cfg *config.Config) *Runner {
	r := &Runner{db: db, ann: ann, aff: aff}
	r.sources = []Source{
		&seedAnnouncementSource{runner: r},
		&seedAffairsSource{runner: r},
	}
	if cfg != nil && strings.TrimSpace(cfg.CrawlerAffairsRSSURL) != "" {
		r.sources = append(r.sources, &rssAffairsSource{
			runner: r,
			url:    strings.TrimSpace(cfg.CrawlerAffairsRSSURL),
			client: &http.Client{Timeout: 20 * time.Second},
		})
	}
	return r
}

func (r *Runner) RunOnce(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, src := range r.sources {
		if err := src.Fetch(ctx); err != nil {
			slog.Error("crawl source failed", "source", src.Name(), "err", err)
			_ = r.db.Create(&model.CrawlFailure{
				Source: src.Name(), Error: err.Error(), CreatedAt: time.Now(),
			}).Error
		}
	}
}

type seedAnnouncementSource struct{ runner *Runner }

func (s *seedAnnouncementSource) Name() string { return "seed-announcement" }

func (s *seedAnnouncementSource) Fetch(ctx context.Context) error {
	_ = ctx
	now := time.Now()
	items := []model.Announcement{
		{
			Type: model.AnnouncementCivil, Title: "2026年度中央机关及其直属机构公务员招考公告（示例）",
			Region: "全国", Source: "seed", SourceID: "civil-2026-central",
			SourceURL: "https://example.com/civil/2026", Summary: "现将2026年度中央机关及其直属机构考试录用公务员有关事项公告如下（演示数据）。",
			RegisterStart: ptrTime(now.Add(24 * time.Hour)), RegisterEnd: ptrTime(now.Add(72 * time.Hour)),
			ConfirmStart: ptrTime(now.Add(96 * time.Hour)), ConfirmEnd: ptrTime(now.Add(120 * time.Hour)),
			ExamAt: ptrTime(now.Add(240 * time.Hour)), PublishedAt: now, Status: model.StatusPublished,
		},
		{
			Type: model.AnnouncementInstitution, Title: "某省事业单位公开招聘工作人员公告（示例）",
			Region: "示例省", Source: "seed", SourceID: "inst-demo-001",
			SourceURL: "https://example.com/inst/001", Summary: "根据工作需要，面向社会公开招聘事业单位工作人员（演示数据）。",
			RegisterStart: ptrTime(now.Add(12 * time.Hour)), RegisterEnd: ptrTime(now.Add(48 * time.Hour)),
			ExamAt: ptrTime(now.Add(168 * time.Hour)), PublishedAt: now.Add(-2 * time.Hour), Status: model.StatusPublished,
		},
		{
			Type: model.AnnouncementCivil, Title: "某市考试录用公务员公告（示例）",
			Region: "示例市", Source: "seed", SourceID: "civil-city-001",
			SourceURL: "https://example.com/civil/city001", Summary: "为充实公务员队伍，计划考试录用公务员若干名（演示数据）。",
			RegisterEnd: ptrTime(now.Add(36 * time.Hour)), ExamAt: ptrTime(now.Add(200 * time.Hour)),
			PublishedAt: now.Add(-5 * time.Hour), Status: model.StatusPublished,
		},
	}
	for i := range items {
		if err := s.runner.ann.UpsertFromCrawl(&items[i]); err != nil {
			_ = s.runner.db.Create(&model.CrawlFailure{
				Source: items[i].Source, URL: items[i].SourceURL, Error: err.Error(), CreatedAt: time.Now(),
			}).Error
			return err
		}
	}
	slog.Info("announcement crawl done", "source", s.Name(), "count", len(items))
	return nil
}

type seedAffairsSource struct{ runner *Runner }

func (s *seedAffairsSource) Name() string { return "seed-affairs" }

func (s *seedAffairsSource) Fetch(ctx context.Context) error {
	_ = ctx
	now := time.Now()
	items := []model.AffairsArticle{
		{
			Title: "示例时政：高质量发展专题学习要点", Source: "seed-rss", SourceURL: "https://example.com/affairs/hq",
			Summary: "围绕高质量发展纲领学习要点整理（演示）。", Content: "坚持创新驱动，推动产业升级与绿色发展。",
			Tags: "学习,经济", PublishedAt: now, Status: model.StatusPublished,
		},
	}
	for i := range items {
		if err := s.runner.aff.UpsertFromCrawl(&items[i]); err != nil {
			_ = s.runner.db.Create(&model.CrawlFailure{
				Source: items[i].Source, URL: items[i].SourceURL, Error: err.Error(), CreatedAt: time.Now(),
			}).Error
			return err
		}
	}
	return nil
}

type rssAffairsSource struct {
	runner *Runner
	url    string
	client *http.Client
}

func (s *rssAffairsSource) Name() string { return "rss-affairs" }

type rssFeed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
}

func (s *rssAffairsSource) Fetch(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "gongkao-crawler/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &httpError{code: resp.StatusCode, body: string(body)}
	}
	var feed rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return err
	}
	n := 0
	for _, it := range feed.Channel.Items {
		if strings.TrimSpace(it.Title) == "" || strings.TrimSpace(it.Link) == "" {
			continue
		}
		pub := time.Now()
		if t, err := time.Parse(time.RFC1123Z, it.PubDate); err == nil {
			pub = t
		} else if t, err := time.Parse(time.RFC1123, it.PubDate); err == nil {
			pub = t
		}
		art := &model.AffairsArticle{
			Title:       strings.TrimSpace(it.Title),
			Source:      "rss",
			SourceURL:   strings.TrimSpace(it.Link),
			Summary:     stripTags(it.Description),
			Content:     stripTags(it.Description),
			Tags:        strings.TrimSpace(it.Category),
			PublishedAt: pub,
			Status:      model.StatusPublished,
		}
		if err := s.runner.aff.UpsertFromCrawl(art); err != nil {
			_ = s.runner.db.Create(&model.CrawlFailure{
				Source: s.Name(), URL: art.SourceURL, Error: err.Error(), CreatedAt: time.Now(),
			}).Error
			continue
		}
		n++
	}
	slog.Info("rss affairs crawl done", "url", s.url, "upserted", n)
	return nil
}

type httpError struct {
	code int
	body string
}

func (e *httpError) Error() string {
	return "http " + http.StatusText(e.code) + ": " + e.body
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func ptrTime(t time.Time) *time.Time { return &t }
