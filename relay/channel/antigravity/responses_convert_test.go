package antigravity

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertResponsesRequestToChatRequestFiltersUnsupportedTools(t *testing.T) {
	req := dto.OpenAIResponsesRequest{
		Model: "gpt-5.4",
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
							Arguments: map[string]any{"city": "Tokyo"},
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
