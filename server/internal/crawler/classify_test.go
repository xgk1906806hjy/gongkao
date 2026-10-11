package crawler

import "testing"

func TestIsExamRelatedTitle(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"2026年度中央机关及其直属机构考试录用公务员公告", true},
		{"某省事业单位公开招聘工作人员公告", true},
		{"关于发展新质生产力的意见", false},
		{"公开招聘工作人员简章", true},
	}
	for _, c := range cases {
		if got := isExamRelatedTitle(c.title); got != c.want {
			t.Fatalf("%q got %v want %v", c.title, got, c.want)
		}
	}
}

func TestClassifyAnnouncementType(t *testing.T) {
	if classifyAnnouncementType("某市事业单位公开招聘") != "institution" {
		t.Fatal("expected institution")
	}
	if classifyAnnouncementType("国家公务员考试录用公告") != "civil" {
		t.Fatal("expected civil")
	}
	if classifyAnnouncementType("湖南城市学院2026年度高层次人才公开招聘公告") != "institution" {
		t.Fatal("expected institution for 公开招聘")
	}
	if classifyAnnouncementType("湖南省2027年定向选调公告") != "civil" {
		t.Fatal("expected civil for 选调")
	}
	if announcementRegion("中南林业科技大学2026年公开招聘公告", "湖南") != "湖南" {
		t.Fatal("expected fallback region")
	}
	if announcementRegion("湖北省2026年度考试录用公务员公告", "湖南") != "湖北" {
		t.Fatal("expected title region")
	}
}
