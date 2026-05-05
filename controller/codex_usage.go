package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func proxyRemoteCodexPoolResponse(c *gin.Context, method string, buildPath func(channelID int) string, body []byte, timeout time.Duration) bool {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return true
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return true
	}
	if _, ok := service.ResolveRemoteCodexPoolProxy(channel); !ok {
		return false
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	proxy, _ := service.ResolveRemoteCodexPoolProxy(channel)
	respBody, _, err := service.ProxyRemoteCodexPoolJSON(ctx, channel, method, buildPath(proxy.ChannelID), body)
	if err != nil {
		common.SysError("failed to proxy remote codex pool response: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return true
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", respBody)
	return true
}

type codexUsageCredential struct {
	AccessToken  string
	RefreshToken string
	AccountID    string
	Email        string
	KeyIndex     int
}

func codexUsageSelectionFailureMessage(ch *model.Channel, err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(strings.ToLower(err.Error()), "no available codex keys") {
		health := service.ComputeCodexPoolHealth(ch, time.Now())
		if health != nil && health.Total > 0 && health.Dead == health.Total {
			return "all_keys_dead"
		}
		return "no_available_codex_keys"
	}
	return "invalid_codex_credential_payload"
}

func codexUsageFetchFailureMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "access_token is required"), strings.Contains(message, "empty accesstoken"):
		return "missing_access_token"
	case strings.Contains(message, "account_id is required"), strings.Contains(message, "empty accountid"):
		return "missing_account_id"
	default:
		return "upstream_fetch_failed"
	}
}

func selectCodexUsageCredential(ch *model.Channel, excluded map[int]bool, now time.Time) (*codexUsageCredential, error) {
	if ch == nil {
		return nil, fmt.Errorf("channel not found")
	}
	if ch.ChannelInfo.IsMultiKey {
		selection, selErr := service.SelectCodexKey(ch, excluded, now)
		if selErr != nil || selection == nil {
			if selErr != nil {
				return nil, selErr
			}
			return nil, fmt.Errorf("no available codex key")
		}
		oauthKey, err := codex.ParseOAuthKey(strings.TrimSpace(selection.Key))
		if err != nil {
			return nil, err
		}
		return &codexUsageCredential{
			AccessToken:  strings.TrimSpace(oauthKey.AccessToken),
			RefreshToken: strings.TrimSpace(oauthKey.RefreshToken),
			AccountID:    strings.TrimSpace(oauthKey.AccountID),
			Email:        strings.TrimSpace(oauthKey.Email),
			KeyIndex:     selection.KeyIndex,
		}, nil
	}

	oauthKey, err := codex.ParseOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, err
	}
	return &codexUsageCredential{
		AccessToken:  strings.TrimSpace(oauthKey.AccessToken),
		RefreshToken: strings.TrimSpace(oauthKey.RefreshToken),
		AccountID:    strings.TrimSpace(oauthKey.AccountID),
		Email:        strings.TrimSpace(oauthKey.Email),
		KeyIndex:     0,
	}, nil
}

func tryFetchCodexUsage(
	ctx context.Context,
	ch *model.Channel,
	client *http.Client,
	cred *codexUsageCredential,
) (int, []byte, error) {
	if cred == nil {
		return 0, nil, fmt.Errorf("missing codex credential")
	}
	if cred.AccessToken == "" {
		return 0, nil, fmt.Errorf("codex channel: access_token is required")
	}
	if cred.AccountID == "" {
		return 0, nil, fmt.Errorf("codex channel: account_id is required")
	}
	return service.FetchCodexWhamUsage(ctx, client, ch.GetBaseURL(), cred.AccessToken, cred.AccountID)
}

func GetCodexChannelUsage(c *gin.Context) {
	if proxyRemoteCodexPoolResponse(c, http.MethodGet, func(channelID int) string {
		return fmt.Sprintf("/api/channel/%d/codex/usage", channelID)
	}, nil, 20*time.Second) {
		return
	}

	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeCodex {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	excluded := map[int]bool{}
	var statusCode int
	var body []byte
	var selected *codexUsageCredential
	var fetchErr error
	var lastMessage string

	maxAttempts := 3
	if !ch.ChannelInfo.IsMultiKey {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		selected, err = selectCodexUsageCredential(ch, excluded, time.Now())
		if err != nil {
			if lastMessage == "" {
				lastMessage = codexUsageSelectionFailureMessage(ch, err)
			}
			break
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		statusCode, body, fetchErr = tryFetchCodexUsage(ctx, ch, client, selected)
		cancel()
		if fetchErr != nil {
			common.SysError("failed to fetch codex usage: " + fetchErr.Error())
			lastMessage = codexUsageFetchFailureMessage(fetchErr)
			break
		}

		if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(selected.RefreshToken) != "" {
			refreshCtx, refreshCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
			if ch.ChannelInfo.IsMultiKey {
				_, _, err = service.RefreshCodexChannelKeyCredential(refreshCtx, ch.Id, selected.KeyIndex, true)
			} else {
				_, _, err = service.RefreshCodexChannelCredential(refreshCtx, ch.Id, service.CodexCredentialRefreshOptions{ResetCaches: true})
			}
			refreshCancel()
			if err == nil {
				ch, _ = model.GetChannelById(channelId, true)
				selected, err = selectCodexUsageCredential(ch, excluded, time.Now())
				if err != nil {
					lastMessage = "刷新凭证后读取账号信息失败"
					break
				}
				ctx2, cancel2 := context.WithTimeout(c.Request.Context(), 15*time.Second)
				statusCode, body, fetchErr = tryFetchCodexUsage(ctx2, ch, client, selected)
				cancel2()
				if fetchErr != nil {
					common.SysError("failed to fetch codex usage after refresh: " + fetchErr.Error())
					lastMessage = codexUsageFetchFailureMessage(fetchErr)
					break
				}
			}
		}

		if statusCode >= 200 && statusCode < 300 {
			break
		}

		if ch.ChannelInfo.IsMultiKey {
			excluded[selected.KeyIndex] = true
		}
		lastMessage = fmt.Sprintf("upstream_status_%d", statusCode)
	}

	if fetchErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": lastMessage})
		return
	}
	if len(body) == 0 {
		message := lastMessage
		if message == "" {
			message = "upstream_fetch_failed"
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": message})
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}
	if selected != nil {
		if payloadMap, ok := payload.(map[string]any); ok {
			if _, exists := payloadMap["email"]; !exists && selected.Email != "" {
				payloadMap["email"] = selected.Email
			}
			if _, exists := payloadMap["account_id"]; !exists && selected.AccountID != "" {
				payloadMap["account_id"] = selected.AccountID
			}
			payloadMap["key_index"] = selected.KeyIndex
		}
	}

	ok := statusCode >= 200 && statusCode < 300
	resp := gin.H{
		"success":         ok,
		"message":         "",
		"upstream_status": statusCode,
		"data":            payload,
	}
	if !ok {
		if lastMessage != "" {
			resp["message"] = lastMessage
		} else {
			resp["message"] = fmt.Sprintf("upstream status: %d", statusCode)
		}
	}
	c.JSON(http.StatusOK, resp)
}

func ImportCursorProCodexChannelTokens(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := service.ImportCursorProExports(ctx, channelId)
	if err != nil {
		common.SysError("failed to import cursorpro codex exports: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "导入 CursorPro token 失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "imported",
		"data":    result,
	})
}

func GetCodexPoolHealth(c *gin.Context) {
	if proxyRemoteCodexPoolResponse(c, http.MethodGet, func(channelID int) string {
		return fmt.Sprintf("/api/channel/%d/codex/pool_health", channelID)
	}, nil, 15*time.Second) {
		return
	}

	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	status, err := service.GetCodexPoolHealthStatus(channelId, time.Now())
	if err != nil {
		common.SysError("failed to get codex pool health: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    status,
	})
}

func GetCursorProReplacementStatus(c *gin.Context) {
	if proxyRemoteCodexPoolResponse(c, http.MethodGet, func(channelID int) string {
		return fmt.Sprintf("/api/channel/%d/codex/replacement_status", channelID)
	}, nil, 15*time.Second) {
		return
	}

	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	status, err := service.GetCursorProReplacementStatus(ctx, channelId, time.Now())
	if err != nil {
		common.SysError("failed to get cursorpro replacement status: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    status,
	})
}
