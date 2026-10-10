package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"gongkao/internal/config"
	"gongkao/internal/model"
	"gongkao/internal/pkg"
	"gongkao/internal/wx"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ReminderService struct {
	db  *gorm.DB
	rdb *redis.Client
	wx  *wx.Client
	cfg *config.Config
}

func NewReminderService(db *gorm.DB, rdb *redis.Client, wxCli *wx.Client, cfg *config.Config) *ReminderService {
	return &ReminderService{db: db, rdb: rdb, wx: wxCli, cfg: cfg}
}

func (s *ReminderService) ProcessDue(ctx context.Context) {
	if s.rdb == nil {
		s.processFromDB(ctx)
		return
	}
	now := float64(time.Now().Unix())
	ids, err := s.rdb.ZRangeByScore(ctx, pkg.RemindZSetKey, &redis.ZRangeBy{
		Min: "-inf", Max: strconv.FormatFloat(now, 'f', 0, 64), Count: 100,
	}).Result()
	if err != nil {
		slog.Error("remind zrange", "err", err)
		return
	}
	for _, idStr := range ids {
		id, _ := strconv.ParseUint(idStr, 10, 64)
		if id == 0 {
			_ = s.rdb.ZRem(ctx, pkg.RemindZSetKey, idStr).Err()
			continue
		}
		s.sendOne(ctx, id)
		_ = s.rdb.ZRem(ctx, pkg.RemindZSetKey, idStr).Err()
	}
}

func (s *ReminderService) processFromDB(ctx context.Context) {
	var todos []model.Todo
	s.db.Where("status = ? AND remind_status = ? AND remind_at <= ?",
		model.TodoPending, model.RemindPending, time.Now()).Limit(100).Find(&todos)
	for _, t := range todos {
		s.sendOne(ctx, t.ID)
	}
}

func (s *ReminderService) sendOne(ctx context.Context, todoID uint64) {
	var todo model.Todo
	if err := s.db.Preload("Announcement").First(&todo, todoID).Error; err != nil {
		return
	}
	if todo.Status != model.TodoPending || todo.RemindStatus != model.RemindPending {
		return
	}
	var user model.User
	if err := s.db.First(&user, todo.UserID).Error; err != nil {
		return
	}
	title := todo.Title
	annTitle := ""
	if todo.Announcement != nil {
		annTitle = todo.Announcement.Title
		if len(annTitle) > 20 {
			annTitle = annTitle[:20] + "…"
		}
	}
	tpl := s.cfg.WXSubscribeTemplateTodo
	if tpl == "" {
		tpl = "TODO_TEMPLATE"
	}
	err := s.wx.SendSubscribe(ctx, wx.SubscribeReq{
		ToUser:     user.OpenID,
		TemplateID: tpl,
		Page:       "pages/mine/todos/todos",
		Data: map[string]interface{}{
			"thing1": map[string]string{"value": truncate(title, 20)},
			"thing2": map[string]string{"value": truncate(annTitle, 20)},
			"time3":  map[string]string{"value": todo.DueAt.Local().Format("2006-01-02 15:04")},
		},
	})
	if err != nil {
		retry := todo.RemindRetry + 1
		status := model.RemindFailed
		if retry < 3 {
			status = model.RemindPending
			if s.rdb != nil {
				next := time.Now().Add(time.Duration(retry) * 2 * time.Minute)
				_ = s.rdb.ZAdd(ctx, pkg.RemindZSetKey, redis.Z{
					Score: float64(next.Unix()), Member: strconv.FormatUint(todo.ID, 10),
				}).Err()
			}
		}
		_ = s.db.Model(&todo).Updates(map[string]interface{}{
			"remind_status":     status,
			"remind_retry":      retry,
			"last_remind_error": err.Error(),
		}).Error
		slog.Warn("remind send failed", "todo", todo.ID, "err", err)
		return
	}
	_ = s.db.Model(&todo).Updates(map[string]interface{}{
		"remind_status": model.RemindSent,
	}).Error
	slog.Info("remind sent", "todo", todo.ID, "user", user.ID)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}