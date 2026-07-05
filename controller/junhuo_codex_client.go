package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type junhuoCodexClientLoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	DeviceId string `json:"deviceId"`
}

type junhuoCodexEntitlementVerifyRequest struct {
	Account            string                 `json:"account"`
	ClientSessionToken string                 `json:"clientSessionToken"`
	DeviceId           string                 `json:"deviceId"`
	Device             map[string]string      `json:"device"`
	App                map[string]interface{} `json:"app"`
}

func JunhuoCodexClientLogin(c *gin.Context) {
	var req junhuoCodexClientLoginRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		junhuoCodexClientError(c, http.StatusBadRequest, "invalid_request", "请求参数错误。")
		return
	}
	account := strings.TrimSpace(req.Account)
	password := strings.TrimSpace(req.Password)
	if account == "" || password == "" {
		junhuoCodexClientError(c, http.StatusBadRequest, "auth_required", "账号和密码不能为空。")
		return
	}
	user := model.User{Username: account, Password: password}
	if err := user.ValidateAndFill(); err != nil {
		junhuoCodexClientError(c, http.StatusUnauthorized, "invalid_credentials", "拼车站账号或密码不正确。")
		return
	}
	deviceId := strings.TrimSpace(req.DeviceId)
	if deviceId == "" {
		deviceId = "device_" + uuid.NewString()
	}
	sessionToken, err := service.GenerateClientSessionToken()
	if err != nil {
		common.SysError("failed to generate junhuo codex client session token: " + err.Error())
		junhuoCodexClientError(c, http.StatusInternalServerError, "activation_failed", "登录服务生成凭证失败。")
		return
	}
	now := common.GetTimestamp()
	session, err := model.CreateClientSession(user.Id, deviceId, sessionToken, now+int64(service.JunhuoCodexClientSessionTTLSeconds()))
	if err != nil {
		common.SysError("failed to create junhuo codex client session: " + err.Error())
		junhuoCodexClientError(c, http.StatusInternalServerError, "activation_failed", "登录服务保存凭证失败。")
		return
	}
	entitled, features, err := model.HasJunhuoCodexEntitlement(&user, service.JunhuoCodexEntitlementGroups(), now)
	if err != nil {
		common.SysError("failed to check junhuo codex entitlement: " + err.Error())
		junhuoCodexClientError(c, http.StatusInternalServerError, "activation_failed", "权益检查失败。")
		return
	}
	status := "plan_not_supported"
	if entitled {
		status = "active"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"account":            user.Username,
		"displayName":        user.DisplayName,
		"userId":             strconv.Itoa(user.Id),
		"deviceId":           deviceId,
		"clientSessionId":    session.Id,
		"clientSessionToken": sessionToken,
		"activationStatus":   status,
		"entitlements":       features,
		"expiresAt":          timestampToRFC3339(session.ExpiresAt),
	})
}

func JunhuoCodexEntitlementVerify(c *gin.Context) {
	var req junhuoCodexEntitlementVerifyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		junhuoCodexClientError(c, http.StatusBadRequest, "invalid_request", "请求参数错误。")
		return
	}
	now := common.GetTimestamp()
	session, err := model.GetValidClientSession(req.ClientSessionToken, req.DeviceId, now)
	if err != nil {
		junhuoCodexClientError(c, http.StatusUnauthorized, "auth_required", "登录凭证无效或已过期。")
		return
	}
	user, err := model.GetUserById(session.UserId, true)
	if err != nil {
		junhuoCodexClientError(c, http.StatusUnauthorized, "auth_required", "账号不存在。")
		return
	}
	if !accountMatchesUser(req.Account, user) {
		junhuoCodexClientError(c, http.StatusForbidden, "activation_revoked", "账号与登录凭证不匹配。")
		return
	}
	entitled, features, err := model.HasJunhuoCodexEntitlement(user, service.JunhuoCodexEntitlementGroups(), now)
	if err != nil {
		common.SysError("failed to check junhuo codex entitlement: " + err.Error())
		junhuoCodexClientError(c, http.StatusInternalServerError, "activation_failed", "权益检查失败。")
		return
	}
	if !entitled {
		junhuoCodexClientError(c, http.StatusForbidden, "plan_not_supported", "当前账号没有 Codex-claw 工具权益。")
		return
	}
	appVersion := ""
	if value, ok := req.App["version"].(string); ok {
		appVersion = value
	}
	exp := now + int64(service.JunhuoCodexEntitlementTTLSeconds())
	token, err := service.SignJunhuoCodexEntitlement(service.JunhuoCodexEntitlementClaims{
		Issuer:     strings.TrimRight(c.Request.Host, "/"),
		Audience:   service.JunhuoCodexAudience,
		Subject:    strconv.Itoa(user.Id),
		Account:    user.Username,
		DeviceId:   session.DeviceId,
		SessionId:  session.Id,
		Features:   features,
		AppVersion: appVersion,
		IssuedAt:   now,
		NotBefore:  now - 5,
		ExpiresAt:  exp,
		Jti:        uuid.NewString(),
	})
	if err != nil {
		common.SysError("failed to sign junhuo codex entitlement: " + err.Error())
		junhuoCodexClientError(c, http.StatusInternalServerError, "activation_failed", "权益签发失败。")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":           "active",
		"account":          user.Username,
		"displayName":      user.DisplayName,
		"userId":           strconv.Itoa(user.Id),
		"deviceId":         session.DeviceId,
		"activationId":     session.Id,
		"activationToken":  token,
		"entitlementToken": token,
		"expiresAt":        timestampToRFC3339(exp),
		"features":         features,
	})
}

func accountMatchesUser(account string, user *model.User) bool {
	account = strings.TrimSpace(account)
	if account == "" {
		return true
	}
	return account == user.Username || account == user.Email
}

func timestampToRFC3339(timestamp int64) string {
	return time.Unix(timestamp, 0).UTC().Format(time.RFC3339)
}

func junhuoCodexClientError(c *gin.Context, status int, kind string, message string) {
	c.JSON(status, gin.H{
		"status":       kind,
		"errorKind":    kind,
		"error":        message,
		"errorMessage": message,
	})
}
