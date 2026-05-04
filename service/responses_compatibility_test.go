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
		Include:              mustMarshalCompatRaw([]string{"reasoning.encrypted_content"}),
		Store:                mustMarshalCompatRaw(false),
		ParallelToolCalls:    mustMarshalCompatRaw(true),
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
		Reasoning:  &dto.Reasoning{Effort: "medium"},
		ToolChoice: mustMarshalCompatRaw("required"),
		Tools: mustMarshalCompatRaw([]map[string]any{
			{"type": "function", "function": map[string]any{"name": "lookup_weather"}},
		}),
	}

	result := ApplyResponsesCompatibilityProfile(ctx, relayconstant.RelayModeResponses, constant.ChannelTypeAntigravity, "gpt-5.5", req)
	require.True(t, result.Applied)
	require.Equal(t, ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55, result.Profile)
	require.Equal(t, "stateless", result.Mode)
	require.True(t, result.NormalizedInput)
	require.True(t, result.RemovedInclude)
	require.True(t, result.RemovedStore)
	require.True(t, result.RemovedParallelToolCall)
	require.Greater(t, result.RemovedToolItems, 0)
	require.GreaterOrEqual(t, result.RemovedHistoryItems, 0)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.Nil(t, req.ContextManagement)
	require.Nil(t, req.PromptCacheKey)
	require.Nil(t, req.PromptCacheRetention)
	require.Nil(t, req.Include)
	require.Nil(t, req.Store)
	require.Nil(t, req.ParallelToolCalls)
	require.NotNil(t, req.Reasoning)
	require.Nil(t, req.ToolChoice)
	require.Nil(t, req.Tools)
	require.True(t, result.RemovedTools)
	require.True(t, result.RemovedToolChoice)
	require.Equal(t, "no_tools_gpt55", result.ToolStrategy)
	require.JSONEq(t, `"be terse"`, string(req.Instructions))
	var input []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &input))
	require.NotEmpty(t, input)
	// Codex sends developer + assistant history; gpt-5.5 Antigravity profile must use
	// normalizeResponsesInputForAntigravity (not the generic stateless branch that drops non-user roles).
	var sawDeveloper, sawAssistant bool
	for _, it := range input {
		if strings.TrimSpace(common.Interface2String(it["type"])) != "message" {
			continue
		}
		role := strings.TrimSpace(common.Interface2String(it["role"]))
		parts, ok := it["content"].([]any)
		if !ok {
			continue
		}
		var sb strings.Builder
		for _, p := range parts {
			pm, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(common.Interface2String(pm["type"])) == "input_text" {
				sb.WriteString(common.Interface2String(pm["text"]))
			}
		}
		text := sb.String()
		switch role {
		case "developer":
			sawDeveloper = true
			require.Contains(t, text, "you are codex")
		case "assistant":
			sawAssistant = true
			require.Contains(t, text, "old assistant text")
			require.Contains(t, text, "tool_call=web_search")
		}
	}
	require.True(t, sawDeveloper, "gpt-5.5 antigravity: developer transcript must survive normalization")
	require.True(t, sawAssistant, "gpt-5.5 antigravity: assistant transcript must survive normalization")
	require.Equal(t, "message", input[len(input)-1]["type"])
	require.Equal(t, "user", input[len(input)-1]["role"])
	lastContent := input[len(input)-1]["content"].([]any)
	require.NotEmpty(t, lastContent)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatApplied))
	require.Equal(t, ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55, common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatProfile))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatNormalizedInput))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatIncludeDropped))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatToolsDropped))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatToolChoiceDropped))
	require.Equal(t, "no_tools_gpt55", common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatToolStrategy))
}

func TestApplyResponsesCompatibilityProfileCompact54(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.4-openai-compact",
		Input:              mustMarshalCompatRaw([]map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hello"}}}}),
		Instructions:       mustMarshalCompatRaw("sum up"),
		PreviousResponseID: "resp_456",
		Conversation:       mustMarshalCompatRaw(map[string]any{"id": "conv_456"}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeResponsesCompact, constant.ChannelTypeAntigravity, "gpt-5.4-openai-compact", req)
	require.True(t, result.Applied)
	require.Equal(t, ResponsesCompatProfileStatelessV2Antigravity, result.Profile)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.JSONEq(t, `"sum up"`, string(req.Instructions))
}

func TestApplyResponsesCompatibilityProfileKeepsToolsForAntigravity54(t *testing.T) {
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

func TestApplyResponsesCompatibilityProfileAntigravity55MiniDropsTools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:      "gpt-5.5-mini",
		Input:      mustMarshalCompatRaw([]map[string]any{{"type": "message", "role": "user", "content": []map[string]any{{"type": "input_text", "text": "hello"}}}}),
		ToolChoice: mustMarshalCompatRaw(map[string]any{"type": "function", "function": map[string]any{"name": "lookup_weather"}}),
		Tools: mustMarshalCompatRaw([]map[string]any{
			{"type": "function", "function": map[string]any{"name": "lookup_weather"}},
		}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeResponses, constant.ChannelTypeAntigravity, "gpt-5.5-mini", req)
	require.True(t, result.Applied)
	require.True(t, result.RemovedTools)
	require.True(t, result.RemovedToolChoice)
	require.Equal(t, ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55, result.Profile)
	require.Equal(t, "no_tools_gpt55", result.ToolStrategy)
	require.Nil(t, req.Tools)
	require.Nil(t, req.ToolChoice)
}

func TestApplyResponsesCompatibilityProfileSkips55ForNonAntigravity(t *testing.T) {
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
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatProfile, ResponsesCompatProfileStatelessV2Antigravity)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatMode, "stateless")
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedFields, []string{"previous_response_id", "input.stateful_history", "include"})
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatOriginModel, "gpt-5.5")
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatChannelType, constant.ChannelTypeAntigravity)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatNormalizedInput, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedToolItems, 2)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedHistoryItems, 3)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatIncludeDropped, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatToolsDropped, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatToolChoiceDropped, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatToolStrategy, "no_tools_gpt55")
	common.SetContextKey(ctx, constant.ContextKeyAntigravityResponsesToolsForcedOff, true)
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeAntigravity)

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
	require.Equal(t, ResponsesCompatProfileStatelessV2Antigravity, other["responses_compat_profile"])
	require.Equal(t, "stateless", other["responses_compat_mode"])
	require.Equal(t, "gpt-5.5", other["responses_compat_origin_model"])
	require.Equal(t, constant.ChannelTypeAntigravity, other["responses_compat_channel_type"])
	require.Equal(t, true, other["responses_compat_normalized_input"])
	require.Equal(t, 2, other["responses_compat_removed_tool_items"])
	require.Equal(t, 3, other["responses_compat_removed_history_items"])
	require.Equal(t, true, other["responses_compat_include_dropped"])
	require.Equal(t, true, other["responses_compat_tools_dropped"])
	require.Equal(t, true, other["responses_compat_tool_choice_dropped"])
	require.Equal(t, "no_tools_gpt55", other["responses_compat_tool_strategy"])
	require.ElementsMatch(t, []string{"previous_response_id", "input.stateful_history", "include"}, other["responses_compat_removed_fields"].([]string))
	adminInfo := other["admin_info"].(map[string]interface{})
	require.Equal(t, "stateless_transcript", adminInfo["antigravity_responses_mode"])
	require.Equal(t, ResponsesCompatProfileStatelessV2Antigravity, adminInfo["antigravity_responses_profile"])
	require.Equal(t, "gemini-3-flash", adminInfo["antigravity_responses_upstream_model"])
	require.Equal(t, true, adminInfo["antigravity_responses_tools_forced_off"])
	require.Equal(t, "no_tools_gpt55", adminInfo["antigravity_responses_tool_strategy"])
}

func mustMarshalCompatRaw(v any) []byte {
	data, _ := common.Marshal(v)
	return data
}
