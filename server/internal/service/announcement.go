package service

import (
	"strings"
	"time"

	"gongkao/internal/model"
	"gongkao/internal/pkg"

	"gorm.io/gorm"
)

type AnnouncementService struct {
	db *gorm.DB
}

func NewAnnouncementService(db *gorm.DB) *AnnouncementService {
	return &AnnouncementService{db: db}
}

type AnnouncementQuery struct {
	Type      string
	Region    string
	Keyword   string
	Page      int
	PageSize  int
	AdminView bool
}

func (s *AnnouncementService) List(q AnnouncementQuery) (*pkg.PageResult, error) {
	db := s.db.Model(&model.Announcement{})
	if !q.AdminView {
		db = db.Where("status = ?", model.StatusPublished)
	}
	if q.Type != "" {
		db = db.Where("type = ?", q.Type)
	}
	if q.Region != "" {
		db = db.Where("region = ?", q.Region)
	}
	if q.Keyword != "" {
		kw := "%" + strings.TrimSpace(q.Keyword) + "%"
		db = db.Where("title ILIKE ? OR summary ILIKE ?", kw, kw)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.Announcement
	offset := (q.Page - 1) * q.PageSize
	if err := db.Order("published_at DESC").Offset(offset).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &pkg.PageResult{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *AnnouncementService) Get(id uint64, adminView bool) (*model.Announcement, error) {
	var a model.Announcement
	db := s.db
	if !adminView {
		db = db.Where("status = ?", model.StatusPublished)
	}
	if err := db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *AnnouncementService) Create(a *model.Announcement) error {
	if a.Status == "" {
		a.Status = model.StatusPublished
	}
	if a.PublishedAt.IsZero() {
		a.PublishedAt = time.Now()
	}
	return s.db.Create(a).Error
}

func (s *AnnouncementService) Update(id uint64, updates map[string]interface{}) (*model.Announcement, error) {
	if err := s.db.Model(&model.Announcement{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.Get(id, true)
}

func (s *AnnouncementService) UpsertFromCrawl(a *model.Announcement) error {
	if a.Source == "" || a.SourceID == "" {
		return s.db.Create(a).Error
	}
	var existing model.Announcement
	err := s.db.Where("source = ? AND source_id = ?", a.Source, a.SourceID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.Create(a).Error
	}
	if err != nil {
		return err
	}
	return s.db.Model(&existing).Updates(map[string]interface{}{
		"title":          a.Title,
		"region":         a.Region,
		"source_url":     a.SourceURL,
		"summary":        a.Summary,
		"register_start": a.RegisterStart,
		"register_end":   a.RegisterEnd,
		"confirm_start":  a.ConfirmStart,
		"confirm_end":    a.ConfirmEnd,
		"exam_at":        a.ExamAt,
		"published_at":   a.PublishedAt,
	}).Error
}