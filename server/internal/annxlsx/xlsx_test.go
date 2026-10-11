package annxlsx

import (
	"path/filepath"
	"testing"
	"time"

	"gongkao/internal/model"
)

func TestWriteReadRoundTrip(t *testing.T) {
	start := time.Date(2026, 1, 19, 9, 0, 0, 0, time.Local)
	end := time.Date(2026, 1, 25, 18, 0, 0, 0, time.Local)
	pub := time.Date(2026, 1, 12, 0, 0, 0, 0, time.Local)
	in := []model.Announcement{{
		Type:          model.AnnouncementCivil,
		Title:         "湖北省2026年度省市县乡考试录用公务员公告",
		Region:        "湖北",
		Source:        "hubei-rsks",
		SourceID:      "abc",
		SourceURL:     "https://rst.hubei.gov.cn/a.shtml",
		Summary:       "报名摘要",
		Content:       "正文摘录",
		RegisterStart: &start,
		RegisterEnd:   &end,
		PublishedAt:   pub,
		Status:        model.StatusPublished,
	}}
	path := filepath.Join(t.TempDir(), "announcements.xlsx")
	if err := Write(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("rows %d", len(out))
	}
	got := out[0]
	if got.Title != in[0].Title || got.SourceURL != in[0].SourceURL || got.Region != "湖北" {
		t.Fatalf("row %+v", got)
	}
	if got.RegisterStart == nil || !got.RegisterStart.Equal(start) {
		t.Fatalf("register_start %v", got.RegisterStart)
	}
	if got.RegisterEnd == nil || !got.RegisterEnd.Equal(end) {
		t.Fatalf("register_end %v", got.RegisterEnd)
	}
	if !got.PublishedAt.Equal(pub) {
		t.Fatalf("published %v", got.PublishedAt)
	}
}

func TestReadMissing(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "missing.xlsx"))
	if err == nil {
		t.Fatal("expected missing file error")
	}
}
