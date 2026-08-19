package router

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupJunhuoLinkRouterTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.LogConsumeEnabled = true
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Token{}, &model.Log{},
		&model.JunhuoLinkUserMapping{}, &model.JunhuoLinkDeviceKey{}, &model.JunhuoLinkUsageReceipt{},
	))
	t.Setenv(middleware.JunhuoLinkInternalSecretEnv, "route-test-secret")
	router := gin.New()
	SetJunhuoLinkInternalRouter(router)
	return router
}

func performJunhuoLinkRequest(router http.Handler, method string, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if len(body) > 0 {
		request.Header.Set("content-type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeJunhuoLinkResponse(t *testing.T, response *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var value map[string]interface{}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &value))
	return value
}

func TestJunhuoLinkInternalRoutesEnforceTrustedMappingIdempotencyScopeUsageAndRevocation(t *testing.T) {
	router := setupJunhuoLinkRouterTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 901, Username: "router_link_user", Status: common.UserStatusEnabled,
		Quota: 5000, AffCode: "jl901",
	}).Error)

	unauthorized := performJunhuoLinkRequest(router, http.MethodPut, "/v1/internal/junhuo-link/users/station-user-901/mapping", []byte(`{"newApiUserId":901}`), nil)
	assert.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	serviceHeaders := map[string]string{
		"x-junhuo-link-management-secret": "route-test-secret",
		"x-junhuo-link-user-id":           "station-user-901",
	}
	mapped := performJunhuoLinkRequest(router, http.MethodPut, "/v1/internal/junhuo-link/users/station-user-901/mapping", []byte(`{"newApiUserId":901}`), serviceHeaders)
	assert.Equal(t, http.StatusOK, mapped.Code)

	issueBody := []byte(`{"userId":"station-user-901","deviceId":"device-901","purpose":"junhuo-link-codex"}`)
	issueHeaders := map[string]string{
		"x-junhuo-link-management-secret": "route-test-secret",
		"x-junhuo-link-user-id":           "station-user-901",
		"x-junhuo-link-device-id":         "device-901",
	}
	first := performJunhuoLinkRequest(router, http.MethodPost, "/v1/internal/junhuo-link/device-keys", issueBody, issueHeaders)
	assert.Equal(t, http.StatusCreated, first.Code)
	firstValue := decodeJunhuoLinkResponse(t, first)
	assert.Equal(t, true, firstValue["created"])
	rawKey, ok := firstValue["key"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, rawKey)
	tokenIdText, ok := firstValue["tokenId"].(string)
	require.True(t, ok)
	tokenId, err := strconv.Atoi(tokenIdText)
	require.NoError(t, err)

	second := performJunhuoLinkRequest(router, http.MethodPost, "/v1/internal/junhuo-link/device-keys", issueBody, issueHeaders)
	assert.Equal(t, http.StatusOK, second.Code)
	secondValue := decodeJunhuoLinkResponse(t, second)
	assert.Equal(t, false, secondValue["created"])
	_, leakedAgain := secondValue["key"]
	assert.False(t, leakedAgain, "an idempotent repeat must never return the original key again")
	assert.Equal(t, tokenIdText, secondValue["tokenId"])

	scope := map[string]string{
		"x-junhuo-link-management-secret": "route-test-secret",
		"x-junhuo-link-user-id":           "station-user-901",
		"x-junhuo-link-device-id":         "device-901",
	}
	status := performJunhuoLinkRequest(router, http.MethodGet, fmt.Sprintf("/v1/internal/junhuo-link/device-keys/%d", tokenId), nil, scope)
	assert.Equal(t, http.StatusOK, status.Code)
	assert.Equal(t, true, decodeJunhuoLinkResponse(t, status)["active"])

	wrongScope := map[string]string{
		"x-junhuo-link-management-secret": "route-test-secret",
		"x-junhuo-link-user-id":           "station-user-other",
		"x-junhuo-link-device-id":         "device-901",
	}
	crossUser := performJunhuoLinkRequest(router, http.MethodGet, fmt.Sprintf("/v1/internal/junhuo-link/device-keys/%d", tokenId), nil, wrongScope)
	assert.Equal(t, http.StatusNotFound, crossUser.Code)

	beforeQuota, err := model.GetUserQuota(901, true)
	require.NoError(t, err)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 901, CreatedAt: model.JunhuoLinkNow(), Type: model.LogTypeConsume,
		PromptTokens: 13, CompletionTokens: 5, Quota: 88,
		TokenId: tokenId, RequestId: "newapi-route-request-1",
	}).Error)
	usageBody := []byte(`{"usageId":"turn-route-1","taskId":"task-route-1","inputTokens":13,"outputTokens":5,"usedQuota":999999}`)
	usage := performJunhuoLinkRequest(router, http.MethodPost, fmt.Sprintf("/v1/internal/junhuo-link/device-keys/%d/usage", tokenId), usageBody, scope)
	assert.Equal(t, http.StatusOK, usage.Code)
	usageValue := decodeJunhuoLinkResponse(t, usage)
	assert.EqualValues(t, 88, usageValue["usedQuota"], "server must use authoritative log quota, not client supplied quota")
	assert.Equal(t, false, usageValue["duplicate"])

	duplicate := performJunhuoLinkRequest(router, http.MethodPost, fmt.Sprintf("/v1/internal/junhuo-link/device-keys/%d/usage", tokenId), usageBody, scope)
	assert.Equal(t, http.StatusOK, duplicate.Code)
	assert.Equal(t, true, decodeJunhuoLinkResponse(t, duplicate)["duplicate"])
	afterQuota, err := model.GetUserQuota(901, true)
	require.NoError(t, err)
	assert.Equal(t, beforeQuota, afterQuota, "usage receipt must not charge the user again")

	revoked := performJunhuoLinkRequest(router, http.MethodDelete, fmt.Sprintf("/v1/internal/junhuo-link/device-keys/%d", tokenId), nil, scope)
	assert.Equal(t, http.StatusNoContent, revoked.Code)
	statusAfter := performJunhuoLinkRequest(router, http.MethodGet, fmt.Sprintf("/v1/internal/junhuo-link/device-keys/%d", tokenId), nil, scope)
	assert.Equal(t, http.StatusOK, statusAfter.Code)
	assert.Equal(t, false, decodeJunhuoLinkResponse(t, statusAfter)["active"])

	var storedToken model.Token
	require.NoError(t, model.DB.Unscoped().Where("id = ?", tokenId).First(&storedToken).Error)
	assert.Equal(t, common.TokenStatusDisabled, storedToken.Status)
	assert.NotEmpty(t, storedToken.Key, "the authoritative existing NewAPI token table remains the sole key store")

	for _, table := range []string{"junhuo_link_user_mappings", "junhuo_link_device_keys", "junhuo_link_usage_receipts"} {
		var columns []struct{ Name string }
		require.NoError(t, model.DB.Raw("PRAGMA table_info("+table+")").Scan(&columns).Error)
		for _, column := range columns {
			assert.NotEqual(t, "key", column.Name)
			assert.NotEqual(t, "raw_key", column.Name)
		}
	}
}
