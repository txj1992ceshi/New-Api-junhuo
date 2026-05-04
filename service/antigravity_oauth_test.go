package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchAntigravityProjectIDsReturnsStringProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1internal:loadCodeAssist", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, "antigravity/windows/amd64", r.Header.Get("User-Agent"))

		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		metadata, ok := payload["metadata"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "ANTIGRAVITY", metadata["ideType"])
		_, hasPlatform := metadata["platform"]
		require.False(t, hasPlatform)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cloudaicompanionProject":"real-project-123"}`))
	}))
	defer server.Close()

	originalEndpoints := antigravityLoadEndpoints
	antigravityLoadEndpoints = []string{server.URL}
	defer func() { antigravityLoadEndpoints = originalEndpoints }()

	resolution, err := fetchAntigravityProjectIDs(context.Background(), server.Client(), "test-token")
	require.NoError(t, err)
	require.NotNil(t, resolution)
	require.Equal(t, "real-project-123", resolution.ProjectID)
	require.Equal(t, "", resolution.ManagedProjectID)
	require.Equal(t, "resolved_from_upstream", resolution.Mode)
	require.Equal(t, "minimal_antigravity", resolution.Variant)
	require.Len(t, resolution.Attempts, 1)
	require.Equal(t, 200, resolution.Attempts[0].HTTPStatus)
}

func TestFetchAntigravityProjectIDsReturnsObjectProjectAfterFallbackVariant(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		metadata := payload["metadata"].(map[string]any)
		if callCount == 1 {
			require.Equal(t, "ANTIGRAVITY", metadata["ideType"])
			require.Nil(t, metadata["platform"])
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"first variant rejected"}}`))
			return
		}

		require.Equal(t, "WINDOWS", metadata["platform"])
		require.Equal(t, `{"ideType":"ANTIGRAVITY","platform":"WINDOWS","pluginType":"GEMINI"}`, r.Header.Get("Client-Metadata"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cloudaicompanionProject":{"id":"managed-project-456"}}`))
	}))
	defer server.Close()

	originalEndpoints := antigravityLoadEndpoints
	antigravityLoadEndpoints = []string{server.URL}
	defer func() { antigravityLoadEndpoints = originalEndpoints }()

	resolution, err := fetchAntigravityProjectIDs(context.Background(), server.Client(), "test-token")
	require.NoError(t, err)
	require.NotNil(t, resolution)
	require.Equal(t, "managed-project-456", resolution.ProjectID)
	require.Equal(t, "managed-project-456", resolution.ManagedProjectID)
	require.Equal(t, "full_antigravity_metadata", resolution.Variant)
	require.Len(t, resolution.Attempts, 2)
	require.Equal(t, 400, resolution.Attempts[0].HTTPStatus)
	require.Contains(t, resolution.Attempts[0].ErrorSummary, "first variant rejected")
	require.Equal(t, 200, resolution.Attempts[1].HTTPStatus)
}

func TestFetchAntigravityProjectIDsFailsWithoutFallbackDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid metadata"}}`))
	}))
	defer server.Close()

	originalEndpoints := antigravityLoadEndpoints
	antigravityLoadEndpoints = []string{server.URL}
	defer func() { antigravityLoadEndpoints = originalEndpoints }()

	resolution, err := fetchAntigravityProjectIDs(context.Background(), server.Client(), "test-token")
	require.Nil(t, resolution)
	require.Error(t, err)

	var projectErr *AntigravityProjectResolutionError
	require.ErrorAs(t, err, &projectErr)
	require.Equal(t, "已获取 OAuth 凭据，但无法解析有效 Antigravity project，请稍后重试或更换账号", projectErr.UserMessage())
	require.Len(t, projectErr.Attempts, 2)
	require.Equal(t, 400, projectErr.Attempts[0].HTTPStatus)
	require.Equal(t, 400, projectErr.Attempts[1].HTTPStatus)
}
