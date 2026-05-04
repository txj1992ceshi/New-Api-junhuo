package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type WindsurfPoolProxy struct {
	BaseURL string
	APIKey  string
}

type WindsurfPoolStatus struct {
	Authenticated bool   `json:"authenticated"`
	Total         int    `json:"total"`
	Active        int    `json:"active"`
	Error         int    `json:"error"`
	FetchedAt     string `json:"fetched_at"`
}

type WindsurfPoolAccount struct {
	ID               string                 `json:"id"`
	Email            string                 `json:"email"`
	Method           string                 `json:"method"`
	Status           string                 `json:"status"`
	ErrorCount       int                    `json:"error_count"`
	LastUsed         string                 `json:"last_used,omitempty"`
	AddedAt          string                 `json:"added_at,omitempty"`
	KeyPrefix        string                 `json:"key_prefix,omitempty"`
	APIKeyMasked     string                 `json:"api_key_masked,omitempty"`
	Tier             string                 `json:"tier,omitempty"`
	LastProbed       int64                  `json:"last_probed,omitempty"`
	RateLimitedUntil int64                  `json:"rate_limited_until,omitempty"`
	RateLimited      bool                   `json:"rate_limited"`
	RPMUsed          int                    `json:"rpm_used,omitempty"`
	RPMLimit         int                    `json:"rpm_limit,omitempty"`
	BlockedModels    []string               `json:"blocked_models,omitempty"`
	AvailableModels  []string               `json:"available_models,omitempty"`
	Credits          map[string]interface{} `json:"credits,omitempty"`
}

type WindsurfPoolStatusView struct {
	ChannelID          int                 `json:"channel_id"`
	ChannelName        string              `json:"channel_name"`
	BaseURL            string              `json:"base_url"`
	DashboardURL       string              `json:"dashboard_url"`
	TunnelHint         string              `json:"tunnel_hint,omitempty"`
	LastFetchedAt      string              `json:"last_fetched_at"`
	ConnectionOK       bool                `json:"connection_ok"`
	Status             *WindsurfPoolStatus `json:"status,omitempty"`
	UpstreamStatusCode int                 `json:"upstream_status_code,omitempty"`
	UpstreamError      string              `json:"upstream_error,omitempty"`
}

type WindsurfPoolAccountsView struct {
	ChannelID          int                   `json:"channel_id"`
	ChannelName        string                `json:"channel_name"`
	BaseURL            string                `json:"base_url"`
	LastFetchedAt      string                `json:"last_fetched_at"`
	ConnectionOK       bool                  `json:"connection_ok"`
	Accounts           []WindsurfPoolAccount `json:"accounts"`
	Total              int                   `json:"total"`
	Active             int                   `json:"active"`
	Error              int                   `json:"error"`
	UpstreamStatusCode int                   `json:"upstream_status_code,omitempty"`
	UpstreamError      string                `json:"upstream_error,omitempty"`
}

func parseWindsurfPoolOtherInfoBool(info map[string]interface{}, key string) bool {
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

func ResolveWindsurfPoolProxy(channel *model.Channel) (*WindsurfPoolProxy, bool) {
	if channel == nil {
		return nil, false
	}
	info := channel.GetOtherInfo()
	if !parseWindsurfPoolOtherInfoBool(info, "windsurf_pool_proxy") {
		return nil, false
	}
	baseURL := strings.TrimRight(strings.TrimSpace(channel.GetBaseURL()), "/")
	apiKey := strings.TrimSpace(channel.Key)
	return &WindsurfPoolProxy{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}, true
}

func (p *WindsurfPoolProxy) Ready() bool {
	return p != nil && p.BaseURL != "" && p.APIKey != ""
}

func windsurfDashboardURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return ""
	}
	return trimmed + "/dashboard"
}

func proxyWindsurfPoolRequest(
	ctx context.Context,
	channel *model.Channel,
	method string,
	path string,
) ([]byte, int, error) {
	proxy, ok := ResolveWindsurfPoolProxy(channel)
	if !ok {
		return nil, 0, fmt.Errorf("windsurf pool proxy is not configured")
	}
	if !proxy.Ready() {
		return nil, 0, fmt.Errorf("windsurf pool proxy is missing base_url or api key")
	}

	req, err := http.NewRequestWithContext(ctx, method, proxy.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+proxy.APIKey)
	req.Header.Set("Accept", "application/json")

	client, err := NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("windsurf upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, resp.StatusCode, nil
}

func GetWindsurfPoolSummary(ctx context.Context, channel *model.Channel) (*model.WindsurfPoolSummary, error) {
	status, err := FetchWindsurfPoolStatus(ctx, channel)
	if err != nil {
		return nil, err
	}
	return &model.WindsurfPoolSummary{
		AvailableCount: status.Active,
		TotalCount:     status.Total,
		ErrorCount:     status.Error,
	}, nil
}

func FetchWindsurfPoolStatus(ctx context.Context, channel *model.Channel) (*WindsurfPoolStatus, error) {
	body, _, err := proxyWindsurfPoolRequest(ctx, channel, http.MethodGet, "/auth/status")
	if err != nil {
		return nil, err
	}
	var status WindsurfPoolStatus
	if err := common.Unmarshal(body, &status); err != nil {
		return nil, err
	}
	status.FetchedAt = time.Now().Format(time.RFC3339)
	return &status, nil
}

func FetchWindsurfPoolAccounts(ctx context.Context, channel *model.Channel) ([]WindsurfPoolAccount, error) {
	body, _, err := proxyWindsurfPoolRequest(ctx, channel, http.MethodGet, "/auth/accounts")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Accounts []WindsurfPoolAccount `json:"accounts"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Accounts == nil {
		payload.Accounts = []WindsurfPoolAccount{}
	}
	return payload.Accounts, nil
}

func GetWindsurfPoolStatusView(ctx context.Context, channelID int) (*WindsurfPoolStatusView, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	proxy, ok := ResolveWindsurfPoolProxy(channel)
	if !ok {
		return nil, fmt.Errorf("channel is not a Windsurf pool proxy")
	}
	view := &WindsurfPoolStatusView{
		ChannelID:     channel.Id,
		ChannelName:   channel.Name,
		BaseURL:       proxy.BaseURL,
		DashboardURL:  windsurfDashboardURL(proxy.BaseURL),
		TunnelHint:    "先在本机建立 SSH 隧道，再访问 Dashboard",
		LastFetchedAt: time.Now().Format(time.RFC3339),
	}
	status, statusErr := FetchWindsurfPoolStatus(ctx, channel)
	if statusErr != nil {
		view.UpstreamError = statusErr.Error()
		return view, nil
	}
	view.ConnectionOK = true
	view.Status = status
	return view, nil
}

func GetWindsurfPoolAccountsView(ctx context.Context, channelID int) (*WindsurfPoolAccountsView, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	proxy, ok := ResolveWindsurfPoolProxy(channel)
	if !ok {
		return nil, fmt.Errorf("channel is not a Windsurf pool proxy")
	}

	view := &WindsurfPoolAccountsView{
		ChannelID:     channel.Id,
		ChannelName:   channel.Name,
		BaseURL:       proxy.BaseURL,
		LastFetchedAt: time.Now().Format(time.RFC3339),
		Accounts:      []WindsurfPoolAccount{},
	}

	status, statusErr := FetchWindsurfPoolStatus(ctx, channel)
	if statusErr == nil && status != nil {
		view.ConnectionOK = true
		view.Total = status.Total
		view.Active = status.Active
		view.Error = status.Error
	} else if statusErr != nil {
		view.UpstreamError = statusErr.Error()
	}

	accounts, accountsErr := FetchWindsurfPoolAccounts(ctx, channel)
	if accountsErr != nil {
		if view.UpstreamError == "" {
			view.UpstreamError = accountsErr.Error()
		}
		return view, nil
	}
	view.ConnectionOK = true
	view.Accounts = accounts
	if view.Total == 0 && len(accounts) > 0 {
		view.Total = len(accounts)
		for _, account := range accounts {
			if account.Status == "active" {
				view.Active++
			}
			if account.Status == "error" {
				view.Error++
			}
		}
	}
	return view, nil
}
