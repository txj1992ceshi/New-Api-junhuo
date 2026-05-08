package controller

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func classifyExternalPoolActionError(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "missing input"):
		return "invalid_input", true
	case strings.Contains(msg, "status 401"), strings.Contains(msg, "status 403"), strings.Contains(msg, "invalid api key"):
		return "auth_rejected", true
	case strings.Contains(msg, "status 404"):
		return "upstream_path_not_found", true
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"):
		return "upstream_unreachable", true
	case strings.Contains(msg, "status 5"):
		return "upstream_server_error", true
	default:
		return "external_pool_action_failed", false
	}
}

func respondExternalPoolView(c *gin.Context, data interface{}, err error) {
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func respondExternalPoolAction(c *gin.Context, data interface{}, err error) {
	if err != nil {
		errorCode, recoverable := classifyExternalPoolActionError(err)
		c.JSON(http.StatusOK, gin.H{
			"success":     false,
			"message":     err.Error(),
			"error_code":  errorCode,
			"recoverable": recoverable,
			"data":        nil,
		})
		return
	}
	if payload, ok := data.(map[string]interface{}); ok && payload != nil {
		normalized := gin.H{
			"success": true,
			"message": "",
			"data":    payload,
		}
		if v, exists := payload["success"]; exists {
			normalized["success"] = v
		}
		if v, exists := payload["message"]; exists {
			normalized["message"] = v
		}
		if v, exists := payload["error_code"]; exists {
			normalized["error_code"] = v
		}
		if v, exists := payload["recoverable"]; exists {
			normalized["recoverable"] = v
		}
		if v, exists := payload["data"]; exists {
			normalized["data"] = v
		}
		c.JSON(http.StatusOK, normalized)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

type externalPoolAuthCompleteRequest struct {
	Input        string `json:"input"`
	AuthStrategy string `json:"auth_strategy"`
}

type externalPoolAuthStartRequest struct {
	AuthStrategy string `json:"auth_strategy"`
}

func externalPoolChannelID(c *gin.Context) (int, bool) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return 0, false
	}
	return channelID, true
}

func GetWindsurfPoolStatus(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetWindsurfPoolStatusView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetWindsurfPoolAccounts(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	data, err := service.GetWindsurfPoolAccountsView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetWindsurfPoolAuthView(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetWindsurfPoolAuthView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func StartWindsurfPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	data, err := service.StartWindsurfPoolAuth(ctx, channelID)
	respondExternalPoolAction(c, data, err)
}

func CompleteWindsurfPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	req := externalPoolAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Input) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "missing input"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	data, err := service.CompleteWindsurfPoolAuth(ctx, channelID, req.Input)
	respondExternalPoolAction(c, data, err)
}

func GetCursorPoolStatus(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetCursorPoolStatusView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetCursorPoolAccounts(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	data, err := service.GetCursorPoolAccountsView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetCursorPoolAuthView(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetCursorPoolAuthView(ctx, channelID, c.Query("auth_strategy"))
	respondExternalPoolView(c, data, err)
}

func StartCursorPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	req := externalPoolAuthStartRequest{}
	_ = c.ShouldBindJSON(&req)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	data, err := service.StartCursorPoolAuth(ctx, channelID, req.AuthStrategy)
	respondExternalPoolAction(c, data, err)
}

func CompleteCursorPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	req := externalPoolAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	data, err := service.CompleteCursorPoolAuth(ctx, channelID, req.Input, req.AuthStrategy)
	respondExternalPoolAction(c, data, err)
}

func GetCodexPoolStatus(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetCodexPoolStatusView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetCodexPoolAccounts(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	data, err := service.GetCodexPoolAccountsView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetCodexPoolAuthView(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetCodexPoolAuthView(ctx, channelID, c.Query("auth_strategy"))
	respondExternalPoolView(c, data, err)
}

func StartCodexPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	req := externalPoolAuthStartRequest{}
	_ = c.ShouldBindJSON(&req)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	data, err := service.StartCodexPoolAuth(ctx, channelID, req.AuthStrategy)
	respondExternalPoolAction(c, data, err)
}

func CompleteCodexPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	req := externalPoolAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	data, err := service.CompleteCodexPoolAuth(ctx, channelID, req.Input, req.AuthStrategy)
	respondExternalPoolAction(c, data, err)
}

func GetKiroPoolStatus(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetKiroPoolStatusView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetKiroPoolAccounts(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()
	data, err := service.GetKiroPoolAccountsView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func GetKiroPoolAuthView(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, err := service.GetKiroPoolAuthView(ctx, channelID)
	respondExternalPoolView(c, data, err)
}

func StartKiroPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	data, err := service.StartKiroPoolAuth(ctx, channelID)
	respondExternalPoolAction(c, data, err)
}

func CompleteKiroPoolAuth(c *gin.Context) {
	channelID, ok := externalPoolChannelID(c)
	if !ok {
		return
	}
	req := externalPoolAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Input) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "missing input"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	data, err := service.CompleteKiroPoolAuth(ctx, channelID, req.Input)
	respondExternalPoolAction(c, data, err)
}
