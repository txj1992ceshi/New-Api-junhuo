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

func TestSanitizeResponsesRequestForModelResponses54(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)

	req := &dto.OpenAIResponsesRequest{
		Model:                "gpt-5.4",
		Input:                mustMarshalTestRaw("hello"),
		Instructions:         mustMarshalTestRaw("be terse"),
		PreviousResponseID:   "resp_123",
		Conversation:         mustMarshalTestRaw(map[string]any{"id": "conv_123"}),
		ContextManagement:    mustMarshalTestRaw(map[string]any{"mode": "resume"}),
		PromptCacheKey:       mustMarshalTestRaw("pc_123"),
		PromptCacheRetention: mustMarshalTestRaw("1h"),
		Metadata: mustMarshalTestRaw(map[string]any{
			"conversation_id":    "conv_123",
			"sessionId":          "sess_123",
			"turn_id":            "turn_123",
			"previousResponseId": "resp_123",
			"keep":               "yes",
		}),
		ToolChoice: mustMarshalTestRaw("required"),
		Tools:      mustMarshalTestRaw([]map[string]any{{"type": "function"}}),
	}

	result := SanitizeResponsesRequestForModel(ctx, relayconstant.RelayModeResponses, req)
	require.True(t, result.Applied)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.Nil(t, req.ContextManagement)
	require.Nil(t, req.PromptCacheKey)
	require.Nil(t, req.PromptCacheRetention)
	require.JSONEq(t, `"required"`, string(req.ToolChoice))
	require.NotNil(t, req.Tools)
	require.JSONEq(t, `"hello"`, string(req.Input))
	require.JSONEq(t, `"be terse"`, string(req.Instructions))
	require.ElementsMatch(t, []string{
		"previous_response_id",
		"conversation",
		"context_management",
		"prompt_cache_key",
		"prompt_cache_retention",
		"metadata.conversation_id",
		"metadata.sessionId",
		"metadata.turn_id",
		"metadata.previousResponseId",
	}, result.RemovedFields)
	var metadata map[string]any
	require.NoError(t, common.Unmarshal(req.Metadata, &metadata))
	require.Equal(t, map[string]any{"keep": "yes"}, metadata)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesStateSanitized))
	require.ElementsMatch(t, result.RemovedFields, common.GetContextKeyStringSlice(ctx, constant.ContextKeyResponsesStateSanitizedFields))
}

func TestSanitizeResponsesRequestForModelCompact54(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.4-openai-compact",
		Input:              mustMarshalTestRaw("hello"),
		Instructions:       mustMarshalTestRaw("sum up"),
		PreviousResponseID: "resp_456",
		Conversation:       mustMarshalTestRaw(map[string]any{"id": "conv_456"}),
	}

	result := SanitizeResponsesRequestForModel(nil, relayconstant.RelayModeResponsesCompact, req)
	require.True(t, result.Applied)
	require.Empty(t, req.PreviousResponseID)
	require.Nil(t, req.Conversation)
	require.JSONEq(t, `"hello"`, string(req.Input))
	require.JSONEq(t, `"sum up"`, string(req.Instructions))
}

func TestSanitizeResponsesRequestForModelSkips55(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.5",
		Input:              mustMarshalTestRaw("hello"),
		PreviousResponseID: "resp_keep",
		Conversation:       mustMarshalTestRaw(map[string]any{"id": "conv_keep"}),
	}

	result := SanitizeResponsesRequestForModel(nil, relayconstant.RelayModeResponses, req)
	require.False(t, result.Applied)
	require.Equal(t, "resp_keep", req.PreviousResponseID)
	require.NotNil(t, req.Conversation)
}

func TestSanitizeResponsesRequestForModelSkipsChatCompletionsRelayMode(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.4",
		Input:              mustMarshalTestRaw("hello"),
		PreviousResponseID: "resp_keep",
		Conversation:       mustMarshalTestRaw(map[string]any{"id": "conv_keep"}),
	}

	result := SanitizeResponsesRequestForModel(nil, relayconstant.RelayModeChatCompletions, req)
	require.False(t, result.Applied)
	require.Equal(t, "resp_keep", req.PreviousResponseID)
	require.NotNil(t, req.Conversation)
}

func TestGenerateTextOtherInfoIncludesResponsesStateSanitized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	common.SetContextKey(ctx, constant.ContextKeyResponsesStateSanitized, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesStateSanitizedFields, []string{"previous_response_id", "conversation"})
	common.SetContextKey(ctx, constant.ContextKeyResponsesStateHadPreviousResponseID, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesStateHadConversation, true)
	common.SetContextKey(ctx, constant.ContextKeyResponsesStateHadContextManagement, false)
	common.SetContextKey(ctx, constant.ContextKeyResponsesStateHadPromptCacheKey, true)

	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}
	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 0)
	require.Equal(t, true, other["responses_state_sanitized"])
	require.ElementsMatch(t, []string{"previous_response_id", "conversation"}, other["responses_state_sanitized_fields"].([]string))
	require.Equal(t, true, other["responses_state_had_previous_response_id"])
	require.Equal(t, true, other["responses_state_had_conversation"])
	require.Equal(t, false, other["responses_state_had_context_management"])
	require.Equal(t, true, other["responses_state_had_prompt_cache_key"])
}

func mustMarshalTestRaw(v any) []byte {
	data, _ := common.Marshal(v)
	return data
}
