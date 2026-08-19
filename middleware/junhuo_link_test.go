package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestJunhuoLinkInternalAuthRequiresDedicatedServerSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(JunhuoLinkInternalSecretEnv, "server-only-secret")
	router := gin.New()
	router.GET("/internal", JunhuoLinkInternalAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/internal", nil))
	assert.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	wrong := httptest.NewRecorder()
	wrongRequest := httptest.NewRequest(http.MethodGet, "/internal", nil)
	wrongRequest.Header.Set("x-junhuo-link-management-secret", "wrong-secret")
	router.ServeHTTP(wrong, wrongRequest)
	assert.Equal(t, http.StatusUnauthorized, wrong.Code)

	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/internal", nil)
	allowedRequest.Header.Set("x-junhuo-link-management-secret", "server-only-secret")
	router.ServeHTTP(allowed, allowedRequest)
	assert.Equal(t, http.StatusNoContent, allowed.Code)
}

func TestJunhuoLinkInternalAuthFailsClosedWhenNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(JunhuoLinkInternalSecretEnv, "")
	router := gin.New()
	router.GET("/internal", JunhuoLinkInternalAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/internal", nil))
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
}
