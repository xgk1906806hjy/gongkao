package middleware

import (
	"strings"

	"gongkao/internal/pkg"

	"github.com/gin-gonic/gin"
)

const CtxUserID = "user_id"
const CtxTokenType = "token_type"

func Auth(jwtm *pkg.JWTManager, requireType pkg.TokenType) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" || !strings.HasPrefix(h, "Bearer ") {
			pkg.Unauthorized(c, "missing token")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		claims, err := jwtm.Parse(token)
		if err != nil {
			pkg.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}
		if requireType != "" && claims.TokenType != requireType {
			pkg.Unauthorized(c, "invalid token type")
			c.Abort()
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxTokenType, string(claims.TokenType))
		c.Next()
	}
}

func UserID(c *gin.Context) uint64 {
	v, _ := c.Get(CtxUserID)
	id, _ := v.(uint64)
	return id
}

// OptionalAuth parses Bearer token if present; does not reject missing token.
func OptionalAuth(jwtm *pkg.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if claims, err := jwtm.Parse(token); err == nil && claims.TokenType == pkg.TokenAccess {
				c.Set(CtxUserID, claims.UserID)
				c.Set(CtxTokenType, string(claims.TokenType))
			}
		}
		c.Next()
	}
}