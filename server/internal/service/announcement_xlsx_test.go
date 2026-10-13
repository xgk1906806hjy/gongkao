package service

import (
	"path/filepath"
	"testing"
	"time"

	"gongkao/internal/annxlsx"
	"gongkao/internal/model"
)

func TestImportFromXLSXOfflinesSeed(t *testing.T) {
	dir := t.TempDir()
	db := newTestDB(t)
	svc := NewAnnouncementService(db)
	if err := svc.UpsertFromCrawl(&model.Announcement{
		Type:        model.AnnouncementCivil,
		Title:       "示例公告",
		Source:      "seed",
		SourceID:    "demo-1",
		SourceURL:   "https://example.com/seed",
		Status:      model.StatusPublished,
		PublishedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 19, 9, 0, 0, 0, time.Local)
	path := filepath.Join(dir, "announcements.xlsx")
	if err := annxlsx.Write(path, []model.Announcement{{
		Type:          model.AnnouncementCivil,
		Title:         "湖北省2026年度省市县乡考试录用公务员公告",
		Region:        "湖北",
		Source:        "hubei-rsks",
		SourceID:      "live-1",
		SourceURL:     "https://rst.hubei.gov.cn/a.shtml",
		Summary:       "摘要",
		RegisterStart: &start,
		PublishedAt:   time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local),
		Status:        model.StatusPublished,
	}}); err != nil {
		t.Fatal(err)
	}
	n, err := svc.ImportFromXLSX(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("imported %d", n)
	}
	var seed model.Announcement
	if err := db.Where("source = ?", "seed").First(&seed).Error; err != nil {
		t.Fatal(err)
	}
	if seed.Status != model.StatusOffline {
		t.Fatalf("seed status %s", seed.Status)
	}
	var live model.Announcement
	if err := db.Where("source = ?", "hubei-rsks").First(&live).Error; err != nil {
		t.Fatal(err)
	}
	if live.RegisterStart == nil || live.RegisterStart.Day() != 19 {
		t.Fatalf("register %#v", live.RegisterStart)
	}
}

func TestImportFromXLSXMissing(t *testing.T) {
	dir := t.TempDir()
	db := newTestDB(t)
	n, err := NewAnnouncementService(db).ImportFromXLSX(filepath.Join(dir, "nope.xlsx"))
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
