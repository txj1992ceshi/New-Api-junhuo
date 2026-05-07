package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupExternalPoolProxyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

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

	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("failed to migrate channels: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestResolveExternalPoolProxyFallbacks(t *testing.T) {
	baseURL := "http://127.0.0.1:4010"
	channel := &model.Channel{
		Key:       "external-pool-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"cursor_pool_proxy":true,"pool_status_path":"/status","pool_accounts_path":"accounts","pool_auth_scheme":"Token"}`,
	}

	proxy, ok := ResolveCursorPoolProxy(channel)
	if !ok {
		t.Fatalf("expected cursor proxy to resolve")
	}
	if proxy.StatusPath != "/status" {
		t.Fatalf("unexpected status path: %s", proxy.StatusPath)
	}
	if proxy.AccountsPath != "/accounts" {
		t.Fatalf("unexpected accounts path: %s", proxy.AccountsPath)
	}
	if proxy.AuthScheme != "Token" {
		t.Fatalf("unexpected auth scheme: %s", proxy.AuthScheme)
	}
}

func TestFetchCursorAndKiroPoolDataWithNestedPayloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pooled-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch r.URL.Path {
		case "/cursor/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"authenticated":true,"available_count":4,"total_count":6,"error_count":1,"models":["gpt-5.4"]}}`))
		case "/cursor/accounts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"accounts":[{"id":"c1","email":"cursor@example.com","status":"active","available_models":["gpt-5.4"]}]}}`))
		case "/kiro/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"authenticated":true,"active":2,"total":3,"error":1}`))
		case "/kiro/accounts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"k1","display_name":"builder","status":"healthy","project_id":"p-1"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cursorChannel := &model.Channel{
		Name:      "cursor-proxy",
		Key:       "pooled-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"cursor_pool_proxy":true,"cursor_pool_status_path":"/cursor/status","cursor_pool_accounts_path":"/cursor/accounts"}`,
	}
	kiroChannel := &model.Channel{
		Name:      "kiro-proxy",
		Key:       "pooled-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"kiro_pool_proxy":true,"kiro_pool_status_path":"/kiro/status","kiro_pool_accounts_path":"/kiro/accounts"}`,
	}

	cursorStatus, err := FetchCursorPoolStatus(context.Background(), cursorChannel)
	if err != nil {
		t.Fatalf("FetchCursorPoolStatus error: %v", err)
	}
	if cursorStatus.Active != 4 || cursorStatus.Total != 6 || cursorStatus.Error != 1 {
		t.Fatalf("unexpected cursor status: %+v", cursorStatus)
	}
	cursorAccounts, err := FetchCursorPoolAccounts(context.Background(), cursorChannel)
	if err != nil {
		t.Fatalf("FetchCursorPoolAccounts error: %v", err)
	}
	if len(cursorAccounts) != 1 || cursorAccounts[0].Email != "cursor@example.com" {
		t.Fatalf("unexpected cursor accounts: %+v", cursorAccounts)
	}

	kiroStatus, err := FetchKiroPoolStatus(context.Background(), kiroChannel)
	if err != nil {
		t.Fatalf("FetchKiroPoolStatus error: %v", err)
	}
	if kiroStatus.Active != 2 || kiroStatus.Total != 3 || kiroStatus.Error != 1 {
		t.Fatalf("unexpected kiro status: %+v", kiroStatus)
	}
	kiroAccounts, err := FetchKiroPoolAccounts(context.Background(), kiroChannel)
	if err != nil {
		t.Fatalf("FetchKiroPoolAccounts error: %v", err)
	}
	if len(kiroAccounts) != 1 || kiroAccounts[0].ProjectID != "p-1" {
		t.Fatalf("unexpected kiro accounts: %+v", kiroAccounts)
	}
}

func TestParseExternalPoolStatusAndAccountsFallbackShapes(t *testing.T) {
	status, err := parseExternalPoolStatus([]byte(`{"data":{"healthy_count":3,"error_count":1,"models":["gpt-5.5"]}}`))
	if err != nil {
		t.Fatalf("parseExternalPoolStatus error: %v", err)
	}
	if status.Active != 3 || status.Total != 4 || status.Error != 1 {
		t.Fatalf("unexpected fallback status: %+v", status)
	}
	if len(status.Models) != 1 || status.Models[0] != "gpt-5.5" {
		t.Fatalf("unexpected fallback models: %+v", status.Models)
	}

	accounts, err := parseExternalPoolAccounts([]byte(`{"items":[{"uuid":"u1","username":"kiro-user","state":"failed","models":["m1","m2"]}]}`))
	if err != nil {
		t.Fatalf("parseExternalPoolAccounts error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("unexpected account len: %d", len(accounts))
	}
	if accounts[0].ID != "u1" || accounts[0].Email != "kiro-user" || accounts[0].Status != "failed" {
		t.Fatalf("unexpected fallback account: %+v", accounts[0])
	}
	if len(accounts[0].AvailableModels) != 2 {
		t.Fatalf("unexpected fallback available models: %+v", accounts[0].AvailableModels)
	}
}

func TestParseExternalPoolAccountsCamelCaseShapes(t *testing.T) {
	accounts, err := parseExternalPoolAccounts([]byte(`{"accounts":[{"id":"w1","email":"ws@example.com","status":"active","statusReason":"ready","availableModels":["gpt-4o-mini"],"tierModels":["gemini-2.5-flash"],"errorCount":2,"lastUsed":"2026-05-07T00:00:00Z"}]}`))
	if err != nil {
		t.Fatalf("parseExternalPoolAccounts error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("unexpected account len: %d", len(accounts))
	}
	if accounts[0].StatusReason != "ready" {
		t.Fatalf("unexpected status reason: %+v", accounts[0])
	}
	if len(accounts[0].AvailableModels) != 1 || accounts[0].AvailableModels[0] != "gpt-4o-mini" {
		t.Fatalf("unexpected available models: %+v", accounts[0].AvailableModels)
	}
	if len(accounts[0].TierModels) != 1 || accounts[0].TierModels[0] != "gemini-2.5-flash" {
		t.Fatalf("unexpected tier models: %+v", accounts[0].TierModels)
	}
	if accounts[0].ErrorCount != 2 || accounts[0].LastUsed != "2026-05-07T00:00:00Z" {
		t.Fatalf("unexpected camelCase account fields: %+v", accounts[0])
	}
}

func TestFetchExternalPoolStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad gateway"}`, http.StatusBadGateway)
	}))
	defer server.Close()

	channel := &model.Channel{
		Name:      "cursor-proxy",
		Key:       "pooled-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"cursor_pool_proxy":true}`,
	}

	_, err := FetchCursorPoolStatus(context.Background(), channel)
	if err == nil {
		t.Fatal("expected FetchCursorPoolStatus to fail")
	}
	diagnosis := ClassifyExternalPoolSummary(nil, nil, err)
	if diagnosis.Diagnosis != "upstream_server_error" || diagnosis.Availability != "unavailable" {
		t.Fatalf("unexpected error diagnosis: %+v", diagnosis)
	}
}

func TestClassifyExternalPoolState(t *testing.T) {
	cases := []struct {
		name     string
		status   *ExternalPoolStatus
		accounts []ExternalPoolAccount
		want     string
	}{
		{
			name:   "ready by active",
			status: &ExternalPoolStatus{Active: 1, Total: 2},
			want:   "ready",
		},
		{
			name:   "empty pool",
			status: &ExternalPoolStatus{Active: 0, Total: 0},
			want:   "empty_pool",
		},
		{
			name:   "degraded by errors",
			status: &ExternalPoolStatus{Active: 0, Total: 2, Error: 2},
			want:   "degraded",
		},
		{
			name:     "ready by accounts",
			status:   nil,
			accounts: []ExternalPoolAccount{{ID: "a1"}},
			want:     "ready",
		},
		{
			name:   "unknown",
			status: nil,
			want:   "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyExternalPoolState(tc.status, tc.accounts)
			if got != tc.want {
				t.Fatalf("unexpected state: got=%s want=%s", got, tc.want)
			}
		})
	}
}

func TestGetExternalPoolSummaryIncludesPoolState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cursor/status":
			_, _ = w.Write([]byte(`{"authenticated":true,"active":0,"total":0,"error":0}`))
		case "/windsurf/status":
			_, _ = w.Write([]byte(`{"authenticated":true,"active":0,"total":2,"error":2}`))
		case "/kiro/status":
			_, _ = w.Write([]byte(`{"authenticated":true,"active":2,"total":3,"error":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cursorChannel := &model.Channel{
		Name:      "cursor-proxy",
		Key:       "pooled-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"cursor_pool_proxy":true,"cursor_pool_status_path":"/cursor/status"}`,
	}
	windsurfChannel := &model.Channel{
		Name:      "windsurf-proxy",
		Key:       "pooled-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"windsurf_pool_proxy":true,"windsurf_pool_status_path":"/windsurf/status"}`,
	}
	kiroChannel := &model.Channel{
		Name:      "kiro-proxy",
		Key:       "pooled-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"kiro_pool_proxy":true,"kiro_pool_status_path":"/kiro/status"}`,
	}

	cursorSummary, err := GetCursorPoolSummary(context.Background(), cursorChannel)
	if err != nil {
		t.Fatalf("GetCursorPoolSummary error: %v", err)
	}
	if cursorSummary.PoolState != "empty_pool" {
		t.Fatalf("unexpected cursor pool state: %+v", cursorSummary)
	}
	if cursorSummary.Diagnosis != "empty_pool" || cursorSummary.Availability != "unavailable" {
		t.Fatalf("unexpected cursor diagnosis: %+v", cursorSummary)
	}

	windsurfSummary, err := GetWindsurfPoolSummary(context.Background(), windsurfChannel)
	if err != nil {
		t.Fatalf("GetWindsurfPoolSummary error: %v", err)
	}
	if windsurfSummary.PoolState != "degraded" {
		t.Fatalf("unexpected windsurf pool state: %+v", windsurfSummary)
	}
	if windsurfSummary.Diagnosis != "degraded" || windsurfSummary.Availability != "degraded" {
		t.Fatalf("unexpected windsurf diagnosis: %+v", windsurfSummary)
	}

	kiroSummary, err := GetKiroPoolSummary(context.Background(), kiroChannel)
	if err != nil {
		t.Fatalf("GetKiroPoolSummary error: %v", err)
	}
	if kiroSummary.PoolState != "ready" {
		t.Fatalf("unexpected kiro pool state: %+v", kiroSummary)
	}
	if kiroSummary.Diagnosis != "ready" || kiroSummary.Availability != "available" {
		t.Fatalf("unexpected kiro diagnosis: %+v", kiroSummary)
	}
}

func TestGetExternalPoolAuthViewUsesConfiguredAuthorizeFields(t *testing.T) {
	db := setupExternalPoolProxyTestDB(t)
	baseURL := "http://127.0.0.1:3401"
	channel := &model.Channel{
		Name:      "cursor-pool-proxy",
		Type:      1,
		Key:       "demo-cursor-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"cursor_pool_proxy":true,"cursor_pool_authorize_url":"http://127.0.0.1:3401/dashboard/login","cursor_pool_authorize_hint":"manual login here","cursor_pool_auth_start_path":"/oauth/start","cursor_pool_auth_complete_path":"/oauth/complete"}`,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	view, err := GetCursorPoolAuthView(context.Background(), channel.Id, "")
	if err != nil {
		t.Fatalf("GetCursorPoolAuthView error: %v", err)
	}
	if !view.Available {
		t.Fatalf("expected auth view to be available: %+v", view)
	}
	if view.AuthorizeURL != "http://127.0.0.1:3401/dashboard/login" {
		t.Fatalf("unexpected authorize url: %s", view.AuthorizeURL)
	}
	if view.AuthorizeHint != "manual login here" {
		t.Fatalf("unexpected authorize hint: %s", view.AuthorizeHint)
	}
	if view.AuthStartPath != "/oauth/start" || view.AuthCompletePath != "/oauth/complete" {
		t.Fatalf("unexpected auth paths: %+v", view)
	}
	if view.AuthStrategy != "local_state_direct" {
		t.Fatalf("unexpected auth strategy: %+v", view)
	}
}

func TestStartAndCompleteExternalPoolAuthProxyRequests(t *testing.T) {
	db := setupExternalPoolProxyTestDB(t)
	var receivedCompleteBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer proxy-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch r.URL.Path {
		case "/cursor/start":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for start: %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"authorize_url":"http://pool.local/login","data":{"session":"abc"}}`))
		case "/cursor/complete":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for complete: %s", r.Method)
			}
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			receivedCompleteBody = string(bodyBytes)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"message":"account imported","data":{"account_id":"acc-1"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Name:      "cursor-pool-proxy",
		Type:      1,
		Key:       "proxy-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"cursor_pool_proxy":true,"cursor_pool_auth_start_path":"/cursor/start","cursor_pool_auth_complete_path":"/cursor/complete"}`,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	startResult, err := StartCursorPoolAuth(context.Background(), channel.Id, "")
	if err != nil {
		t.Fatalf("StartCursorPoolAuth error: %v", err)
	}
	startPayload, ok := startResult.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected start payload type: %T", startResult)
	}
	if startPayload["authorize_url"] != "http://pool.local/login" {
		t.Fatalf("unexpected authorize_url: %+v", startPayload)
	}

	completeResult, err := CompleteCursorPoolAuth(context.Background(), channel.Id, "https://callback.example?code=123", "")
	if err != nil {
		t.Fatalf("CompleteCursorPoolAuth error: %v", err)
	}
	if strings.TrimSpace(receivedCompleteBody) != `{"auth_strategy":"local_state_direct","input":"https://callback.example?code=123"}` {
		t.Fatalf("unexpected complete request body: %s", receivedCompleteBody)
	}
	completePayload, ok := completeResult.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected complete payload type: %T", completeResult)
	}
	if completePayload["message"] != "account imported" {
		t.Fatalf("unexpected complete payload: %+v", completePayload)
	}
}

func TestGetCursorPoolAuthViewSupportsManualTokenImportStrategy(t *testing.T) {
	db := setupExternalPoolProxyTestDB(t)
	baseURL := "http://127.0.0.1:3401"
	channel := &model.Channel{
		Name:      "cursor-pool-proxy",
		Type:      1,
		Key:       "demo-cursor-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"cursor_pool_proxy":true,"cursor_pool_auth_strategy":"manual_token_import"}`,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	view, err := GetCursorPoolAuthView(context.Background(), channel.Id, "")
	if err != nil {
		t.Fatalf("GetCursorPoolAuthView error: %v", err)
	}
	if view.AuthStrategy != "manual_token_import" {
		t.Fatalf("unexpected auth strategy: %+v", view)
	}
}
