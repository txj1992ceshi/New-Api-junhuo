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

var responsesStateSanitizeModels = map[int]map[string]struct{}{
	relayconstant.RelayModeResponses: {
		"gpt-5.4":      {},
		"gpt-5.4-mini": {},
	},
	relayconstant.RelayModeResponsesCompact: {
		"gpt-5.4-openai-compact":      {},
		"gpt-5.4-mini-openai-compact": {},
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

type ResponsesStateSanitizeResult struct {
	Applied                 bool
	RemovedFields           []string
	HadPreviousResponseID   bool
	HadConversation         bool
	HadContextManagement    bool
	HadPromptCacheKey       bool
	HadPromptCacheRetention bool
	HadStatefulMetadata     bool
}

func SanitizeResponsesRequestForModel(c *gin.Context, relayMode int, request *dto.OpenAIResponsesRequest) ResponsesStateSanitizeResult {
	result := ResponsesStateSanitizeResult{}
	if request == nil || !shouldSanitizeResponsesState(relayMode, request.Model) {
		recordResponsesSanitizeResult(c, result)
		return result
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

	if sanitizedMetadata, removedKeys, hadStatefulMetadata := sanitizeResponsesMetadata(request.Metadata); hadStatefulMetadata {
		result.HadStatefulMetadata = true
		request.Metadata = sanitizedMetadata
		result.RemovedFields = append(result.RemovedFields, removedKeys...)
	}

	if len(result.RemovedFields) > 0 {
		result.Applied = true
	}
	recordResponsesSanitizeResult(c, result)
	return result
}

func shouldSanitizeResponsesState(relayMode int, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	models, ok := responsesStateSanitizeModels[relayMode]
	if !ok {
		return false
	}
	_, ok = models[modelName]
	return ok
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

func hasRawMessageValue(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func recordResponsesSanitizeResult(c *gin.Context, result ResponsesStateSanitizeResult) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyResponsesStateSanitized, result.Applied)
	common.SetContextKey(c, constant.ContextKeyResponsesStateSanitizedFields, result.RemovedFields)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadPreviousResponseID, result.HadPreviousResponseID)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadConversation, result.HadConversation)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadContextManagement, result.HadContextManagement)
	common.SetContextKey(c, constant.ContextKeyResponsesStateHadPromptCacheKey, result.HadPromptCacheKey)
}
