package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	antigravityOAuthClientID       = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityOAuthClientSecret   = "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"
	antigravityOAuthAuthorizeURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	antigravityOAuthTokenURL       = "https://oauth2.googleapis.com/token"
	antigravityOAuthUserInfoURL    = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
	antigravityOAuthRedirectURI    = "http://localhost:51121/oauth-callback"
	antigravityDefaultProjectID    = "rising-fact-p41fc"
	antigravityFetchTimeout        = 15 * time.Second
	antigravityProjectFetchTimeout = 10 * time.Second
)

var antigravityScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/experimentsandconfigs",
}

var antigravityLoadEndpoints = []string{
	"https://cloudcode-pa.googleapis.com",
	"https://daily-cloudcode-pa.sandbox.googleapis.com",
	"https://autopush-cloudcode-pa.sandbox.googleapis.com",
}

type AntigravityOAuthTokenResult struct {
	AccessToken      string
	RefreshToken     string
	ExpiresAt        time.Time
	Email            string
	ProjectID        string
	ManagedProjectID string
}

type AntigravityAuthorizationFlow struct {
	State        string
	Verifier     string
	Challenge    string
	AuthorizeURL string
}

func antigravityHeaders() map[string]string {
	return map[string]string{
		"User-Agent":        "google-api-nodejs-client/9.15.1",
		"X-Goog-Api-Client": "gl-node/22.17.0",
		"Client-Metadata":   `ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI`,
	}
}

func CreateAntigravityOAuthAuthorizationFlow() (*AntigravityAuthorizationFlow, error) {
	state, err := createStateHex(16)
	if err != nil {
		return nil, err
	}
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(antigravityOAuthAuthorizeURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("client_id", antigravityOAuthClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", antigravityOAuthRedirectURI)
	q.Set("scope", strings.Join(antigravityScopes, " "))
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	u.RawQuery = q.Encode()
	return &AntigravityAuthorizationFlow{
		State:        state,
		Verifier:     verifier,
		Challenge:    challenge,
		AuthorizeURL: u.String(),
	}, nil
}

func ExchangeAntigravityAuthorizationCodeWithProxy(ctx context.Context, code string, verifier string, proxyURL string) (*AntigravityOAuthTokenResult, error) {
	client, err := getAntigravityOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("client_id", antigravityOAuthClientID)
	form.Set("client_secret", antigravityOAuthClientSecret)
	form.Set("code", strings.TrimSpace(code))
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", antigravityOAuthRedirectURI)
	form.Set("code_verifier", strings.TrimSpace(verifier))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("User-Agent", antigravityHeaders()["User-Agent"])

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := decodeAntigravityJSONResponse(resp, &payload); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("antigravity oauth code exchange failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.RefreshToken) == "" || payload.ExpiresIn <= 0 {
		return nil, errors.New("antigravity oauth token response missing fields")
	}

	email, _ := fetchAntigravityEmail(ctx, client, payload.AccessToken)
	projectID, managedProjectID, err := fetchAntigravityProjectIDs(ctx, client, payload.AccessToken)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		projectID = antigravityDefaultProjectID
	}

	return &AntigravityOAuthTokenResult{
		AccessToken:      strings.TrimSpace(payload.AccessToken),
		RefreshToken:     strings.TrimSpace(payload.RefreshToken),
		ExpiresAt:        time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Email:            email,
		ProjectID:        projectID,
		ManagedProjectID: managedProjectID,
	}, nil
}

func RefreshAntigravityOAuthTokenWithProxy(ctx context.Context, refreshToken string, proxyURL string) (*AntigravityOAuthTokenResult, error) {
	client, err := getAntigravityOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", strings.TrimSpace(refreshToken))
	form.Set("client_id", antigravityOAuthClientID)
	form.Set("client_secret", antigravityOAuthClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := decodeAntigravityJSONResponse(resp, &payload); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("antigravity oauth refresh failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || payload.ExpiresIn <= 0 {
		return nil, errors.New("antigravity oauth refresh response missing fields")
	}

	email, _ := fetchAntigravityEmail(ctx, client, payload.AccessToken)

	return &AntigravityOAuthTokenResult{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		Email:        email,
	}, nil
}

func getAntigravityOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	baseClient, err := GetHttpClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	if baseClient == nil {
		return &http.Client{Timeout: antigravityFetchTimeout}, nil
	}
	clientCopy := *baseClient
	clientCopy.Timeout = antigravityFetchTimeout
	return &clientCopy, nil
}

func fetchAntigravityEmail(ctx context.Context, client *http.Client, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, antigravityOAuthUserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("User-Agent", antigravityHeaders()["User-Agent"])
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("userinfo request failed: status=%d", resp.StatusCode)
	}
	var payload struct {
		Email string `json:"email"`
	}
	if err := decodeAntigravityJSONResponse(resp, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Email), nil
}

func decodeAntigravityJSONResponse(resp *http.Response, v any) error {
	if resp == nil || resp.Body == nil {
		return errors.New("empty antigravity oauth response")
	}
	reader, err := antigravityResponseReader(resp)
	if err != nil {
		return err
	}
	defer func() {
		if closer, ok := reader.(io.Closer); ok {
			_ = closer.Close()
		}
	}()
	return common.DecodeJson(reader, v)
}

func antigravityResponseReader(resp *http.Response) (io.Reader, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("empty antigravity oauth response body")
	}
	if strings.EqualFold(strings.TrimSpace(resp.Header.Get("Content-Encoding")), "gzip") {
		return gzip.NewReader(resp.Body)
	}
	return resp.Body, nil
}

func fetchAntigravityProjectIDs(ctx context.Context, client *http.Client, accessToken string) (string, string, error) {
	headers := antigravityHeaders()
	for _, baseURL := range antigravityLoadEndpoints {
		payload := map[string]any{
			"metadata": map[string]any{
				"ideType":    "ANTIGRAVITY",
				"platform":   "MACOS",
				"pluginType": "GEMINI",
			},
		}
		body, err := common.Marshal(payload)
		if err != nil {
			return "", "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1internal:loadCodeAssist", bytes.NewReader(body))
		if err != nil {
			return "", "", err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", headers["User-Agent"])
		req.Header.Set("Client-Metadata", `{"ideType":"ANTIGRAVITY","platform":"MACOS","pluginType":"GEMINI"}`)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		var envelope map[string]any
		decodeErr := common.DecodeJson(resp.Body, &envelope)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || decodeErr != nil {
			continue
		}
		if value, ok := envelope["cloudaicompanionProject"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), "", nil
		}
		if m, ok := envelope["cloudaicompanionProject"].(map[string]any); ok {
			if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id), strings.TrimSpace(id), nil
			}
		}
	}
	return "", "", nil
}

func DecodeAntigravityStatePayload(state string) (map[string]any, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(state), "-", "+"), "_", "/")
	padded := normalized
	if rem := len(normalized) % 4; rem != 0 {
		padded = normalized + strings.Repeat("=", 4-rem)
	}
	data, err := base64.StdEncoding.DecodeString(padded)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
