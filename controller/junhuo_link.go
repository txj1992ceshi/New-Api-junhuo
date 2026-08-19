package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type junhuoLinkUserMappingRequest struct {
	NewAPIUserId int `json:"newApiUserId"`
}

type junhuoLinkIssueDeviceKeyRequest struct {
	UserId   string `json:"userId"`
	DeviceId string `json:"deviceId"`
	Purpose  string `json:"purpose"`
}

type junhuoLinkUsageReceiptRequest struct {
	UsageId      string `json:"usageId"`
	TaskId       string `json:"taskId"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
}

func GetJunhuoLinkInternalHealth(c *gin.Context) {
	sqlDB, err := model.DB.DB()
	if err != nil || sqlDB.Ping() != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "junhuo_link_database_unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "service": "junhuo-link-internal"})
}

func PutJunhuoLinkUserMapping(c *gin.Context) {
	linkUserId, ok := junhuoLinkScopedUserPath(c)
	if !ok {
		return
	}
	var request junhuoLinkUserMappingRequest
	if linkUserId == "" || c.ShouldBindJSON(&request) != nil || request.NewAPIUserId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "junhuo_link_invalid_mapping_request"})
		return
	}
	mapping, err := model.UpsertJunhuoLinkUserMapping(linkUserId, request.NewAPIUserId, model.JunhuoLinkNow())
	if err != nil {
		if errors.Is(err, model.ErrJunhuoLinkMappingConflict) {
			common.SysLog("security_event=junhuo_link_mapping_conflict link_user_id=" + linkUserId + " newapi_user_id=" + strconv.Itoa(request.NewAPIUserId))
		}
		writeJunhuoLinkError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"linkUserId":   mapping.LinkUserId,
		"newApiUserId": mapping.NewAPIUserId,
		"status":       mapping.Status,
	})
}

func DeleteJunhuoLinkUserMapping(c *gin.Context) {
	linkUserId, ok := junhuoLinkScopedUserPath(c)
	if !ok {
		return
	}
	if err := model.RevokeJunhuoLinkUserMapping(linkUserId, model.JunhuoLinkNow()); err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func PostJunhuoLinkDeviceKey(c *gin.Context) {
	var request junhuoLinkIssueDeviceKeyRequest
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "junhuo_link_invalid_device_key_request"})
		return
	}
	request.UserId = strings.TrimSpace(request.UserId)
	request.DeviceId = strings.TrimSpace(request.DeviceId)
	scopedUser := strings.TrimSpace(c.GetHeader("x-junhuo-link-user-id"))
	scopedDevice := strings.TrimSpace(c.GetHeader("x-junhuo-link-device-id"))
	if request.UserId == "" || request.DeviceId == "" || scopedUser != request.UserId || scopedDevice != request.DeviceId || (request.Purpose != "" && request.Purpose != "junhuo-link-codex") {
		if request.UserId != "" && request.DeviceId != "" && (scopedUser != request.UserId || scopedDevice != request.DeviceId) {
			common.SysLog("security_event=junhuo_link_scope_mismatch link_user_id=" + request.UserId + " device_id=" + request.DeviceId)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "junhuo_link_invalid_device_key_request"})
		return
	}
	storedKey, err := common.GenerateKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "junhuo_link_key_generation_failed"})
		return
	}
	exposedKey := "sk-" + storedKey
	fingerprint := junhuoLinkKeyFingerprint(exposedKey)
	issued, err := model.IssueJunhuoLinkDeviceKey(request.UserId, request.DeviceId, storedKey, fingerprint, model.JunhuoLinkNow())
	if err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	response := gin.H{
		"tokenId":        strconv.Itoa(issued.Binding.TokenId),
		"keyFingerprint": issued.Binding.KeyFingerprint,
		"created":        issued.Created,
	}
	if issued.Created {
		response["key"] = exposedKey
		c.JSON(http.StatusCreated, response)
		return
	}
	response["status"] = "already_provisioned"
	c.JSON(http.StatusOK, response)
}

func DeleteJunhuoLinkDeviceKey(c *gin.Context) {
	linkUserId, deviceId, tokenId, ok := junhuoLinkScopedToken(c)
	if !ok {
		return
	}
	if err := model.RevokeJunhuoLinkDeviceKey(linkUserId, deviceId, tokenId, model.JunhuoLinkNow()); err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func GetJunhuoLinkDeviceKey(c *gin.Context) {
	linkUserId, deviceId, tokenId, ok := junhuoLinkScopedToken(c)
	if !ok {
		return
	}
	binding, active, err := model.GetJunhuoLinkDeviceKeyStatus(linkUserId, deviceId, tokenId)
	if err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	response := gin.H{
		"active":         active,
		"tokenId":        strconv.Itoa(binding.TokenId),
		"keyFingerprint": binding.KeyFingerprint,
	}
	if binding.RevokedAt > 0 {
		response["revokedAt"] = time.Unix(binding.RevokedAt, 0).UTC().Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, response)
}

func GetJunhuoLinkDeviceUsage(c *gin.Context) {
	linkUserId, deviceId, tokenId, ok := junhuoLinkScopedToken(c)
	if !ok {
		return
	}
	usage, err := model.GetJunhuoLinkDeviceUsage(linkUserId, deviceId, tokenId)
	if err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func PostJunhuoLinkDeviceUsage(c *gin.Context) {
	linkUserId, deviceId, tokenId, ok := junhuoLinkScopedToken(c)
	if !ok {
		return
	}
	var request junhuoLinkUsageReceiptRequest
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "junhuo_link_invalid_usage_receipt"})
		return
	}
	receipt, duplicate, err := model.RecordJunhuoLinkUsageReceipt(
		linkUserId, deviceId, tokenId, request.UsageId, request.TaskId,
		request.InputTokens, request.OutputTokens, model.JunhuoLinkNow(),
	)
	if err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	usage, err := model.GetJunhuoLinkDeviceUsage(linkUserId, deviceId, tokenId)
	if err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"inputTokens":  usage.InputTokens,
		"outputTokens": usage.OutputTokens,
		"usedQuota":    usage.UsedQuota,
		"duplicate":    duplicate,
		"usageId":      receipt.UsageId,
		"requestId":    receipt.RequestId,
	})
}

func GetJunhuoLinkUserUsage(c *gin.Context) {
	linkUserId, ok := junhuoLinkScopedUserPath(c)
	if !ok {
		return
	}
	usage, err := model.GetJunhuoLinkUserUsage(linkUserId)
	if err != nil {
		writeJunhuoLinkError(c, err)
		return
	}
	c.JSON(http.StatusOK, usage)
}

func junhuoLinkScopedUserPath(c *gin.Context) (string, bool) {
	linkUserId := strings.TrimSpace(c.Param("userId"))
	scopedUser := strings.TrimSpace(c.GetHeader("x-junhuo-link-user-id"))
	if linkUserId == "" || scopedUser == "" || scopedUser != linkUserId {
		c.JSON(http.StatusForbidden, gin.H{"error": "junhuo_link_scope_mismatch"})
		return "", false
	}
	return linkUserId, true
}

func junhuoLinkScopedToken(c *gin.Context) (string, string, int, bool) {
	linkUserId := strings.TrimSpace(c.GetHeader("x-junhuo-link-user-id"))
	deviceId := strings.TrimSpace(c.GetHeader("x-junhuo-link-device-id"))
	tokenId, err := strconv.Atoi(strings.TrimSpace(c.Param("tokenId")))
	if linkUserId == "" || deviceId == "" || err != nil || tokenId <= 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "junhuo_link_scope_required"})
		return "", "", 0, false
	}
	return linkUserId, deviceId, tokenId, true
}

func junhuoLinkKeyFingerprint(key string) string {
	digest := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(digest[:8])
}

func writeJunhuoLinkError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrJunhuoLinkMappingNotFound), errors.Is(err, model.ErrJunhuoLinkBindingNotFound), errors.Is(err, model.ErrJunhuoLinkUsageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, model.ErrJunhuoLinkMappingConflict), errors.Is(err, model.ErrJunhuoLinkBindingConflict), errors.Is(err, model.ErrJunhuoLinkUsageConflict), errors.Is(err, model.ErrJunhuoLinkUsageAmbiguous), errors.Is(err, model.ErrJunhuoLinkUsageMismatch):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "junhuo_link_internal_error"})
	}
}
