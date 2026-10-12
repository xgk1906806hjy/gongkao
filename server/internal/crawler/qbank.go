package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gongkao/internal/config"
	"gongkao/internal/model"
	"gongkao/internal/service"

	"gorm.io/gorm"
)

// CollectQuestions gathers practice items from local packs, remote feeds, and affairs→quiz.
// Does not write the database; caller imports via PracticeService.
func CollectQuestions(ctx context.Context, db *gorm.DB, cfg *config.Config) ([]service.ImportQuestion, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	seen := map[string]struct{}{}
	out := make([]service.ImportQuestion, 0, 256)

	add := func(items []service.ImportQuestion, source string) {
		kept := 0
		for _, it := range items {
			it = normalizeImportQuestion(it)
			if it.Stem == "" || it.AnswerKey == "" || it.Category == "" || len(it.Options) < 2 {
				continue
			}
			key := strings.TrimSpace(it.Stem)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, it)
			kept++
		}
		slog.Info("qbank source", "source", source, "kept", kept)
	}

	// 1) Local open packs (self-owned / project seed).
	for _, p := range localQBankPaths(cfg) {
		items, err := loadImportQuestionsFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			slog.Warn("qbank local pack", "path", p, "err", err)
			continue
		}
		add(items, "local:"+filepath.Base(p))
	}

	// 2) Remote authorized JSON feeds (ImportQuestion schema or pei-pei-shua-like).
	for _, u := range splitCSV(cfgQuestionFeeds(cfg)) {
		items, err := fetchRemoteQuestionFeed(ctx, u)
		if err != nil {
			slog.Warn("qbank remote feed", "url", u, "err", err)
			if db != nil {
				_ = db.Create(&model.CrawlFailure{
					Source: "qbank-feed", URL: u, Error: err.Error(),
				}).Error
			}
			continue
		}
		add(items, "remote:"+truncateRunes(u, 48))
	}

	// 3) Convert crawled 时政 articles into 时政常识 items.
	if db != nil {
		items, err := buildAffairsQuiz(db, 40)
		if err != nil {
			slog.Warn("qbank affairs quiz", "err", err)
		} else {
			add(items, "affairs-quiz")
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no questions collected")
	}
	return out, nil
}

// WriteQuestionBankJSON collects questions and writes a staging JSON file.
func WriteQuestionBankJSON(ctx context.Context, db *gorm.DB, cfg *config.Config) (string, int, error) {
	items, err := CollectQuestions(ctx, db, cfg)
	if err != nil {
		return "", 0, err
	}
	path, err := SaveQuestionBankJSON(cfg, items)
	if err != nil {
		return "", 0, err
	}
	return path, len(items), nil
}

// SaveQuestionBankJSON writes items to the configured staging path.
func SaveQuestionBankJSON(cfg *config.Config, items []service.ImportQuestion) (string, error) {
	path := "data/question_bank/crawled.json"
	if cfg != nil && cfg.QuestionBankJSON != "" {
		path = cfg.QuestionBankJSON
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	slog.Info("question bank written", "path", path, "count", len(items))
	return path, nil
}

func cfgQuestionFeeds(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.QuestionBankFeedURLs
}

func localQBankPaths(cfg *config.Config) []string {
	paths := []string{
		"data/question_bank/questions.json",
		"data/question_bank/open_pack.json",
	}
	if cfg != nil && cfg.QuestionBankExtraPacks != "" {
		for _, p := range splitCSV(cfg.QuestionBankExtraPacks) {
			if p != "" {
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func loadImportQuestionsFile(path string) ([]service.ImportQuestion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseQuestionPayload(raw)
}

func fetchRemoteQuestionFeed(ctx context.Context, feedURL string) ([]service.ImportQuestion, error) {
	body, err := fetchBytes(ctx, defaultHTTPClient(), feedURL)
	if err != nil {
		return nil, err
	}
	return parseQuestionPayload(body)
}

func parseQuestionPayload(raw []byte) ([]service.ImportQuestion, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	// Native schema: []ImportQuestion
	var native []service.ImportQuestion
	if err := json.Unmarshal(raw, &native); err == nil && len(native) > 0 && native[0].Stem != "" {
		return native, nil
	}
	// Wrapped: {"items":[...]} or {"questions":[...]}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrap); err == nil {
		for _, key := range []string{"items", "questions", "data", "list"} {
			if v, ok := wrap[key]; ok {
				items, err := parseQuestionPayload(v)
				if err == nil {
					return items, nil
				}
			}
		}
	}
	// pei-pei-shua / companion style
	var alt []struct {
		Title          string `json:"title"`
		Stem           string `json:"stem"`
		Question       string `json:"question"`
		Answer         string `json:"answer"`
		AnswerKey      string `json:"answer_key"`
		Analysis       string `json:"analysis"`
		Explanation    string `json:"explanation"`
		Category       string `json:"category"`
		Module         string `json:"module"`
		KnowledgePoint string `json:"knowledge_point"`
		Tags           string `json:"tags"`
		Difficulty     int    `json:"difficulty"`
		Options        []struct {
			Key     string `json:"key"`
			Content string `json:"content"`
			Text    string `json:"text"`
		} `json:"options"`
	}
	if err := json.Unmarshal(raw, &alt); err != nil {
		return nil, fmt.Errorf("unsupported question json: %w", err)
	}
	out := make([]service.ImportQuestion, 0, len(alt))
	for _, a := range alt {
		stem := firstNonEmpty(a.Stem, a.Title, a.Question)
		ans := firstNonEmpty(a.AnswerKey, a.Answer)
		cat := mapModuleCategory(firstNonEmpty(a.Category, a.Module, a.KnowledgePoint))
		opts := make([]map[string]string, 0, 4)
		keys := []string{"A", "B", "C", "D"}
		for i, o := range a.Options {
			if i >= 4 {
				break
			}
			k := strings.ToUpper(strings.TrimSpace(o.Key))
			if k == "" {
				k = keys[i]
			}
			content := firstNonEmpty(o.Content, o.Text)
			if content == "" {
				continue
			}
			// Strip leading "A." prefixes common in dumps.
			content = strings.TrimSpace(strings.TrimPrefix(content, k+"."))
			content = strings.TrimSpace(strings.TrimPrefix(content, k+"、"))
			opts = append(opts, map[string]string{"key": k, "content": content})
		}
		ans = strings.ToUpper(strings.TrimSpace(ans))
		if len(ans) > 1 {
			ans = string(ans[0])
		}
		out = append(out, service.ImportQuestion{
			Category:   cat,
			Stem:       stem,
			AnswerKey:  ans,
			Analysis:   firstNonEmpty(a.Analysis, a.Explanation),
			Difficulty: a.Difficulty,
			Tags:       firstNonEmpty(a.Tags, a.KnowledgePoint, "open-feed"),
			Options:    opts,
		})
	}
	return out, nil
}

func normalizeImportQuestion(it service.ImportQuestion) service.ImportQuestion {
	it.Category = mapModuleCategory(it.Category)
	it.Stem = strings.TrimSpace(it.Stem)
	it.AnswerKey = strings.ToUpper(strings.TrimSpace(it.AnswerKey))
	if len(it.AnswerKey) > 1 {
		it.AnswerKey = string(it.AnswerKey[0])
	}
	if it.Difficulty <= 0 {
		it.Difficulty = 1
	}
	opts := make([]map[string]string, 0, len(it.Options))
	keys := []string{"A", "B", "C", "D"}
	for i, o := range it.Options {
		k := strings.ToUpper(strings.TrimSpace(o["key"]))
		if k == "" && i < len(keys) {
			k = keys[i]
		}
		c := strings.TrimSpace(o["content"])
		if c == "" {
			continue
		}
		opts = append(opts, map[string]string{"key": k, "content": c})
	}
	it.Options = opts
	return it
}

func mapModuleCategory(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "时政常识"
	}
	lower := strings.ToLower(n)
	switch {
	case strings.Contains(n, "言语") || strings.Contains(lower, "language"):
		return "行测-言语理解"
	case strings.Contains(n, "数量") || strings.Contains(lower, "math") || strings.Contains(n, "数理"):
		return "行测-数量关系"
	case strings.Contains(n, "判断") || strings.Contains(n, "逻辑") || strings.Contains(lower, "logic"):
		return "行测-判断推理"
	case strings.Contains(n, "资料") || strings.Contains(lower, "data"):
		return "行测-资料分析"
	case strings.Contains(n, "政治"):
		return "公基-政治理论"
	case strings.Contains(n, "法律") || strings.Contains(n, "法治"):
		return "公基-法律基础"
	case strings.Contains(n, "人文") || strings.Contains(n, "历史") || strings.Contains(n, "文学"):
		return "公基-人文常识"
	case strings.Contains(n, "科技") || strings.Contains(n, "自然") || strings.Contains(n, "地理"):
		return "公基-科技自然"
	case strings.Contains(n, "时政") || strings.Contains(n, "常识判断") || strings.Contains(n, "常识"):
		return "时政常识"
	default:
		// Keep known project categories as-is.
		known := map[string]struct{}{
			"行测-言语理解": {}, "行测-数量关系": {}, "行测-判断推理": {}, "行测-资料分析": {},
			"公基-政治理论": {}, "公基-法律基础": {}, "公基-人文常识": {}, "公基-科技自然": {}, "时政常识": {},
		}
		if _, ok := known[n]; ok {
			return n
		}
		return "时政常识"
	}
}

func buildAffairsQuiz(db *gorm.DB, limit int) ([]service.ImportQuestion, error) {
	if limit <= 0 {
		limit = 40
	}
	var rows []model.AffairsArticle
	err := db.Where("status = ?", model.StatusPublished).
		Order("published_at DESC, id DESC").
		Limit(limit * 2).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) < 4 {
		return nil, nil
	}
	titles := make([]string, 0, len(rows))
	for _, r := range rows {
		t := strings.TrimSpace(r.Title)
		if t != "" {
			titles = append(titles, t)
		}
	}
	out := make([]service.ImportQuestion, 0, limit)
	for i, r := range rows {
		if len(out) >= limit {
			break
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			continue
		}
		distractors := pickDistractors(titles, title, 3)
		if len(distractors) < 3 {
			continue
		}
		opts := shuffleFour(
			truncateRunes(title, 40),
			truncateRunes(distractors[0], 40),
			truncateRunes(distractors[1], 40),
			truncateRunes(distractors[2], 40),
		)
		ans := ""
		for _, o := range opts {
			if o["content"] == truncateRunes(title, 40) {
				ans = o["key"]
				break
			}
		}
		if ans == "" {
			continue
		}
		src := strings.TrimSpace(r.Source)
		if src == "" {
			src = "时政"
		}
		analysis := "本题根据已采集时政文章生成，正确答案为近期热点标题。"
		if s := strings.TrimSpace(r.Summary); s != "" {
			analysis = "时政要点：" + truncateRunes(s, 120)
		}
		_ = i
		stem := fmt.Sprintf("根据公开时政资讯（源：%s），下列正确的要闻标题是哪一项？（#%d）", src, r.ID)

		out = append(out, service.ImportQuestion{
			Category:   "时政常识",
			Stem:       stem,
			AnswerKey:  ans,
			Analysis:   analysis,
			Difficulty: 1,
			Tags:       "时政热点,采集生成",
			Options:    opts,
		})
	}
	return out, nil
}

func pickDistractors(all []string, correct string, n int) []string {
	out := make([]string, 0, n)
	for _, t := range all {
		if t == correct {
			continue
		}
		out = append(out, t)
		if len(out) >= n {
			break
		}
	}
	return out
}

func shuffleFour(a, b, c, d string) []map[string]string {
	// Deterministic rotate based on first rune of correct-ish content to vary keys.
	items := []string{a, b, c, d}
	shift := 0
	if r, _ := utf8.DecodeRuneInString(a); r != utf8.RuneError {
		shift = int(r) % 4
	}
	keys := []string{"A", "B", "C", "D"}
	out := make([]map[string]string, 4)
	for i := 0; i < 4; i++ {
		out[i] = map[string]string{
			"key":     keys[i],
			"content": items[(i+shift)%4],
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
