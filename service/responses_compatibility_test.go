package service

import (
	"net/http/httptest"
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

func TestApplyResponsesCompatibilityProfileStatelessResponses54(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5.4",
		Input: mustMarshalCompatRaw([]map[string]any{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": "old assistant text"},
					{"type": "function_call", "name": "web_search", "arguments": "{}"},
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
		Reasoning:  &dto.Reasoning{Effort: "medium"},
		ToolChoice: mustMarshalCompatRaw("required"),
		Tools: mustMarshalCompatRaw([]map[string]any{
			{"type": "function", "function": map[string]any{"name": "lookup_weather"}},
		}),
	}

	result := ApplyResponsesCompatibilityProfile(ctx, relayconstant.RelayModeResponses, constant.ChannelTypeAntigravity, "gpt-5.4", req)
	require.True(t, result.Applied)
	require.Equal(t, ResponsesCompatProfileStatelessV1, result.Profile)
	require.Equal(t, "stateless", result.Mode)
	require.True(t, result.NormalizedInput)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.Nil(t, req.ContextManagement)
	require.Nil(t, req.PromptCacheKey)
	require.Nil(t, req.PromptCacheRetention)
	require.NotNil(t, req.Reasoning)
	require.JSONEq(t, `"required"`, string(req.ToolChoice))
	require.NotNil(t, req.Tools)
	require.JSONEq(t, `"be terse"`, string(req.Instructions))
	var input []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &input))
	require.Len(t, input, 1)
	require.Equal(t, "message", input[0]["type"])
	require.Equal(t, "user", input[0]["role"])
	content := input[0]["content"].([]any)
	require.Len(t, content, 2)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatApplied))
	require.Equal(t, ResponsesCompatProfileStatelessV1, common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatProfile))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatNormalizedInput))
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
	require.Equal(t, ResponsesCompatProfileStatelessV1, result.Profile)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.JSONEq(t, `"sum up"`, string(req.Instructions))
}

func TestApplyResponsesCompatibilityProfileSkips55(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.5",
		Input:              mustMarshalCompatRaw("hello"),
		PreviousResponseID: "resp_keep",
		Conversation:       mustMarshalCompatRaw(map[string]any{"id": "conv_keep"}),
	}

	result := ApplyResponsesCompatibilityProfile(nil, relayconstant.RelayModeResponses, constant.ChannelTypeAntigravity, "gpt-5.5", req)
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
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatRemovedFields, []string{"previous_response_id", "input.stateful_history"})
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatOriginModel, "gpt-5.4")
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatChannelType, constant.ChannelTypeAntigravity)
	common.SetContextKey(ctx, constant.ContextKeyResponsesCompatNormalizedInput, true)

	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 0)
	require.Equal(t, true, other["responses_compat_applied"])
	require.Equal(t, ResponsesCompatProfileStatelessV1, other["responses_compat_profile"])
	require.Equal(t, "stateless", other["responses_compat_mode"])
	require.Equal(t, "gpt-5.4", other["responses_compat_origin_model"])
	require.Equal(t, constant.ChannelTypeAntigravity, other["responses_compat_channel_type"])
	require.Equal(t, true, other["responses_compat_normalized_input"])
	require.ElementsMatch(t, []string{"previous_response_id", "input.stateful_history"}, other["responses_compat_removed_fields"].([]string))
}

func mustMarshalCompatRaw(v any) []byte {
	data, _ := common.Marshal(v)
	return data
}
