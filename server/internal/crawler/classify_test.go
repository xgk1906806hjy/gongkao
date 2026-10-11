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
}
