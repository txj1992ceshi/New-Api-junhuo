package antigravity

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertResponsesRequestToChatRequestFiltersUnsupportedTools(t *testing.T) {
	req := dto.OpenAIResponsesRequest{
		Model:        "gpt-5.4",
		Instructions: mustMarshalRaw("be terse"),
		Input: mustMarshalRaw([]map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "hello"},
				},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  map[string]any{"ok": true},
			},
		}),
		ToolChoice: mustMarshalRaw("required"),
		Tools: mustMarshalRaw([]map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "lookup_weather",
					"description": "lookup weather",
					"parameters": map[string]any{
						"type": "object",
					},
				},
			},
			{
				"type": "computer",
			},
		}),
	}

	chatReq, filtered, err := convertResponsesRequestToChatRequest(req)
	require.NoError(t, err)
	require.Equal(t, []string{"computer"}, filtered)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "lookup_weather", chatReq.Tools[0].Function.Name)
	require.Equal(t, "required", chatReq.ToolChoice)
	require.Len(t, chatReq.Messages, 3)
	require.Equal(t, "developer", chatReq.Messages[0].Role)
	require.Equal(t, "be terse", chatReq.Messages[0].Content)
	require.Equal(t, "user", chatReq.Messages[1].Role)
	require.Equal(t, "hello", chatReq.Messages[1].StringContent())
	require.Equal(t, "tool", chatReq.Messages[2].Role)
	require.Equal(t, "call_1", chatReq.Messages[2].ToolCallId)
	require.JSONEq(t, `{"ok":true}`, chatReq.Messages[2].StringContent())
}

func TestConvertResponsesRequestToChatRequestDropsUnsupportedToolState(t *testing.T) {
	req := dto.OpenAIResponsesRequest{
		Model: "gpt-5.4",
		Input: mustMarshalRaw([]map[string]any{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":      "function_call",
						"name":      "web_search",
						"call_id":   "call_bad",
						"arguments": "{}",
					},
				},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_bad",
				"output":  "should be dropped",
			},
			{
				"type":      "function_call",
				"name":      "lookup_weather",
				"call_id":   "call_good",
				"arguments": "{\"city\":\"Tokyo\"}",
			},
			{
				"type":    "function_call_output",
				"call_id": "call_good",
				"output":  map[string]any{"ok": true},
			},
		}),
		ToolChoice: mustMarshalRaw(map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "web_search",
			},
		}),
		Tools: mustMarshalRaw([]map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name": "lookup_weather",
				},
			},
			{
				"type": "web_search",
			},
		}),
	}

	chatReq, filtered, err := convertResponsesRequestToChatRequest(req)
	require.NoError(t, err)
	require.Equal(t, []string{"web_search"}, filtered)
	require.Nil(t, chatReq.ToolChoice)
	require.Len(t, chatReq.Tools, 1)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.Len(t, chatReq.Messages[0].ParseToolCalls(), 1)
	require.Equal(t, "lookup_weather", chatReq.Messages[0].ParseToolCalls()[0].Function.Name)
	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Equal(t, "call_good", chatReq.Messages[1].ToolCallId)
	require.JSONEq(t, `{"ok":true}`, chatReq.Messages[1].StringContent())
}

func TestBuildGeminiRequestFromResponsesIncludesSystemAndFunctionTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       58,
			UpstreamModelName: "gpt-5.4",
		},
	}
	req := dto.OpenAIResponsesRequest{
		Model:        "gpt-5.4",
		Instructions: mustMarshalRaw("system note"),
		Input: mustMarshalRaw([]map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "write code"},
				},
			},
		}),
		Tools: mustMarshalRaw([]map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name": "apply_patch",
				},
			},
		}),
	}

	geminiReq, err := buildGeminiRequestFromResponses(ctx, info, req)
	require.NoError(t, err)
	require.NotNil(t, geminiReq.SystemInstructions)
	require.Equal(t, "system note", geminiReq.SystemInstructions.Parts[0].Text)
	require.Len(t, geminiReq.Contents, 1)
	require.Equal(t, "user", geminiReq.Contents[0].Role)
	require.Equal(t, "write code", geminiReq.Contents[0].Parts[0].Text)
	tools := geminiReq.GetTools()
	require.Len(t, tools, 1)
	data, err := common.Marshal(tools[0].FunctionDeclarations)
	require.NoError(t, err)
	require.Contains(t, string(data), "apply_patch")
}

func TestBuildGeminiRequestFromResponsesNoToolsFor55Compatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       58,
			UpstreamModelName: "gpt-5.5",
		},
	}
	req := dto.OpenAIResponsesRequest{
		Model:        "gpt-5.5",
		Instructions: mustMarshalRaw("system note"),
		Input: mustMarshalRaw([]map[string]any{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": "tool call happened"},
					{"type": "function_call", "name": "web_search", "call_id": "call_0", "arguments": "{}"},
				},
			},
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "write code"},
				},
			},
		}),
		ToolChoice: mustMarshalRaw("required"),
		Tools: mustMarshalRaw([]map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name": "apply_patch",
				},
			},
		}),
	}

	compat := service.ApplyResponsesCompatibilityProfile(ctx, relayconstant.RelayModeResponses, constant.ChannelTypeAntigravity, "gpt-5.5", &req)
	require.True(t, compat.RemovedTools)
	require.True(t, compat.RemovedToolChoice)

	geminiReq, err := buildGeminiRequestFromResponses(ctx, info, req)
	require.NoError(t, err)
	require.Nil(t, geminiReq.ToolConfig)
	require.Empty(t, geminiReq.GetTools())
	require.Len(t, geminiReq.Contents, 1)
	require.Equal(t, "user", geminiReq.Contents[0].Role)
	require.Equal(t, "write code", geminiReq.Contents[0].Parts[0].Text)
}

func TestBuildResponsesResponseFromGeminiProducesMessageAndFunctionCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4",
	}
	info.SetEstimatePromptTokens(11)
	resp, usage := buildResponsesResponseFromGemini(ctx, info, &dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
						{FunctionCall: &dto.FunctionCall{
							FunctionName: "lookup_weather",
							Arguments:    map[string]any{"city": "Tokyo"},
						}},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     11,
			CandidatesTokenCount: 7,
			TotalTokenCount:      18,
		},
	})
	require.NotNil(t, resp)
	require.NotNil(t, usage)
	require.Equal(t, "response", resp.Object)
	require.Equal(t, "gpt-5.4", resp.Model)
	require.Len(t, resp.Output, 2)
	require.Equal(t, "function_call", resp.Output[0].Type)
	require.Equal(t, "lookup_weather", resp.Output[0].Name)
	require.JSONEq(t, `{"city":"Tokyo"}`, resp.Output[0].Arguments)
	require.Equal(t, "message", resp.Output[1].Type)
	require.Equal(t, "hello", resp.Output[1].Content[0].Text)
	require.Equal(t, 11, usage.InputTokens)
	require.Equal(t, 7, usage.OutputTokens)
}

func TestBuildCompactOutputReturnsMessageArray(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Output: []dto.ResponsesOutput{
			{
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []dto.ResponsesOutputContent{{
					Type: "output_text",
					Text: "summary text",
				}},
			},
		},
	}
	output := buildCompactOutput(resp)
	var decoded []map[string]any
	err := json.Unmarshal(output, &decoded)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	require.Equal(t, "message", decoded[0]["type"])
}

func TestNormalizeAntigravitySystemInstructionUsesUserRole(t *testing.T) {
	req := &dto.GeminiChatRequest{
		SystemInstructions: &dto.GeminiChatContent{
			Role: "developer",
			Parts: []dto.GeminiPart{{
				Text: "be terse",
			}},
		},
	}

	normalizeAntigravityGeminiRequest(req)
	require.Equal(t, "user", req.SystemInstructions.Role)
}

func TestDeriveAntigravitySessionIDIsStable(t *testing.T) {
	req := &dto.GeminiChatRequest{
		SystemInstructions: &dto.GeminiChatContent{
			Role:  "user",
			Parts: []dto.GeminiPart{{Text: "be terse"}},
		},
		Contents: []dto.GeminiChatContent{{
			Role:  "user",
			Parts: []dto.GeminiPart{{Text: "hello"}},
		}},
	}

	s1 := deriveAntigravitySessionID(req, "gemini-3-flash", "proj-1")
	s2 := deriveAntigravitySessionID(req, "gemini-3-flash", "proj-1")
	require.Equal(t, s1, s2)
	require.Contains(t, s1, "newapi-antigravity:")
}

func TestConvertOpenAIResponsesRequestReusesChatEnvelopePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)

	keyJSON := mustMarshalRaw(map[string]any{
		"access_token":  "test-access",
		"refresh_token": "test-refresh",
		"project_id":    "proj-1",
		"type":          "antigravity",
		"expired":       time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	})
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "gpt-5.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAntigravity,
			UpstreamModelName: "gpt-5.5",
			ApiKey:            string(keyJSON),
		},
	}

	chatReq := &dto.GeneralOpenAIRequest{
		Model: "gpt-5.5",
		Messages: []dto.Message{
			{Role: "developer", Content: "Be concise."},
			{Role: "user", Content: "Say hello."},
		},
		Stream: common.GetPointer(true),
	}
	respReq := dto.OpenAIResponsesRequest{
		Model:        "gpt-5.5",
		Stream:       common.GetPointer(true),
		Instructions: mustMarshalRaw("Be concise."),
		Input: mustMarshalRaw([]map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Say hello."},
				},
			},
		}),
	}

	adaptor := &Adaptor{}
	chatAny, err := adaptor.ConvertOpenAIRequest(ctx, info, chatReq)
	require.NoError(t, err)
	respAny, err := adaptor.ConvertOpenAIResponsesRequest(ctx, info, respReq)
	require.NoError(t, err)

	chatEnvelope := chatAny.(*requestEnvelope)
	respEnvelope := respAny.(*requestEnvelope)
	require.Equal(t, chatEnvelope.Project, respEnvelope.Project)
	require.Equal(t, chatEnvelope.Model, respEnvelope.Model)
	require.Equal(t, chatEnvelope.RequestType, respEnvelope.RequestType)
	require.Equal(t, chatEnvelope.UserAgent, respEnvelope.UserAgent)
	require.Equal(t, chatEnvelope.Request.SessionID, respEnvelope.Request.SessionID)
	require.Equal(t, "user", respEnvelope.Request.SystemInstructions.Role)
	require.Equal(t, 1, len(respEnvelope.Request.Contents))
	require.Equal(t, "Say hello.", respEnvelope.Request.Contents[0].Parts[0].Text)
}

func TestAntigravityResponsesStreamHandlerFailsEmptyStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	defer func() {
		constant.StreamingTimeout = oldTimeout
	}()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Set(common.RequestIdKey, "req-empty")

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash",
		},
	}
	info.SetEstimatePromptTokens(12)
	stop := "STOP"

	chunk, err := common.Marshal(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{Role: "model"},
				FinishReason: &stop,
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     12,
			CandidatesTokenCount: 0,
			TotalTokenCount:      12,
		},
	})
	require.NoError(t, err)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("data: " + string(chunk) + "\n\n")),
	}

	usage, apiErr := antigravityResponsesStreamHandler(ctx, info, resp)
	require.NotNil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "Antigravity Responses compatibility empty output")
	require.Contains(t, rec.Body.String(), "event: response.created")
	require.NotContains(t, rec.Body.String(), "event: response.failed")
	require.NotContains(t, rec.Body.String(), "event: response.completed")
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityResponsesEmptyStream))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityResponsesFailedAsEmpty))
	require.Equal(t, "no_visible_assistant_output", common.GetContextKeyString(ctx, constant.ContextKeyAntigravityResponsesEmptyReason))
	require.Equal(t, string(service.AntigravityErrorClassProtocolIncompatible), common.GetContextKeyString(ctx, constant.ContextKeyAntigravityErrorClass))
}

func TestAntigravityResponsesStreamHandlerKeepsSuccessfulTextStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	defer func() {
		constant.StreamingTimeout = oldTimeout
	}()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Set(common.RequestIdKey, "req-ok")

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-flash",
		},
	}
	info.SetEstimatePromptTokens(10)
	stop := "STOP"

	chunk, err := common.Marshal(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{{Text: "hello from antigravity"}},
				},
				FinishReason: &stop,
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 4,
			TotalTokenCount:      14,
		},
	})
	require.NoError(t, err)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("data: " + string(chunk) + "\n\n")),
	}

	usage, apiErr := antigravityResponsesStreamHandler(ctx, info, resp)
	require.Nil(t, apiErr, "%+v", apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 4, usage.OutputTokens)
	require.Contains(t, rec.Body.String(), "event: response.created")
	require.Contains(t, rec.Body.String(), "event: response.output_text.delta")
	require.Contains(t, rec.Body.String(), "event: response.completed")
	require.NotContains(t, rec.Body.String(), "event: response.failed")
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityResponsesTextDelta))
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityResponsesVisibleOutput))
	require.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityResponsesEmptyStream))
}
