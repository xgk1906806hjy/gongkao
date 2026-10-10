package pkg

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TokenAccess  TokenType = "access"
	TokenRefresh TokenType = "refresh"
	TokenAdmin   TokenType = "admin"
)

type Claims struct {
	UserID    uint64    `json:"uid"`
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *JWTManager) IssueUser(userID uint64) (access, refresh string, err error) {
	access, err = m.sign(userID, TokenAccess, m.accessTTL)
	if err != nil {
		return
	}
	refresh, err = m.sign(userID, TokenRefresh, m.refreshTTL)
	return
}

func (m *JWTManager) IssueAdmin(adminID uint64) (string, error) {
	return m.sign(adminID, TokenAdmin, m.accessTTL)
}

func (m *JWTManager) sign(uid uint64, typ TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    uid,
		TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}