package service

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyResponsesCompatibilityProfileAntigravity55StatelessTranscript(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5.5",
		Input: mustMarshalCompatRaw([]map[string]any{
			{
				"type": "message",
				"role": "developer",
				"content": []map[string]any{
					{"type": "input_text", "text": "you are codex"},
				},
			},
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": "old assistant text"},
					{"type": "function_call", "name": "web_search", "call_id": "call_0", "arguments": "{}"},
				},
			},
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "hello"},
					{"type": "input_image", "image_url": "https://example.com/a.png"},
				},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "dropped tool output",
			},
		}),
		Instructions:         mustMarshalCompatRaw("be terse"),
		PreviousResponseID:   "resp_123",
		Conversation:         mustMarshalCompatRaw(map[string]any{"id": "conv_123"}),
		ContextManagement:    mustMarshalCompatRaw(map[string]any{"mode": "resume"}),
		PromptCacheKey:       mustMarshalCompatRaw("pc_123"),
		PromptCacheRetention: mustMarshalCompatRaw("1h"),
		Metadata: mustMarshalCompatRaw(map[string]any{
			"conversation_id":    "conv_123",
			"sessionId":          "sess_123",
			"turn_id":            "turn_123",
			"previousResponseId": "resp_123",
			"keep":               "yes",
		}),
		Reasoning: &dto.Reasoning{Effort: "medium"},
	}

	result := ApplyResponsesCompatibilityProfile(ctx, relayconstant.RelayModeResponses, constant.ChannelTypeOpenAI, "gpt-5.4", req)
	require.True(t, result.Applied)
	require.Equal(t, ResponsesCompatProfileStatelessV1, result.Profile)
	require.Equal(t, "stateless", result.Mode)
	require.True(t, result.NormalizedInput)
	require.Equal(t, 0, result.RemovedToolItems)
	require.GreaterOrEqual(t, result.RemovedHistoryItems, 0)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.Nil(t, req.ContextManagement)
	require.Nil(t, req.PromptCacheKey)
	require.Nil(t, req.PromptCacheRetention)
	require.NotNil(t, req.Reasoning)
	require.Empty(t, result.ToolStrategy)
	require.JSONEq(t, `"be terse"`, string(req.Instructions))
	var input []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &input))
	require.NotEmpty(t, input)
	var sawUser bool
	for _, it := range input {
		if strings.TrimSpace(common.Interface2String(it["type"])) != "message" {
			continue
		}
		role := strings.TrimSpace(common.Interface2String(it["role"]))
		require.Equal(t, "user", role)
		sawUser = true
	}
	require.True(t, sawUser)
	require.Equal(t, "message", input[len(input)-1]["type"])
	require.Equal(t, "user", input[len(input)-1]["role"])
	lastContent := input[len(input)-1]["content"].([]any)
	require.NotEmpty(t, lastContent)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatApplied))
	require.Equal(t, ResponsesCompatProfileStatelessV1, common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatProfile))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatNormalizedInput))
	require.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatIncludeDropped))
	require.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatToolsDropped))
	require.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatToolChoiceDropped))
	require.Empty(t, common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatToolStrategy))
}

func TestApplyResponsesCompatibilityProfileCompact54(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.4-openai-compact",
		Input:              mustMarshalCompatRaw([]map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hello"}}}}),
		Instructions:       mustMarshalCompatRaw("sum up"),
		PreviousResponseID: "resp_456",
		Conversation:       mustMarshalCompatRaw(map[string]any{"id": "conv_456"}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeResponsesCompact, constant.ChannelTypeOpenAI, "gpt-5.4-openai-compact", req)
	require.True(t, result.Applied)
	require.Equal(t, ResponsesCompatProfileStatelessV1, result.Profile)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.JSONEq(t, `"sum up"`, string(req.Instructions))
}

func TestApplyResponsesCompatibilityProfileSkipsAntigravity54(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:      "gpt-5.4",
		Input:      mustMarshalCompatRaw([]map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hello"}}}}),
		ToolChoice: mustMarshalCompatRaw("required"),
		Tools: mustMarshalCompatRaw([]map[string]any{
			{"type": "function", "function": map[string]any{"name": "lookup_weather"}},
		}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeResponses, constant.ChannelTypeAntigravity, "gpt-5.4", req)
	require.False(t, result.Applied)
	require.False(t, result.RemovedTools)
	require.False(t, result.RemovedToolChoice)
	require.Empty(t, result.ToolStrategy)
	require.NotNil(t, req.Tools)
	require.JSONEq(t, `"required"`, string(req.ToolChoice))
}

func TestApplyResponsesCompatibilityProfileSkips55ForOpenAI(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.5",
		Input:              mustMarshalCompatRaw("hello"),
		PreviousResponseID: "resp_keep",
		Conversation:       mustMarshalCompatRaw(map[string]any{"id": "conv_keep"}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeResponses, constant.ChannelTypeOpenAI, "gpt-5.5", req)
	require.False(t, result.Applied)
	require.Equal(t, "resp_keep", req.PreviousResponseID)
	require.NotNil(t, req.Conversation)
}

func TestApplyResponsesCompatibilityProfileSkipsChatCompletionsRelayMode(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.4",
		Input:              mustMarshalCompatRaw("hello"),
		PreviousResponseID: "resp_keep",
		Conversation:       mustMarshalCompatRaw(map[string]any{"id": "conv_keep"}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeChatCompletions, constant.ChannelTypeAntigravity, "gpt-5.4", req)
	require.False(t, result.Applied)
}

func TestGenerateTextOtherInfoIncludesResponsesCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatApplied, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatProfile, ResponsesCompatProfileStatelessV1)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatMode, "stateless")
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedFields, []string{"previous_response_id", "input.stateful_history", "include"})
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatOriginModel, "gpt-5.4")
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatNormalizedInput, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedToolItems, 2)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedHistoryItems, 3)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatIncludeDropped, true)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)

	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		RelayMode:         relayconstant.RelayModeResponses,
	}
	info.UpstreamModelName = "gemini-3-flash"
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 0)
	require.Equal(t, true, other["responses_compat_applied"])
	require.Equal(t, ResponsesCompatProfileStatelessV1, other["responses_compat_profile"])
	require.Equal(t, "stateless", other["responses_compat_mode"])
	require.Equal(t, "gpt-5.4", other["responses_compat_origin_model"])
	require.Equal(t, constant.ChannelTypeOpenAI, other["responses_compat_channel_type"])
	require.Equal(t, true, other["responses_compat_normalized_input"])
	require.Equal(t, 2, other["responses_compat_removed_tool_items"])
	require.Equal(t, 3, other["responses_compat_removed_history_items"])
	require.Equal(t, true, other["responses_compat_include_dropped"])
	require.ElementsMatch(t, []string{"previous_response_id", "input.stateful_history", "include"}, other["responses_compat_removed_fields"].([]string))
	adminInfo := other["admin_info"].(map[string]interface{})
	_, exists := adminInfo["antigravity_responses_mode"]
	require.False(t, exists)
}

func TestGenerateTextOtherInfoIncludesExternalPoolAdminInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(ctx, constant.ContextKeyChannelName, "cursor-pool-proxy")
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:3401")
	common.SetContextKey(ctx, constant.ContextKeyChannelOtherInfo, map[string]any{
		"cursor_pool_proxy":         true,
		"cursor_pool_status_path":   "/auth/status",
		"cursor_pool_accounts_path": "/auth/accounts",
	})

	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		RelayMode:         relayconstant.RelayModeResponses,
		OriginModelName:   "gpt-5.4",
	}
	info.UpstreamModelName = "gpt-5.5"
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 0)
	adminInfo := other["admin_info"].(map[string]interface{})
	require.Equal(t, ExternalPoolKindCursor, adminInfo["external_pool_proxy"])
	require.Equal(t, "Cursor", adminInfo["external_pool_display_name"])
	require.Equal(t, "cursor-pool-proxy", adminInfo["external_pool_channel_name"])
	require.Equal(t, "http://127.0.0.1:3401", adminInfo["external_pool_base_url"])
	require.Equal(t, "gpt-5.4", adminInfo["external_pool_origin_model"])
	require.Equal(t, "gpt-5.5", adminInfo["external_pool_upstream_model"])
	require.Equal(t, "/auth/status", adminInfo["external_pool_status_path"])
	require.Equal(t, "/auth/accounts", adminInfo["external_pool_accounts_path"])
}

func mustMarshalCompatRaw(v any) []byte {
	data, _ := common.Marshal(v)
	return data
}
