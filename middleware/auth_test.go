package middleware

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAuthMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("failed to migrate auth middleware tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestSetupContextForTokenSetsSpecificChannelContextKeyForAdmin(t *testing.T) {
	db := setupAuthMiddlewareTestDB(t)

	admin := &model.User{
		Id:     1,
		Role:   common.RoleAdminUser,
		Status: common.UserStatusEnabled,
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	token := &model.Token{
		UserId:         admin.Id,
		Key:            "test-token",
		Name:           "admin-token",
		Status:         common.TokenStatusEnabled,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	ctx, _ := gin.CreateTestContext(nil)
	if err := SetupContextForToken(ctx, token, token.Key, "5"); err != nil {
		t.Fatalf("SetupContextForToken returned error: %v", err)
	}

	if got := common.GetContextKeyString(ctx, constant.ContextKeyTokenSpecificChannelId); got != "5" {
		t.Fatalf("expected specific channel context key to be 5, got %q", got)
	}
}
