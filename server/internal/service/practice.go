package service

import (
	"errors"
	"math/rand"
	"time"

	"gongkao/internal/model"
	"gongkao/internal/pkg"

	"gorm.io/gorm"
)

type PracticeService struct {
	db *gorm.DB
}

func NewPracticeService(db *gorm.DB) *PracticeService {
	return &PracticeService{db: db}
}

func (s *PracticeService) ListCategories() ([]model.QuestionCategory, error) {
	var list []model.QuestionCategory
	err := s.db.Where("status = ?", model.StatusPublished).Order("sort ASC, id ASC").Find(&list).Error
	return list, err
}

type StartSessionReq struct {
	CategoryID uint64 `json:"category_id"`
	Mode       string `json:"mode"` // sequential | random
	Count      int    `json:"count"`
}

type StartSessionResp struct {
	SessionID uint64           `json:"session_id"`
	Questions []model.Question `json:"questions"`
}

func (s *PracticeService) StartSession(userID uint64, req StartSessionReq) (*StartSessionResp, error) {
	if req.Count <= 0 {
		req.Count = 10
	}
	if req.Count > 50 {
		req.Count = 50
	}
	if req.Mode == "" {
		req.Mode = "sequential"
	}
	q := s.db.Preload("Options", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort ASC, id ASC")
	}).Where("status = ?", model.StatusPublished)
	if req.CategoryID > 0 {
		q = q.Where("category_id = ?", req.CategoryID)
	}
	var questions []model.Question
	if err := q.Order("id ASC").Find(&questions).Error; err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return nil, errors.New("no questions")
	}
	if req.Mode == "random" {
		rand.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })
	}
	if len(questions) > req.Count {
		questions = questions[:req.Count]
	}
	// hide answer key to client until submit — keep in DB only; still return for offline? hide
	safe := make([]model.Question, len(questions))
	for i, qq := range questions {
		safe[i] = qq
		safe[i].AnswerKey = ""
		safe[i].Analysis = ""
	}
	sess := model.PracticeSession{
		UserID:     userID,
		CategoryID: req.CategoryID,
		Mode:       req.Mode,
		Total:      len(questions),
		Status:     "ongoing",
		StartedAt:  time.Now(),
	}
	if err := s.db.Create(&sess).Error; err != nil {
		return nil, err
	}
	return &StartSessionResp{SessionID: sess.ID, Questions: safe}, nil
}

type AnswerItem struct {
	QuestionID uint64 `json:"question_id"`
	UserKey    string `json:"user_key"`
}

type SubmitReq struct {
	SessionID uint64       `json:"session_id"`
	Answers   []AnswerItem `json:"answers"`
}

type SubmitResp struct {
	SessionID  uint64 `json:"session_id"`
	Total      int    `json:"total"`
	Correct    int    `json:"correct"`
	Accuracy   float64 `json:"accuracy"`
	WrongIDs   []uint64 `json:"wrong_ids"`
}

func (s *PracticeService) Submit(userID uint64, req SubmitReq) (*SubmitResp, error) {
	var sess model.PracticeSession
	if err := s.db.Where("id = ? AND user_id = ?", req.SessionID, userID).First(&sess).Error; err != nil {
		return nil, err
	}
	if sess.Status == "finished" {
		return nil, errors.New("session already finished")
	}
	ids := make([]uint64, 0, len(req.Answers))
	ansMap := map[uint64]string{}
	for _, a := range req.Answers {
		ids = append(ids, a.QuestionID)
		ansMap[a.QuestionID] = a.UserKey
	}
	var questions []model.Question
	if err := s.db.Where("id IN ?", ids).Find(&questions).Error; err != nil {
		return nil, err
	}
	correct := 0
	wrongIDs := []uint64{}
	now := time.Now()
	for _, q := range questions {
		uk := ansMap[q.ID]
		ok := uk != "" && uk == q.AnswerKey
		_ = s.db.Create(&model.UserAnswer{
			SessionID:  sess.ID,
			QuestionID: q.ID,
			UserKey:    uk,
			IsCorrect:  ok,
		}).Error
		if ok {
			correct++
		} else {
			wrongIDs = append(wrongIDs, q.ID)
			var wq model.WrongQuestion
			err := s.db.Where("user_id = ? AND question_id = ?", userID, q.ID).First(&wq).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_ = s.db.Create(&model.WrongQuestion{
					UserID: userID, QuestionID: q.ID, WrongCount: 1, LastWrongAt: now,
				}).Error
			} else if err == nil {
				_ = s.db.Model(&wq).Updates(map[string]interface{}{
					"wrong_count":   wq.WrongCount + 1,
					"last_wrong_at": now,
					"mastered_at":   nil,
				}).Error
			}
		}
	}
	fin := now
	_ = s.db.Model(&sess).Updates(map[string]interface{}{
		"correct":     correct,
		"total":       len(questions),
		"status":      "finished",
		"finished_at": fin,
	}).Error
	acc := 0.0
	if len(questions) > 0 {
		acc = float64(correct) / float64(len(questions))
	}
	return &SubmitResp{
		SessionID: sess.ID,
		Total:     len(questions),
		Correct:   correct,
		Accuracy:  acc,
		WrongIDs:  wrongIDs,
	}, nil
}

func (s *PracticeService) ListWrong(userID uint64, page, pageSize int) (*pkg.PageResult, error) {
	db := s.db.Model(&model.WrongQuestion{}).Where("user_id = ? AND mastered_at IS NULL", userID)
	var total int64
	_ = db.Count(&total).Error
	var list []model.WrongQuestion
	err := db.Preload("Question.Options").Order("last_wrong_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return &pkg.PageResult{List: list, Total: total, Page: page, PageSize: pageSize}, err
}

func (s *PracticeService) RemoveWrong(userID, questionID uint64) error {
	return s.db.Where("user_id = ? AND question_id = ?", userID, questionID).Delete(&model.WrongQuestion{}).Error
}

type PracticeStats struct {
	TotalAnswered int64            `json:"total_answered"`
	TotalCorrect  int64            `json:"total_correct"`
	Accuracy      float64          `json:"accuracy"`
	Last7Days     []DailyPractice  `json:"last_7_days"`
}

type DailyPractice struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (s *PracticeService) Stats(userID uint64) (*PracticeStats, error) {
	var total, correct int64
	s.db.Model(&model.UserAnswer{}).
		Joins("JOIN practice_sessions ON practice_sessions.id = user_answers.session_id").
		Where("practice_sessions.user_id = ?", userID).Count(&total)
	s.db.Model(&model.UserAnswer{}).
		Joins("JOIN practice_sessions ON practice_sessions.id = user_answers.session_id").
		Where("practice_sessions.user_id = ? AND user_answers.is_correct = ?", userID, true).Count(&correct)
	acc := 0.0
	if total > 0 {
		acc = float64(correct) / float64(total)
	}
	days := make([]DailyPractice, 0, 7)
	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i)
		start := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
		end := start.Add(24 * time.Hour)
		var cnt int64
		s.db.Model(&model.UserAnswer{}).
			Joins("JOIN practice_sessions ON practice_sessions.id = user_answers.session_id").
			Where("practice_sessions.user_id = ? AND user_answers.id > 0", userID).
			Where("practice_sessions.started_at >= ? AND practice_sessions.started_at < ?", start, end).
			Count(&cnt)
		days = append(days, DailyPractice{Date: start.Format("2006-01-02"), Count: cnt})
	}
	return &PracticeStats{TotalAnswered: total, TotalCorrect: correct, Accuracy: acc, Last7Days: days}, nil
}

type ImportQuestion struct {
	Category   string              `json:"category"`
	Stem       string              `json:"stem"`
	AnswerKey  string              `json:"answer_key"`
	Analysis   string              `json:"analysis"`
	Difficulty int                 `json:"difficulty"`
	Tags       string              `json:"tags"`
	Options    []map[string]string `json:"options"` // [{"key":"A","content":"..."}]
}

func (s *PracticeService) ImportQuestions(items []ImportQuestion) (int, error) {
	n := 0
	for _, it := range items {
		if it.Stem == "" || it.AnswerKey == "" || it.Category == "" {
			continue
		}
		var cat model.QuestionCategory
		err := s.db.Where("name = ?", it.Category).First(&cat).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cat = model.QuestionCategory{Name: it.Category, Status: model.StatusPublished}
			if err := s.db.Create(&cat).Error; err != nil {
				return n, err
			}
		} else if err != nil {
			return n, err
		}
		diff := it.Difficulty
		if diff == 0 {
			diff = 1
		}
		q := model.Question{
			CategoryID: cat.ID,
			Stem:       it.Stem,
			Type:       "single",
			AnswerKey:  it.AnswerKey,
			Analysis:   it.Analysis,
			Difficulty: diff,
			Tags:       it.Tags,
			Status:     model.StatusPublished,
		}
		if err := s.db.Create(&q).Error; err != nil {
			return n, err
		}
		for i, opt := range it.Options {
			_ = s.db.Create(&model.QuestionOption{
				QuestionID: q.ID,
				Key:        opt["key"],
				Content:    opt["content"],
				Sort:       i,
			}).Error
		}
		n++
	}
	return n, nil
}

func (s *PracticeService) SeedIfEmpty() error {
	var n int64
	s.db.Model(&model.Question{}).Count(&n)
	if n > 0 {
		return nil
	}
	_, err := s.ImportQuestions([]ImportQuestion{
		{
			Category: "行测", Stem: "下列哪个数是质数？", AnswerKey: "B", Analysis: "质数只有1和自身两个正因数。",
			Options: []map[string]string{{"key": "A", "content": "1"}, {"key": "B", "content": "13"}, {"key": "C", "content": "15"}, {"key": "D", "content": "21"}},
		},
		{
			Category: "行测", Stem: "成语“画蛇添足”比喻：", AnswerKey: "C", Analysis: "比喻做多余的事，反而不恰当。",
			Options: []map[string]string{{"key": "A", "content": "做事认真"}, {"key": "B", "content": "善于创新"}, {"key": "C", "content": "多此一举"}, {"key": "D", "content": "精益求精"}},
		},
		{
			Category: "公基", Stem: "我国的根本政治制度是：", AnswerKey: "A", Analysis: "人民代表大会制度是我国的根本政治制度。",
			Options: []map[string]string{{"key": "A", "content": "人民代表大会制度"}, {"key": "B", "content": "多党合作制度"}, {"key": "C", "content": "民族区域自治制度"}, {"key": "D", "content": "基层群众自治制度"}},
		},
		{
			Category: "公基", Stem: "《中华人民共和国宪法》规定，中华人民共和国的一切权力属于：", AnswerKey: "B", Analysis: "一切权力属于人民。",
			Options: []map[string]string{{"key": "A", "content": "政府"}, {"key": "B", "content": "人民"}, {"key": "C", "content": "全国人大常委会"}, {"key": "D", "content": "国务院"}},
		},
	})
	return err
}