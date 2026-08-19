package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const JunhuoLinkInternalSecretEnv = "JUNHUO_LINK_INTERNAL_SECRET"

func JunhuoLinkInternalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := strings.TrimSpace(os.Getenv(JunhuoLinkInternalSecretEnv))
		if expected == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "junhuo_link_internal_auth_not_configured"})
			c.Abort()
			return
		}
		provided := strings.TrimSpace(c.GetHeader("x-junhuo-link-management-secret"))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "junhuo_link_internal_authentication_required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
