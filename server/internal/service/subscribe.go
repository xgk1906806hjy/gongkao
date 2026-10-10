package service

import (
	"errors"
	"fmt"
	"time"

	"gongkao/internal/model"

	"gorm.io/gorm"
)

type SubscribeService struct {
	db *gorm.DB
}

func NewSubscribeService(db *gorm.DB) *SubscribeService {
	return &SubscribeService{db: db}
}

type SubscribePrefs struct {
	Todo    string `json:"todo"`    // accept | reject | ""
	Affairs string `json:"affairs"` // accept | reject | ""
}

func (s *SubscribeService) GetPrefs(userID uint64) (*SubscribePrefs, error) {
	var rows []model.UserSubscribePref
	if err := s.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := &SubscribePrefs{}
	for _, r := range rows {
		switch r.Kind {
		case model.SubscribeKindTodo:
			out.Todo = r.Status
		case model.SubscribeKindAffairs:
			out.Affairs = r.Status
		}
	}
	return out, nil
}

func (s *SubscribeService) UpsertPrefs(userID uint64, prefs SubscribePrefs) (*SubscribePrefs, error) {
	if err := s.upsertOne(userID, model.SubscribeKindTodo, prefs.Todo); err != nil {
		return nil, err
	}
	if err := s.upsertOne(userID, model.SubscribeKindAffairs, prefs.Affairs); err != nil {
		return nil, err
	}
	return s.GetPrefs(userID)
}

func (s *SubscribeService) upsertOne(userID uint64, kind, status string) error {
	if status == "" {
		return nil
	}
	if status != model.SubscribeAccept && status != model.SubscribeReject {
		return fmt.Errorf("invalid status for %s", kind)
	}
	var pref model.UserSubscribePref
	err := s.db.Where("user_id = ? AND kind = ?", userID, kind).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.Create(&model.UserSubscribePref{
			UserID: userID, Kind: kind, Status: status, UpdatedAt: time.Now(),
		}).Error
	}
	if err != nil {
		return err
	}
	return s.db.Model(&pref).Updates(map[string]interface{}{
		"status": status, "updated_at": time.Now(),
	}).Error
}

func (s *SubscribeService) HasAccept(userID uint64, kind string) bool {
	var pref model.UserSubscribePref
	err := s.db.Where("user_id = ? AND kind = ? AND status = ?", userID, kind, model.SubscribeAccept).First(&pref).Error
	return err == nil
}
