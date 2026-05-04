package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const (
	remoteCodexPoolAdminBaseURLEnv     = "REMOTE_CODEX_POOL_ADMIN_BASE_URL"
	remoteCodexPoolAdminAccessTokenEnv = "REMOTE_CODEX_POOL_ADMIN_ACCESS_TOKEN"
	remoteCodexPoolAdminUserIDEnv      = "REMOTE_CODEX_POOL_ADMIN_USER_ID"
	remoteCodexPoolChannelIDEnv        = "REMOTE_CODEX_POOL_CHANNEL_ID"
)

type RemoteCodexPoolProxy struct {
	BaseURL     string
	AccessToken string
	AdminUserID int
	ChannelID   int
}

type RemoteCodexPoolChannelSummary struct {
	ChannelInfo      *model.ChannelInfo      `json:"channel_info,omitempty"`
	CodexPoolSummary *model.CodexPoolSummary `json:"codex_pool_summary,omitempty"`
}

type remoteCodexPoolHealthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    *struct {
		Health *CodexPoolHealth `json:"health"`
	} `json:"data"`
}

func fetchRemoteCodexPoolSummary(ctx context.Context, channel *model.Channel, channelID int) (*model.CodexPoolSummary, error) {
	body, _, err := ProxyRemoteCodexPoolJSON(ctx, channel, http.MethodGet, fmt.Sprintf("/api/channel/%d/codex/pool_health", channelID), nil)
	if err != nil {
		return nil, err
	}
	var resp remoteCodexPoolHealthResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		if resp.Message == "" {
			resp.Message = "remote codex pool health request failed"
		}
		return nil, fmt.Errorf("%s", resp.Message)
	}
	if resp.Data == nil || resp.Data.Health == nil {
		return nil, fmt.Errorf("remote codex pool health missing")
	}
	health := resp.Data.Health
	return &model.CodexPoolSummary{
		AvailableCount: health.AvailableCount,
		HealthyCount:   health.Healthy + health.New,
		CooldownCount:  health.Cooldown,
		SuspectCount:   health.Suspect,
		DeadCount:      health.Dead,
		TotalCount:     health.Total,
	}, nil
}

func parseRemoteCodexPoolInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func parseRemoteCodexPoolOtherInfoInt(info map[string]interface{}, key string) int {
	if info == nil {
		return 0
	}
	raw, ok := info[key]
	if !ok || raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		return parseRemoteCodexPoolInt(v)
	default:
		return 0
	}
}

func parseRemoteCodexPoolOtherInfoBool(info map[string]interface{}, key string) bool {
	if info == nil {
		return false
	}
	raw, ok := info[key]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func parseRemoteCodexPoolOtherInfoString(info map[string]interface{}, key string) string {
	if info == nil {
		return ""
	}
	raw, ok := info[key]
	if !ok || raw == nil {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func parseSpecificChannelIDFromToken(raw string) int {
	token := strings.TrimSpace(raw)
	token = strings.TrimPrefix(token, "sk-")
	parts := strings.Split(token, "-")
	if len(parts) < 2 {
		return 0
	}
	return parseRemoteCodexPoolInt(parts[len(parts)-1])
}

func isRemoteCodexPoolProxyFallback(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(channel.Name))
	baseURL := strings.ToLower(strings.TrimSpace(channel.GetBaseURL()))
	if name != "codex-e2e-temp" {
		return false
	}
	return strings.Contains(baseURL, "127.0.0.1:18080") || strings.Contains(baseURL, "localhost:18080")
}

func ResolveRemoteCodexPoolProxy(channel *model.Channel) (*RemoteCodexPoolProxy, bool) {
	if channel == nil || channel.Type == constant.ChannelTypeCodex {
		return nil, false
	}

	info := channel.GetOtherInfo()
	enabled := parseRemoteCodexPoolOtherInfoBool(info, "remote_codex_pool_proxy")
	if !enabled && !isRemoteCodexPoolProxyFallback(channel) {
		return nil, false
	}

	channelID := parseRemoteCodexPoolOtherInfoInt(info, "remote_codex_pool_channel_id")
	if channelID == 0 {
		channelID = parseSpecificChannelIDFromToken(channel.Key)
	}
	if channelID == 0 {
		channelID = parseRemoteCodexPoolInt(os.Getenv(remoteCodexPoolChannelIDEnv))
	}
	if channelID == 0 {
		channelID = 2
	}

	baseURL := parseRemoteCodexPoolOtherInfoString(info, "remote_codex_admin_base_url")
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(remoteCodexPoolAdminBaseURLEnv))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(channel.GetBaseURL())
	}

	accessToken := parseRemoteCodexPoolOtherInfoString(info, "remote_codex_admin_access_token")
	if accessToken == "" {
		accessToken = strings.TrimSpace(os.Getenv(remoteCodexPoolAdminAccessTokenEnv))
	}

	adminUserID := parseRemoteCodexPoolOtherInfoInt(info, "remote_codex_admin_user_id")
	if adminUserID == 0 {
		adminUserID = parseRemoteCodexPoolInt(os.Getenv(remoteCodexPoolAdminUserIDEnv))
	}

	return &RemoteCodexPoolProxy{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		AccessToken: accessToken,
		AdminUserID: adminUserID,
		ChannelID:   channelID,
	}, true
}

func (p *RemoteCodexPoolProxy) Ready() bool {
	return p != nil && p.BaseURL != "" && p.AccessToken != "" && p.AdminUserID > 0 && p.ChannelID > 0
}

func ProxyRemoteCodexPoolJSON(
	ctx context.Context,
	channel *model.Channel,
	method string,
	path string,
	body []byte,
) ([]byte, int, error) {
	proxy, ok := ResolveRemoteCodexPoolProxy(channel)
	if !ok {
		return nil, 0, fmt.Errorf("remote codex pool proxy is not configured")
	}
	if !proxy.Ready() {
		return nil, 0, fmt.Errorf("remote codex pool proxy is missing admin credentials")
	}

	req, err := http.NewRequestWithContext(ctx, method, proxy.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+proxy.AccessToken)
	req.Header.Set("New-Api-User", strconv.Itoa(proxy.AdminUserID))
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}

func GetRemoteCodexPoolChannelSummary(ctx context.Context, channel *model.Channel) (*RemoteCodexPoolChannelSummary, error) {
	proxy, ok := ResolveRemoteCodexPoolProxy(channel)
	if !ok {
		return nil, fmt.Errorf("remote codex pool proxy is not configured")
	}
	body, _, err := ProxyRemoteCodexPoolJSON(ctx, channel, http.MethodGet, fmt.Sprintf("/api/channel/%d", proxy.ChannelID), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    *model.Channel `json:"data"`
	}
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		if resp.Message == "" {
			resp.Message = "remote codex channel request failed"
		}
		return nil, fmt.Errorf("%s", resp.Message)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("remote codex channel not found")
	}
	summary := resp.Data.CodexPoolSummary
	if summary == nil {
		summary, _ = fetchRemoteCodexPoolSummary(ctx, channel, proxy.ChannelID)
	}
	return &RemoteCodexPoolChannelSummary{
		ChannelInfo:      &resp.Data.ChannelInfo,
		CodexPoolSummary: summary,
	}, nil
}
