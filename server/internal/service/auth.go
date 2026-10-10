package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"gongkao/internal/config"
	"gongkao/internal/model"
	"gongkao/internal/pkg"
	"gongkao/internal/wx"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db   *gorm.DB
	wx   *wx.Client
	jwt  *pkg.JWTManager
	cfg  *config.Config
}

func NewAuthService(db *gorm.DB, wxCli *wx.Client, jwtm *pkg.JWTManager, cfg *config.Config) *AuthService {
	return &AuthService{db: db, wx: wxCli, jwt: jwtm, cfg: cfg}
}

type LoginResult struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         *model.User `json:"user"`
}

func (s *AuthService) WechatLogin(ctx context.Context, code, nickname, avatar string) (*LoginResult, error) {
	sess, err := s.wx.Code2Session(ctx, code)
	if err != nil {
		return nil, err
	}
	var user model.User
	err = s.db.Where("open_id = ?", sess.OpenID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = model.User{
			OpenID:    sess.OpenID,
			UnionID:   sess.UnionID,
			Nickname:  nickname,
			AvatarURL: avatar,
		}
		if user.Nickname == "" {
			user.Nickname = "考生" + sess.OpenID[len(sess.OpenID)-4:]
		}
		enc, _ := encryptSessionKey(s.cfg.SessionKeyAESKey, sess.SessionKey)
		user.SessionKeyEnc = enc
		if err := s.db.Create(&user).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		enc, _ := encryptSessionKey(s.cfg.SessionKeyAESKey, sess.SessionKey)
		updates := map[string]interface{}{"session_key_enc": enc}
		if nickname != "" {
			updates["nickname"] = nickname
		}
		if avatar != "" {
			updates["avatar_url"] = avatar
		}
		if sess.UnionID != "" {
			updates["union_id"] = sess.UnionID
		}
		_ = s.db.Model(&user).Updates(updates).Error
		_ = s.db.First(&user, user.ID).Error
	}
	access, refresh, err := s.jwt.IssueUser(user.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{AccessToken: access, RefreshToken: refresh, User: &user}, nil
}

func (s *AuthService) Refresh(refreshToken string) (access, refresh string, err error) {
	claims, err := s.jwt.Parse(refreshToken)
	if err != nil || claims.TokenType != pkg.TokenRefresh {
		return "", "", errors.New("invalid refresh token")
	}
	return s.jwt.IssueUser(claims.UserID)
}

func (s *AuthService) GetUser(id uint64) (*model.User, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *AuthService) UpdateProfile(id uint64, nickname, avatar string) (*model.User, error) {
	updates := map[string]interface{}{}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if avatar != "" {
		updates["avatar_url"] = avatar
	}
	if len(updates) > 0 {
		if err := s.db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetUser(id)
}

func (s *AuthService) EnsureAdmin() error {
	var n int64
	s.db.Model(&model.AdminUser{}).Count(&n)
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.cfg.AdminBootstrapPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Create(&model.AdminUser{
		Username:     s.cfg.AdminBootstrapUser,
		PasswordHash: string(hash),
		Role:         "admin",
	}).Error
}

func (s *AuthService) AdminLogin(username, password string) (string, error) {
	var admin model.AdminUser
	if err := s.db.Where("username = ?", username).First(&admin).Error; err != nil {
		return "", errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return "", errors.New("invalid credentials")
	}
	return s.jwt.IssueAdmin(admin.ID)
}

func encryptSessionKey(keyStr, plain string) (string, error) {
	key := []byte(keyStr)
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		key = []byte("0123456789abcdef0123456789abcdef")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}