package crawler

import (
	"context"
	"log/slog"
	"time"

	"gongkao/internal/model"
	"gongkao/internal/service"

	"gorm.io/gorm"
)

// SeedCrawler inserts demo announcements when DB is empty / on schedule for demo sources.
type Runner struct {
	db   *gorm.DB
	ann  *service.AnnouncementService
	aff  *service.AffairsService
}

func NewRunner(db *gorm.DB, ann *service.AnnouncementService, aff *service.AffairsService) *Runner {
	return &Runner{db: db, ann: ann, aff: aff}
}

func (r *Runner) RunOnce(ctx context.Context) {
	r.crawlAnnouncements()
	r.crawlAffairs()
}

func (r *Runner) crawlAnnouncements() {
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
		if err := r.ann.UpsertFromCrawl(&items[i]); err != nil {
			slog.Error("crawl announcement upsert", "err", err)
			_ = r.db.Create(&model.CrawlFailure{
				Source: items[i].Source, URL: items[i].SourceURL, Error: err.Error(), CreatedAt: time.Now(),
			}).Error
		}
	}
	slog.Info("announcement crawl done", "count", len(items))
}

func (r *Runner) crawlAffairs() {
	now := time.Now()
	items := []model.AffairsArticle{
		{
			Title: "示例时政：高质量发展专题学习要点", Source: "seed-rss", SourceURL: "https://example.com/affairs/hq",
			Summary: "围绕高质量发展内涵学习要点整理（演示）。", Content: "坚持创新驱动，推动产业升级与绿色发展。",
			Tags: "学习,经济", PublishedAt: now, Status: model.StatusPublished,
		},
	}
	for i := range items {
		if err := r.aff.UpsertFromCrawl(&items[i]); err != nil {
			slog.Error("crawl affairs upsert", "err", err)
			_ = r.db.Create(&model.CrawlFailure{
				Source: items[i].Source, URL: items[i].SourceURL, Error: err.Error(), CreatedAt: time.Now(),
			}).Error
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }