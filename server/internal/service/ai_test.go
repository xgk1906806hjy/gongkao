package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gongkao/internal/ai"
	"gongkao/internal/model"
	"gongkao/internal/pkg"

	"gorm.io/gorm"
)

const sampleResume = `张三 | 湖南大学 会计学专业 本科 2027届应届毕业生
政治面貌：中共党员
证书：英语六级、初级会计师、计算机二级
求职意向：意向地区 湖南、广东`

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := pkg.OpenDB("sqlite:" + filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := pkg.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func fixedNow() time.Time { return time.Date(2026, 10, 13, 9, 0, 0, 0, time.Local) }

func seedAnnouncements(t *testing.T, db *gorm.DB, now time.Time) (hunan, beijing uint64) {
	t.Helper()
	start := now.AddDate(0, 0, -2)
	end := now.AddDate(0, 0, 5)
	exam := now.AddDate(0, 1, 0)
	a1 := model.Announcement{
		Type: model.AnnouncementCivil, Title: "湖南省2027年考试录用公务员公告（含选调生、税务会计岗位）",
		Region: "湖南", Source: "t", SourceID: "1", Summary: "面向应届高校毕业生，财会类专业",
		RegisterStart: &start, RegisterEnd: &end, ExamAt: &exam,
		PublishedAt: now.AddDate(0, 0, -3), Status: model.StatusPublished,
	}
	a2 := model.Announcement{
		Type: model.AnnouncementInstitution, Title: "北京市某研究院招聘博士研究生公告",
		Region: "北京", Source: "t", SourceID: "2", Summary: "招聘物理学博士",
		PublishedAt: now.AddDate(0, 0, -10), Status: model.StatusPublished,
	}
	if err := db.Create(&a1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&a2).Error; err != nil {
		t.Fatal(err)
	}
	return a1.ID, a2.ID
}

func TestExtractProfileFromText(t *testing.T) {
	p := ExtractProfileFromText(sampleResume, fixedNow())
	if p.Education != "本科" || p.Major != "会计学" {
		t.Fatalf("education/major: %+v", p)
	}
	if p.GraduationYear != 2027 || !p.FreshGraduate {
		t.Fatalf("graduation: %+v", p)
	}
	if p.PoliticalStatus != "中共党员" {
		t.Fatalf("political: %q", p.PoliticalStatus)
	}
	for _, c := range []string{"英语六级", "初级会计师", "计算机二级"} {
		if !strings.Contains(p.Certificates, c) {
			t.Fatalf("missing cert %s in %q", c, p.Certificates)
		}
	}
	if p.TargetRegions != "湖南,广东" {
		t.Fatalf("regions: %q", p.TargetRegions)
	}
}

func TestAnalyzeResumeRuleEngine(t *testing.T) {
	db := newTestDB(t)
	svc := NewAIService(db, ai.NewClient("", "", "", 0), 5)
	svc.now = fixedNow
	hunan, _ := seedAnnouncements(t, db, fixedNow())

	res, err := svc.AnalyzeResume(context.Background(), 1, ResumeRequest{ResumeText: sampleResume})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "rule" || res.ReportID == 0 {
		t.Fatalf("provider/report: %+v", res)
	}
	if len(res.Recommendations) == 0 || res.Recommendations[0].AnnouncementID != hunan {
		t.Fatalf("want hunan announcement first, got %+v", res.Recommendations)
	}
	if res.Profile.ResumeText != "" {
		t.Fatal("resume text must not be echoed back")
	}
	stored, _ := svc.GetProfile(1)
	if stored.Major != "会计学" || stored.ResumeText == "" {
		t.Fatalf("profile not persisted: %+v", stored)
	}
	if len(res.Directions) == 0 || !strings.Contains(res.Directions[0].Name, "税务") {
		t.Fatalf("directions: %+v", res.Directions)
	}
}

func TestIsRecruitmentAnnouncement(t *testing.T) {
	cases := map[string]bool{
		"重庆市属事业单位2026年第三季度公开招聘工作人员公告":           true,
		"中央机关及其直属机构2026年度考试录用公务员公告":             true,
		"北京工商大学2026年人才引进公告（第二批）":                true,
		"关于2026年度社会工作者职业资格考试单独划线地区合格标准的通告":      false,
		"关于公布重庆市事业单位2026年第三季度公开招聘（遴选）报名确认人数的公告": false,
		"北京市商务局综合事务中心2026年度面向退役大学生士兵招聘工作人员面试公告": false,
		"2026年度新闻记者职业资格考试报考提醒":                  false,
		"中央机关及其直属机构2026年度考试录用公务员（湖南考区）笔试参考提示":   false,
		"湖南省教育厅所属事业单位2026年公开招聘第二批拟聘用人员公示":       false,
		"湖南省2027年常规选调公告":                        true,
	}
	for title, want := range cases {
		if got := IsRecruitmentAnnouncement(&model.Announcement{Title: title}); got != want {
			t.Errorf("%s: want %v got %v", title, want, got)
		}
	}
	if IsRecruitmentAnnouncement(&model.Announcement{Title: "某市考试录用公务员公告（示例）", Source: "seed"}) {
		t.Error("seed rows must be excluded")
	}
}

func TestScoreAnnouncementPenalizesTransferForFreshGraduate(t *testing.T) {
	p := &model.AIProfile{FreshGraduate: true, TargetRegions: "湖南", PoliticalStatus: "中共党员"}
	xuandiao := &model.Announcement{Type: model.AnnouncementCivil, Title: "湖南省2027年常规选调公告", Region: "湖南"}
	linxuan := &model.Announcement{Type: model.AnnouncementCivil, Title: "2026年度湖南省省直机关公开遴选公务员公告", Region: "湖南", Summary: "选调"}
	s1, _, _ := ScoreAnnouncement(p, xuandiao, fixedNow())
	s2, _, cautions := ScoreAnnouncement(p, linxuan, fixedNow())
	if s2 >= s1 {
		t.Fatalf("遴选 should rank below 选调 for fresh graduates: %d vs %d", s2, s1)
	}
	if !strings.Contains(strings.Join(cautions, ""), "遴选") {
		t.Fatalf("missing 遴选 caution: %v", cautions)
	}
}

func TestMajorTokensSkipGenericWords(t *testing.T) {
	got := majorTokens("计算机科学与技术")
	if !containsStr(got, "计算机科学与技术") || !containsStr(got, "计算机") || containsStr(got, "技术") {
		t.Fatalf("tokens: %v", got)
	}
	p := &model.AIProfile{Major: "计算机科学与技术"}
	_, reasons, _ := ScoreAnnouncement(p, &model.Announcement{Title: "重庆水利电力职业技术学院2026年考核招聘事业单位工作人员公告"}, fixedNow())
	for _, r := range reasons {
		if strings.Contains(r, "专业关键词") {
			t.Fatalf("generic 技术 must not count as a major match: %v", reasons)
		}
	}
}

func TestScoreAnnouncementPenalizesRestrictedGroups(t *testing.T) {
	p := &model.AIProfile{FreshGraduate: true, TargetRegions: "重庆", Major: "计算机科学与技术"}
	open := &model.Announcement{Type: model.AnnouncementInstitution, Title: "重庆市属事业单位2026年第三季度公开招聘工作人员公告", Region: "重庆"}
	targeted := &model.Announcement{Type: model.AnnouncementInstitution, Title: "重庆市潼南区教育事业单位2027年面向应届毕业公费师范生考核招聘工作人员公告", Region: "重庆"}
	s1, _, _ := ScoreAnnouncement(p, open, fixedNow())
	s2, _, cautions := ScoreAnnouncement(p, targeted, fixedNow())
	if s2 >= s1 || !strings.Contains(strings.Join(cautions, ""), "公费师范生") {
		t.Fatalf("restricted group should rank lower with caution: %d vs %d %v", s2, s1, cautions)
	}
}

func TestAnalyzeResumeRequiresInput(t *testing.T) {
	svc := NewAIService(newTestDB(t), ai.NewClient("", "", "", 0), 5)
	if _, err := svc.AnalyzeResume(context.Background(), 1, ResumeRequest{}); err == nil {
		t.Fatal("expected error for empty resume")
	}
}

func fakeLLM(t *testing.T, content func(system string) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusUnauthorized)
			return
		}
		var req struct {
			Messages []ai.Message `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": content(req.Messages[0].Content)}}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
}

func TestAnalyzeResumeLLMFiltersUnknownIDs(t *testing.T) {
	db := newTestDB(t)
	hunan, _ := seedAnnouncements(t, db, fixedNow())
	srv := fakeLLM(t, func(string) string {
		return fmt.Sprintf("```json\n{\"summary\":\"LLM总结\",\"directions\":[{\"name\":\"税务系统\",\"reason\":\"对口\"}],"+
			"\"recommendations\":[{\"announcement_id\":99999,\"score\":95},{\"announcement_id\":%d,\"score\":88,\"reasons\":[\"专业对口\"]}]}\n```", hunan)
	})
	defer srv.Close()

	svc := NewAIService(db, ai.NewClient(srv.URL, "k", "test-model", 5*time.Second), 5)
	svc.now = fixedNow
	res, err := svc.AnalyzeResume(context.Background(), 1, ResumeRequest{ResumeText: sampleResume})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "test-model" || res.Summary != "LLM总结" {
		t.Fatalf("llm result not applied: %+v", res)
	}
	if len(res.Recommendations) != 1 || res.Recommendations[0].AnnouncementID != hunan || res.Recommendations[0].Score != 88 {
		t.Fatalf("unknown ids must be dropped: %+v", res.Recommendations)
	}
	if st := svc.Status(1); st.UsedToday != 1 || !st.LLMEnabled {
		t.Fatalf("status: %+v", st)
	}
}

func TestLLMFailureFallsBackAndQuota(t *testing.T) {
	db := newTestDB(t)
	seedAnnouncements(t, db, fixedNow())
	svc := NewAIService(db, ai.NewClient("http://127.0.0.1:1", "k", "m", time.Second), 1)
	svc.now = fixedNow
	res, err := svc.AnalyzeResume(context.Background(), 1, ResumeRequest{ResumeText: sampleResume})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "rule" || res.Notice == "" {
		t.Fatalf("expected rule fallback with notice: %+v", res)
	}

	srv := fakeLLM(t, func(string) string { return "好的回答" })
	defer srv.Close()
	svc.llm = ai.NewClient(srv.URL, "k", "m", time.Second)
	ans, err := svc.Ask(context.Background(), 1, "申论怎么准备？")
	if err != nil || ans.Provider != "m" {
		t.Fatalf("first ask should use llm: %+v %v", ans, err)
	}
	ans, err = svc.Ask(context.Background(), 1, "申论怎么准备？")
	if err != nil || ans.Provider != "rule" || !strings.Contains(ans.Notice, "次数") {
		t.Fatalf("second ask should hit quota: %+v %v", ans, err)
	}
}

func TestStudyPlanUsesStats(t *testing.T) {
	db := newTestDB(t)
	svc := NewAIService(db, nil, 5)
	svc.now = fixedNow
	prac := NewPracticeService(db)
	weak, _ := prac.EnsureCategory("行测-数量关系", 20)
	good, _ := prac.EnsureCategory("行测-言语理解", 10)

	sess := model.PracticeSession{UserID: 1, StartedAt: fixedNow()}
	db.Create(&sess)
	for i := 0; i < 12; i++ {
		qw := model.Question{CategoryID: weak.ID, Stem: fmt.Sprintf("w%d", i), AnswerKey: "A", Status: model.StatusPublished}
		qg := model.Question{CategoryID: good.ID, Stem: fmt.Sprintf("g%d", i), AnswerKey: "A", Status: model.StatusPublished}
		db.Create(&qw)
		db.Create(&qg)
		db.Create(&model.UserAnswer{SessionID: sess.ID, QuestionID: qw.ID, IsCorrect: i < 3})
		db.Create(&model.UserAnswer{SessionID: sess.ID, QuestionID: qg.ID, IsCorrect: true})
	}

	plan, err := svc.StudyPlan(context.Background(), 1, PlanRequest{ExamDate: "2026-12-13", DailyHours: 4})
	if err != nil {
		t.Fatal(err)
	}
	if plan.DaysLeft != 61 || len(plan.Phases) != 3 {
		t.Fatalf("days/phases: %d %d", plan.DaysLeft, len(plan.Phases))
	}
	if plan.Phases[0].Start != "2026-10-13" || plan.Phases[len(plan.Phases)-1].End != "2026-12-12" {
		t.Fatalf("phases must span today to the day before the exam: %+v", plan.Phases)
	}
	minutes := map[string]int{}
	total := 0
	for _, b := range plan.DailyTemplate {
		minutes[b.Module] = b.Minutes
		total += b.Minutes
	}
	if minutes["数量关系"] <= minutes["言语理解"] || total < 200 || total > 280 {
		t.Fatalf("daily template: %+v total=%d", plan.DailyTemplate, total)
	}
	var weakDiag, goodDiag CategoryDiagnosis
	for _, d := range plan.Diagnosis {
		switch d.Category {
		case "行测-数量关系":
			weakDiag = d
		case "行测-言语理解":
			goodDiag = d
		}
	}
	if weakDiag.Level != "薄弱" || goodDiag.Level != "较好" {
		t.Fatalf("diagnosis: %+v %+v", weakDiag, goodDiag)
	}
	p, _ := svc.GetProfile(1)
	if p.DailyHours != 4 || p.ExamDate == nil {
		t.Fatalf("profile not updated: %+v", p)
	}
}

func TestStudyPlanRejectsPastDate(t *testing.T) {
	svc := NewAIService(newTestDB(t), nil, 5)
	svc.now = fixedNow
	if _, err := svc.StudyPlan(context.Background(), 1, PlanRequest{ExamDate: "2026-01-01"}); err == nil {
		t.Fatal("expected error for past exam date")
	}
}

func TestReportsScopedToUser(t *testing.T) {
	db := newTestDB(t)
	svc := NewAIService(db, nil, 5)
	ans, err := svc.Ask(context.Background(), 1, "报名流程是什么")
	if err != nil || !strings.Contains(ans.Answer, "报名") {
		t.Fatalf("faq: %+v %v", ans, err)
	}
	if _, err := svc.GetReport(2, ans.ReportID); err == nil {
		t.Fatal("other users must not read the report")
	}
	d, err := svc.GetReport(1, ans.ReportID)
	if err != nil || !json.Valid(d.Result) {
		t.Fatalf("detail: %+v %v", d, err)
	}
	if err := svc.DeleteProfile(1); err != nil {
		t.Fatal(err)
	}
	if res, _ := svc.ListReports(1, "", 1, 20); res.Total != 0 {
		t.Fatalf("reports should be deleted, total=%d", res.Total)
	}
}
