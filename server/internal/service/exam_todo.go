package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gongkao/internal/model"
	"gongkao/internal/pkg"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ExamTodoService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewExamTodoService(db *gorm.DB, rdb *redis.Client) *ExamTodoService {
	return &ExamTodoService{db: db, rdb: rdb}
}

func (s *ExamTodoService) ListExams(userID uint64) ([]model.UserExam, error) {
	var list []model.UserExam
	err := s.db.Preload("Announcement").Where("user_id = ?", userID).Order("id DESC").Find(&list).Error
	return list, err
}

func (s *ExamTodoService) Join(ctx context.Context, userID, announcementID uint64) (*model.UserExam, error) {
	var ann model.Announcement
	if err := s.db.Where("status = ?", model.StatusPublished).First(&ann, announcementID).Error; err != nil {
		return nil, err
	}
	var existing model.UserExam
	err := s.db.Where("user_id = ? AND announcement_id = ?", userID, announcementID).First(&existing).Error
	if err == nil {
		_ = s.generateTodos(ctx, userID, existing.ID, &ann)
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	ue := model.UserExam{UserID: userID, AnnouncementID: announcementID}
	if err := s.db.Create(&ue).Error; err != nil {
		return nil, err
	}
	if err := s.generateTodos(ctx, userID, ue.ID, &ann); err != nil {
		return nil, err
	}
	ue.Announcement = &ann
	return &ue, nil
}

func (s *ExamTodoService) Leave(ctx context.Context, userID, announcementID uint64) error {
	var ue model.UserExam
	if err := s.db.Where("user_id = ? AND announcement_id = ?", userID, announcementID).First(&ue).Error; err != nil {
		return err
	}
	var todos []model.Todo
	s.db.Where("user_exam_id = ? AND status = ?", ue.ID, model.TodoPending).Find(&todos)
	for _, t := range todos {
		_ = s.db.Model(&t).Updates(map[string]interface{}{
			"status":        model.TodoCancelled,
			"remind_status": model.RemindSkipped,
		}).Error
		if s.rdb != nil {
			_ = s.rdb.ZRem(ctx, pkg.RemindZSetKey, strconv.FormatUint(t.ID, 10)).Err()
		}
	}
	return s.db.Delete(&ue).Error
}

func (s *ExamTodoService) generateTodos(ctx context.Context, userID, userExamID uint64, ann *model.Announcement) error {
	type rule struct {
		typ   string
		title string
		due   *time.Time
	}
	rules := []rule{
		{model.TodoRegister, "报名截止", coalesceTime(ann.RegisterEnd, ann.RegisterStart)},
		{model.TodoConfirm, "资格确认截止", coalesceTime(ann.ConfirmEnd, ann.ConfirmStart)},
		{model.TodoExam, "参加考试", ann.ExamAt},
	}
	if ann.RegisterEnd == nil && ann.RegisterStart != nil {
		rules[0].title = "报名开始"
	}
	if ann.ConfirmEnd == nil && ann.ConfirmStart != nil {
		rules[1].title = "资格确认开始"
	}
	for _, r := range rules {
		if r.due == nil {
			continue
		}
		due := r.due.UTC()
		remind := due.Add(-10 * time.Minute)
		var todo model.Todo
		err := s.db.Where("user_exam_id = ? AND type = ? AND status <> ?", userExamID, r.typ, model.TodoCancelled).First(&todo).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			todo = model.Todo{
				UserID:         userID,
				UserExamID:     userExamID,
				AnnouncementID: ann.ID,
				Type:           r.typ,
				Title:          r.title,
				DueAt:          due,
				RemindAt:       remind,
				Status:         model.TodoPending,
				RemindStatus:   model.RemindPending,
			}
			if err := s.db.Create(&todo).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if todo.Status == model.TodoPending && todo.RemindStatus != model.RemindSent {
			_ = s.db.Model(&todo).Updates(map[string]interface{}{
				"title":     r.title,
				"due_at":    due,
				"remind_at": remind,
			}).Error
			todo.RemindAt = remind
		}
		if s.rdb != nil && todo.Status == model.TodoPending && todo.RemindStatus == model.RemindPending {
			_ = s.rdb.ZAdd(ctx, pkg.RemindZSetKey, redis.Z{
				Score:  float64(todo.RemindAt.Unix()),
				Member: strconv.FormatUint(todo.ID, 10),
			}).Err()
		}
	}
	return nil
}

func coalesceTime(a, b *time.Time) *time.Time {
	if a != nil {
		return a
	}
	return b
}

func (s *ExamTodoService) ListTodos(userID uint64) ([]model.Todo, error) {
	var list []model.Todo
	err := s.db.Preload("Announcement").
		Where("user_id = ? AND status <> ?", userID, model.TodoCancelled).
		Order("due_at ASC").Find(&list).Error
	return list, err
}

func (s *ExamTodoService) PatchTodo(ctx context.Context, userID, todoID uint64, status string) (*model.Todo, error) {
	var todo model.Todo
	if err := s.db.Where("id = ? AND user_id = ?", todoID, userID).First(&todo).Error; err != nil {
		return nil, err
	}
	switch status {
	case model.TodoDone, model.TodoCancelled, model.TodoPending:
	default:
		return nil, fmt.Errorf("invalid status")
	}
	updates := map[string]interface{}{"status": status}
	if status == model.TodoCancelled || status == model.TodoDone {
		updates["remind_status"] = model.RemindSkipped
		if s.rdb != nil {
			_ = s.rdb.ZRem(ctx, pkg.RemindZSetKey, strconv.FormatUint(todo.ID, 10)).Err()
		}
	}
	if err := s.db.Model(&todo).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = s.db.Preload("Announcement").First(&todo, todo.ID).Error
	return &todo, nil
}

// EnqueuePendingReminders loads pending reminds into redis (startup compensation).
func (s *ExamTodoService) EnqueuePendingReminders(ctx context.Context) error {
	if s.rdb == nil {
		return nil
	}
	var todos []model.Todo
	if err := s.db.Where("status = ? AND remind_status = ?", model.TodoPending, model.RemindPending).Find(&todos).Error; err != nil {
		return err
	}
	for _, t := range todos {
		_ = s.rdb.ZAdd(ctx, pkg.RemindZSetKey, redis.Z{
			Score:  float64(t.RemindAt.Unix()),
			Member: strconv.FormatUint(t.ID, 10),
		}).Err()
	}
	return nil
}