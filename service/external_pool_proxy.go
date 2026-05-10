package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	ExternalPoolKindCodex    = "codex"
	ExternalPoolKindCursor   = "cursor"
	ExternalPoolKindWindsurf = "windsurf"
	ExternalPoolKindKiro     = "kiro"
)

type ExternalPoolProxy struct {
	Kind             string
	DisplayName      string
	BaseURL          string
	APIKey           string
	StatusPath       string
	AccountsPath     string
	DashboardURL     string
	AuthorizeURL     string
	AuthorizeHint    string
	AuthStartPath    string
	AuthCompletePath string
	AuthHeader       string
	AuthScheme       string
	TunnelHint       string
}

type ExternalPoolStatus struct {
	Provider        string                    `json:"provider,omitempty"`
	LicenseStatus   string                    `json:"license_status,omitempty"`
	Authenticated   bool                      `json:"authenticated"`
	Total           int                       `json:"total"`
	Active          int                       `json:"active"`
	Error           int                       `json:"error"`
	FetchedAt       string                    `json:"fetched_at"`
	Models          []string                  `json:"models,omitempty"`
	Bridge          *ExternalPoolBridgeStatus `json:"bridge,omitempty"`
	BridgeStatus    string                    `json:"bridge_status,omitempty"`
	BridgeBaseURL   string                    `json:"bridge_base_url,omitempty"`
	BridgeLastError string                    `json:"bridge_last_error,omitempty"`
}

type ExternalPoolBridgeStatus struct {
	Status    string `json:"status,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

type ExternalPoolAccount struct {
	ID               string                 `json:"id"`
	Email            string                 `json:"email,omitempty"`
	DisplayName      string                 `json:"display_name,omitempty"`
	Method           string                 `json:"method,omitempty"`
	Status           string                 `json:"status,omitempty"`
	StatusReason     string                 `json:"status_reason,omitempty"`
	ErrorCount       int                    `json:"error_count,omitempty"`
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
	TierModels       []string               `json:"tier_models,omitempty"`
	ProjectID        string                 `json:"project_id,omitempty"`
	Credits          map[string]interface{} `json:"credits,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type ExternalPoolStatusView struct {
	Kind               string              `json:"kind"`
	ChannelID          int                 `json:"channel_id"`
	ChannelName        string              `json:"channel_name"`
	BaseURL            string              `json:"base_url"`
	DashboardURL       string              `json:"dashboard_url,omitempty"`
	TunnelHint         string              `json:"tunnel_hint,omitempty"`
	LastFetchedAt      string              `json:"last_fetched_at"`
	ConnectionOK       bool                `json:"connection_ok"`
	PoolState          string              `json:"pool_state,omitempty"`
	AuthCapable        bool                `json:"auth_capable"`
	InferenceProbed    bool                `json:"inference_probed"`
	InferenceCapable   bool                `json:"inference_capable"`
	InferenceError     string              `json:"inference_error,omitempty"`
	Availability       string              `json:"availability,omitempty"`
	Diagnosis          string              `json:"diagnosis,omitempty"`
	Status             *ExternalPoolStatus `json:"status,omitempty"`
	UpstreamStatusCode int                 `json:"upstream_status_code,omitempty"`
	UpstreamError      string              `json:"upstream_error,omitempty"`
}

type ExternalPoolAccountsView struct {
	Kind               string                `json:"kind"`
	ChannelID          int                   `json:"channel_id"`
	ChannelName        string                `json:"channel_name"`
	BaseURL            string                `json:"base_url"`
	LastFetchedAt      string                `json:"last_fetched_at"`
	ConnectionOK       bool                  `json:"connection_ok"`
	PoolState          string                `json:"pool_state,omitempty"`
	Accounts           []ExternalPoolAccount `json:"accounts"`
	Total              int                   `json:"total"`
	Active             int                   `json:"active"`
	Error              int                   `json:"error"`
	UpstreamStatusCode int                   `json:"upstream_status_code,omitempty"`
	UpstreamError      string                `json:"upstream_error,omitempty"`
}

type ExternalPoolAuthView struct {
	Kind             string `json:"kind"`
	ChannelID        int    `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	DisplayName      string `json:"display_name"`
	BaseURL          string `json:"base_url"`
	DashboardURL     string `json:"dashboard_url,omitempty"`
	AuthorizeURL     string `json:"authorize_url,omitempty"`
	AuthorizeHint    string `json:"authorize_hint,omitempty"`
	AuthStartPath    string `json:"auth_start_path,omitempty"`
	AuthCompletePath string `json:"auth_complete_path,omitempty"`
	AuthStrategy     string `json:"auth_strategy,omitempty"`
	Available        bool   `json:"available"`
}

type ExternalPoolConfigError struct {
	Kind        string
	DisplayName string
	Missing     []string
}

func (e *ExternalPoolConfigError) Error() string {
	if e == nil {
		return ""
	}
	display := strings.TrimSpace(e.DisplayName)
	if display == "" {
		display = "external pool"
	}
	if len(e.Missing) == 0 {
		return fmt.Sprintf("%s pool proxy config is invalid", strings.ToLower(display))
	}
	return fmt.Sprintf(
		"%s pool proxy config is invalid (missing: %s)",
		strings.ToLower(display),
		strings.Join(e.Missing, ", "),
	)
}

func parseExternalPoolOtherInfoBool(info map[string]interface{}, key string) bool {
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

func parseExternalPoolOtherInfoString(info map[string]interface{}, key string) string {
	if info == nil {
		return ""
	}
	raw, ok := info[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
}

func parseExternalPoolOtherInfoInt64(info map[string]interface{}, key string) int64 {
	if info == nil {
		return 0
	}
	raw, ok := info[key]
	if !ok || raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case string:
		var out int64
		_, _ = fmt.Sscan(strings.TrimSpace(v), &out)
		return out
	default:
		return 0
	}
}

func externalPoolDisplayName(kind string) string {
	switch kind {
	case ExternalPoolKindCodex:
		return "Codex"
	case ExternalPoolKindCursor:
		return "Cursor"
	case ExternalPoolKindKiro:
		return "Kiro"
	default:
		return "Windsurf"
	}
}

type ExternalPoolDiagnosis struct {
	Availability string
	Diagnosis    string
}

func ValidateExternalPoolProxy(channel *model.Channel, kind string) error {
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok {
		return nil
	}
	if proxy == nil {
		return &ExternalPoolConfigError{
			Kind:        kind,
			DisplayName: externalPoolDisplayName(kind),
			Missing:     []string{"base_url", "key"},
		}
	}
	missing := make([]string, 0, 2)
	if strings.TrimSpace(proxy.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(proxy.APIKey) == "" {
		missing = append(missing, "key")
	}
	if len(missing) == 0 {
		return nil
	}
	return &ExternalPoolConfigError{
		Kind:        kind,
		DisplayName: proxy.DisplayName,
		Missing:     missing,
	}
}

func classifyExternalPoolUpstreamError(err error) ExternalPoolDiagnosis {
	if err == nil {
		return ExternalPoolDiagnosis{}
	}
	var cfgErr *ExternalPoolConfigError
	if errors.As(err, &cfgErr) {
		return ExternalPoolDiagnosis{Availability: "config_invalid", Diagnosis: "missing_base_url_or_key"}
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "missing base_url or api key"),
		strings.Contains(msg, "missing base_url"),
		strings.Contains(msg, "missing api key"),
		strings.Contains(msg, "missing key"):
		return ExternalPoolDiagnosis{Availability: "config_invalid", Diagnosis: "missing_base_url_or_key"}
	case strings.Contains(msg, "status 401"),
		strings.Contains(msg, "status 403"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "forbidden"),
		strings.Contains(msg, "invalid api key"):
		return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "auth_rejected"}
	case strings.Contains(msg, "status 404"),
		strings.Contains(msg, "not found"):
		return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "upstream_path_not_found"}
	case strings.Contains(msg, "status 429"),
		strings.Contains(msg, "rate limit"):
		return ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "rate_limited"}
	case strings.Contains(msg, "status 5"):
		return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "upstream_server_error"}
	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "eof"):
		return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "upstream_unreachable"}
	default:
		return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "upstream_error"}
	}
}

func classifyExternalPoolSummary(status *ExternalPoolStatus, accounts []ExternalPoolAccount, err error) ExternalPoolDiagnosis {
	if err != nil {
		return classifyExternalPoolUpstreamError(err)
	}
	if status != nil {
		switch strings.ToLower(strings.TrimSpace(status.LicenseStatus)) {
		case "expired":
			return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "sidecar_expired"}
		case "invalid", "unavailable":
			return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "sidecar_unactivated"}
		}
		switch strings.ToLower(strings.TrimSpace(status.BridgeStatus)) {
		case "unreachable", "error", "down":
			return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "bridge_unreachable"}
		}
		if status.Authenticated && status.Active <= 0 {
			return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "no_active_accounts"}
		}
		if !status.Authenticated && strings.EqualFold(strings.TrimSpace(status.LicenseStatus), "activated") {
			return ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "provider_not_ready"}
		}
	}
	switch classifyExternalPoolState(status, accounts) {
	case "ready":
		return ExternalPoolDiagnosis{Availability: "available", Diagnosis: "ready"}
	case "empty_pool":
		return ExternalPoolDiagnosis{Availability: "unavailable", Diagnosis: "empty_pool"}
	case "degraded":
		return ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "degraded"}
	default:
		return ExternalPoolDiagnosis{Availability: "unknown", Diagnosis: "unknown"}
	}
}

func ClassifyExternalPoolSummary(status *ExternalPoolStatus, accounts []ExternalPoolAccount, err error) ExternalPoolDiagnosis {
	return classifyExternalPoolSummary(status, accounts, err)
}

func ProbeExternalPoolInference(ctx context.Context, channel *model.Channel, kind string, status *ExternalPoolStatus) (bool, bool, string) {
	info := channel.GetOtherInfo()
	if !parseExternalPoolOtherInfoBool(info, kind+"_pool_probe_inference") && !parseExternalPoolOtherInfoBool(info, "pool_probe_inference") {
		return false, false, ""
	}
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok || proxy == nil {
		return true, false, "proxy_not_configured"
	}
	if strings.TrimSpace(proxy.BaseURL) == "" || strings.TrimSpace(proxy.APIKey) == "" {
		return true, false, "missing_base_url_or_key"
	}
	// If not authenticated (or no active inventory), inference probe is not meaningful.
	if status == nil || !status.Authenticated {
		return false, false, "not_authenticated"
	}
	if status != nil {
		if licenseStatus := strings.ToLower(strings.TrimSpace(status.LicenseStatus)); licenseStatus != "" && licenseStatus != "activated" {
			if licenseStatus == "expired" {
				return false, false, "sidecar_expired"
			}
			return false, false, "sidecar_unactivated"
		}
		if bridgeStatus := strings.ToLower(strings.TrimSpace(status.BridgeStatus)); bridgeStatus != "" && bridgeStatus != "ready" {
			return false, false, "bridge_unreachable"
		}
	}
	if status.Active <= 0 {
		return false, false, "no_active_accounts"
	}
	inferenceMode := resolveExternalPoolInferenceMode(info, kind)
	candidateModels := buildExternalPoolInferenceCandidates(ctx, channel, kind, status)
	tryProbe := func(path string, payload map[string]interface{}) error {
		body, marshalErr := common.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		_, _, requestErr := proxyExternalPoolRequest(ctx, channel, proxy, http.MethodPost, path, body)
		return requestErr
	}
	if len(candidateModels) == 0 {
		candidateModels = []string{"gpt-5"}
	}
	lastErr := ""
	for _, modelName := range candidateModels {
		var err error
		switch inferenceMode {
		case "chat_completions":
			err = tryProbe("/v1/chat/completions", map[string]interface{}{
				"model": modelName,
				"messages": []map[string]string{
					{"role": "user", "content": "pool-probe"},
				},
			})
		case "dual":
			err = tryProbe("/v1/responses", map[string]interface{}{
				"model": modelName,
				"input": "pool-probe",
			})
			if err != nil && shouldFallbackToChatCompletions(err) {
				err = tryProbe("/v1/chat/completions", map[string]interface{}{
					"model": modelName,
					"messages": []map[string]string{
						{"role": "user", "content": "pool-probe"},
					},
				})
			}
		default:
			err = tryProbe("/v1/responses", map[string]interface{}{
				"model": modelName,
				"input": "pool-probe",
			})
		}
		if err == nil {
			return true, true, ""
		}
		lastErr = fmt.Sprintf("%s: %s", modelName, strings.TrimSpace(err.Error()))
		if !shouldTryNextInferenceModel(err) {
			return true, false, lastErr
		}
	}
	if strings.TrimSpace(lastErr) == "" {
		lastErr = "no_probe_candidate_succeeded"
	}
	return true, false, lastErr
}

func buildExternalPoolInferenceCandidates(ctx context.Context, channel *model.Channel, kind string, status *ExternalPoolStatus) []string {
	candidates := make([]string, 0, 16)
	seen := make(map[string]struct{})
	appendModel := func(modelName string) {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			return
		}
		if _, ok := seen[modelName]; ok {
			return
		}
		seen[modelName] = struct{}{}
		candidates = append(candidates, modelName)
	}
	appendModels := func(models []string) {
		for _, modelName := range models {
			appendModel(modelName)
		}
	}
	if channel != nil && channel.TestModel != nil {
		appendModel(*channel.TestModel)
	}
	appendModels(channel.GetOtherSettings().PublicModels)
	if status != nil {
		appendModels(status.Models)
	}
	if accounts, err := fetchExternalPoolAccounts(ctx, channel, kind); err == nil {
		for _, account := range accounts {
			appendModels(account.AvailableModels)
			appendModels(account.TierModels)
		}
	}
	switch kind {
	case ExternalPoolKindCodex:
		appendModels([]string{"gpt-5.5", "gpt-5.4", "gpt-5", "gpt-5-mini", "o3-mini"})
	case ExternalPoolKindCursor:
		appendModels([]string{"default", "auto", "gpt-4.1-mini", "gpt-4o-mini", "gpt-5-mini"})
	case ExternalPoolKindKiro:
		appendModels([]string{"auto", "claude-sonnet-4.5", "claude-sonnet-4", "claude-haiku-4.5"})
	case ExternalPoolKindWindsurf:
		appendModels([]string{"gpt-4o-mini", "gpt-4.1-mini", "gpt-5-mini", "gemini-2.5-flash", "claude-4.5-haiku", "claude-sonnet-4.6"})
	}
	return candidates
}

func shouldTryNextInferenceModel(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "model_not_entitled") ||
		strings.Contains(msg, "not entitled") ||
		strings.Contains(msg, "model_not_found") ||
		strings.Contains(msg, "unsupported model") ||
		strings.Contains(msg, "invalid model") ||
		strings.Contains(msg, "model_deprecated") ||
		strings.Contains(msg, "已被 windsurf 上游废弃") ||
		strings.Contains(msg, "不可用（未订阅或已被封禁）") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporarily unavailable") ||
		strings.Contains(msg, "upstream unavailable") ||
		strings.Contains(msg, "model_not_available") ||
		strings.Contains(msg, "internal server error") ||
		strings.Contains(msg, "bad gateway") ||
		strings.Contains(msg, "status 500") ||
		strings.Contains(msg, "status 502") ||
		strings.Contains(msg, "status 503") ||
		strings.Contains(msg, "status 504")
}

func resolveExternalPoolInferenceMode(info map[string]interface{}, kind string) string {
	mode := strings.ToLower(strings.TrimSpace(parseExternalPoolOtherInfoString(info, kind+"_pool_inference_mode")))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(parseExternalPoolOtherInfoString(info, "pool_inference_mode")))
	}
	switch mode {
	case "responses", "chat_completions", "dual":
		return mode
	default:
		return "responses"
	}
}

func shouldFallbackToChatCompletions(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "status 404") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "path disabled") ||
		strings.Contains(msg, "responses path disabled") ||
		strings.Contains(msg, "disabled by inference_mode") ||
		strings.Contains(msg, "disabled by inference mode")
}

func resolveExternalPoolProxy(channel *model.Channel, kind string) (*ExternalPoolProxy, bool) {
	if channel == nil {
		return nil, false
	}
	info := channel.GetOtherInfo()
	flagKey := kind + "_pool_proxy"
	if !parseExternalPoolOtherInfoBool(info, flagKey) {
		return nil, false
	}

	baseURL := parseExternalPoolOtherInfoString(info, kind+"_pool_base_url")
	if baseURL == "" {
		baseURL = parseExternalPoolOtherInfoString(info, "pool_base_url")
	}
	if baseURL == "" {
		baseURL = channel.GetBaseURL()
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	apiKey := parseExternalPoolOtherInfoString(info, kind+"_pool_api_key")
	if apiKey == "" {
		apiKey = parseExternalPoolOtherInfoString(info, "pool_api_key")
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(channel.Key)
	}

	statusPath := parseExternalPoolOtherInfoString(info, kind+"_pool_status_path")
	if statusPath == "" {
		statusPath = parseExternalPoolOtherInfoString(info, "pool_status_path")
	}
	if statusPath == "" {
		statusPath = "/auth/status"
	}

	accountsPath := parseExternalPoolOtherInfoString(info, kind+"_pool_accounts_path")
	if accountsPath == "" {
		accountsPath = parseExternalPoolOtherInfoString(info, "pool_accounts_path")
	}
	if accountsPath == "" {
		accountsPath = "/auth/accounts"
	}

	dashboardPath := parseExternalPoolOtherInfoString(info, kind+"_pool_dashboard_path")
	if dashboardPath == "" {
		dashboardPath = parseExternalPoolOtherInfoString(info, "pool_dashboard_path")
	}
	if dashboardPath == "" {
		dashboardPath = "/dashboard"
	}
	dashboardURL := buildExternalPoolDashboardURL(baseURL, dashboardPath)

	authorizeURL := parseExternalPoolOtherInfoString(info, kind+"_pool_authorize_url")
	if authorizeURL == "" {
		authorizeURL = parseExternalPoolOtherInfoString(info, "pool_authorize_url")
	}
	if authorizeURL == "" {
		authorizeURL = dashboardURL
	}

	authorizeHint := parseExternalPoolOtherInfoString(info, kind+"_pool_authorize_hint")
	if authorizeHint == "" {
		authorizeHint = parseExternalPoolOtherInfoString(info, "pool_authorize_hint")
	}
	if authorizeHint == "" {
		switch resolveExternalPoolAuthStrategy(channel, kind) {
		case "provider_bridge":
			authorizeHint = fmt.Sprintf("读取本机 %s 当前 provider 配置并导入当前渠道池，完成后立即执行最小验池。", externalPoolDisplayName(kind))
		case "local_state_direct":
			authorizeHint = fmt.Sprintf("读取本机 %s 登录态并导入当前渠道池，完成后立即执行最小验池。", externalPoolDisplayName(kind))
		default:
			authorizeHint = fmt.Sprintf("预留手动授权入口。完成 %s 登录授权后，回到池状态页确认账号数和可用数。", externalPoolDisplayName(kind))
		}
	}

	authStartPath := parseExternalPoolOtherInfoString(info, kind+"_pool_auth_start_path")
	if authStartPath == "" {
		authStartPath = parseExternalPoolOtherInfoString(info, "pool_auth_start_path")
	}
	if authStartPath == "" {
		authStartPath = "/auth/start"
	}

	authCompletePath := parseExternalPoolOtherInfoString(info, kind+"_pool_auth_complete_path")
	if authCompletePath == "" {
		authCompletePath = parseExternalPoolOtherInfoString(info, "pool_auth_complete_path")
	}
	if authCompletePath == "" {
		authCompletePath = "/auth/complete"
	}

	authHeader := parseExternalPoolOtherInfoString(info, kind+"_pool_auth_header")
	if authHeader == "" {
		authHeader = parseExternalPoolOtherInfoString(info, "pool_auth_header")
	}
	if authHeader == "" {
		authHeader = "Authorization"
	}

	authScheme := parseExternalPoolOtherInfoString(info, kind+"_pool_auth_scheme")
	if authScheme == "" {
		authScheme = parseExternalPoolOtherInfoString(info, "pool_auth_scheme")
	}
	if authScheme == "" {
		authScheme = "Bearer"
	}

	tunnelHint := parseExternalPoolOtherInfoString(info, kind+"_pool_tunnel_hint")
	if tunnelHint == "" {
		tunnelHint = parseExternalPoolOtherInfoString(info, "pool_tunnel_hint")
	}
	if tunnelHint == "" {
		tunnelHint = "先建立到池服务的安全通道，再访问 Dashboard 或执行补池操作"
	}

	return &ExternalPoolProxy{
		Kind:             kind,
		DisplayName:      externalPoolDisplayName(kind),
		BaseURL:          baseURL,
		APIKey:           apiKey,
		StatusPath:       ensureLeadingSlash(statusPath),
		AccountsPath:     ensureLeadingSlash(accountsPath),
		DashboardURL:     dashboardURL,
		AuthorizeURL:     strings.TrimSpace(authorizeURL),
		AuthorizeHint:    strings.TrimSpace(authorizeHint),
		AuthStartPath:    ensureLeadingSlash(authStartPath),
		AuthCompletePath: ensureLeadingSlash(authCompletePath),
		AuthHeader:       authHeader,
		AuthScheme:       authScheme,
		TunnelHint:       tunnelHint,
	}, true
}

func ensureLeadingSlash(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

func buildExternalPoolDashboardURL(baseURL string, dashboardPath string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBase == "" {
		return ""
	}
	trimmedPath := strings.TrimSpace(dashboardPath)
	if trimmedPath == "" {
		return ""
	}
	if strings.HasPrefix(trimmedPath, "http://") || strings.HasPrefix(trimmedPath, "https://") {
		return trimmedPath
	}
	return trimmedBase + ensureLeadingSlash(trimmedPath)
}

func (p *ExternalPoolProxy) Ready() bool {
	return p != nil && p.BaseURL != "" && p.APIKey != ""
}

func (p *ExternalPoolProxy) HasAuthorizeEntry() bool {
	return p != nil && (strings.TrimSpace(p.AuthorizeURL) != "" || strings.TrimSpace(p.DashboardURL) != "")
}

func (p *ExternalPoolProxy) SupportsAuthFlow() bool {
	return p != nil && strings.TrimSpace(p.AuthStartPath) != "" && strings.TrimSpace(p.AuthCompletePath) != ""
}

func externalPoolAuthValue(proxy *ExternalPoolProxy) string {
	if proxy == nil {
		return ""
	}
	scheme := strings.TrimSpace(proxy.AuthScheme)
	if scheme == "" {
		return proxy.APIKey
	}
	return scheme + " " + proxy.APIKey
}

func getExternalPoolAuthView(ctx context.Context, channelID int, kind string, authStrategyOverride string) (*ExternalPoolAuthView, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok || proxy == nil {
		return nil, fmt.Errorf("%s pool proxy is not configured on channel %d", externalPoolDisplayName(kind), channelID)
	}
	return &ExternalPoolAuthView{
		Kind:             kind,
		ChannelID:        channel.Id,
		ChannelName:      channel.Name,
		DisplayName:      proxy.DisplayName,
		BaseURL:          proxy.BaseURL,
		DashboardURL:     proxy.DashboardURL,
		AuthorizeURL:     proxy.AuthorizeURL,
		AuthorizeHint:    proxy.AuthorizeHint,
		AuthStartPath:    proxy.AuthStartPath,
		AuthCompletePath: proxy.AuthCompletePath,
		AuthStrategy:     resolveExternalPoolAuthStrategyOverride(channel, kind, authStrategyOverride),
		Available:        proxy.HasAuthorizeEntry() || proxy.SupportsAuthFlow(),
	}, nil
}

func resolveExternalPoolAuthStrategy(channel *model.Channel, kind string) string {
	return resolveExternalPoolAuthStrategyOverride(channel, kind, "")
}

func resolveExternalPoolAuthStrategyOverride(channel *model.Channel, kind string, override string) string {
	if channel == nil {
		return ""
	}
	strategy := strings.ToLower(strings.TrimSpace(override))
	if strategy == "" {
		info := channel.GetOtherInfo()
		strategy = strings.ToLower(strings.TrimSpace(parseExternalPoolOtherInfoString(info, kind+"_pool_auth_strategy")))
		if strategy == "" {
			strategy = strings.ToLower(strings.TrimSpace(parseExternalPoolOtherInfoString(info, "pool_auth_strategy")))
		}
	}
	if kind == ExternalPoolKindCodex {
		switch strategy {
		case "", "provider_bridge":
			return "provider_bridge"
		case "manual_token_import":
			return "manual_token_import"
		default:
			return strategy
		}
	}
	if kind == ExternalPoolKindCursor {
		switch strategy {
		case "", "local_state_direct":
			return "local_state_direct"
		case "manual_token_import":
			return "manual_token_import"
		case "oauth_callback":
			return "oauth_callback"
		default:
			return strategy
		}
	}
	if kind == ExternalPoolKindWindsurf || kind == ExternalPoolKindKiro {
		switch strategy {
		case "", "local_state_direct":
			return "local_state_direct"
		case "manual_token_import":
			return "manual_token_import"
		default:
			return strategy
		}
	}
	return strategy
}

func proxyExternalPoolRequest(
	ctx context.Context,
	channel *model.Channel,
	proxy *ExternalPoolProxy,
	method string,
	path string,
	body []byte,
) ([]byte, int, error) {
	if proxy == nil {
		return nil, 0, fmt.Errorf("external pool proxy is not configured")
	}
	if !proxy.Ready() {
		missing := make([]string, 0, 2)
		if strings.TrimSpace(proxy.BaseURL) == "" {
			missing = append(missing, "base_url")
		}
		if strings.TrimSpace(proxy.APIKey) == "" {
			missing = append(missing, "key")
		}
		return nil, 0, &ExternalPoolConfigError{
			Kind:        proxy.Kind,
			DisplayName: proxy.DisplayName,
			Missing:     missing,
		}
	}

	targetURL := path
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = proxy.BaseURL + ensureLeadingSlash(path)
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set(proxy.AuthHeader, externalPoolAuthValue(proxy))
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	client, err := NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("%s upstream status %d: %s", strings.ToLower(proxy.DisplayName), resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, resp.StatusCode, nil
}

func parseExternalPoolResponseBody(body []byte) interface{} {
	if len(body) == 0 {
		return map[string]interface{}{}
	}
	var payload interface{}
	if err := common.Unmarshal(body, &payload); err == nil {
		return payload
	}
	return map[string]interface{}{
		"raw": string(body),
	}
}

func normalizeExternalPoolActionPayload(raw interface{}, fallbackMessage string) map[string]interface{} {
	out := map[string]interface{}{
		"success": true,
		"message": fallbackMessage,
		"data":    raw,
	}
	payload, ok := raw.(map[string]interface{})
	if !ok || payload == nil {
		return out
	}
	if v, exists := payload["success"]; exists {
		out["success"] = v
	}
	if v, exists := payload["message"]; exists {
		if msg := strings.TrimSpace(fmt.Sprintf("%v", v)); msg != "" {
			out["message"] = msg
		}
	}
	if v, exists := payload["error_code"]; exists {
		out["error_code"] = v
	}
	if v, exists := payload["recoverable"]; exists {
		out["recoverable"] = v
	}
	if v, exists := payload["data"]; exists {
		out["data"] = v
	}
	if _, exists := payload["authorize_url"]; exists {
		out["authorize_url"] = payload["authorize_url"]
	}
	return out
}

func getStringFromMap(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		default:
			value := strings.TrimSpace(fmt.Sprintf("%v", raw))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func getBoolFromMap(payload map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case bool:
			return v, true
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "1", "true", "yes", "on":
				return true, true
			case "0", "false", "no", "off":
				return false, true
			}
		case float64:
			return v != 0, true
		case int:
			return v != 0, true
		}
	}
	return false, false
}

func getIntFromMap(payload map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return int(v), true
		case float32:
			return int(v), true
		case int:
			return v, true
		case int32:
			return int(v), true
		case int64:
			return int(v), true
		case string:
			var out int
			if _, err := fmt.Sscan(strings.TrimSpace(v), &out); err == nil {
				return out, true
			}
		}
	}
	return 0, false
}

func getStringSliceFromMap(payload map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case []interface{}:
			values := make([]string, 0, len(v))
			for _, item := range v {
				str := strings.TrimSpace(fmt.Sprintf("%v", item))
				if str != "" && str != "<nil>" {
					values = append(values, str)
				}
			}
			return values
		case []string:
			values := make([]string, 0, len(v))
			for _, item := range v {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					values = append(values, trimmed)
				}
			}
			return values
		}
	}
	return nil
}

func unwrapExternalPoolData(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	raw, ok := payload["data"]
	if !ok || raw == nil {
		return payload
	}
	if nested, ok := raw.(map[string]interface{}); ok {
		return nested
	}
	return payload
}

func parseExternalPoolStatus(body []byte) (*ExternalPoolStatus, error) {
	var raw map[string]interface{}
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	payload := unwrapExternalPoolData(raw)
	root := raw

	authenticated, ok := getBoolFromMap(payload, "authenticated", "connection_ok")
	if !ok {
		authenticated, _ = getBoolFromMap(root, "authenticated", "connection_ok")
	}

	total, ok := getIntFromMap(payload, "total", "total_count", "accounts_total")
	if !ok {
		total, _ = getIntFromMap(root, "total", "total_count", "accounts_total")
	}
	active, ok := getIntFromMap(payload, "active", "available", "available_count", "healthy", "healthy_count")
	if !ok {
		active, _ = getIntFromMap(root, "active", "available", "available_count", "healthy", "healthy_count")
	}
	errCount, ok := getIntFromMap(payload, "error", "error_count", "errors")
	if !ok {
		errCount, _ = getIntFromMap(root, "error", "error_count", "errors")
	}
	models := getStringSliceFromMap(payload, "models", "available_models")
	if len(models) == 0 {
		models = getStringSliceFromMap(root, "models", "available_models")
	}
	provider := getStringFromMap(payload, "provider")
	if provider == "" {
		provider = getStringFromMap(root, "provider")
	}
	licenseStatus := getStringFromMap(payload, "license_status")
	if licenseStatus == "" {
		licenseStatus = getStringFromMap(root, "license_status")
	}
	var bridge *ExternalPoolBridgeStatus
	if rawBridge, ok := payload["bridge"].(map[string]interface{}); ok && rawBridge != nil {
		bridge = &ExternalPoolBridgeStatus{
			Status:    getStringFromMap(rawBridge, "status"),
			BaseURL:   getStringFromMap(rawBridge, "base_url", "baseUrl"),
			LastError: getStringFromMap(rawBridge, "last_error", "lastError"),
		}
	} else if rawBridge, ok := root["bridge"].(map[string]interface{}); ok && rawBridge != nil {
		bridge = &ExternalPoolBridgeStatus{
			Status:    getStringFromMap(rawBridge, "status"),
			BaseURL:   getStringFromMap(rawBridge, "base_url", "baseUrl"),
			LastError: getStringFromMap(rawBridge, "last_error", "lastError"),
		}
	}
	bridgeStatus := getStringFromMap(payload, "bridge_status")
	if bridgeStatus == "" {
		bridgeStatus = getStringFromMap(root, "bridge_status")
	}
	bridgeBaseURL := getStringFromMap(payload, "bridge_base_url")
	if bridgeBaseURL == "" {
		bridgeBaseURL = getStringFromMap(root, "bridge_base_url")
	}
	bridgeLastError := getStringFromMap(payload, "bridge_last_error")
	if bridgeLastError == "" {
		bridgeLastError = getStringFromMap(root, "bridge_last_error")
	}
	if bridge != nil {
		if bridgeStatus == "" {
			bridgeStatus = bridge.Status
		}
		if bridgeBaseURL == "" {
			bridgeBaseURL = bridge.BaseURL
		}
		if bridgeLastError == "" {
			bridgeLastError = bridge.LastError
		}
	}
	if total == 0 && active > 0 {
		total = active + errCount
	}
	return &ExternalPoolStatus{
		Provider:        provider,
		LicenseStatus:   licenseStatus,
		Authenticated:   authenticated,
		Total:           total,
		Active:          active,
		Error:           errCount,
		Models:          models,
		FetchedAt:       time.Now().Format(time.RFC3339),
		Bridge:          bridge,
		BridgeStatus:    bridgeStatus,
		BridgeBaseURL:   bridgeBaseURL,
		BridgeLastError: bridgeLastError,
	}, nil
}

func extractExternalPoolAccountsList(body []byte) ([]interface{}, error) {
	var raw interface{}
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	switch v := raw.(type) {
	case []interface{}:
		return v, nil
	case map[string]interface{}:
		payload := unwrapExternalPoolData(v)
		for _, key := range []string{"accounts", "items"} {
			if rawList, ok := payload[key]; ok {
				if list, ok := rawList.([]interface{}); ok {
					return list, nil
				}
			}
		}
		for _, key := range []string{"accounts", "items"} {
			if rawList, ok := v[key]; ok {
				if list, ok := rawList.([]interface{}); ok {
					return list, nil
				}
			}
		}
	}
	return []interface{}{}, nil
}

func parseExternalPoolAccounts(body []byte) ([]ExternalPoolAccount, error) {
	list, err := extractExternalPoolAccountsList(body)
	if err != nil {
		return nil, err
	}
	accounts := make([]ExternalPoolAccount, 0, len(list))
	for _, item := range list {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		account := ExternalPoolAccount{
			ID:              getStringFromMap(row, "id", "account_id", "uuid"),
			Email:           getStringFromMap(row, "email", "account", "username"),
			DisplayName:     getStringFromMap(row, "display_name", "displayName", "name"),
			Method:          getStringFromMap(row, "method", "auth_type", "authType"),
			Status:          getStringFromMap(row, "status", "state"),
			StatusReason:    getStringFromMap(row, "status_reason", "statusReason"),
			LastUsed:        getStringFromMap(row, "last_used", "lastUsed", "last_used_at", "lastUsedAt"),
			AddedAt:         getStringFromMap(row, "added_at", "addedAt", "created_at", "createdAt"),
			KeyPrefix:       getStringFromMap(row, "key_prefix", "keyPrefix"),
			APIKeyMasked:    getStringFromMap(row, "api_key_masked", "apiKey_masked", "apiKeyMasked", "token_masked", "tokenMasked"),
			Tier:            getStringFromMap(row, "tier", "plan"),
			ProjectID:       getStringFromMap(row, "project_id", "projectId"),
			BlockedModels:   getStringSliceFromMap(row, "blocked_models", "blockedModels"),
			AvailableModels: getStringSliceFromMap(row, "available_models", "availableModels", "models"),
			TierModels:      getStringSliceFromMap(row, "tier_models", "tierModels"),
			Credits:         map[string]interface{}{},
			Metadata:        map[string]interface{}{},
		}
		if errorCount, ok := getIntFromMap(row, "error_count", "errorCount", "errors"); ok {
			account.ErrorCount = errorCount
		}
		account.LastProbed = parseExternalPoolOtherInfoInt64(row, "last_probed")
		if account.LastProbed == 0 {
			account.LastProbed = parseExternalPoolOtherInfoInt64(row, "lastProbed")
		}
		account.RateLimitedUntil = parseExternalPoolOtherInfoInt64(row, "rate_limited_until")
		if account.RateLimitedUntil == 0 {
			account.RateLimitedUntil = parseExternalPoolOtherInfoInt64(row, "rateLimitedUntil")
		}
		account.RPMUsed, _ = getIntFromMap(row, "rpm_used", "rpmUsed")
		account.RPMLimit, _ = getIntFromMap(row, "rpm_limit", "rpmLimit")
		if rateLimited, ok := getBoolFromMap(row, "rate_limited", "rateLimited"); ok {
			account.RateLimited = rateLimited
		}
		if credits, ok := row["credits"].(map[string]interface{}); ok {
			account.Credits = credits
		}
		if metadata, ok := row["metadata"].(map[string]interface{}); ok {
			account.Metadata = metadata
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func fetchExternalPoolStatus(ctx context.Context, channel *model.Channel, kind string) (*ExternalPoolStatus, error) {
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok {
		return nil, fmt.Errorf("channel is not a %s pool proxy", externalPoolDisplayName(kind))
	}
	body, _, err := proxyExternalPoolRequest(ctx, channel, proxy, http.MethodGet, proxy.StatusPath, nil)
	if err != nil {
		return nil, err
	}
	return parseExternalPoolStatus(body)
}

func fetchExternalPoolAccounts(ctx context.Context, channel *model.Channel, kind string) ([]ExternalPoolAccount, error) {
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok {
		return nil, fmt.Errorf("channel is not a %s pool proxy", externalPoolDisplayName(kind))
	}
	body, _, err := proxyExternalPoolRequest(ctx, channel, proxy, http.MethodGet, proxy.AccountsPath, nil)
	if err != nil {
		return nil, err
	}
	accounts, err := parseExternalPoolAccounts(body)
	if err != nil {
		return nil, err
	}
	if accounts == nil {
		accounts = []ExternalPoolAccount{}
	}
	return accounts, nil
}

func getExternalPoolStatusView(ctx context.Context, channelID int, kind string) (*ExternalPoolStatusView, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok {
		return nil, fmt.Errorf("channel is not a %s pool proxy", proxyDisplayNameLower(kind))
	}
	view := &ExternalPoolStatusView{
		Kind:          kind,
		ChannelID:     channel.Id,
		ChannelName:   channel.Name,
		BaseURL:       proxy.BaseURL,
		DashboardURL:  proxy.DashboardURL,
		TunnelHint:    proxy.TunnelHint,
		LastFetchedAt: time.Now().Format(time.RFC3339),
	}
	status, statusErr := fetchExternalPoolStatus(ctx, channel, kind)
	if statusErr != nil {
		diagnosis := classifyExternalPoolSummary(nil, nil, statusErr)
		view.UpstreamError = statusErr.Error()
		view.PoolState = "upstream_error"
		view.Availability = diagnosis.Availability
		view.Diagnosis = diagnosis.Diagnosis
		return view, nil
	}
	view.ConnectionOK = true
	view.Status = status
	view.PoolState = classifyExternalPoolState(status, nil)
	view.AuthCapable = status.Authenticated
	probed, inferenceCapable, inferenceErr := ProbeExternalPoolInference(ctx, channel, kind, status)
	view.InferenceProbed = probed
	view.InferenceCapable = probed && inferenceCapable
	view.InferenceError = inferenceErr
	diagnosis := classifyExternalPoolSummary(status, nil, nil)
	if probed && status.Authenticated && status.Active > 0 && !inferenceCapable {
		diagnosis = ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "auth_only"}
	}
	view.Availability = diagnosis.Availability
	view.Diagnosis = diagnosis.Diagnosis
	return view, nil
}

func getExternalPoolAccountsView(ctx context.Context, channelID int, kind string) (*ExternalPoolAccountsView, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok {
		return nil, fmt.Errorf("channel is not a %s pool proxy", proxyDisplayNameLower(kind))
	}
	view := &ExternalPoolAccountsView{
		Kind:          kind,
		ChannelID:     channel.Id,
		ChannelName:   channel.Name,
		BaseURL:       proxy.BaseURL,
		LastFetchedAt: time.Now().Format(time.RFC3339),
		Accounts:      []ExternalPoolAccount{},
	}
	status, statusErr := fetchExternalPoolStatus(ctx, channel, kind)
	if statusErr == nil && status != nil {
		view.ConnectionOK = true
		view.Total = status.Total
		view.Active = status.Active
		view.Error = status.Error
	} else if statusErr != nil {
		view.UpstreamError = statusErr.Error()
		view.PoolState = "upstream_error"
	}

	accounts, accountsErr := fetchExternalPoolAccounts(ctx, channel, kind)
	if accountsErr != nil {
		if view.UpstreamError == "" {
			view.UpstreamError = accountsErr.Error()
		}
		if view.PoolState == "" {
			view.PoolState = "upstream_error"
		}
		return view, nil
	}
	view.ConnectionOK = true
	view.Accounts = accounts
	if view.Total == 0 && len(accounts) > 0 {
		view.Total = len(accounts)
		for _, account := range accounts {
			switch strings.ToLower(strings.TrimSpace(account.Status)) {
			case "active", "healthy", "ok":
				view.Active++
			case "error", "failed":
				view.Error++
			}
		}
	}
	view.PoolState = classifyExternalPoolState(status, accounts)
	return view, nil
}

func proxyDisplayNameLower(kind string) string {
	return strings.ToLower(externalPoolDisplayName(kind))
}

func classifyExternalPoolState(status *ExternalPoolStatus, accounts []ExternalPoolAccount) string {
	if status == nil {
		if len(accounts) > 0 {
			return "ready"
		}
		return "unknown"
	}
	if status.Total <= 0 && len(accounts) == 0 {
		return "empty_pool"
	}
	if status.Active > 0 {
		return "ready"
	}
	if len(accounts) > 0 {
		return "degraded"
	}
	if status.Error > 0 {
		return "degraded"
	}
	return "empty_pool"
}

func startExternalPoolAuth(ctx context.Context, channelID int, kind string, authStrategyOverride string) (interface{}, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok || proxy == nil {
		return nil, fmt.Errorf("%s pool proxy is not configured on channel %d", externalPoolDisplayName(kind), channelID)
	}
	var payload []byte
	authStrategy := resolveExternalPoolAuthStrategyOverride(channel, kind, authStrategyOverride)
	if authStrategy != "" {
		payload, err = common.Marshal(map[string]string{
			"auth_strategy": authStrategy,
		})
		if err != nil {
			return nil, err
		}
	}
	body, _, err := proxyExternalPoolRequest(ctx, channel, proxy, http.MethodPost, proxy.AuthStartPath, payload)
	if err != nil {
		if shouldFallbackExternalPoolAuthStart(err, authStrategy) {
			return normalizeExternalPoolActionPayload(map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("%s local state scan is ready", strings.ToLower(proxy.DisplayName)),
				"data": map[string]interface{}{
					"auth_strategy":   authStrategy,
					"next_action":     "complete_auth",
					"authorize_hint":  proxy.AuthorizeHint,
					"required_fields": []string{},
				},
			}, "auth start requested"), nil
		}
		return nil, err
	}
	raw := parseExternalPoolResponseBody(body)
	return normalizeExternalPoolActionPayload(raw, "auth start requested"), nil
}

func shouldFallbackExternalPoolAuthStart(err error, authStrategy string) bool {
	if err == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(authStrategy)) {
	case "local_state_direct", "provider_bridge":
	default:
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "status 404")
}

func completeExternalPoolAuth(ctx context.Context, channelID int, kind string, input string, authStrategyOverride string) (interface{}, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	proxy, ok := resolveExternalPoolProxy(channel, kind)
	if !ok || proxy == nil {
		return nil, fmt.Errorf("%s pool proxy is not configured on channel %d", externalPoolDisplayName(kind), channelID)
	}
	requestPayload := map[string]string{
		"input": strings.TrimSpace(input),
	}
	authStrategy := resolveExternalPoolAuthStrategyOverride(channel, kind, authStrategyOverride)
	if authStrategy != "" {
		requestPayload["auth_strategy"] = authStrategy
	}
	payload, err := common.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}
	body, _, err := proxyExternalPoolRequest(ctx, channel, proxy, http.MethodPost, proxy.AuthCompletePath, payload)
	if err != nil {
		return nil, err
	}
	raw := parseExternalPoolResponseBody(body)
	return normalizeExternalPoolActionPayload(raw, "auth complete submitted"), nil
}
