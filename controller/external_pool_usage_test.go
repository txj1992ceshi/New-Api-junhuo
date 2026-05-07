package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClassifyExternalPoolActionError(t *testing.T) {
	cases := []struct {
		name            string
		err             error
		wantCode        string
		wantRecoverable bool
	}{
		{
			name:            "missing input",
			err:             errors.New("missing input"),
			wantCode:        "invalid_input",
			wantRecoverable: true,
		},
		{
			name:            "auth rejected",
			err:             errors.New("upstream status 401: unauthorized"),
			wantCode:        "auth_rejected",
			wantRecoverable: true,
		},
		{
			name:            "path missing",
			err:             errors.New("upstream status 404: not found"),
			wantCode:        "upstream_path_not_found",
			wantRecoverable: true,
		},
		{
			name:            "connectivity",
			err:             errors.New("dial tcp: connection refused"),
			wantCode:        "upstream_unreachable",
			wantRecoverable: true,
		},
		{
			name:            "server error",
			err:             errors.New("upstream status 502: bad gateway"),
			wantCode:        "upstream_server_error",
			wantRecoverable: true,
		},
		{
			name:            "fallback",
			err:             errors.New("weird failure"),
			wantCode:        "external_pool_action_failed",
			wantRecoverable: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCode, gotRecoverable := classifyExternalPoolActionError(tc.err)
			require.Equal(t, tc.wantCode, gotCode)
			require.Equal(t, tc.wantRecoverable, gotRecoverable)
		})
	}
}

func setupExternalPoolControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestCompleteCursorPoolAuthAllowsMissingInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupExternalPoolControllerTestDB(t)
	baseURL := "http://127.0.0.1:65534"
	channel := &model.Channel{
		Name:      "cursor-pool-proxy",
		Type:      1,
		Key:       "demo-cursor-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"cursor_pool_proxy":true}`,
	}
	require.NoError(t, db.Create(channel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", channel.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/channel/%d/cursor/auth/complete", channel.Id), bytes.NewBufferString(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CompleteCursorPoolAuth(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]interface{}
	err := common.Unmarshal(recorder.Body.Bytes(), &payload)
	require.NoError(t, err)
	require.Equal(t, false, payload["success"])
	require.NotEqual(t, "missing input", payload["message"])
}
