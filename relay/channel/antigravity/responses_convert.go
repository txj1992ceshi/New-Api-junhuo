package antigravity

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

type responsesInputItem struct {
	Type      string          `json:"type,omitempty"`
	Role      string          `json:"role,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Output    any             `json:"output,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
}

func buildGeminiRequestFromResponses(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (*dto.GeminiChatRequest, error) {
	chatReq, filteredTools, err := convertResponsesRequestToChatRequest(request)
	if err != nil {
		return nil, err
	}
	if len(filteredTools) > 0 {
		logger.LogInfo(c, fmt.Sprintf("antigravity filtered unsupported responses tools: %s", strings.Join(filteredTools, ", ")))
	}
	if info != nil && info.ChannelSetting.SystemPrompt != "" {
		applyAntigravitySystemPrompt(info, chatReq)
	}
	return gemini.CovertOpenAI2Gemini(c, *chatReq, info)
}

func convertResponsesRequestToChatRequest(request dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, []string, error) {
	chatReq := &dto.GeneralOpenAIRequest{
		Model:       request.Model,
		Messages:    make([]dto.Message, 0),
		Stream:      request.Stream,
		Temperature: request.Temperature,
		TopP:        request.TopP,
	}
	if request.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = request.MaxOutputTokens
	}
	if request.Reasoning != nil {
		chatReq.ReasoningEffort = strings.TrimSpace(request.Reasoning.Effort)
	}

	if len(request.Tools) > 0 {
		tools, filtered := filterResponsesTools(request.GetToolsMap())
		chatReq.Tools = tools
		if len(tools) > 0 {
			if toolChoice, ok := normalizeResponsesToolChoice(request.ToolChoice); ok {
				chatReq.ToolChoice = toolChoice
			}
			return populateResponsesMessages(chatReq, request, filtered)
		}
		return populateResponsesMessages(chatReq, request, filtered)
	}
	return populateResponsesMessages(chatReq, request, nil)
}

func populateResponsesMessages(chatReq *dto.GeneralOpenAIRequest, request dto.OpenAIResponsesRequest, filtered []string) (*dto.GeneralOpenAIRequest, []string, error) {
	if len(request.Instructions) > 0 {
		var instructions string
		if err := common.Unmarshal(request.Instructions, &instructions); err == nil {
			if strings.TrimSpace(instructions) != "" {
				chatReq.Messages = append(chatReq.Messages, dto.Message{
					Role:    antigravitySystemRoleName(chatReq.Model),
					Content: instructions,
				})
			}
		}
	}

	inputs, err := parseResponsesInputItems(request.Input)
	if err != nil {
		return nil, filtered, err
	}
	for _, item := range inputs {
		switch item.Type {
		case "", "message":
			msg, err := mapResponsesMessageItem(item)
			if err != nil {
				return nil, filtered, err
			}
			if msg != nil {
				chatReq.Messages = append(chatReq.Messages, *msg)
			}
		case "function_call_output":
			msg := mapResponsesFunctionOutputItem(item)
			chatReq.Messages = append(chatReq.Messages, msg)
		case "function_call":
			msg, err := mapResponsesFunctionCallItem(item)
			if err != nil {
				return nil, filtered, err
			}
			if msg != nil {
				chatReq.Messages = append(chatReq.Messages, *msg)
			}
		case "input_text":
			chatReq.Messages = append(chatReq.Messages, dto.Message{
				Role:    "user",
				Content: item.ContentString(),
			})
		default:
			// Ignore unsupported input item types rather than failing the request.
		}
	}
	return chatReq, filtered, nil
}

func filterResponsesTools(input []map[string]any) ([]dto.ToolCallRequest, []string) {
	tools := make([]dto.ToolCallRequest, 0, len(input))
	filtered := make([]string, 0)
	for _, tool := range input {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType != "function" {
			if toolType != "" {
				filtered = append(filtered, toolType)
			}
			continue
		}
		fnMap, ok := tool["function"].(map[string]any)
		if !ok {
			filtered = append(filtered, "function(invalid)")
			continue
		}
		name := strings.TrimSpace(common.Interface2String(fnMap["name"]))
		if name == "" {
			filtered = append(filtered, "function(unnamed)")
			continue
		}
		tools = append(tools, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        name,
				Description: common.Interface2String(fnMap["description"]),
				Parameters:  fnMap["parameters"],
			},
		})
	}
	return tools, filtered
}

func normalizeResponsesToolChoice(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	rawType := common.GetJsonType(raw)
	switch rawType {
	case "string":
		var v string
		if err := common.Unmarshal(raw, &v); err != nil {
			return nil, false
		}
		v = strings.TrimSpace(v)
		switch v {
		case "auto", "none", "required":
			return v, true
		default:
			return nil, false
		}
	case "object":
		var m map[string]any
		if err := common.Unmarshal(raw, &m); err != nil {
			return nil, false
		}
		toolType := strings.TrimSpace(common.Interface2String(m["type"]))
		if toolType != "function" {
			return nil, false
		}
		return m, true
	default:
		return nil, false
	}
}

func parseResponsesInputItems(raw json.RawMessage) ([]responsesInputItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	switch common.GetJsonType(raw) {
	case "string":
		var text string
		if err := common.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		return []responsesInputItem{{Type: "input_text", Role: "user", Content: mustMarshalRaw(text)}}, nil
	case "array":
		var items []responsesInputItem
		if err := common.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		return items, nil
	default:
		return nil, nil
	}
}

func mapResponsesMessageItem(item responsesInputItem) (*dto.Message, error) {
	role := strings.TrimSpace(item.Role)
	if role == "" {
		role = "user"
	}
	if len(item.Content) == 0 {
		return &dto.Message{Role: role, Content: ""}, nil
	}
	if common.GetJsonType(item.Content) == "string" {
		var text string
		if err := common.Unmarshal(item.Content, &text); err != nil {
			return nil, err
		}
		return &dto.Message{Role: role, Content: text}, nil
	}
	if common.GetJsonType(item.Content) != "array" {
		return &dto.Message{Role: role, Content: string(item.Content)}, nil
	}

	var parts []map[string]any
	if err := common.Unmarshal(item.Content, &parts); err != nil {
		return nil, err
	}
	contentParts := make([]dto.MediaContent, 0, len(parts))
	toolCalls := make([]dto.ToolCallResponse, 0)
	for _, part := range parts {
		partType := strings.TrimSpace(common.Interface2String(part["type"]))
		switch partType {
		case "input_text", "output_text", "text":
			contentParts = append(contentParts, dto.MediaContent{
				Type: dto.ContentTypeText,
				Text: common.Interface2String(part["text"]),
			})
		case "input_image":
			contentParts = append(contentParts, dto.MediaContent{
				Type:     dto.ContentTypeImageURL,
				ImageUrl: part["image_url"],
			})
		case "input_file":
			contentParts = append(contentParts, dto.MediaContent{
				Type: dto.ContentTypeFile,
				File: part["file"],
			})
		case "function_call":
			name := strings.TrimSpace(common.Interface2String(part["name"]))
			if name == "" {
				continue
			}
			callID := strings.TrimSpace(common.Interface2String(part["call_id"]))
			if callID == "" {
				callID = "call_" + common.GetUUID()
			}
			toolCalls = append(toolCalls, dto.ToolCallResponse{
				ID:   callID,
				Type: "function",
				Function: dto.FunctionResponse{
					Name:      name,
					Arguments: common.Interface2String(part["arguments"]),
				},
			})
		}
	}

	msg := &dto.Message{Role: role}
	if len(contentParts) == 0 {
		msg.Content = ""
	} else if len(contentParts) == 1 && contentParts[0].Type == dto.ContentTypeText {
		msg.Content = contentParts[0].Text
	} else {
		msg.Content = contentParts
	}
	if len(toolCalls) > 0 {
		msg.SetToolCalls(toolCalls)
	}
	return msg, nil
}

func mapResponsesFunctionOutputItem(item responsesInputItem) dto.Message {
	output := item.OutputString()
	callID := strings.TrimSpace(item.CallID)
	if callID == "" {
		callID = "call_" + common.GetUUID()
	}
	return dto.Message{
		Role:       "tool",
		Content:    output,
		ToolCallId: callID,
	}
}

func mapResponsesFunctionCallItem(item responsesInputItem) (*dto.Message, error) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return nil, nil
	}
	callID := strings.TrimSpace(item.CallID)
	if callID == "" {
		callID = "call_" + common.GetUUID()
	}
	msg := &dto.Message{Role: "assistant", Content: ""}
	msg.SetToolCalls([]dto.ToolCallResponse{{
		ID:   callID,
		Type: "function",
		Function: dto.FunctionResponse{
			Name:      name,
			Arguments: item.Arguments,
		},
	}})
	return msg, nil
}

func applyAntigravitySystemPrompt(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) {
	if info == nil || request == nil || info.ChannelSetting.SystemPrompt == "" {
		return
	}
	systemRole := request.GetSystemRoleName()
	for i := range request.Messages {
		if request.Messages[i].Role != systemRole {
			continue
		}
		if !info.ChannelSetting.SystemPromptOverride {
			return
		}
		if request.Messages[i].IsStringContent() {
			request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + request.Messages[i].StringContent())
			return
		}
	}
	request.Messages = append([]dto.Message{{
		Role:    systemRole,
		Content: info.ChannelSetting.SystemPrompt,
	}}, request.Messages...)
}

func mustMarshalRaw(v any) json.RawMessage {
	data, _ := common.Marshal(v)
	return data
}

func (i responsesInputItem) ContentString() string {
	if len(i.Content) == 0 {
		return ""
	}
	if common.GetJsonType(i.Content) == "string" {
		var text string
		_ = common.Unmarshal(i.Content, &text)
		return text
	}
	return string(i.Content)
}

func (i responsesInputItem) OutputString() string {
	switch v := i.Output.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		data, err := common.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}
