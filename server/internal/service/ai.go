package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"gongkao/internal/ai"
	"gongkao/internal/model"
	"gongkao/internal/pkg"

	"gorm.io/gorm"
)

const (
	providerRule       = "rule"
	maxResumeRunes     = 8000
	maxQuestionRunes   = 500
	maxRecommendations = 8
	llmCandidateLimit  = 15
)

type PositionDirection struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type JobRecommendation struct {
	AnnouncementID uint64     `json:"announcement_id"`
	Title          string     `json:"title"`
	Type           string     `json:"type"`
	Region         string     `json:"region"`
	RegisterEnd    *time.Time `json:"register_end"`
	ExamAt         *time.Time `json:"exam_at"`
	Score          int        `json:"score"`
	Reasons        []string   `json:"reasons"`
	Cautions       []string   `json:"cautions"`
}

type ResumeAnalysis struct {
	ReportID        uint64              `json:"report_id"`
	Provider        string              `json:"provider"`
	Notice          string              `json:"notice,omitempty"`
	Profile         *model.AIProfile    `json:"profile"`
	Summary         string              `json:"summary"`
	Strengths       []string            `json:"strengths"`
	Weaknesses      []string            `json:"weaknesses"`
	Directions      []PositionDirection `json:"directions"`
	Recommendations []JobRecommendation `json:"recommendations"`
	NextSteps       []string            `json:"next_steps"`
}

type CategoryDiagnosis struct {
	Category string  `json:"category"`
	Answered int64   `json:"answered"`
	Correct  int64   `json:"correct"`
	Accuracy float64 `json:"accuracy"`
	Wrong    int64   `json:"wrong"`
	Level    string  `json:"level"`
	Advice   string  `json:"advice"`
}

type PlanPhase struct {
	Name  string   `json:"name"`
	Start string   `json:"start"`
	End   string   `json:"end"`
	Goal  string   `json:"goal"`
	Focus []string `json:"focus"`
}

type DailyBlock struct {
	Module  string `json:"module"`
	Minutes int    `json:"minutes"`
	Task    string `json:"task"`
}

type WeekPlan struct {
	Week  int      `json:"week"`
	Start string   `json:"start"`
	End   string   `json:"end"`
	Focus string   `json:"focus"`
	Tasks []string `json:"tasks"`
}

type StudyPlan struct {
	ReportID      uint64              `json:"report_id"`
	Provider      string              `json:"provider"`
	Notice        string              `json:"notice,omitempty"`
	ExamDate      string              `json:"exam_date"`
	ExamDateNote  string              `json:"exam_date_note,omitempty"`
	DaysLeft      int                 `json:"days_left"`
	DailyHours    float64             `json:"daily_hours"`
	Summary       string              `json:"summary"`
	Diagnosis     []CategoryDiagnosis `json:"diagnosis"`
	Phases        []PlanPhase         `json:"phases"`
	DailyTemplate []DailyBlock        `json:"daily_template"`
	Weekly        []WeekPlan          `json:"weekly"`
	Tips          []string            `json:"tips"`
}

type AskAnswer struct {
	ReportID uint64 `json:"report_id"`
	Provider string `json:"provider"`
	Notice   string `json:"notice,omitempty"`
	Answer   string `json:"answer"`
}

type AIStatus struct {
	LLMEnabled bool   `json:"llm_enabled"`
	Model      string `json:"model,omitempty"`
	DailyLimit int    `json:"daily_limit"`
	UsedToday  int64  `json:"used_today"`
}

type AIService struct {
	db         *gorm.DB
	llm        *ai.Client
	dailyLimit int
	now        func() time.Time
}

func NewAIService(db *gorm.DB, llm *ai.Client, dailyLimit int) *AIService {
	if dailyLimit <= 0 {
		dailyLimit = 30
	}
	return &AIService{db: db, llm: llm, dailyLimit: dailyLimit, now: time.Now}
}

func (s *AIService) Status(userID uint64) AIStatus {
	return AIStatus{
		LLMEnabled: s.llm.Enabled(),
		Model:      s.llm.Model(),
		DailyLimit: s.dailyLimit,
		UsedToday:  s.usedToday(userID),
	}
}

func (s *AIService) usedToday(userID uint64) int64 {
	now := s.now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var n int64
	s.db.Model(&model.AIReport{}).
		Where("user_id = ? AND provider <> ? AND created_at >= ?", userID, providerRule, start).
		Count(&n)
	return n
}

// llmReady reports whether an LLM call is allowed, with a user-facing notice when not.
func (s *AIService) llmReady(userID uint64) (bool, string) {
	if !s.llm.Enabled() {
		return false, ""
	}
	if s.usedToday(userID) >= int64(s.dailyLimit) {
		return false, "今日 AI 次数已用完，已切换为规则引擎分析"
	}
	return true, ""
}

// ---------- profile ----------

func (s *AIService) GetProfile(userID uint64) (*model.AIProfile, error) {
	var p model.AIProfile
	res := s.db.Where("user_id = ?", userID).Limit(1).Find(&p)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return &model.AIProfile{UserID: userID, DailyHours: 3}, nil
	}
	return &p, nil
}

type ProfileInput struct {
	Education       *string  `json:"education"`
	Major           *string  `json:"major"`
	GraduationYear  *int     `json:"graduation_year"`
	FreshGraduate   *bool    `json:"fresh_graduate"`
	PoliticalStatus *string  `json:"political_status"`
	WorkYears       *int     `json:"work_years"`
	GrassrootsYears *int     `json:"grassroots_years"`
	Certificates    *string  `json:"certificates"`
	TargetRegions   *string  `json:"target_regions"`
	TargetTypes     *string  `json:"target_types"`
	ExamDate        *string  `json:"exam_date"` // YYYY-MM-DD, empty to clear
	DailyHours      *float64 `json:"daily_hours"`
	ResumeText      *string  `json:"resume_text"`
}

func (s *AIService) SaveProfile(userID uint64, in ProfileInput) (*model.AIProfile, error) {
	p, err := s.GetProfile(userID)
	if err != nil {
		return nil, err
	}
	if err := applyProfileInput(p, in); err != nil {
		return nil, err
	}
	return p, s.saveProfile(p)
}

func applyProfileInput(p *model.AIProfile, in ProfileInput) error {
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&p.Education, in.Education)
	set(&p.Major, in.Major)
	set(&p.PoliticalStatus, in.PoliticalStatus)
	set(&p.Certificates, in.Certificates)
	set(&p.TargetRegions, in.TargetRegions)
	set(&p.TargetTypes, in.TargetTypes)
	if in.GraduationYear != nil {
		p.GraduationYear = *in.GraduationYear
	}
	if in.FreshGraduate != nil {
		p.FreshGraduate = *in.FreshGraduate
	}
	if in.WorkYears != nil {
		p.WorkYears = *in.WorkYears
	}
	if in.GrassrootsYears != nil {
		p.GrassrootsYears = *in.GrassrootsYears
	}
	if in.DailyHours != nil {
		if *in.DailyHours < 0 || *in.DailyHours > 16 {
			return errors.New("daily_hours must be between 0 and 16")
		}
		p.DailyHours = *in.DailyHours
	}
	if in.ExamDate != nil {
		v := strings.TrimSpace(*in.ExamDate)
		if v == "" {
			p.ExamDate = nil
		} else {
			t, err := time.ParseInLocation("2006-01-02", v, time.Local)
			if err != nil {
				return errors.New("exam_date must be YYYY-MM-DD")
			}
			p.ExamDate = &t
		}
	}
	if in.ResumeText != nil {
		p.ResumeText = truncateRunes(strings.TrimSpace(*in.ResumeText), maxResumeRunes)
	}
	return nil
}

func (s *AIService) saveProfile(p *model.AIProfile) error {
	if p.ID == 0 {
		return s.db.Create(p).Error
	}
	return s.db.Save(p).Error
}

// DeleteProfile removes the stored profile, resume text and AI reports.
func (s *AIService) DeleteProfile(userID uint64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.AIProfile{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&model.AIReport{}).Error
	})
}

// ---------- resume → positions ----------

type ResumeRequest struct {
	ResumeText string        `json:"resume_text"`
	Profile    *ProfileInput `json:"profile"`
}

type llmResumeOutput struct {
	Profile struct {
		Education       string   `json:"education"`
		Major           string   `json:"major"`
		GraduationYear  int      `json:"graduation_year"`
		FreshGraduate   bool     `json:"fresh_graduate"`
		PoliticalStatus string   `json:"political_status"`
		WorkYears       int      `json:"work_years"`
		GrassrootsYears int      `json:"grassroots_years"`
		Certificates    []string `json:"certificates"`
		TargetRegions   []string `json:"target_regions"`
	} `json:"profile"`
	Summary         string              `json:"summary"`
	Strengths       []string            `json:"strengths"`
	Weaknesses      []string            `json:"weaknesses"`
	Directions      []PositionDirection `json:"directions"`
	Recommendations []struct {
		AnnouncementID uint64   `json:"announcement_id"`
		Score          int      `json:"score"`
		Reasons        []string `json:"reasons"`
		Cautions       []string `json:"cautions"`
	} `json:"recommendations"`
	NextSteps []string `json:"next_steps"`
}

const resumeSystemPrompt = `你是资深的中国公务员/事业单位考试报考顾问。根据考生简历和候选考试公告，分析考生的报考优势并推荐合适的考试与岗位方向。
要求：
1. 只能从给定候选公告中推荐，announcement_id 必须来自候选列表，最多推荐 8 个，按匹配度从高到低排序，score 为 0-100。
2. 公告未提供具体职位表，推荐理由要基于地区、考试类型、报名时间、身份（应届/党员/基层经历）、学历和专业关键词，并在 cautions 中提示需核对职位表。
3. directions 给出 3-5 个适合的岗位方向（如“税务系统综合管理岗”），结合专业说明理由。
4. 不要编造考生没有的经历；无法确定的字段留空或为 0。
5. 只输出 JSON，结构：
{"profile":{"education":"大专|本科|硕士|博士","major":"","graduation_year":0,"fresh_graduate":false,"political_status":"","work_years":0,"grassroots_years":0,"certificates":[],"target_regions":[]},
"summary":"一段话总结","strengths":[],"weaknesses":[],"directions":[{"name":"","reason":""}],
"recommendations":[{"announcement_id":0,"score":0,"reasons":[],"cautions":[]}],"next_steps":[]}`

func (s *AIService) AnalyzeResume(ctx context.Context, userID uint64, req ResumeRequest) (*ResumeAnalysis, error) {
	now := s.now()
	p, err := s.GetProfile(userID)
	if err != nil {
		return nil, err
	}
	if req.Profile != nil {
		if err := applyProfileInput(p, *req.Profile); err != nil {
			return nil, err
		}
	}
	text := strings.TrimSpace(req.ResumeText)
	if text != "" {
		p.ResumeText = truncateRunes(text, maxResumeRunes)
	}
	text = p.ResumeText
	if text == "" && p.Major == "" && p.Education == "" {
		return nil, errors.New("请粘贴简历内容或先填写学历、专业等基本信息")
	}
	MergeProfile(p, ExtractProfileFromText(text, now))

	candidates, err := s.rankAnnouncements(p, now)
	if err != nil {
		return nil, err
	}
	ruleRecs := topRecommendations(candidates, maxRecommendations)
	result := ruleResumeAnalysis(p, ruleRecs)
	result.Provider = providerRule

	ok, notice := s.llmReady(userID)
	result.Notice = notice
	if ok {
		llmOut, err := s.llmResume(ctx, p, text, candidates)
		if err != nil {
			slog.Warn("ai resume llm failed", "user", userID, "err", err)
			result.Notice = "AI 服务暂不可用，已使用规则引擎分析"
		} else {
			applyLLMResume(&result, p, llmOut, candidates)
			result.Provider = s.llm.Model()
		}
	}

	if err := s.saveProfile(p); err != nil {
		return nil, err
	}
	view := *p
	view.ResumeText = ""
	result.Profile = &view

	title := "简历分析 · " + nz(p.Major, nz(p.Education, "未命名"))
	input := map[string]any{"resume_chars": len([]rune(text))}
	id, err := s.saveReport(userID, model.AIReportResume, title, result.Provider, input, &result)
	if err != nil {
		return nil, err
	}
	result.ReportID = id
	return &result, nil
}

type scoredAnnouncement struct {
	ann      model.Announcement
	raw      int
	reasons  []string
	cautions []string
}

func (s *AIService) rankAnnouncements(p *model.AIProfile, now time.Time) ([]scoredAnnouncement, error) {
	q := s.db.Model(&model.Announcement{}).
		Where("status = ?", model.StatusPublished).
		Where("register_end IS NULL OR register_end >= ?", now).
		Where("published_at >= ? OR published_at < ?", now.AddDate(0, 0, -180), time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC))
	if types := splitList(p.TargetTypes); len(types) > 0 {
		q = q.Where("type IN ?", types)
	}
	var list []model.Announcement
	if err := q.Order("published_at DESC").Limit(400).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]scoredAnnouncement, 0, len(list))
	for _, a := range list {
		if !IsRecruitmentAnnouncement(&a) {
			continue
		}
		raw, reasons, cautions := ScoreAnnouncement(p, &a, now)
		out = append(out, scoredAnnouncement{ann: a, raw: raw, reasons: reasons, cautions: cautions})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].raw > out[j].raw })
	return out, nil
}

func topRecommendations(c []scoredAnnouncement, n int) []JobRecommendation {
	out := []JobRecommendation{}
	for _, it := range c {
		if len(out) >= n {
			break
		}
		if it.raw <= 0 && len(out) >= 3 {
			break
		}
		out = append(out, toRecommendation(it.ann, normalizeScore(it.raw), it.reasons, it.cautions))
	}
	return out
}

func toRecommendation(a model.Announcement, score int, reasons, cautions []string) JobRecommendation {
	return JobRecommendation{
		AnnouncementID: a.ID, Title: a.Title, Type: a.Type, Region: a.Region,
		RegisterEnd: a.RegisterEnd, ExamAt: a.ExamAt,
		Score: score, Reasons: reasons, Cautions: cautions,
	}
}

func (s *AIService) llmResume(ctx context.Context, p *model.AIProfile, text string, candidates []scoredAnnouncement) (*llmResumeOutput, error) {
	type cand struct {
		ID            uint64 `json:"announcement_id"`
		Title         string `json:"title"`
		Type          string `json:"type"`
		Region        string `json:"region"`
		RegisterStart string `json:"register_start,omitempty"`
		RegisterEnd   string `json:"register_end,omitempty"`
		ExamAt        string `json:"exam_at,omitempty"`
		Summary       string `json:"summary"`
		RuleScore     int    `json:"rule_score"`
	}
	list := []cand{}
	for i, c := range candidates {
		if i >= llmCandidateLimit {
			break
		}
		list = append(list, cand{
			ID: c.ann.ID, Title: c.ann.Title, Type: c.ann.Type, Region: c.ann.Region,
			RegisterStart: fmtDate(c.ann.RegisterStart), RegisterEnd: fmtDate(c.ann.RegisterEnd),
			ExamAt: fmtDate(c.ann.ExamAt), Summary: truncateRunes(c.ann.Summary, 120),
			RuleScore: normalizeScore(c.raw),
		})
	}
	prof := *p
	prof.ResumeText = ""
	profJSON, _ := json.Marshal(prof)
	candJSON, _ := json.Marshal(list)
	user := fmt.Sprintf("今天：%s\n\n考生已知资料：%s\n\n简历原文：\n%s\n\n候选公告（type: civil=公务员, institution=事业单位）：%s",
		s.now().Format("2006-01-02"), profJSON, truncateRunes(text, 6000), candJSON)

	var out llmResumeOutput
	if err := s.llm.ChatJSON(ctx, resumeSystemPrompt, user, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func applyLLMResume(r *ResumeAnalysis, p *model.AIProfile, out *llmResumeOutput, candidates []scoredAnnouncement) {
	MergeProfile(p, model.AIProfile{
		Education:       normalizeEducation(out.Profile.Education),
		Major:           out.Profile.Major,
		GraduationYear:  out.Profile.GraduationYear,
		FreshGraduate:   out.Profile.FreshGraduate,
		PoliticalStatus: out.Profile.PoliticalStatus,
		WorkYears:       out.Profile.WorkYears,
		GrassrootsYears: out.Profile.GrassrootsYears,
		Certificates:    strings.Join(out.Profile.Certificates, ","),
		TargetRegions:   strings.Join(out.Profile.TargetRegions, ","),
	})
	if out.Summary != "" {
		r.Summary = out.Summary
	}
	if len(out.Strengths) > 0 {
		r.Strengths = out.Strengths
	}
	if len(out.Weaknesses) > 0 {
		r.Weaknesses = out.Weaknesses
	}
	if len(out.Directions) > 0 {
		r.Directions = out.Directions
	}
	if len(out.NextSteps) > 0 {
		r.NextSteps = out.NextSteps
	}

	byID := map[uint64]scoredAnnouncement{}
	for _, c := range candidates {
		byID[c.ann.ID] = c
	}
	recs := []JobRecommendation{}
	seen := map[uint64]bool{}
	for _, rec := range out.Recommendations {
		c, ok := byID[rec.AnnouncementID]
		if !ok || seen[rec.AnnouncementID] {
			continue
		}
		seen[rec.AnnouncementID] = true
		score := rec.Score
		if score <= 0 || score > 100 {
			score = normalizeScore(c.raw)
		}
		reasons := rec.Reasons
		if len(reasons) == 0 {
			reasons = c.reasons
		}
		cautions := rec.Cautions
		if len(cautions) == 0 {
			cautions = c.cautions
		}
		recs = append(recs, toRecommendation(c.ann, score, reasons, cautions))
		if len(recs) >= maxRecommendations {
			break
		}
	}
	if len(recs) > 0 {
		r.Recommendations = recs
	}
}

func normalizeEducation(e string) string {
	switch {
	case strings.Contains(e, "博士"):
		return "博士"
	case strings.Contains(e, "硕士"), strings.Contains(e, "研究生"):
		return "硕士"
	case strings.Contains(e, "本科"), strings.Contains(e, "学士"):
		return "本科"
	case strings.Contains(e, "专科"), strings.Contains(e, "大专"):
		return "大专"
	}
	return ""
}

// ---------- study plan ----------

type PlanRequest struct {
	ExamDate   string  `json:"exam_date"`
	DailyHours float64 `json:"daily_hours"`
	Focus      string  `json:"focus"` // optional free-text wishes
}

const planSystemPrompt = `你是公考备考规划师。根据考生练习数据和一份规则生成的草案，优化为更具体、可执行的复习计划。
要求：
1. 保持 exam_date、days_left、daily_hours 不变；daily_template 各项分钟数之和约等于 daily_hours*60。
2. 薄弱模块（level=薄弱 或 数据不足）分配更多时间，任务要具体到题量和方法。
3. phases 的日期必须在今天到考试日之间且连续；weekly 最多 12 周。
4. 结合考生的 focus 诉求（如有）。
5. 只输出 JSON，结构与草案一致：
{"summary":"","phases":[{"name":"","start":"YYYY-MM-DD","end":"YYYY-MM-DD","goal":"","focus":[]}],
"daily_template":[{"module":"","minutes":0,"task":""}],"weekly":[{"week":1,"start":"","end":"","focus":"","tasks":[]}],"tips":[]}`

func (s *AIService) StudyPlan(ctx context.Context, userID uint64, req PlanRequest) (*StudyPlan, error) {
	now := s.now()
	p, err := s.GetProfile(userID)
	if err != nil {
		return nil, err
	}
	hours := req.DailyHours
	if hours < 0 || hours > 16 {
		return nil, errors.New("daily_hours must be between 0 and 16")
	}
	if hours == 0 {
		hours = p.DailyHours
	}
	if hours == 0 {
		hours = 3
	}

	exam, note, err := s.resolveExamDate(userID, p, req.ExamDate, now)
	if err != nil {
		return nil, err
	}
	stats, err := s.CategoryStats(userID)
	if err != nil {
		return nil, err
	}

	plan := BuildRulePlan(p, stats, exam, hours, now)
	plan.ExamDateNote = note
	plan.Provider = providerRule

	ok, notice := s.llmReady(userID)
	plan.Notice = notice
	if ok {
		if err := s.llmPlan(ctx, p, &plan, req.Focus); err != nil {
			slog.Warn("ai plan llm failed", "user", userID, "err", err)
			plan.Notice = "AI 服务暂不可用，已使用规则引擎生成计划"
		} else {
			plan.Provider = s.llm.Model()
		}
	}

	p.DailyHours = hours
	if req.ExamDate != "" {
		p.ExamDate = &exam
	}
	if err := s.saveProfile(p); err != nil {
		return nil, err
	}

	title := fmt.Sprintf("复习计划 · %s 考试（剩 %d 天）", plan.ExamDate, plan.DaysLeft)
	id, err := s.saveReport(userID, model.AIReportPlan, title, plan.Provider, req, &plan)
	if err != nil {
		return nil, err
	}
	plan.ReportID = id
	return &plan, nil
}

func (s *AIService) resolveExamDate(userID uint64, p *model.AIProfile, raw string, now time.Time) (time.Time, string, error) {
	if raw = strings.TrimSpace(raw); raw != "" {
		t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			return time.Time{}, "", errors.New("exam_date must be YYYY-MM-DD")
		}
		if t.Before(now.AddDate(0, 0, -1)) {
			return time.Time{}, "", errors.New("考试日期不能早于今天")
		}
		return t, "", nil
	}
	if p.ExamDate != nil && p.ExamDate.After(now) {
		return *p.ExamDate, "", nil
	}
	var ann model.Announcement
	err := s.db.Model(&model.Announcement{}).
		Joins("JOIN user_exams ON user_exams.announcement_id = announcements.id").
		Where("user_exams.user_id = ? AND announcements.exam_at > ?", userID, now).
		Order("announcements.exam_at ASC").First(&ann).Error
	if err == nil && ann.ExamAt != nil {
		return *ann.ExamAt, "取自「我的考试」：" + truncateRunes(ann.Title, 30), nil
	}
	return now.AddDate(0, 0, 90), "未设置考试日期，默认按 90 天规划，可在下方修改", nil
}

// CategoryStats aggregates the user's answers and open wrong questions per category.
func (s *AIService) CategoryStats(userID uint64) ([]CategoryDiagnosis, error) {
	var cats []model.QuestionCategory
	if err := s.db.Where("status = ?", model.StatusPublished).Order("sort ASC, id ASC").Find(&cats).Error; err != nil {
		return nil, err
	}
	type row struct {
		Name     string
		Answered int64
		Correct  int64
	}
	var answers []row
	if err := s.db.Table("user_answers").
		Select("question_categories.name AS name, COUNT(user_answers.id) AS answered, SUM(CASE WHEN user_answers.is_correct THEN 1 ELSE 0 END) AS correct").
		Joins("JOIN practice_sessions ON practice_sessions.id = user_answers.session_id").
		Joins("JOIN questions ON questions.id = user_answers.question_id").
		Joins("JOIN question_categories ON question_categories.id = questions.category_id").
		Where("practice_sessions.user_id = ?", userID).
		Group("question_categories.name").Scan(&answers).Error; err != nil {
		return nil, err
	}
	type wrow struct {
		Name  string
		Wrong int64
	}
	var wrongs []wrow
	if err := s.db.Table("wrong_questions").
		Select("question_categories.name AS name, COUNT(wrong_questions.id) AS wrong").
		Joins("JOIN questions ON questions.id = wrong_questions.question_id").
		Joins("JOIN question_categories ON question_categories.id = questions.category_id").
		Where("wrong_questions.user_id = ? AND wrong_questions.mastered_at IS NULL", userID).
		Group("question_categories.name").Scan(&wrongs).Error; err != nil {
		return nil, err
	}
	ans := map[string]row{}
	for _, a := range answers {
		ans[a.Name] = a
	}
	wr := map[string]int64{}
	for _, w := range wrongs {
		wr[w.Name] = w.Wrong
	}
	out := make([]CategoryDiagnosis, 0, len(cats))
	for _, c := range cats {
		a := ans[c.Name]
		acc := 0.0
		if a.Answered > 0 {
			acc = float64(a.Correct) / float64(a.Answered)
		}
		level := diagLevel(a.Answered, acc)
		out = append(out, CategoryDiagnosis{
			Category: c.Name, Answered: a.Answered, Correct: a.Correct,
			Accuracy: float64(int(acc*1000)) / 1000, Wrong: wr[c.Name],
			Level: level, Advice: diagAdvice(level, c.Name),
		})
	}
	return out, nil
}

func (s *AIService) llmPlan(ctx context.Context, p *model.AIProfile, plan *StudyPlan, focus string) error {
	draft, _ := json.Marshal(plan)
	prof := map[string]any{
		"education": p.Education, "major": p.Major, "target_types": p.TargetTypes,
		"target_regions": p.TargetRegions, "fresh_graduate": p.FreshGraduate,
	}
	profJSON, _ := json.Marshal(prof)
	user := fmt.Sprintf("今天：%s\n考生资料：%s\n考生诉求：%s\n\n规则草案：%s",
		s.now().Format("2006-01-02"), profJSON, nz(truncateRunes(focus, 300), "无"), draft)

	var out struct {
		Summary       string       `json:"summary"`
		Phases        []PlanPhase  `json:"phases"`
		DailyTemplate []DailyBlock `json:"daily_template"`
		Weekly        []WeekPlan   `json:"weekly"`
		Tips          []string     `json:"tips"`
	}
	if err := s.llm.ChatJSON(ctx, planSystemPrompt, user, &out); err != nil {
		return err
	}
	if len(out.DailyTemplate) == 0 || len(out.Phases) == 0 {
		return errors.New("ai: incomplete plan")
	}
	if out.Summary != "" {
		plan.Summary = out.Summary
	}
	plan.Phases = out.Phases
	plan.DailyTemplate = out.DailyTemplate
	if len(out.Weekly) > 0 {
		if len(out.Weekly) > 12 {
			out.Weekly = out.Weekly[:12]
		}
		plan.Weekly = out.Weekly
	}
	if len(out.Tips) > 0 {
		plan.Tips = out.Tips
	}
	return nil
}

// ---------- free Q&A ----------

const askSystemPrompt = `你是公考备考助手，回答中国公务员、事业单位、选调生考试相关的报考、备考、面试问题。
回答要求：简洁、具体、可执行，不超过 400 字；涉及具体政策或报考条件时提醒以官方公告为准；与公考无关的问题礼貌拒答。`

func (s *AIService) Ask(ctx context.Context, userID uint64, question string) (*AskAnswer, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, errors.New("question required")
	}
	question = truncateRunes(question, maxQuestionRunes)
	res := AskAnswer{Provider: providerRule, Answer: ruleAnswer(question)}

	ok, notice := s.llmReady(userID)
	res.Notice = notice
	if ok {
		p, _ := s.GetProfile(userID)
		ctxText := ""
		if p != nil && (p.Education != "" || p.Major != "") {
			ctxText = fmt.Sprintf("（考生背景：%s，%s，意向地区：%s）\n", nz(p.Education, "学历未知"), nz(p.Major, "专业未知"), nz(p.TargetRegions, "未设置"))
		}
		answer, err := s.llm.Chat(ctx, askSystemPrompt, ctxText+question)
		if err != nil {
			slog.Warn("ai ask llm failed", "user", userID, "err", err)
			res.Notice = "AI 服务暂不可用，以下为常见问题解答"
		} else {
			res.Answer = answer
			res.Provider = s.llm.Model()
		}
	}

	id, err := s.saveReport(userID, model.AIReportAsk, truncateRunes(question, 60), res.Provider, map[string]string{"question": question}, &res)
	if err != nil {
		return nil, err
	}
	res.ReportID = id
	return &res, nil
}

// ---------- reports ----------

func (s *AIService) saveReport(userID uint64, kind, title, provider string, input, output any) (uint64, error) {
	in, _ := json.Marshal(input)
	out, _ := json.Marshal(output)
	r := model.AIReport{
		UserID: userID, Kind: kind, Title: title, Provider: provider,
		Input: string(in), Output: string(out),
	}
	if err := s.db.Create(&r).Error; err != nil {
		return 0, err
	}
	return r.ID, nil
}

func (s *AIService) ListReports(userID uint64, kind string, page, pageSize int) (*pkg.PageResult, error) {
	q := s.db.Model(&model.AIReport{}).Where("user_id = ?", userID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AIReport
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &pkg.PageResult{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

type ReportDetail struct {
	model.AIReport
	Result json.RawMessage `json:"result"`
}

func (s *AIService) GetReport(userID, id uint64) (*ReportDetail, error) {
	var r model.AIReport
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&r).Error; err != nil {
		return nil, err
	}
	res := json.RawMessage(r.Output)
	if !json.Valid(res) {
		res = json.RawMessage("null")
	}
	return &ReportDetail{AIReport: r, Result: res}, nil
}

func fmtDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
