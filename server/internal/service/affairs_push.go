package service

import (
	"context"
	"log/slog"
	"strconv"

	"gongkao/internal/config"
	"gongkao/internal/model"
	"gongkao/internal/wx"

	"gorm.io/gorm"
)

// AffairsPushService sends optional daily affairs digest via subscribe message.
type AffairsPushService struct {
	db  *gorm.DB
	wx  *wx.Client
	cfg *config.Config
	sub *SubscribeService
}

func NewAffairsPushService(db *gorm.DB, wxCli *wx.Client, cfg *config.Config, sub *SubscribeService) *AffairsPushService {
	return &AffairsPushService{db: db, wx: wxCli, cfg: cfg, sub: sub}
}

func (s *AffairsPushService) PushDaily(ctx context.Context) {
	if s.cfg == nil || !s.cfg.AffairsDailyPushEnabled {
		return
	}
	tpl := s.cfg.WXSubscribeTemplateAff
	if tpl == "" {
		if s.cfg.WXMock {
			tpl = "MOCK_AFFAIRS_TEMPLATE"
		} else {
			slog.Warn("affairs daily push skipped: empty template")
			return
		}
	}
	var article model.AffairsArticle
	if err := s.db.Where("status = ?", model.StatusPublished).Order("published_at DESC").First(&article).Error; err != nil {
		slog.Warn("affairs daily push: no article", "err", err)
		return
	}
	var prefs []model.UserSubscribePref
	s.db.Where("kind = ? AND status = ?", model.SubscribeKindAffairs, model.SubscribeAccept).Find(&prefs)
	for _, p := range prefs {
		var user model.User
		if err := s.db.First(&user, p.UserID).Error; err != nil {
			continue
		}
		err := s.wx.SendSubscribe(ctx, wx.SubscribeReq{
			ToUser:     user.OpenID,
			TemplateID: tpl,
			Page:       "pages/affairs/detail?id=" + strconv.FormatUint(article.ID, 10),
			Data: map[string]interface{}{
				"thing1": map[string]string{"value": truncate(article.Title, 20)},
				"thing2": map[string]string{"value": truncate(article.Summary, 20)},
				"time3":  map[string]string{"value": article.PublishedAt.Local().Format("2006-01-02 15:04")},
			},
		})
		if err != nil {
			slog.Warn("affairs daily push failed", "user", user.ID, "err", err)
			continue
		}
		slog.Info("affairs daily push sent", "user", user.ID, "article", article.ID)
	}
}
