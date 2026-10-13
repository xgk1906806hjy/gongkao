package service

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gongkao/internal/model"
)

var provinces = []string{
	"北京", "天津", "河北", "山西", "内蒙古", "辽宁", "吉林", "黑龙江", "上海", "江苏", "浙江",
	"安徽", "福建", "江西", "山东", "河南", "湖北", "湖南", "广东", "广西", "海南", "重庆",
	"四川", "贵州", "云南", "西藏", "陕西", "甘肃", "青海", "宁夏", "新疆",
}

var certificateKeywords = []struct {
	Name     string
	Patterns []string
}{
	{"法律职业资格", []string{"法律职业资格", "司法考试", "法考"}},
	{"注册会计师", []string{"注册会计师", "CPA"}},
	{"中级会计师", []string{"中级会计"}},
	{"初级会计师", []string{"初级会计"}},
	{"教师资格证", []string{"教师资格"}},
	{"计算机二级", []string{"计算机二级", "计算机等级考试二级"}},
	{"软考", []string{"软考", "软件设计师", "系统架构设计师"}},
	{"英语六级", []string{"英语六级", "CET-6", "CET6"}},
	{"英语四级", []string{"英语四级", "CET-4", "CET4"}},
	{"英语专八", []string{"专业八级", "专八", "TEM-8"}},
	{"普通话证书", []string{"普通话"}},
	{"驾驶证", []string{"驾驶证", "驾照"}},
	{"人力资源管理师", []string{"人力资源管理师"}},
}

var (
	reMajorLabel = regexp.MustCompile(`专业[：:]\s*([\p{Han}A-Za-z（）()与及、]{2,20})`)
	reMajorTail  = regexp.MustCompile(`([\p{Han}]{2,12})专业`)
	reGradYear   = regexp.MustCompile(`(20\d{2})\s*(届|年\s*\d{0,2}\s*月?\s*毕业)`)
	reWorkYears  = regexp.MustCompile(`(?:工作(?:经验|经历)?\s*(\d{1,2})\s*年|(\d{1,2})\s*年(?:以上)?(?:的)?工作经验)`)
	reGrassroots = regexp.MustCompile(`(三支一扶|西部计划|大学生村官|村官|基层)[^。\n]{0,20}?(\d)\s*年`)
)

// ExtractProfileFromText heuristically parses a resume into profile fields.
// Zero values mean "not found".
func ExtractProfileFromText(text string, now time.Time) model.AIProfile {
	var p model.AIProfile
	t := strings.ReplaceAll(text, "\r", "")
	switch {
	case strings.Contains(t, "博士"):
		p.Education = "博士"
	case strings.Contains(t, "硕士") || strings.Contains(t, "研究生"):
		p.Education = "硕士"
	case strings.Contains(t, "本科") || strings.Contains(t, "学士"):
		p.Education = "本科"
	case strings.Contains(t, "大专") || strings.Contains(t, "专科"):
		p.Education = "大专"
	}

	if m := reMajorLabel.FindStringSubmatch(t); m != nil {
		p.Major = strings.TrimSpace(m[1])
	} else if m := reMajorTail.FindStringSubmatch(t); m != nil {
		p.Major = strings.TrimSpace(strings.TrimPrefix(m[1], "所学"))
	}

	if m := reGradYear.FindStringSubmatch(t); m != nil {
		if y, err := strconv.Atoi(m[1]); err == nil {
			p.GraduationYear = y
		}
	}
	if strings.Contains(t, "应届") || (p.GraduationYear > 0 && p.GraduationYear >= now.Year()) {
		p.FreshGraduate = true
	}

	switch {
	case strings.Contains(t, "预备党员"):
		p.PoliticalStatus = "中共预备党员"
	case strings.Contains(t, "中共党员") || strings.Contains(t, "党员"):
		p.PoliticalStatus = "中共党员"
	case strings.Contains(t, "共青团员") || strings.Contains(t, "团员"):
		p.PoliticalStatus = "共青团员"
	}

	if m := reWorkYears.FindStringSubmatch(t); m != nil {
		for _, g := range m[1:] {
			if n, err := strconv.Atoi(g); err == nil && n > 0 {
				p.WorkYears = n
				break
			}
		}
	}
	if m := reGrassroots.FindStringSubmatch(t); m != nil {
		if n, err := strconv.Atoi(m[2]); err == nil {
			p.GrassrootsYears = n
		}
	} else if strings.Contains(t, "三支一扶") || strings.Contains(t, "西部计划") || strings.Contains(t, "村官") {
		p.GrassrootsYears = 1
	}

	certs := []string{}
	upper := strings.ToUpper(t)
	for _, c := range certificateKeywords {
		for _, pat := range c.Patterns {
			if strings.Contains(upper, strings.ToUpper(pat)) {
				certs = append(certs, c.Name)
				break
			}
		}
	}
	p.Certificates = strings.Join(certs, ",")

	regions := []string{}
	for _, line := range strings.Split(t, "\n") {
		if !containsAny(line, "意向", "期望", "求职地", "工作地", "生源地", "籍贯", "户籍") {
			continue
		}
		for _, pv := range provinces {
			if strings.Contains(line, pv) && !containsStr(regions, pv) {
				regions = append(regions, pv)
			}
		}
	}
	p.TargetRegions = strings.Join(regions, ",")
	return p
}

// MergeProfile copies non-zero fields of src into dst.
func MergeProfile(dst *model.AIProfile, src model.AIProfile) {
	if src.Education != "" {
		dst.Education = src.Education
	}
	if src.Major != "" {
		dst.Major = src.Major
	}
	if src.GraduationYear > 0 {
		dst.GraduationYear = src.GraduationYear
	}
	if src.FreshGraduate {
		dst.FreshGraduate = true
	}
	if src.PoliticalStatus != "" {
		dst.PoliticalStatus = src.PoliticalStatus
	}
	if src.WorkYears > 0 {
		dst.WorkYears = src.WorkYears
	}
	if src.GrassrootsYears > 0 {
		dst.GrassrootsYears = src.GrassrootsYears
	}
	if src.Certificates != "" {
		dst.Certificates = joinUnique(dst.Certificates, src.Certificates)
	}
	if src.TargetRegions != "" {
		dst.TargetRegions = joinUnique(dst.TargetRegions, src.TargetRegions)
	}
	if src.TargetTypes != "" {
		dst.TargetTypes = src.TargetTypes
	}
	if src.ExamDate != nil {
		dst.ExamDate = src.ExamDate
	}
	if src.DailyHours > 0 {
		dst.DailyHours = src.DailyHours
	}
}

var majorDirectionTable = []struct {
	Keywords   []string
	Directions []PositionDirection
}{
	{[]string{"法学", "法律", "知识产权"}, []PositionDirection{
		{Name: "法院/检察院（书记员、司法辅助）", Reason: "法学类专业是法检系统核心专业要求"},
		{Name: "司法行政、公安法制、市场监管执法", Reason: "执法类岗位常限法学类"},
	}},
	{[]string{"会计", "财务", "审计", "金融", "经济", "财政", "税"}, []PositionDirection{
		{Name: "税务系统", Reason: "国考税务岗招录量大，经济/财会类专业对口"},
		{Name: "财政、审计、统计部门", Reason: "财会审计岗位通常限经济学/会计学类"},
	}},
	{[]string{"计算机", "软件", "信息", "网络", "数据", "电子", "通信", "人工智能"}, []PositionDirection{
		{Name: "公安网安/信息通信岗", Reason: "技术类岗位竞争相对较小"},
		{Name: "数据局、海关技术、各单位信息化岗", Reason: "计算机类专业需求稳定"},
	}},
	{[]string{"汉语言", "中文", "新闻", "文秘", "传播", "历史", "哲学"}, []PositionDirection{
		{Name: "综合管理/文字材料岗", Reason: "文字功底是机关办公室岗位核心能力"},
		{Name: "宣传部门、党政办公室", Reason: "新闻传播、中文类专业对口"},
	}},
	{[]string{"师范", "教育", "学前"}, []PositionDirection{
		{Name: "教育局、事业单位教师岗", Reason: "师范/教育类专业对口"},
	}},
	{[]string{"医学", "临床", "护理", "药学", "公共卫生", "预防医学"}, []PositionDirection{
		{Name: "卫健委、疾控中心、医院事业编", Reason: "医学类专业事业单位需求大"},
	}},
	{[]string{"土木", "建筑", "规划", "工程", "测绘", "交通"}, []PositionDirection{
		{Name: "住建、自然资源、交通运输部门", Reason: "工程类专业岗位对口"},
	}},
	{[]string{"农学", "林学", "畜牧", "兽医", "水产", "园艺", "植物"}, []PositionDirection{
		{Name: "农业农村局、林业局、乡镇农技站", Reason: "涉农专业基层岗位多、竞争较小"},
	}},
	{[]string{"环境", "化学", "生态", "化工"}, []PositionDirection{
		{Name: "生态环境局、环境监测站", Reason: "环境类专业对口"},
	}},
	{[]string{"英语", "外语", "日语", "俄语", "翻译"}, []PositionDirection{
		{Name: "海关、外事办、出入境管理", Reason: "外语类专业有专门岗位"},
	}},
	{[]string{"管理", "行政", "人力", "社会", "政治", "马克思"}, []PositionDirection{
		{Name: "综合管理类、组织/人社部门", Reason: "管理类、政治类专业适配面广"},
	}},
}

// MajorDirections maps a major to suggested position directions.
func MajorDirections(major string) []PositionDirection {
	out := []PositionDirection{}
	for _, row := range majorDirectionTable {
		if containsAny(major, row.Keywords...) {
			out = append(out, row.Directions...)
		}
	}
	out = append(out, PositionDirection{Name: "不限专业岗位（乡镇机关、综合管理类）", Reason: "专业不限岗位可作为保底选择，但竞争比通常较高"})
	return out
}

func majorTokens(major string) []string {
	major = strings.TrimSpace(major)
	if major == "" {
		return nil
	}
	parts := strings.FieldsFunc(major, func(r rune) bool {
		return r == '、' || r == ',' || r == '，' || r == '/' || r == ' '
	})
	out := []string{}
	add := func(tok string) {
		if len([]rune(tok)) >= 2 && !containsStr(genericMajorWords, tok) && !containsStr(out, tok) {
			out = append(out, tok)
		}
	}
	for _, p := range parts {
		add(strings.TrimSuffix(strings.TrimSpace(p), "专业"))
	}
	for _, row := range majorDirectionTable {
		for _, kw := range row.Keywords {
			if strings.Contains(major, kw) {
				add(kw)
			}
		}
	}
	return out
}

// restrictedGroups mark announcements open only to specific candidate groups.
var restrictedGroups = []string{"公费师范生", "定向生", "退役", "残疾人", "随军家属", "博士后", "协理员", "服务期满", "高层次人才"}

// genericMajorWords are too broad to signal a real major match in announcement text.
var genericMajorWords = []string{"技术", "工程", "管理", "科学", "信息", "数据", "社会", "电子", "交通", "政治", "行政"}

func educationRank(e string) int {
	switch e {
	case "博士":
		return 4
	case "硕士":
		return 3
	case "本科":
		return 2
	case "大专":
		return 1
	}
	return 0
}

// IsRecruitmentAnnouncement keeps notices a candidate can still apply to,
// dropping score lines, name lists, interview notices and qualification-exam reminders.
func IsRecruitmentAnnouncement(a *model.Announcement) bool {
	if a.Source == "seed" {
		return false
	}
	if !containsAny(a.Title, "招聘", "招录", "录用", "招考", "选调", "遴选", "人才引进", "引进高层次人才") {
		return false
	}
	return !containsAny(a.Title, "名单", "公示", "成绩", "合格标准", "确认人数", "调整情况", "面试公告", "体检", "考察", "声明",
		"提示", "减免", "笔试", "拟聘用", "拟录用")
}

// ScoreAnnouncement rates how well an announcement fits the profile.
func ScoreAnnouncement(p *model.AIProfile, a *model.Announcement, now time.Time) (int, []string, []string) {
	score := 0
	reasons := []string{}
	cautions := []string{}
	text := a.Title + " " + a.Summary
	full := text + " " + truncateRunes(a.Content, 3000)

	regions := splitList(p.TargetRegions)
	if len(regions) > 0 {
		matched := ""
		for _, r := range regions {
			if strings.Contains(a.Region, r) || strings.Contains(a.Title, r) {
				matched = r
				break
			}
		}
		switch {
		case matched != "":
			score += 30
			reasons = append(reasons, "地区匹配："+matched)
		case containsAny(a.Title, "国家公务员", "中央机关", "国考", "全国"):
			score += 10
			reasons = append(reasons, "全国性考试，可跨地区报考")
		default:
			score -= 15
		}
	}

	types := splitList(p.TargetTypes)
	if len(types) > 0 && containsStr(types, a.Type) {
		score += 5
	}

	switch {
	case a.RegisterStart != nil && a.RegisterEnd != nil && !now.Before(*a.RegisterStart) && !now.After(*a.RegisterEnd):
		score += 20
		reasons = append(reasons, "正在报名，截止 "+a.RegisterEnd.Format("01-02 15:04"))
	case a.RegisterStart != nil && now.Before(*a.RegisterStart):
		score += 10
		reasons = append(reasons, "即将开始报名（"+a.RegisterStart.Format("01-02")+"）")
	case a.RegisterEnd == nil:
		cautions = append(cautions, "报名时间以公告原文为准")
	}

	if containsAny(text, "选调", "应届", "高校毕业生") {
		if p.FreshGraduate {
			score += 15
			reasons = append(reasons, "面向应届/选调，符合你的应届身份")
		} else {
			score -= 10
			cautions = append(cautions, "可能限应届毕业生，请核对身份要求")
		}
		if strings.Contains(text, "选调") && strings.Contains(p.PoliticalStatus, "党员") {
			score += 5
			reasons = append(reasons, "选调生常要求党员/学生干部，你具备党员身份")
		}
	}
	for _, g := range restrictedGroups {
		if strings.Contains(a.Title, g) && !strings.Contains(p.ResumeText+p.Major+p.Certificates, g) {
			score -= 30
			cautions = append(cautions, "面向特定人群（"+g+"），请确认自己是否符合")
			break
		}
	}
	if strings.Contains(a.Title, "遴选") && a.Type == model.AnnouncementCivil && (p.FreshGraduate || p.WorkYears < 2) {
		score -= 40
		cautions = append(cautions, "公务员遴选面向在职公务员/参公人员，应届生或工作不满 2 年通常不能报考")
	}
	if p.GrassrootsYears > 0 && containsAny(text, "三支一扶", "西部计划", "服务基层", "服务期满", "村官") {
		score += 10
		reasons = append(reasons, "有基层服务经历，可关注定向岗位")
	}

	rank := educationRank(p.Education)
	if rank > 0 {
		if strings.Contains(text, "博士") && rank < 4 {
			score -= 20
			cautions = append(cautions, "涉及博士岗位，注意学历门槛")
		} else if containsAny(text, "硕士", "研究生") && rank < 3 {
			score -= 15
			cautions = append(cautions, "涉及研究生岗位，注意学历门槛")
		}
	}

	for _, tok := range majorTokens(p.Major) {
		if strings.Contains(full, tok) {
			score += 15
			reasons = append(reasons, "公告内容出现你的专业关键词「"+tok+"」")
			break
		}
	}

	if !a.PublishedAt.IsZero() && now.Sub(a.PublishedAt) <= 30*24*time.Hour {
		score += 5
	}
	cautions = append(cautions, "具体岗位条件以公告附件职位表为准")
	return score, reasons, cautions
}

func normalizeScore(raw int) int {
	s := 40 + raw
	if s < 1 {
		s = 1
	}
	if s > 99 {
		s = 99
	}
	return s
}

func ruleResumeAnalysis(p *model.AIProfile, recs []JobRecommendation) ResumeAnalysis {
	strengths := []string{}
	weaknesses := []string{}
	if p.Education != "" {
		s := "学历：" + p.Education
		if educationRank(p.Education) >= 3 {
			s += "，可报考研究生及以上学历岗位，竞争面更窄"
		}
		strengths = append(strengths, s)
	} else {
		weaknesses = append(weaknesses, "未识别到学历，请在资料中补充")
	}
	if p.Major != "" {
		strengths = append(strengths, "专业：「"+p.Major+"」，可优先报考专业对口岗位")
	} else {
		weaknesses = append(weaknesses, "未识别到专业，专业是岗位筛选最关键的条件")
	}
	if p.FreshGraduate {
		strengths = append(strengths, "应届身份：可报考限应届岗位与选调生")
	} else if p.WorkYears > 0 {
		strengths = append(strengths, fmt.Sprintf("有 %d 年工作经验，可关注要求基层/工作经历的岗位", p.WorkYears))
	} else {
		weaknesses = append(weaknesses, "非应届且未识别到工作经历，部分岗位要求 2 年以上基层工作经历")
	}
	if strings.Contains(p.PoliticalStatus, "党员") {
		strengths = append(strengths, "政治面貌："+p.PoliticalStatus+"，可报考限党员岗位")
	}
	if p.GrassrootsYears > 0 {
		strengths = append(strengths, "有基层服务经历，可报考服务基层项目人员定向岗位")
	}
	if p.Certificates != "" {
		strengths = append(strengths, "证书："+p.Certificates)
	}
	if p.TargetRegions == "" {
		weaknesses = append(weaknesses, "未设置意向地区，推荐结果覆盖全部地区")
	}

	summary := fmt.Sprintf("%s%s%s，", nz(p.Education, "学历未知"), sep(p.Major), nz(p.Major, "专业未知"))
	if p.FreshGraduate {
		summary += "应届生，"
	}
	if len(recs) > 0 {
		summary += fmt.Sprintf("在当前公告中为你筛出 %d 个较匹配的考试，", len(recs))
	} else {
		summary += "当前公告库中暂未找到明显匹配的考试，"
	}
	summary += "建议结合职位表按专业、学历、身份逐条核对。"

	next := []string{
		"打开推荐公告的附件职位表，按「专业 → 学历 → 政治面貌 → 工作经历」顺序筛选",
		"优先选择专业限制严格、招录人数多的岗位，可降低竞争比",
		"把目标考试加入「我的考试」，自动生成报名/确认/考试代办提醒",
		"生成一份复习计划，按考试日期倒排时间",
	}
	return ResumeAnalysis{
		Summary:         summary,
		Strengths:       strengths,
		Weaknesses:      weaknesses,
		Directions:      MajorDirections(p.Major),
		Recommendations: recs,
		NextSteps:       next,
	}
}

type planModule struct {
	Name       string
	Categories []string
	Weight     float64
	PerQMin    float64
	TaskFmt    string
}

var planModules = []planModule{
	{Name: "言语理解", Categories: []string{"行测-言语理解"}, Weight: 1.0, PerQMin: 1.2, TaskFmt: "刷题·言语理解 随机练 %d 题，整理逻辑填空生词"},
	{Name: "数量关系", Categories: []string{"行测-数量关系"}, Weight: 0.8, PerQMin: 3, TaskFmt: "刷题·数量关系 练 %d 题，总结方程/工程/行程题型"},
	{Name: "判断推理", Categories: []string{"行测-判断推理"}, Weight: 1.1, PerQMin: 1.2, TaskFmt: "刷题·判断推理 练 %d 题，重点逻辑判断与削弱加强"},
	{Name: "资料分析", Categories: []string{"行测-资料分析"}, Weight: 1.1, PerQMin: 1.5, TaskFmt: "刷题·资料分析 练 %d 题，练习速算与增长率公式"},
	{Name: "公基常识", Categories: []string{"公基-政治理论", "公基-法律基础", "公基-人文常识", "公基-科技自然"}, Weight: 0.7, PerQMin: 0.8, TaskFmt: "刷题·公基（政治/法律/人文/科技）练 %d 题"},
	{Name: "时政", Categories: []string{"时政常识"}, Weight: 0.5, PerQMin: 0.8, TaskFmt: "阅读时政要闻 + 时政常识练 %d 题"},
}

func diagLevel(answered int64, acc float64) string {
	switch {
	case answered < 10:
		return "数据不足"
	case acc < 0.6:
		return "薄弱"
	case acc < 0.75:
		return "一般"
	default:
		return "较好"
	}
}

func diagAdvice(level, category string) string {
	switch level {
	case "数据不足":
		return "练习量不足，先做 20 题摸底"
	case "薄弱":
		return "重点补强：回顾错题，按题型专项突破"
	case "一般":
		return "保持练习，提升速度与稳定性"
	default:
		return "保持手感，少量练习维持"
	}
}

func moduleFactor(stats []CategoryDiagnosis, m planModule) (float64, string) {
	var answered, correct int64
	for _, s := range stats {
		if containsStr(m.Categories, s.Category) {
			answered += s.Answered
			correct += s.Correct
		}
	}
	if answered < 10 {
		return 1.2, "数据不足"
	}
	acc := float64(correct) / float64(answered)
	switch {
	case acc < 0.6:
		return 1.5, "薄弱"
	case acc >= 0.8:
		return 0.8, "较好"
	}
	return 1.0, "一般"
}

func roundTo5(v float64) int {
	n := int(math.Round(v/5)) * 5
	if n < 10 {
		n = 10
	}
	return n
}

// BuildRulePlan builds a deterministic study plan from practice stats.
func BuildRulePlan(p *model.AIProfile, stats []CategoryDiagnosis, examDate time.Time, dailyHours float64, now time.Time) StudyPlan {
	if dailyHours <= 0 {
		dailyHours = 3
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	exam := time.Date(examDate.Year(), examDate.Month(), examDate.Day(), 0, 0, 0, 0, now.Location())
	daysLeft := int(exam.Sub(today).Hours() / 24)
	if daysLeft < 1 {
		daysLeft = 1
	}

	total := dailyHours * 60
	institutionOnly := p != nil && strings.TrimSpace(p.TargetTypes) == model.AnnouncementInstitution
	writingName, writingTask := "申论", "申论：精读 1 篇范文 + 练写 1 道小题（周末写大作文）"
	if institutionOnly {
		writingName, writingTask = "综合应用", "综合应用能力：练 1 道案例分析/公文写作题"
	}
	review := roundTo5(total * 0.1)
	writing := roundTo5(total * 0.2)
	remain := total - float64(review+writing)

	weights := make([]float64, len(planModules))
	var sum float64
	for i, m := range planModules {
		f, _ := moduleFactor(stats, m)
		weights[i] = m.Weight * f
		sum += weights[i]
	}
	daily := []DailyBlock{}
	weakModules, unknownModules := []string{}, []string{}
	for i, m := range planModules {
		minutes := roundTo5(remain * weights[i] / sum)
		n := int(float64(minutes) / m.PerQMin)
		if n < 5 {
			n = 5
		}
		daily = append(daily, DailyBlock{Module: m.Name, Minutes: minutes, Task: fmt.Sprintf(m.TaskFmt, n)})
		switch _, lvl := moduleFactor(stats, m); lvl {
		case "薄弱":
			weakModules = append(weakModules, m.Name)
		case "数据不足":
			unknownModules = append(unknownModules, m.Name)
		}
	}
	daily = append(daily,
		DailyBlock{Module: writingName, Minutes: writing, Task: writingTask},
		DailyBlock{Module: "错题复盘", Minutes: review, Task: "错题本「再练错题」，弄懂每道错题的考点"},
	)
	sort.SliceStable(daily[:len(planModules)], func(i, j int) bool { return daily[i].Minutes > daily[j].Minutes })

	focusModules := weakModules
	if len(focusModules) == 0 && len(unknownModules) < len(planModules) {
		focusModules = unknownModules
	}
	phases := buildPhases(today, daysLeft, focusModules, writingName)
	weekly := buildWeekly(today, daysLeft, phases, focusModules, writingName)

	var weakText string
	switch {
	case len(weakModules) > 0:
		weakText = "需重点关注：" + strings.Join(weakModules, "、")
	case len(unknownModules) == len(planModules):
		weakText = "暂无刷题数据，建议先在各模块练 20 题摸底，再重新生成计划会更精准"
	case len(unknownModules) > 0:
		weakText = "以下模块练习不足，建议补做摸底：" + strings.Join(unknownModules, "、")
	default:
		weakText = "暂无明显短板"
	}
	summary := fmt.Sprintf("距离考试还有 %d 天，每天投入约 %.1f 小时。%s。计划分为 %d 个阶段，按每日模板执行，并每周至少完成 1 次整卷模考。",
		daysLeft, dailyHours, weakText, len(phases))

	tips := []string{
		"行测考试时间紧，平时练习就要计时，单题不超过 1 分钟",
		"错题比新题更有价值，每周清理一次错题本",
		"每天花 15 分钟浏览时政，考前一个月集中背诵重要会议与政策",
		"考前一周调整作息，与考试时间保持一致",
	}
	if daysLeft < 20 {
		tips = append([]string{"时间紧张：放弃难度大的数量关系难题，优先保证言语、判断、资料的正确率"}, tips...)
	}

	return StudyPlan{
		ExamDate:      exam.Format("2006-01-02"),
		DaysLeft:      daysLeft,
		DailyHours:    dailyHours,
		Summary:       summary,
		Diagnosis:     stats,
		Phases:        phases,
		DailyTemplate: daily,
		Weekly:        weekly,
		Tips:          tips,
	}
}

func buildPhases(start time.Time, days int, weak []string, writingName string) []PlanPhase {
	type spec struct {
		name, goal string
		ratio      float64
		focus      []string
	}
	weakFocus := append([]string{}, weak...)
	if len(weakFocus) == 0 {
		weakFocus = []string{"言语理解", "判断推理"}
	}
	var specs []spec
	base := spec{"基础阶段", "系统学习各模块题型与方法，完成摸底", 0.4, []string{"各模块题型梳理", "按分类顺序练", writingName + "基础"}}
	strong := spec{"强化阶段", "专项突破薄弱模块，提升正确率", 0.4, append(weakFocus, writingName+"专项")}
	sprint := spec{"冲刺阶段", "整卷模考、限时训练、查漏补缺", 0.2, []string{"整卷模考", "错题回顾", "时政背诵"}}
	switch {
	case days >= 60:
		specs = []spec{base, strong, sprint}
	case days >= 20:
		strong.ratio, sprint.ratio = 0.7, 0.3
		specs = []spec{strong, sprint}
	default:
		sprint.ratio = 1
		specs = []spec{sprint}
	}
	out := []PlanPhase{}
	cursor := start
	used := 0
	for i, s := range specs {
		n := int(math.Round(float64(days) * s.ratio))
		if i == len(specs)-1 || n < 1 {
			n = days - used
		}
		if n < 1 {
			n = 1
		}
		end := cursor.AddDate(0, 0, n-1)
		out = append(out, PlanPhase{
			Name: s.name, Start: cursor.Format("2006-01-02"), End: end.Format("2006-01-02"),
			Goal: s.goal, Focus: s.focus,
		})
		cursor = end.AddDate(0, 0, 1)
		used += n
	}
	return out
}

func buildWeekly(start time.Time, days int, phases []PlanPhase, weak []string, writingName string) []WeekPlan {
	weeks := (days + 6) / 7
	if weeks > 12 {
		weeks = 12
	}
	rotation := append([]string{}, weak...)
	if len(rotation) == 0 {
		rotation = []string{"言语理解", "判断推理", "资料分析", "数量关系"}
	}
	out := []WeekPlan{}
	for w := 0; w < weeks; w++ {
		ws := start.AddDate(0, 0, w*7)
		we := ws.AddDate(0, 0, 6)
		phase := phases[len(phases)-1].Name
		for _, p := range phases {
			if ws.Format("2006-01-02") <= p.End {
				phase = p.Name
				break
			}
		}
		focus := rotation[w%len(rotation)]
		tasks := []string{"周一至周五按每日模板执行"}
		if phase == "冲刺阶段" {
			focus = "整卷模考"
			tasks = append(tasks, "周三、周六各完成 1 套行测模考（限时 120 分钟）", "周日："+writingName+"整卷练习 + 错题清零")
		} else {
			tasks = append(tasks, "本周专项："+focus+"，额外练 50 题", "周六：行测整卷模考 1 套并复盘", "周日："+writingName+"大作文 1 篇 + 错题本清理")
		}
		out = append(out, WeekPlan{
			Week: w + 1, Start: ws.Format("2006-01-02"), End: we.Format("2006-01-02"),
			Focus: phase + "·" + focus, Tasks: tasks,
		})
	}
	return out
}

var faqRules = []struct {
	Keywords []string
	Answer   string
}{
	{[]string{"报名", "报考"}, "报名一般流程：阅读公告与职位表 → 网上注册并填报信息 → 等待资格初审 → 审核通过后网上确认/缴费 → 打印准考证。建议把目标考试加入「我的考试」，系统会按公告时间提醒你。"},
	{[]string{"时间分配", "分配时间", "怎么分配", "做不完", "来不及"}, "行测常见时间分配（120 分钟）：常识 10 分钟、言语 30 分钟、数量 10~15 分钟、判断 30 分钟、资料 25 分钟，并预留 5 分钟涂卡。先做擅长模块，数量难题可战略性放弃。"},
	{[]string{"申论"}, "申论备考：先练归纳概括（找要点、合并同类项），再练提出对策与综合分析，最后练大作文。每周至少写 1 篇大作文，对照范文修改结构与论据。"},
	{[]string{"面试"}, "结构化面试常见题型：综合分析、组织管理、人际关系、应急应变。建议每天练 2 道题并录音复盘，注意答题结构与时间控制。"},
	{[]string{"选调"}, "选调生通常面向应届毕业生，多要求中共党员或学生干部经历，部分省份限定重点高校。请以当年选调公告为准。"},
	{[]string{"资料分析"}, "资料分析要点：熟记增长率、增长量、比重、平均数公式，练习截位直除与估算，先看问题再回原文找数据。"},
	{[]string{"数量"}, "数量关系建议优先掌握方程法、工程问题、行程问题、利润问题和排列组合基础题，难题果断跳过。"},
}

func ruleAnswer(question string) string {
	for _, r := range faqRules {
		if containsAny(question, r.Keywords...) {
			return r.Answer
		}
	}
	return "目前 AI 大模型未配置，只能回答常见问题（报名流程、时间分配、申论、面试、选调、资料分析、数量关系）。你也可以使用「简历分析」和「复习计划」功能获取个性化建议。"
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '，' || r == '、' || r == ' ' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinUnique(a, b string) string {
	out := splitList(a)
	for _, v := range splitList(b) {
		if !containsStr(out, v) {
			out = append(out, v)
		}
	}
	return strings.Join(out, ",")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func nz(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func sep(s string) string {
	if s == "" {
		return "，"
	}
	return "·"
}
