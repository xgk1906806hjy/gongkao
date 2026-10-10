package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID            uint64         `gorm:"primaryKey" json:"id"`
	OpenID        string         `gorm:"size:64;uniqueIndex;not null" json:"openid"`
	UnionID       string         `gorm:"size:64;index" json:"unionid,omitempty"`
	Nickname      string         `gorm:"size:64" json:"nickname"`
	AvatarURL     string         `gorm:"size:512" json:"avatar_url"`
	SessionKeyEnc string         `gorm:"size:512" json:"-"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type AdminUser struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:32;default:admin" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	AnnouncementCivil       = "civil"
	AnnouncementInstitution = "institution"

	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusOffline   = "offline"
)

type Announcement struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	Type         string         `gorm:"size:32;index;not null" json:"type"`
	Title        string         `gorm:"size:256;not null" json:"title"`
	Region       string         `gorm:"size:64;index" json:"region"`
	Source       string         `gorm:"size:64;index" json:"source"`
	SourceID     string         `gorm:"size:128" json:"source_id"`
	SourceURL    string         `gorm:"size:512" json:"source_url"`
	Summary      string         `gorm:"type:text" json:"summary"`
	Content      string         `gorm:"type:text" json:"content"`
	RegisterStart *time.Time    `json:"register_start"`
	RegisterEnd   *time.Time    `json:"register_end"`
	ConfirmStart  *time.Time    `json:"confirm_start"`
	ConfirmEnd    *time.Time    `json:"confirm_end"`
	ExamAt        *time.Time    `json:"exam_at"`
	PublishedAt  time.Time      `gorm:"index" json:"published_at"`
	Status       string         `gorm:"size:32;index;default:published" json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Announcement) TableName() string { return "announcements" }

type UserExam struct {
	ID             uint64        `gorm:"primaryKey" json:"id"`
	UserID         uint64        `gorm:"uniqueIndex:idx_user_exams_user_ann;not null" json:"user_id"`
	AnnouncementID uint64        `gorm:"uniqueIndex:idx_user_exams_user_ann;not null" json:"announcement_id"`
	CreatedAt      time.Time     `json:"created_at"`
	Announcement   *Announcement `gorm:"foreignKey:AnnouncementID" json:"announcement,omitempty"`
}

func (UserExam) TableName() string { return "user_exams" }

const (
	TodoRegister  = "register"
	TodoConfirm   = "confirm"
	TodoExam      = "exam"
	TodoCustom    = "custom"

	TodoPending   = "pending"
	TodoDone      = "done"
	TodoCancelled = "cancelled"

	RemindPending = "pending"
	RemindSent    = "sent"
	RemindSkipped = "skipped"
	RemindFailed  = "failed"
)

type Todo struct {
	ID             uint64         `gorm:"primaryKey" json:"id"`
	UserID         uint64         `gorm:"index;not null" json:"user_id"`
	UserExamID     uint64         `gorm:"index;not null" json:"user_exam_id"`
	AnnouncementID uint64         `gorm:"index;not null" json:"announcement_id"`
	Type           string         `gorm:"size:32;not null" json:"type"`
	Title          string         `gorm:"size:128;not null" json:"title"`
	DueAt          time.Time      `gorm:"index;not null" json:"due_at"`
	RemindAt       time.Time      `gorm:"index;not null" json:"remind_at"`
	Status         string         `gorm:"size:32;index;default:pending" json:"status"`
	RemindStatus   string         `gorm:"size:32;index;default:pending" json:"remind_status"`
	RemindRetry    int            `gorm:"default:0" json:"remind_retry"`
	LastRemindErr  string         `gorm:"type:text" json:"last_remind_error,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Announcement   *Announcement  `gorm:"foreignKey:AnnouncementID" json:"announcement,omitempty"`
}

type QuestionCategory struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Sort      int       `gorm:"default:0" json:"sort"`
	Status    string    `gorm:"size:32;default:published" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Question struct {
	ID         uint64            `gorm:"primaryKey" json:"id"`
	CategoryID uint64            `gorm:"index;not null" json:"category_id"`
	Stem       string            `gorm:"type:text;not null" json:"stem"`
	Type       string            `gorm:"size:32;default:single" json:"type"`
	AnswerKey  string            `gorm:"size:8;not null" json:"answer_key"`
	Analysis   string            `gorm:"type:text" json:"analysis"`
	Difficulty int               `gorm:"default:1" json:"difficulty"`
	Tags       string            `gorm:"size:256" json:"tags"`
	Status     string            `gorm:"size:32;default:published" json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Options    []QuestionOption  `gorm:"foreignKey:QuestionID" json:"options,omitempty"`
}

type QuestionOption struct {
	ID         uint64 `gorm:"primaryKey" json:"id"`
	QuestionID uint64 `gorm:"index;not null" json:"question_id"`
	Key        string `gorm:"size:8;not null" json:"key"`
	Content    string `gorm:"type:text;not null" json:"content"`
	Sort       int    `gorm:"default:0" json:"sort"`
}

type PracticeSession struct {
	ID         uint64     `gorm:"primaryKey" json:"id"`
	UserID     uint64     `gorm:"index;not null" json:"user_id"`
	CategoryID uint64     `gorm:"index" json:"category_id"`
	Mode       string     `gorm:"size:32;default:sequential" json:"mode"`
	Total      int        `gorm:"default:0" json:"total"`
	Correct    int        `gorm:"default:0" json:"correct"`
	Status     string     `gorm:"size:32;default:ongoing" json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type UserAnswer struct {
	ID         uint64 `gorm:"primaryKey" json:"id"`
	SessionID  uint64 `gorm:"index;not null" json:"session_id"`
	QuestionID uint64 `gorm:"index;not null" json:"question_id"`
	UserKey    string `gorm:"size:8" json:"user_key"`
	IsCorrect  bool   `json:"is_correct"`
}

type WrongQuestion struct {
	ID          uint64     `gorm:"primaryKey" json:"id"`
	UserID      uint64     `gorm:"uniqueIndex:idx_user_q;not null" json:"user_id"`
	QuestionID  uint64     `gorm:"uniqueIndex:idx_user_q;not null" json:"question_id"`
	WrongCount  int        `gorm:"default:1" json:"wrong_count"`
	LastWrongAt time.Time  `json:"last_wrong_at"`
	MasteredAt  *time.Time `json:"mastered_at"`
	Question    *Question  `gorm:"foreignKey:QuestionID" json:"question,omitempty"`
}

type AffairsArticle struct {
	ID          uint64         `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"size:256;not null" json:"title"`
	Source      string         `gorm:"size:64" json:"source"`
	SourceURL   string         `gorm:"size:512" json:"source_url"`
	Summary     string         `gorm:"type:text" json:"summary"`
	Content     string         `gorm:"type:text" json:"content"`
	Tags        string         `gorm:"size:256;index" json:"tags"`
	PublishedAt time.Time      `gorm:"index" json:"published_at"`
	Status      string         `gorm:"size:32;index;default:published" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type AffairsFavorite struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex:idx_fav;not null" json:"user_id"`
	ArticleID uint64    `gorm:"uniqueIndex:idx_fav;not null" json:"article_id"`
	CreatedAt time.Time `json:"created_at"`
	Article   *AffairsArticle `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
}

type AffairsRead struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `gorm:"index;not null" json:"user_id"`
	ArticleID uint64    `gorm:"index;not null" json:"article_id"`
	ReadAt    time.Time `json:"read_at"`
}

type CrawlFailure struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Source    string    `gorm:"size:64;index" json:"source"`
	URL       string    `gorm:"size:512" json:"url"`
	Error     string    `gorm:"type:text" json:"error"`
	Payload   string    `gorm:"type:text" json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

func AllModels() []interface{} {
	return []interface{}{
		&User{}, &AdminUser{}, &Announcement{}, &UserExam{}, &Todo{},
		&QuestionCategory{}, &Question{}, &QuestionOption{},
		&PracticeSession{}, &UserAnswer{}, &WrongQuestion{},
		&AffairsArticle{}, &AffairsFavorite{}, &AffairsRead{},
		&CrawlFailure{},
	}
}