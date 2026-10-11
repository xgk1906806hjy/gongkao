package crawler

import (
	"strings"
	"testing"
	"time"
)

func TestExtractTimelineRegisterRange(t *testing.T) {
	text := "本次报名时间为2026年1月19日9:00至1月25日18:00。公共科目笔试时间2026年3月15日上午。"
	tl := extractTimeline(text)
	if tl.RegisterStart == nil || tl.RegisterStart.Year() != 2026 || tl.RegisterStart.Day() != 19 || tl.RegisterStart.Hour() != 9 {
		t.Fatalf("start %#v", tl.RegisterStart)
	}
	if tl.RegisterEnd == nil || tl.RegisterEnd.Month() != time.January || tl.RegisterEnd.Day() != 25 || tl.RegisterEnd.Hour() != 18 {
		t.Fatalf("end %#v", tl.RegisterEnd)
	}
	if tl.ExamAt == nil || tl.ExamAt.Month() != time.March || tl.ExamAt.Day() != 15 {
		t.Fatalf("exam %#v", tl.ExamAt)
	}
}

func TestExtractTimelineNearestDate(t *testing.T) {
	text := "不得报名应聘。四、报名及审核。1．报名。7月29日11:00—8月7日17:00。2．资格审查。7月29日11:00—8月8日9:00。"
	tl := extractTimeline(text)
	if tl.RegisterStart == nil || tl.RegisterStart.Month() != time.July || tl.RegisterStart.Day() != 29 || tl.RegisterStart.Hour() != 11 {
		t.Fatalf("start %#v", tl.RegisterStart)
	}
	if tl.RegisterEnd == nil || tl.RegisterEnd.Month() != time.August || tl.RegisterEnd.Day() != 7 || tl.RegisterEnd.Hour() != 17 {
		t.Fatalf("end %#v", tl.RegisterEnd)
	}
	if tl.ConfirmEnd == nil || tl.ConfirmEnd.Day() != 8 {
		t.Fatalf("confirm %#v", tl.ConfirmEnd)
	}
}

func TestParseURLDate(t *testing.T) {
	got := parseURLDate("https://rst.hubei.gov.cn/zpgg/202607/t20260727_5984486.shtml")
	if got == nil || got.Month() != time.July || got.Day() != 27 || got.Year() != 2026 {
		t.Fatalf("%v", got)
	}
}

func TestSplitTrailingDate(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.Local)
	title, pub := splitTrailingDate("湖北省2026年度省市县乡考试录用公务员公告2026-01-12", now)
	if title != "湖北省2026年度省市县乡考试录用公务员公告" {
		t.Fatalf("title %q", title)
	}
	if pub.Year() != 2026 || pub.Month() != time.January || pub.Day() != 12 {
		t.Fatalf("pub %v", pub)
	}
	title, pub = splitTrailingDate("湖北省事业单位2026年统一公开招聘高校专职辅导员公告07-23", now)
	if !strings.Contains(title, "公告") || strings.Contains(title, "07-23") {
		t.Fatalf("title %q", title)
	}
	if pub.Month() != time.July || pub.Day() != 23 || pub.Year() != 2026 {
		t.Fatalf("pub %v", pub)
	}
}
