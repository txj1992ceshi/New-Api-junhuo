package antigravity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func antigravityResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	var geminiResp dto.GeminiChatResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if err := common.Unmarshal(body, &geminiResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	oaiResp, usage := buildResponsesResponseFromGemini(c, info, &geminiResp)
	responseBody, err := common.Marshal(oaiResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

func antigravityResponsesCompactHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	var geminiResp dto.GeminiChatResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if err := common.Unmarshal(body, &geminiResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	responsesResp, usage := buildResponsesResponseFromGemini(c, info, &geminiResp)
	compactResp := dto.OpenAIResponsesCompactionResponse{
		ID:        responsesResp.ID,
		Object:    "response.compaction",
		CreatedAt: int(responsesResp.CreatedAt),
		Output:    buildCompactOutput(responsesResp),
		Usage:     usage,
	}
	responseBody, err := common.Marshal(compactResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

func antigravityResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	helper.SetEventStreamHeaders(c)

	responseID := helper.GetResponseID(c)
	createdAt := int(time.Now().Unix())
	usage := &dto.Usage{}
	emittedCreated := false
	outputIndex := 0
	var aggregatedText strings.Builder
	state := &antigravityResponsesStreamState{}

	sendEvent := func(event dto.ResponsesStreamResponse) error {
		data, err := common.Marshal(event)
		if err != nil {
			return err
		}
		helper.ResponseChunkData(c, event, string(data))
		return nil
	}

	sendCreated := func() error {
		if emittedCreated {
			return nil
		}
		emittedCreated = true
		state.createdEmitted = true
		return sendEvent(dto.ResponsesStreamResponse{
			Type: "response.created",
			Response: &dto.OpenAIResponsesResponse{
				ID:        responseID,
				Object:    "response",
				CreatedAt: createdAt,
				Model:     info.OriginModelName,
				Output:    []dto.ResponsesOutput{},
			},
		})
	}

	var latestResponse *dto.OpenAIResponsesResponse
	var streamErr *types.NewAPIError
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		var geminiResp dto.GeminiChatResponse
		if err := common.UnmarshalJsonStr(data, &geminiResp); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(err)
			return
		}
		currentResp, currentUsage := buildResponsesResponseFromGemini(c, info, &geminiResp)
		latestResponse = currentResp
		*usage = *currentUsage
		state.candidateCount += len(geminiResp.Candidates)
		state.completionTokens = usage.OutputTokens
		for _, candidate := range geminiResp.Candidates {
			if candidate.FinishReason != nil {
				reason := strings.TrimSpace(common.Interface2String(*candidate.FinishReason))
				if reason != "" {
					state.finishReasons = append(state.finishReasons, reason)
				}
			}
		}

		if err := sendCreated(); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			sr.Stop(err)
			return
		}

		for _, out := range currentResp.Output {
			switch out.Type {
			case "message":
				text := extractResponsesOutputText(out)
				if text == "" {
					continue
				}
				aggregatedText.WriteString(text)
				state.textDeltaEmitted = true
				state.visibleOutputPresent = true
				event := dto.ResponsesStreamResponse{
					Type:        "response.output_text.delta",
					Delta:       text,
					OutputIndex: common.GetPointer(outputIndex),
					ContentIndex: common.GetPointer(0),
				}
				if err := sendEvent(event); err != nil {
					streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
					sr.Stop(err)
					return
				}
				outputIndex++
			case "function_call":
				state.itemDoneEmitted = true
				item := out
				event := dto.ResponsesStreamResponse{
					Type:        dto.ResponsesOutputTypeItemDone,
					Item:        &item,
					OutputIndex: common.GetPointer(outputIndex),
					ItemID:      item.ID,
				}
				if err := sendEvent(event); err != nil {
					streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
					sr.Stop(err)
					return
				}
				outputIndex++
			}
		}

		if hasGeminiStop(&geminiResp) {
			if latestResponse != nil {
				setAntigravityResponsesStreamContext(c, state)
				if shouldFailAntigravityResponsesEmptyStream(state, latestResponse, usage) {
					common.SetContextKey(c, constant.ContextKeyAntigravityResponsesEmptyStream, true)
					common.SetContextKey(c, constant.ContextKeyAntigravityResponsesEmptyReason, antigravityEmptyStreamReason(state))
					if usage != nil && usage.OutputTokens >= 0 {
						common.SetContextKey(c, constant.ContextKeyAntigravityResponsesCompletionToken, usage.OutputTokens)
					}
					logger.LogInfo(c, fmt.Sprintf("antigravity responses empty stream detected: request_path=%s origin_model_name=%s upstream_model_name=%s reason=%s candidates=%d completion_tokens=%d finish_reasons=%s",
						info.RequestURLPath, info.OriginModelName, info.UpstreamModelName, antigravityEmptyStreamReason(state), state.candidateCount, usage.OutputTokens, summarizeFinishReasons(state.finishReasons)))
				}
				fillMissingResponsesText(latestResponse, aggregatedText.String())
				event := dto.ResponsesStreamResponse{
					Type:     "response.completed",
					Response: latestResponse,
				}
				if err := sendEvent(event); err != nil {
					streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
					sr.Stop(err)
					return
				}
			}
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}
	return usage, nil
}

type antigravityResponsesStreamState struct {
	createdEmitted       bool
	textDeltaEmitted     bool
	itemDoneEmitted      bool
	visibleOutputPresent bool
	candidateCount       int
	completionTokens     int
	finishReasons        []string
}

func buildResponsesResponseFromGemini(c *gin.Context, info *relaycommon.RelayInfo, geminiResp *dto.GeminiChatResponse) (*dto.OpenAIResponsesResponse, *dto.Usage) {
	usage := buildResponsesUsageFromGemini(geminiResp.UsageMetadata, info.GetEstimatePromptTokens())
	output := make([]dto.ResponsesOutput, 0)
	for _, candidate := range geminiResp.Candidates {
		textParts := make([]dto.ResponsesOutputContent, 0)
		for _, part := range candidate.Content.Parts {
			switch {
			case part.FunctionCall != nil:
				output = append(output, dto.ResponsesOutput{
					Type:      "function_call",
					ID:        "fc_" + common.GetUUID(),
					Status:    "completed",
					CallId:    "call_" + common.GetUUID(),
					Name:      part.FunctionCall.FunctionName,
					Arguments: mustMarshalString(part.FunctionCall.Arguments),
				})
			case part.Text != "" && !part.Thought:
				textParts = append(textParts, dto.ResponsesOutputContent{
					Type: "output_text",
					Text: part.Text,
				})
			}
		}
		if len(textParts) > 0 {
			output = append(output, dto.ResponsesOutput{
				Type:    "message",
				ID:      "msg_" + common.GetUUID(),
				Status:  "completed",
				Role:    "assistant",
				Content: textParts,
			})
		}
		_ = mapGeminiFinishReason(candidate.FinishReason)
	}
	if len(output) == 0 {
		output = append(output, dto.ResponsesOutput{
			Type:   "message",
			ID:     "msg_" + common.GetUUID(),
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{{
				Type: "output_text",
				Text: "",
			}},
		})
	}
	return &dto.OpenAIResponsesResponse{
		ID:                 helper.GetResponseID(c),
		Object:             "response",
		CreatedAt:          int(common.GetTimestamp()),
		Model:              info.OriginModelName,
		Output:             output,
		ParallelToolCalls:  false,
		PreviousResponseID: nil,
		Usage:              usage,
	}, usage
}

func buildResponsesUsageFromGemini(metadata dto.GeminiUsageMetadata, fallbackPromptTokens int) *dto.Usage {
	promptTokens := metadata.PromptTokenCount + metadata.ToolUsePromptTokenCount
	if promptTokens <= 0 && fallbackPromptTokens > 0 {
		promptTokens = fallbackPromptTokens
	}
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: metadata.CandidatesTokenCount + metadata.ThoughtsTokenCount,
		TotalTokens:      metadata.TotalTokenCount,
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	usage.InputTokens = usage.PromptTokens
	usage.OutputTokens = usage.CompletionTokens
	usage.PromptTokensDetails.CachedTokens = metadata.CachedContentTokenCount
	usage.CompletionTokenDetails.ReasoningTokens = metadata.ThoughtsTokenCount
	return usage
}

func hasGeminiStop(resp *dto.GeminiChatResponse) bool {
	if resp == nil {
		return false
	}
	for _, candidate := range resp.Candidates {
		if candidate.FinishReason != nil {
			return true
		}
	}
	return false
}

func extractResponsesOutputText(out dto.ResponsesOutput) string {
	if len(out.Content) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, part := range out.Content {
		if part.Type == "output_text" && part.Text != "" {
			sb.WriteString(part.Text)
		}
	}
	return sb.String()
}

func buildCompactOutput(resp *dto.OpenAIResponsesResponse) json.RawMessage {
	output := make([]map[string]any, 0, len(resp.Output))
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			output = append(output, map[string]any{
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"content": item.Content,
			})
		case "function_call":
			output = append(output, map[string]any{
				"type":      "function_call",
				"call_id":   item.CallId,
				"name":      item.Name,
				"arguments": item.Arguments,
				"status":    "completed",
			})
		}
	}
	if len(output) == 0 {
		output = append(output, map[string]any{
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []map[string]any{{
				"type": "output_text",
				"text": "",
			}},
		})
	}
	data, _ := common.Marshal(output)
	return data
}

func mustMarshalString(v any) string {
	switch vv := v.(type) {
	case nil:
		return "{}"
	case string:
		if strings.TrimSpace(vv) == "" {
			return "{}"
		}
		return vv
	default:
		data, err := common.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

func fillMissingResponsesText(resp *dto.OpenAIResponsesResponse, text string) {
	if resp == nil || text == "" {
		return
	}
	for i := range resp.Output {
		if resp.Output[i].Type != "message" {
			continue
		}
		if extractResponsesOutputText(resp.Output[i]) != "" {
			return
		}
		resp.Output[i].Content = []dto.ResponsesOutputContent{{
			Type: "output_text",
			Text: text,
		}}
		return
	}
	resp.Output = append(resp.Output, dto.ResponsesOutput{
		Type:   "message",
		ID:     "msg_" + common.GetUUID(),
		Status: "completed",
		Role:   "assistant",
		Content: []dto.ResponsesOutputContent{{
			Type: "output_text",
			Text: text,
		}},
	})
}

func shouldFailAntigravityResponsesEmptyStream(state *antigravityResponsesStreamState, resp *dto.OpenAIResponsesResponse, usage *dto.Usage) bool {
	if state == nil {
		return false
	}
	if state.visibleOutputPresent || state.textDeltaEmitted {
		return false
	}
	if usage != nil && usage.OutputTokens > 0 {
		return false
	}
	if resp == nil {
		return true
	}
	for _, out := range resp.Output {
		if extractResponsesOutputText(out) != "" {
			return false
		}
	}
	return true
}

func antigravityEmptyStreamReason(state *antigravityResponsesStreamState) string {
	if state == nil {
		return "unknown"
	}
	if state.itemDoneEmitted {
		return "no_visible_assistant_output_with_tool_items"
	}
	if state.createdEmitted {
		return "no_visible_assistant_output"
	}
	return "no_stream_events_emitted"
}

func summarizeFinishReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	uniq := make([]string, 0, len(reasons))
	seen := make(map[string]struct{}, len(reasons))
	for _, reason := range reasons {
		if reason == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		uniq = append(uniq, reason)
	}
	return strings.Join(uniq, ",")
}

func setAntigravityResponsesStreamContext(c *gin.Context, state *antigravityResponsesStreamState) {
	if c == nil || state == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesStreamCreated, state.createdEmitted)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesTextDelta, state.textDeltaEmitted)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesItemDone, state.itemDoneEmitted)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesVisibleOutput, state.visibleOutputPresent)
	if state.candidateCount > 0 {
		common.SetContextKey(c, constant.ContextKeyAntigravityResponsesCandidateCount, state.candidateCount)
	}
	if state.completionTokens > 0 {
		common.SetContextKey(c, constant.ContextKeyAntigravityResponsesCompletionToken, state.completionTokens)
	}
	if summary := summarizeFinishReasons(state.finishReasons); summary != "" {
		common.SetContextKey(c, constant.ContextKeyAntigravityResponsesFinishSummary, summary)
	}
}

func setAntigravityResponsesEmptyFailureContext(c *gin.Context, state *antigravityResponsesStreamState, usage *dto.Usage) {
	setAntigravityResponsesStreamContext(c, state)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesEmptyStream, true)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesFailedAsEmpty, true)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesEmptyReason, antigravityEmptyStreamReason(state))
	common.SetContextKey(c, constant.ContextKeyAntigravityErrorClass, string(service.AntigravityErrorClassProtocolIncompatible))
	if usage != nil && usage.OutputTokens >= 0 {
		common.SetContextKey(c, constant.ContextKeyAntigravityResponsesCompletionToken, usage.OutputTokens)
	}
}
