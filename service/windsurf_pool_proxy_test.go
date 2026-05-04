package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestResolveWindsurfPoolProxy(t *testing.T) {
	baseURL := "http://127.0.0.1:3003"
	channel := &model.Channel{
		Key:       "windsurf-api-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"windsurf_pool_proxy":true}`,
	}

	proxy, ok := ResolveWindsurfPoolProxy(channel)
	if !ok {
		t.Fatalf("expected channel to be recognized as windsurf pool proxy")
	}
	if proxy.BaseURL != baseURL {
		t.Fatalf("unexpected base url: %s", proxy.BaseURL)
	}
	if proxy.APIKey != "windsurf-api-key" {
		t.Fatalf("unexpected api key: %s", proxy.APIKey)
	}
}

func TestFetchWindsurfPoolStatusAndAccounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer windsurf-api-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch r.URL.Path {
		case "/auth/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"authenticated":true,"total":3,"active":2,"error":1}`))
		case "/auth/accounts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accounts":[{"id":"a1","email":"u1@example.com","method":"token","status":"active","tier":"pro","blocked_models":["x"]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	channel := &model.Channel{
		Name:      "windsurf-api",
		Key:       "windsurf-api-key",
		BaseURL:   &server.URL,
		OtherInfo: `{"windsurf_pool_proxy":true}`,
	}

	status, err := FetchWindsurfPoolStatus(context.Background(), channel)
	if err != nil {
		t.Fatalf("FetchWindsurfPoolStatus error: %v", err)
	}
	if status.Active != 2 || status.Total != 3 || status.Error != 1 {
		t.Fatalf("unexpected status payload: %+v", status)
	}

	accounts, err := FetchWindsurfPoolAccounts(context.Background(), channel)
	if err != nil {
		t.Fatalf("FetchWindsurfPoolAccounts error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("unexpected account count: %d", len(accounts))
	}
	if accounts[0].Email != "u1@example.com" || accounts[0].Tier != "pro" {
		t.Fatalf("unexpected account payload: %+v", accounts[0])
	}
}
