package openai

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

type responsesStreamAggregate struct {
	responseID       string
	responseCreated  bool
	completedSeen    bool
	textDeltaSeen    bool
	visibleOutput    bool
	lastModel        string
	outputText       strings.Builder
	outputs          []dto.ResponsesOutput
	usage            *dto.Usage
	lastResponse     *dto.OpenAIResponsesResponse
	synthReason      string
	completedPayload *dto.OpenAIResponsesResponse
}

func (a *responsesStreamAggregate) record(streamResponse dto.ResponsesStreamResponse) {
	if streamResponse.Type == "response.created" {
		a.responseCreated = true
	}
	if streamResponse.Response != nil {
		resp := streamResponse.Response
		a.lastResponse = resp
		if resp.ID != "" {
			a.responseID = resp.ID
		}
		if resp.Model != "" {
			a.lastModel = resp.Model
		}
		if len(resp.Output) > 0 {
			a.outputs = cloneResponsesOutput(resp.Output)
			if extractResponsesVisibleText(resp.Output) != "" {
				a.visibleOutput = true
			}
		}
		if resp.Usage != nil {
			usageCopy := *resp.Usage
			a.usage = &usageCopy
		}
	}
	switch streamResponse.Type {
	case "response.completed":
		a.completedSeen = true
	case "response.output_text.delta":
		a.textDeltaSeen = true
		if streamResponse.Delta != "" {
			a.visibleOutput = true
			a.outputText.WriteString(streamResponse.Delta)
		}
	case dto.ResponsesOutputTypeItemDone:
		if streamResponse.Item != nil {
			itemCopy := *streamResponse.Item
			a.outputs = append(a.outputs, itemCopy)
			if extractResponsesVisibleText([]dto.ResponsesOutput{itemCopy}) != "" {
				a.visibleOutput = true
			}
		}
	}
}

func (a *responsesStreamAggregate) shouldSynthesize(status *relaycommon.StreamStatus) bool {
	if a == nil || status == nil {
		return false
	}
	if a.completedSeen {
		return false
	}
	if status.HasErrors() {
		return false
	}
	switch status.EndReason {
	case relaycommon.StreamEndReasonEOF:
		a.synthReason = "eof_without_completed"
	case relaycommon.StreamEndReasonDone:
		a.synthReason = "done_without_completed"
	default:
		return false
	}
	return a.visibleOutput || len(a.outputs) > 0
}

func (a *responsesStreamAggregate) buildCompletedResponse(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) *dto.OpenAIResponsesResponse {
	if a == nil {
		return nil
	}
	if a.completedPayload != nil {
		return a.completedPayload
	}
	resp := &dto.OpenAIResponsesResponse{
		ID:        a.responseID,
		Object:    "response",
		Model:     a.resolvedModel(info),
		CreatedAt: int(time.Now().Unix()),
		Status:    json.RawMessage(`"completed"`),
		Output:    cloneResponsesOutput(a.outputs),
	}
	if resp.ID == "" {
		resp.ID = helper.GetResponseID(c)
	}
	if len(resp.Output) == 0 {
		if text := a.outputText.String(); text != "" {
			resp.Output = []dto.ResponsesOutput{{
				Type:   "message",
				ID:     "msg_" + common.GetUUID(),
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{{
					Type: "output_text",
					Text: text,
				}},
			}}
		}
	}
	if usageCopy := mergeResponsesUsage(a.usage, usage); usageCopy != nil {
		resp.Usage = usageCopy
	}
	a.completedPayload = resp
	return resp
}

func (a *responsesStreamAggregate) resolvedModel(info *relaycommon.RelayInfo) string {
	if a == nil {
		return ""
	}
	if a.lastModel != "" {
		return a.lastModel
	}
	if info != nil {
		if info.OriginModelName != "" {
			return info.OriginModelName
		}
		if info.UpstreamModelName != "" {
			return info.UpstreamModelName
		}
	}
	return ""
}

func cloneResponsesOutput(src []dto.ResponsesOutput) []dto.ResponsesOutput {
	if len(src) == 0 {
		return nil
	}
	dst := make([]dto.ResponsesOutput, len(src))
	for i := range src {
		dst[i] = src[i]
		if len(src[i].Content) > 0 {
			dst[i].Content = append([]dto.ResponsesOutputContent(nil), src[i].Content...)
		}
	}
	return dst
}

func extractResponsesVisibleText(outputs []dto.ResponsesOutput) string {
	var b strings.Builder
	for _, output := range outputs {
		for _, content := range output.Content {
			if content.Type == "output_text" && content.Text != "" {
				b.WriteString(content.Text)
			}
		}
	}
	return b.String()
}

func mergeResponsesUsage(streamUsage *dto.Usage, fallbackUsage *dto.Usage) *dto.Usage {
	if streamUsage == nil && fallbackUsage == nil {
		return nil
	}
	var merged dto.Usage
	if fallbackUsage != nil {
		merged = *fallbackUsage
	}
	if streamUsage != nil {
		if streamUsage.InputTokens != 0 {
			merged.InputTokens = streamUsage.InputTokens
		}
		if streamUsage.OutputTokens != 0 {
			merged.OutputTokens = streamUsage.OutputTokens
		}
		if streamUsage.TotalTokens != 0 {
			merged.TotalTokens = streamUsage.TotalTokens
		}
		if streamUsage.PromptTokens != 0 {
			merged.PromptTokens = streamUsage.PromptTokens
		}
		if streamUsage.CompletionTokens != 0 {
			merged.CompletionTokens = streamUsage.CompletionTokens
		}
		if streamUsage.InputTokensDetails != nil {
			details := *streamUsage.InputTokensDetails
			merged.InputTokensDetails = &details
			merged.PromptTokensDetails.CachedTokens = details.CachedTokens
		}
	}
	if merged.PromptTokens == 0 && merged.InputTokens != 0 {
		merged.PromptTokens = merged.InputTokens
	}
	if merged.CompletionTokens == 0 && merged.OutputTokens != 0 {
		merged.CompletionTokens = merged.OutputTokens
	}
	if merged.InputTokens == 0 && merged.PromptTokens != 0 {
		merged.InputTokens = merged.PromptTokens
	}
	if merged.OutputTokens == 0 && merged.CompletionTokens != 0 {
		merged.OutputTokens = merged.CompletionTokens
	}
	if merged.TotalTokens == 0 {
		merged.TotalTokens = merged.PromptTokens + merged.CompletionTokens
	}
	return &merged
}

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
		}
	}
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	aggregate := &responsesStreamAggregate{}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		aggregate.record(streamResponse)
		sendResponsesStreamData(c, streamResponse, data)
		switch streamResponse.Type {
		case "response.completed":
			common.SetContextKey(c, constant.ContextKeyResponsesCompletedSeen, true)
			if streamResponse.Response != nil {
				if streamResponse.Response.Usage != nil {
					if streamResponse.Response.Usage.InputTokens != 0 {
						usage.PromptTokens = streamResponse.Response.Usage.InputTokens
					}
					if streamResponse.Response.Usage.OutputTokens != 0 {
						usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
					}
					if streamResponse.Response.Usage.TotalTokens != 0 {
						usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
					}
					if streamResponse.Response.Usage.InputTokensDetails != nil {
						usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
					}
				}
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
			}
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
		case dto.ResponsesOutputTypeItemDone:
			// 函数调用处理
			if streamResponse.Item != nil {
				switch streamResponse.Item.Type {
				case dto.BuildInCallWebSearchCall:
					if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
						if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
							webSearchTool.CallCount++
						}
					}
				}
			}
		}
	})

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	if info != nil && info.StreamStatus != nil && aggregate.shouldSynthesize(info.StreamStatus) {
		completedResp := aggregate.buildCompletedResponse(c, info, usage)
		if completedResp != nil {
			payload := dto.ResponsesStreamResponse{
				Type:     "response.completed",
				Response: completedResp,
			}
			jsonData, err := common.Marshal(payload)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			}
			sendResponsesStreamData(c, payload, string(jsonData))
			helper.Done(c)
			common.SetContextKey(c, constant.ContextKeyResponsesCompletedSeen, true)
			common.SetContextKey(c, constant.ContextKeyResponsesCompletedSynthesized, true)
			common.SetContextKey(c, constant.ContextKeyResponsesCompletedSynthReason, aggregate.synthReason)
		}
	}

	return usage, nil
}
