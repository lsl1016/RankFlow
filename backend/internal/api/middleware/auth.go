package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"rankflow/internal/dto"
)

const AuthRoleKey = "authRole"

const (
	RoleAdmin  = "admin"
	RoleWriter = "writer"
)

type AuthConfig struct {
	Enabled     bool
	AdminToken  string
	WriterToken string
}

func secureTokenEqual(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func bearerToken(c *gin.Context) (string, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return "", false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func authenticate(c *gin.Context, cfg AuthConfig) (string, bool) {
	if !cfg.Enabled {
		c.Set(AuthRoleKey, RoleAdmin)
		return RoleAdmin, true
	}
	token, ok := bearerToken(c)
	if !ok {
		return "", false
	}
	if secureTokenEqual(token, cfg.AdminToken) {
		c.Set(AuthRoleKey, RoleAdmin)
		return RoleAdmin, true
	}
	if secureTokenEqual(token, cfg.WriterToken) {
		c.Set(AuthRoleKey, RoleWriter)
		return RoleWriter, true
	}
	return "", false
}

func abortUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer realm="rankflow"`)
	c.AbortWithStatusJSON(http.StatusUnauthorized, dto.Fail(dto.CodeUnauthorized, "valid bearer token required"))
}

// RequireAdmin permits only the administrator token. A valid writer token gets
// 403 instead of 401 so callers can distinguish authentication from privilege.
func RequireAdmin(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := authenticate(c, cfg)
		if !ok {
			abortUnauthorized(c)
			return
		}
		if role != RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.Fail(dto.CodeForbidden, "admin permission required"))
			return
		}
		c.Next()
	}
}

// RequireWriter permits both writer and administrator tokens. Administrator is
// a strict superset of writer permissions.
func RequireWriter(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := authenticate(c, cfg)
		if !ok {
			abortUnauthorized(c)
			return
		}
		if role != RoleWriter && role != RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, dto.Fail(dto.CodeForbidden, "writer permission required"))
			return
		}
		c.Next()
	}
}
