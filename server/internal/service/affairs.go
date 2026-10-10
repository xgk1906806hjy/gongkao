package service

import (
	"errors"
	"strings"
	"time"

	"gongkao/internal/model"
	"gongkao/internal/pkg"

	"gorm.io/gorm"
)

type AffairsService struct {
	db *gorm.DB
}

func NewAffairsService(db *gorm.DB) *AffairsService {
	return &AffairsService{db: db}
}

type AffairsQuery struct {
	Tag       string
	Keyword   string
	Page      int
	PageSize  int
	AdminView bool
}

func (s *AffairsService) List(q AffairsQuery) (*pkg.PageResult, error) {
	db := s.db.Model(&model.AffairsArticle{})
	if !q.AdminView {
		db = db.Where("status = ?", model.StatusPublished)
	}
	if q.Tag != "" {
		db = db.Where("tags LIKE ?", "%"+q.Tag+"%")
	}
	if q.Keyword != "" {
		kw := "%" + strings.TrimSpace(q.Keyword) + "%"
		db = db.Where("title LIKE ? OR summary LIKE ?", kw, kw)
	}
	var total int64
	_ = db.Count(&total).Error
	var list []model.AffairsArticle
	err := db.Order("published_at DESC").Offset((q.Page-1)*q.PageSize).Limit(q.PageSize).Find(&list).Error
	return &pkg.PageResult{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, err
}

func (s *AffairsService) Get(id uint64, userID uint64, adminView bool) (*model.AffairsArticle, error) {
	var a model.AffairsArticle
	db := s.db
	if !adminView {
		db = db.Where("status = ?", model.StatusPublished)
	}
	if err := db.First(&a, id).Error; err != nil {
		return nil, err
	}
	if userID > 0 {
		_ = s.db.Create(&model.AffairsRead{UserID: userID, ArticleID: id, ReadAt: time.Now()}).Error
	}
	return &a, nil
}

func (s *AffairsService) Create(a *model.AffairsArticle) error {
	if a.Status == "" {
		a.Status = model.StatusPublished
	}
	if a.PublishedAt.IsZero() {
		a.PublishedAt = time.Now()
	}
	return s.db.Create(a).Error
}

func (s *AffairsService) Update(id uint64, updates map[string]interface{}) (*model.AffairsArticle, error) {
	if err := s.db.Model(&model.AffairsArticle{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	var a model.AffairsArticle
	if err := s.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *AffairsService) Favorite(userID, articleID uint64) error {
	var a model.AffairsArticle
	if err := s.db.Where("status = ?", model.StatusPublished).First(&a, articleID).Error; err != nil {
		return err
	}
	var fav model.AffairsFavorite
	err := s.db.Where("user_id = ? AND article_id = ?", userID, articleID).First(&fav).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.AffairsFavorite{UserID: userID, ArticleID: articleID}).Error
}

func (s *AffairsService) Unfavorite(userID, articleID uint64) error {
	return s.db.Where("user_id = ? AND article_id = ?", userID, articleID).Delete(&model.AffairsFavorite{}).Error
}

func (s *AffairsService) ListFavorites(userID uint64, page, pageSize int) (*pkg.PageResult, error) {
	db := s.db.Model(&model.AffairsFavorite{}).Where("user_id = ?", userID)
	var total int64
	_ = db.Count(&total).Error
	var list []model.AffairsFavorite
	err := db.Preload("Article").Order("id DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&list).Error
	return &pkg.PageResult{List: list, Total: total, Page: page, PageSize: pageSize}, err
}

func (s *AffairsService) UpsertFromCrawl(a *model.AffairsArticle) error {
	if a.SourceURL == "" {
		return s.db.Create(a).Error
	}
	var existing model.AffairsArticle
	err := s.db.Where("source_url = ?", a.SourceURL).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.Create(a).Error
	}
	if err != nil {
		return err
	}
	return s.db.Model(&existing).Updates(map[string]interface{}{
		"title": a.Title, "summary": a.Summary, "tags": a.Tags, "published_at": a.PublishedAt,
	}).Error
}

func (s *AffairsService) SeedIfEmpty() error {
	var n int64
	s.db.Model(&model.AffairsArticle{}).Count(&n)
	if n > 0 {
		return nil
	}
	now := time.Now()
	items := []model.AffairsArticle{
		{Title: "中央经济工作会议召开", Source: "seed", Summary: "会议分析当前经济形势，部署明年经济工作。", Content: "会议强调要坚持稳中求进工作总基调，完整准确全面贯彻新发展理念。", Tags: "经济,会议", PublishedAt: now, Status: model.StatusPublished},
		{Title: "国务院常务会议部署重点工作", Source: "seed", Summary: "会议研究部署若干民生与发展举措。", Content: "会议指出要持续推动高质量发展，切实保障和改善民生。", Tags: "国务院,民生", PublishedAt: now.Add(-24 * time.Hour), Status: model.StatusPublished},
		{Title: "多地发布促就业政策举措", Source: "seed", Summary: "各地出台稳岗扩就业支持政策。", Content: "政策涵盖技能培训、创业补贴、岗位对接等内容。", Tags: "就业,地方", PublishedAt: now.Add(-48 * time.Hour), Status: model.StatusPublished},
	}
	for i := range items {
		if err := s.db.Create(&items[i]).Error; err != nil {
			return err
		}
	}
	return nil
}