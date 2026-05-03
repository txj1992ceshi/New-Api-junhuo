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

type ResponsesCompatibilityProfile struct {
	Name string
	Mode string
}

type ResponsesCompatibilityResult struct {
	Applied                 bool
	Profile                 string
	Mode                    string
	RemovedFields           []string
	NormalizedInput         bool
	HadPreviousResponseID   bool
	HadConversation         bool
	HadContextManagement    bool
	HadPromptCacheKey       bool
	HadPromptCacheRetention bool
	HadStatefulMetadata     bool
	ChannelType             int
	OriginModel             string
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

	applyStatelessResponsesCompatibility(request, &result)
	result.Applied = len(result.RemovedFields) > 0 || result.NormalizedInput

	recordResponsesCompatibilityResult(c, result)
	return result
}

func resolveResponsesCompatibilityProfile(relayMode int, originModel string, channelType int) (ResponsesCompatibilityProfile, bool) {
	originModel = strings.TrimSpace(originModel)
	if originModel == "" {
		return ResponsesCompatibilityProfile{}, false
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

func applyStatelessResponsesCompatibility(request *dto.OpenAIResponsesRequest, result *ResponsesCompatibilityResult) {
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

	if normalizedInput, normalized := normalizeResponsesInputToStateless(request.Input); normalized {
		request.Input = normalizedInput
		result.NormalizedInput = true
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

func normalizeResponsesInputToStateless(raw json.RawMessage) (json.RawMessage, bool) {
	if !hasRawMessageValue(raw) || common.GetJsonType(raw) != "array" {
		return raw, false
	}
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return raw, false
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
			normalized = append(normalized, cloneSimpleResponsesItem(item))
		case "function_call", "function_call_output":
			hadStatefulContent = true
		default:
			if itemType != "" {
				hadStatefulContent = true
			}
		}
	}

	if !hadStatefulContent {
		return raw, false
	}
	if len(normalized) == 0 {
		return raw, false
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return raw, false
	}
	return data, true
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
	common.SetContextKey(c, constant.ContextKeyResponsesStateSanitized, result.Applied)
	common.SetContextKey(c, constant.ContextKeyResponsesStateSanitizedFields, result.RemovedFields)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadPreviousResponseID, result.HadPreviousResponseID)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadConversation, result.HadConversation)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadContextManagement, result.HadContextManagement)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadPromptCacheKey, result.HadPromptCacheKey)
}
