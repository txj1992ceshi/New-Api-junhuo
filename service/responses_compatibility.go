package service

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

const ResponsesCompatProfileStatelessV1 = "stateless_responses_v1"
const ResponsesCompatProfileStatelessV2Antigravity = "responses_stateless_v2_antigravity"
const ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55 = "responses_stateless_v2_antigravity_no_tools_gpt55"

type ResponsesCompatibilityProfile struct {
	Name         string
	Mode         string
	ForceNoTools bool
	ToolStrategy string
}

type ResponsesCompatibilityResult struct {
	Applied                 bool
	Profile                 string
	Mode                    string
	RemovedFields           []string
	NormalizedInput         bool
	RemovedToolItems        int
	RemovedHistoryItems     int
	RemovedInclude          bool
	RemovedStore            bool
	RemovedParallelToolCall bool
	RemovedTools            bool
	RemovedToolChoice       bool
	HadPreviousResponseID   bool
	HadConversation         bool
	HadContextManagement    bool
	HadPromptCacheKey       bool
	HadPromptCacheRetention bool
	HadStatefulMetadata     bool
	ChannelType             int
	OriginModel             string
	ToolStrategy            string
}

var responsesCompatibilityProfiles = map[int]map[string]ResponsesCompatibilityProfile{
	relayconstant.RelayModeResponses: {
		"gpt-5.4":      {Name: ResponsesCompatProfileStatelessV1, Mode: "stateless"},
		"gpt-5.4-mini": {Name: ResponsesCompatProfileStatelessV1, Mode: "stateless"},
	},
	relayconstant.RelayModeResponsesCompact: {
		"gpt-5.4-openai-compact":      {Name: ResponsesCompatProfileStatelessV1, Mode: "stateless"},
		"gpt-5.4-mini-openai-compact": {Name: ResponsesCompatProfileStatelessV1, Mode: "stateless"},
	},
}

var responsesStateMetadataKeys = map[string]struct{}{
	"conversation_id":      {},
	"conversationId":       {},
	"session_id":           {},
	"sessionId":            {},
	"turn_id":              {},
	"turnId":               {},
	"parent_turn_id":       {},
	"parentTurnId":         {},
	"previous_response_id": {},
	"previousResponseId":   {},
}

func ApplyResponsesCompatibilityProfile(c *gin.Context, relayMode int, channelType int, originModel string, request *dto.OpenAIResponsesRequest) ResponsesCompatibilityResult {
	result := ResponsesCompatibilityResult{
		ChannelType: channelType,
		OriginModel: strings.TrimSpace(originModel),
	}
	profile, ok := resolveResponsesCompatibilityProfile(relayMode, result.OriginModel, channelType)
	if request == nil || !ok {
		recordResponsesCompatibilityResult(c, result)
		return result
	}

	result.Profile = profile.Name
	result.Mode = profile.Mode
	result.ToolStrategy = profile.ToolStrategy

	applyStatelessResponsesCompatibility(request, &result, profile)
	result.Applied = len(result.RemovedFields) > 0 || result.NormalizedInput

	recordResponsesCompatibilityResult(c, result)
	return result
}

func resolveResponsesCompatibilityProfile(relayMode int, originModel string, channelType int) (ResponsesCompatibilityProfile, bool) {
	originModel = strings.TrimSpace(originModel)
	if originModel == "" {
		return ResponsesCompatibilityProfile{}, false
	}
	if channelType == constant.ChannelTypeAntigravity {
		if profile, ok := resolveAntigravityResponsesCompatibilityProfile(relayMode, originModel); ok {
			return profile, true
		}
	}
	profiles, ok := responsesCompatibilityProfiles[relayMode]
	if !ok {
		return ResponsesCompatibilityProfile{}, false
	}
	profile, ok := profiles[originModel]
	if !ok {
		return ResponsesCompatibilityProfile{}, false
	}
	_ = channelType
	return profile, true
}

func resolveAntigravityResponsesCompatibilityProfile(relayMode int, originModel string) (ResponsesCompatibilityProfile, bool) {
	switch relayMode {
	case relayconstant.RelayModeResponses:
		switch originModel {
		case "gpt-5.5", "gpt-5.5-mini":
			return ResponsesCompatibilityProfile{
				Name:         ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55,
				Mode:         "stateless",
				ForceNoTools: true,
				ToolStrategy: "no_tools_gpt55",
			}, true
		case "gpt-5.4", "gpt-5.4-mini":
			return ResponsesCompatibilityProfile{Name: ResponsesCompatProfileStatelessV2Antigravity, Mode: "stateless"}, true
		}
	case relayconstant.RelayModeResponsesCompact:
		switch originModel {
		case "gpt-5.5-openai-compact", "gpt-5.5-mini-openai-compact":
			return ResponsesCompatibilityProfile{
				Name:         ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55,
				Mode:         "stateless",
				ForceNoTools: true,
				ToolStrategy: "no_tools_gpt55",
			}, true
		case "gpt-5.4-openai-compact", "gpt-5.4-mini-openai-compact":
			return ResponsesCompatibilityProfile{Name: ResponsesCompatProfileStatelessV2Antigravity, Mode: "stateless"}, true
		}
	}
	return ResponsesCompatibilityProfile{}, false
}

func applyStatelessResponsesCompatibility(request *dto.OpenAIResponsesRequest, result *ResponsesCompatibilityResult, profile ResponsesCompatibilityProfile) {
	if request == nil || result == nil {
		return
	}

	result.HadPreviousResponseID = strings.TrimSpace(request.PreviousResponseID) != ""
	result.HadConversation = hasRawMessageValue(request.Conversation)
	result.HadContextManagement = hasRawMessageValue(request.ContextManagement)
	result.HadPromptCacheKey = hasRawMessageValue(request.PromptCacheKey)
	result.HadPromptCacheRetention = hasRawMessageValue(request.PromptCacheRetention)

	if result.HadPreviousResponseID {
		request.PreviousResponseID = ""
		result.RemovedFields = append(result.RemovedFields, "previous_response_id")
	}
	if result.HadConversation {
		request.Conversation = nil
		result.RemovedFields = append(result.RemovedFields, "conversation")
	}
	if result.HadContextManagement {
		request.ContextManagement = nil
		result.RemovedFields = append(result.RemovedFields, "context_management")
	}
	if result.HadPromptCacheKey {
		request.PromptCacheKey = nil
		result.RemovedFields = append(result.RemovedFields, "prompt_cache_key")
	}
	if result.HadPromptCacheRetention {
		request.PromptCacheRetention = nil
		result.RemovedFields = append(result.RemovedFields, "prompt_cache_retention")
	}

	if result.Profile == ResponsesCompatProfileStatelessV2Antigravity || result.Profile == ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55 {
		if hasRawMessageValue(request.Include) {
			request.Include = nil
			result.RemovedInclude = true
			result.RemovedFields = append(result.RemovedFields, "include")
		}
		if hasRawMessageValue(request.Store) {
			request.Store = nil
			result.RemovedStore = true
			result.RemovedFields = append(result.RemovedFields, "store")
		}
		if hasRawMessageValue(request.ParallelToolCalls) {
			request.ParallelToolCalls = nil
			result.RemovedParallelToolCall = true
			result.RemovedFields = append(result.RemovedFields, "parallel_tool_calls")
		}
		if profile.ForceNoTools {
			if toolCount := len(request.GetToolsMap()); toolCount > 0 {
				request.Tools = nil
				result.RemovedTools = true
				result.RemovedToolItems += toolCount
				result.RemovedFields = append(result.RemovedFields, "tools")
			}
			if hasRawMessageValue(request.ToolChoice) {
				request.ToolChoice = nil
				result.RemovedToolChoice = true
				result.RemovedFields = append(result.RemovedFields, "tool_choice")
			}
		}
	}

	if normalizedInput, normalizeResult := normalizeResponsesInputToStateless(request.Input, result.Profile); normalizeResult.Normalized {
		request.Input = normalizedInput
		result.NormalizedInput = true
		result.RemovedToolItems += normalizeResult.RemovedToolItems
		result.RemovedHistoryItems += normalizeResult.RemovedHistoryItems
		result.RemovedFields = append(result.RemovedFields, "input.stateful_history")
	}

	if sanitizedMetadata, removedKeys, hadStatefulMetadata := sanitizeResponsesMetadata(request.Metadata); hadStatefulMetadata {
		result.HadStatefulMetadata = true
		request.Metadata = sanitizedMetadata
		result.RemovedFields = append(result.RemovedFields, removedKeys...)
	}
}

func sanitizeResponsesMetadata(raw json.RawMessage) (json.RawMessage, []string, bool) {
	if !hasRawMessageValue(raw) || common.GetJsonType(raw) != "object" {
		return raw, nil, false
	}
	var metadata map[string]any
	if err := common.Unmarshal(raw, &metadata); err != nil {
		return raw, nil, false
	}
	removed := make([]string, 0)
	for key := range responsesStateMetadataKeys {
		if _, ok := metadata[key]; ok {
			delete(metadata, key)
			removed = append(removed, "metadata."+key)
		}
	}
	if len(removed) == 0 {
		return raw, nil, false
	}
	if len(metadata) == 0 {
		return nil, removed, true
	}
	data, err := common.Marshal(metadata)
	if err != nil {
		return raw, removed, true
	}
	return data, removed, true
}

type responsesInputNormalizationResult struct {
	Normalized          bool
	RemovedToolItems    int
	RemovedHistoryItems int
}

func normalizeResponsesInputToStateless(raw json.RawMessage, profile string) (json.RawMessage, responsesInputNormalizationResult) {
	if !hasRawMessageValue(raw) || common.GetJsonType(raw) != "array" {
		return raw, responsesInputNormalizationResult{}
	}
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return raw, responsesInputNormalizationResult{}
	}

	if profile == ResponsesCompatProfileStatelessV2Antigravity || profile == ResponsesCompatProfileStatelessV2AntigravityNoToolsGPT55 {
		return normalizeResponsesInputForAntigravity(items, raw)
	}

	normalized := make([]map[string]any, 0)
	hadStatefulContent := false
	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case "", "message":
			role := strings.TrimSpace(common.Interface2String(item["role"]))
			if role != "" && role != "user" {
				hadStatefulContent = true
				continue
			}
			content, ok := item["content"]
			if !ok {
				continue
			}
			normalizedContent, changed := normalizeResponsesContentToStateless(content)
			hadStatefulContent = hadStatefulContent || changed
			if len(normalizedContent) == 0 {
				continue
			}
			normalized = append(normalized, map[string]any{
				"type":    "message",
				"role":    "user",
				"content": normalizedContent,
			})
		case "input_text", "input_image", "input_file":
			normalized = append(normalized, map[string]any{
				"type":    "message",
				"role":    "user",
				"content": []map[string]any{cloneSimpleResponsesItem(item)},
			})
			if itemType != "input_text" {
				hadStatefulContent = true
			}
		case "function_call", "function_call_output":
			hadStatefulContent = true
		default:
			if itemType != "" {
				hadStatefulContent = true
			}
		}
	}

	if !hadStatefulContent {
		return raw, responsesInputNormalizationResult{}
	}
	if len(normalized) == 0 {
		return raw, responsesInputNormalizationResult{}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return raw, responsesInputNormalizationResult{}
	}
	return data, responsesInputNormalizationResult{Normalized: true}
}

func normalizeResponsesInputForAntigravity(items []map[string]any, raw json.RawMessage) (json.RawMessage, responsesInputNormalizationResult) {
	normalized := make([]map[string]any, 0, len(items))
	result := responsesInputNormalizationResult{}
	hadChanges := false

	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case "", "message":
			role := normalizeResponsesTranscriptRole(common.Interface2String(item["role"]))
			content, changed, toolCount, dropCount := normalizeResponsesMessageContentForAntigravity(item["content"])
			if toolCount > 0 {
				result.RemovedToolItems += toolCount
			}
			if dropCount > 0 {
				result.RemovedHistoryItems += dropCount
			}
			hadChanges = hadChanges || changed || role != strings.TrimSpace(common.Interface2String(item["role"]))
			if len(content) == 0 {
				if itemType != "" || strings.TrimSpace(common.Interface2String(item["role"])) != "user" {
					result.RemovedHistoryItems++
					hadChanges = true
				}
				continue
			}
			normalized = append(normalized, map[string]any{
				"type":    "message",
				"role":    role,
				"content": content,
			})
		case "input_text", "input_image", "input_file":
			normalized = append(normalized, map[string]any{
				"type":    "message",
				"role":    "user",
				"content": []map[string]any{cloneSimpleResponsesItem(item)},
			})
			if itemType != "input_text" {
				hadChanges = true
			}
		case "function_call":
			text := strings.TrimSpace(formatResponsesFunctionCallTranscript(item))
			result.RemovedToolItems++
			result.RemovedHistoryItems++
			hadChanges = true
			if text == "" {
				continue
			}
			normalized = append(normalized, newResponsesTranscriptMessage("assistant", text))
		case "function_call_output":
			text := strings.TrimSpace(formatResponsesFunctionOutputTranscript(item))
			result.RemovedToolItems++
			result.RemovedHistoryItems++
			hadChanges = true
			if text == "" {
				continue
			}
			normalized = append(normalized, newResponsesTranscriptMessage("user", text))
		default:
			if itemType != "" {
				result.RemovedHistoryItems++
				hadChanges = true
			}
		}
	}

	if !hadChanges {
		return raw, responsesInputNormalizationResult{}
	}
	if len(normalized) == 0 {
		return raw, responsesInputNormalizationResult{}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return raw, responsesInputNormalizationResult{}
	}
	result.Normalized = true
	return data, result
}

func normalizeResponsesMessageContentForAntigravity(content any) ([]map[string]any, bool, int, int) {
	parts := toResponsesParts(content)
	if len(parts) == 0 {
		return nil, false, 0, 0
	}
	normalized := make([]map[string]any, 0, len(parts))
	changed := false
	removedTools := 0
	removedHistory := 0
	for _, part := range parts {
		partType := strings.TrimSpace(common.Interface2String(part["type"]))
		switch partType {
		case "input_text", "output_text", "text":
			text := strings.TrimSpace(common.Interface2String(part["text"]))
			if text == "" {
				if partType != "" {
					changed = true
				}
				continue
			}
			normalized = append(normalized, map[string]any{
				"type": "input_text",
				"text": text,
			})
			if partType != "input_text" {
				changed = true
			}
		case "input_image", "input_file":
			normalized = append(normalized, cloneSimpleResponsesItem(part))
		case "function_call":
			removedTools++
			removedHistory++
			changed = true
			if text := strings.TrimSpace(formatResponsesFunctionCallTranscript(part)); text != "" {
				normalized = append(normalized, map[string]any{
					"type": "input_text",
					"text": text,
				})
			}
		case "function_call_output":
			removedTools++
			removedHistory++
			changed = true
			if text := strings.TrimSpace(formatResponsesFunctionOutputTranscript(part)); text != "" {
				normalized = append(normalized, map[string]any{
					"type": "input_text",
					"text": text,
				})
			}
		default:
			if partType != "" {
				removedHistory++
				changed = true
			}
		}
	}
	return normalized, changed, removedTools, removedHistory
}

func toResponsesParts(content any) []map[string]any {
	switch v := content.(type) {
	case []any:
		parts := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if part, ok := item.(map[string]any); ok {
				parts = append(parts, part)
			}
		}
		return parts
	case []map[string]any:
		return v
	default:
		return nil
	}
}

func normalizeResponsesTranscriptRole(role string) string {
	switch strings.TrimSpace(role) {
	case "developer", "system", "user", "assistant":
		return strings.TrimSpace(role)
	default:
		return "user"
	}
}

func formatResponsesFunctionCallTranscript(item map[string]any) string {
	name := strings.TrimSpace(common.Interface2String(item["name"]))
	arguments := strings.TrimSpace(common.Interface2String(item["arguments"]))
	callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
	parts := make([]string, 0, 3)
	if name != "" {
		parts = append(parts, "tool_call="+name)
	}
	if callID != "" {
		parts = append(parts, "call_id="+callID)
	}
	if arguments != "" {
		parts = append(parts, "arguments="+arguments)
	}
	return strings.Join(parts, "\n")
}

func formatResponsesFunctionOutputTranscript(item map[string]any) string {
	callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
	output := strings.TrimSpace(common.Interface2String(item["output"]))
	if output == "" {
		if raw, ok := item["output"]; ok && raw != nil {
			data, err := common.Marshal(raw)
			if err == nil {
				output = strings.TrimSpace(string(data))
			}
		}
	}
	parts := make([]string, 0, 2)
	if callID != "" {
		parts = append(parts, "tool_result_call_id="+callID)
	}
	if output != "" {
		parts = append(parts, "output="+output)
	}
	return strings.Join(parts, "\n")
}

func newResponsesTranscriptMessage(role string, text string) map[string]any {
	return map[string]any{
		"type": "message",
		"role": normalizeResponsesTranscriptRole(role),
		"content": []map[string]any{{
			"type": "input_text",
			"text": text,
		}},
	}
}

func isAntigravityResponsesCompatModel(relayMode int, originModel string) bool {
	switch relayMode {
	case relayconstant.RelayModeResponses:
		switch originModel {
		case "gpt-5.5", "gpt-5.5-mini", "gpt-5.4", "gpt-5.4-mini":
			return true
		}
	case relayconstant.RelayModeResponsesCompact:
		switch originModel {
		case "gpt-5.5-openai-compact", "gpt-5.5-mini-openai-compact", "gpt-5.4-openai-compact", "gpt-5.4-mini-openai-compact":
			return true
		}
	}
	return false
}

func normalizeResponsesContentToStateless(content any) ([]map[string]any, bool) {
	parts, ok := content.([]any)
	if !ok {
		if directParts, ok := content.([]map[string]any); ok {
			parts = make([]any, 0, len(directParts))
			for _, p := range directParts {
				parts = append(parts, p)
			}
		} else {
			return nil, false
		}
	}
	normalized := make([]map[string]any, 0)
	changed := false
	for _, partAny := range parts {
		part, ok := partAny.(map[string]any)
		if !ok {
			continue
		}
		partType := strings.TrimSpace(common.Interface2String(part["type"]))
		switch partType {
		case "input_text", "input_image", "input_file":
			normalized = append(normalized, cloneSimpleResponsesItem(part))
		default:
			if partType != "" {
				changed = true
			}
		}
	}
	return normalized, changed
}

func cloneSimpleResponsesItem(item map[string]any) map[string]any {
	cloned := make(map[string]any)
	for _, key := range []string{"type", "text", "image_url", "file_url", "file", "detail"} {
		if value, ok := item[key]; ok {
			cloned[key] = value
		}
	}
	if value, ok := item["image_url"]; ok {
		cloned["image_url"] = value
	}
	return cloned
}

func hasRawMessageValue(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func recordResponsesCompatibilityResult(c *gin.Context, result ResponsesCompatibilityResult) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyResponsesCompatApplied, result.Applied)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatProfile, result.Profile)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatMode, result.Mode)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatRemovedFields, result.RemovedFields)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatOriginModel, result.OriginModel)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatChannelType, result.ChannelType)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatNormalizedInput, result.NormalizedInput)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatRemovedToolItems, result.RemovedToolItems)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatRemovedHistoryItems, result.RemovedHistoryItems)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatIncludeDropped, result.RemovedInclude)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatStoreDropped, result.RemovedStore)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatParallelToolsDropped, result.RemovedParallelToolCall)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatToolsDropped, result.RemovedTools)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatToolChoiceDropped, result.RemovedToolChoice)
	common.SetContextKey(c, constant.ContextKeyResponsesCompatToolStrategy, result.ToolStrategy)
	common.SetContextKey(c, constant.ContextKeyAntigravityResponsesToolsForcedOff, result.RemovedTools || result.RemovedToolChoice)
	common.SetContextKey(c, constant.ContextKeyResponsesStateSanitized, result.Applied)
	common.SetContextKey(c, constant.ContextKeyResponsesStateSanitizedFields, result.RemovedFields)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadPreviousResponseID, result.HadPreviousResponseID)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadConversation, result.HadConversation)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadContextManagement, result.HadContextManagement)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadPromptCacheKey, result.HadPromptCacheKey)
}
