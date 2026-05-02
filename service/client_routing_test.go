package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestDetectRequestClientClass(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		userAgent  string
		originator string
		want       requestClientClass
	}{
		{name: "codex by user agent", userAgent: "codex-cli/0.1", want: requestClientClassCodex},
		{name: "codex by originator", originator: "codex_cli_rs", want: requestClientClassCodex},
		{name: "openclaw by user agent", userAgent: "OpenClaw/1.0", want: requestClientClassOpenClaw},
		{name: "unknown client", userAgent: "curl/8.0", want: requestClientClassUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest("POST", "/v1/responses", nil)
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}
			if tt.originator != "" {
				req.Header.Set("Originator", tt.originator)
			}
			ctx.Request = req
			if got := detectRequestClientClass(ctx); got != tt.want {
				t.Fatalf("detectRequestClientClass() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectPreferredChannelFromCandidates(t *testing.T) {
	codexChannel := &model.Channel{Id: 2, Name: "codex-e2e-temp", Type: constant.ChannelTypeCodex}
	antigravityChannel := &model.Channel{Id: 3, Name: "antigravity-openclaw", Type: constant.ChannelTypeAntigravity}
	caowoTag := "caowo_pool"
	caowoChannel := &model.Channel{Id: 1, Name: "caowo", Type: constant.ChannelTypeOpenAI, Tag: &caowoTag}

	tests := []struct {
		name       string
		roles      []channelRouteRole
		candidates []*model.Channel
		wantID     int
	}{
		{
			name:       "codex prefers codex over antigravity",
			roles:      []channelRouteRole{channelRouteRoleCodex, channelRouteRoleAntigravity},
			candidates: []*model.Channel{antigravityChannel, codexChannel},
			wantID:     2,
		},
		{
			name:       "openclaw prefers antigravity over caowo",
			roles:      []channelRouteRole{channelRouteRoleAntigravity, channelRouteRoleCaowo},
			candidates: []*model.Channel{caowoChannel, antigravityChannel},
			wantID:     3,
		},
		{
			name:       "generic clients never fall into codex private pool",
			roles:      []channelRouteRole{channelRouteRoleAntigravity, channelRouteRoleCaowo},
			candidates: []*model.Channel{codexChannel, caowoChannel},
			wantID:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := selectPreferredChannelFromCandidates(tt.candidates, tt.roles)
			if channel == nil {
				t.Fatal("selectPreferredChannelFromCandidates() returned nil")
			}
			if channel.Id != tt.wantID {
				t.Fatalf("selectPreferredChannelFromCandidates() = %d, want %d", channel.Id, tt.wantID)
			}
		})
	}
}

func TestShouldApplyClientRoute(t *testing.T) {
	tests := map[string]bool{
		"gpt-5.4":                true,
		"gpt-5.5-openai-compact": true,
		"gpt-5.5-mini":           true,
		"gpt-5":                  false,
		"claude-3-7-sonnet":      false,
		"":                       false,
	}
	for modelName, want := range tests {
		if got := shouldApplyClientRoute(modelName); got != want {
			t.Fatalf("shouldApplyClientRoute(%q) = %v, want %v", modelName, got, want)
		}
	}
}
