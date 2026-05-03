package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
)

const (
	ResponsesStatefulCapabilityNone   = "none"
	ResponsesStatefulCapabilityNative = "native"
	ResponsesStatefulCapabilityReplay = "replay"

	responsesReplayEntityNamespace = "new-api:responses_replay_entity:v1"
	responsesReplayEntityTTL       = 24 * time.Hour
	responsesReplayEntityMaxTurns  = 20
	responsesReplayEntityMaxBytes  = 64 * 1024
)

type ResponsesRoutingIntent struct {
	StatefulRequested  bool
	PreviousResponseID string
}

type ResponsesReplayEntity struct {
	ResponseID        string          `json:"response_id"`
	RelayMode         int             `json:"relay_mode"`
	OriginModelName   string          `json:"origin_model_name"`
	UpstreamModelName string          `json:"upstream_model_name"`
	ChannelID         int             `json:"channel_id"`
	ChannelType       int             `json:"channel_type"`
	UsingGroup        string          `json:"using_group"`
	Instructions      json.RawMessage `json:"instructions,omitempty"`
	Input             json.RawMessage `json:"input,omitempty"`
	ToolChoice        json.RawMessage `json:"tool_choice,omitempty"`
	Tools             json.RawMessage `json:"tools,omitempty"`
	CreatedAt         int64           `json:"created_at"`
	UpdatedAt         int64           `json:"updated_at"`
	ExpiresAt         int64           `json:"expires_at"`
}

var (
	responsesReplayEntityCacheOnce sync.Once
	responsesReplayEntityCache     *cachex.HybridCache[ResponsesReplayEntity]
)

func getResponsesReplayEntityCache() *cachex.HybridCache[ResponsesReplayEntity] {
	responsesReplayEntityCacheOnce.Do(func() {
		responsesReplayEntityCache = cachex.NewHybridCache[ResponsesReplayEntity](cachex.HybridCacheConfig[ResponsesReplayEntity]{
			Namespace: responsesReplayEntityNamespace,
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[ResponsesReplayEntity]{},
			Memory: func() *hot.HotCache[string, ResponsesReplayEntity] {
				return hot.NewHotCache[string, ResponsesReplayEntity](hot.LRU, 4096).
					WithTTL(responsesReplayEntityTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return responsesReplayEntityCache
}

func IsStatefulResponsesRelayMode(relayMode int) bool {
	return relayMode == relayconstant.RelayModeResponses || relayMode == relayconstant.RelayModeResponsesCompact
}

func IsStatefulResponsesModel(relayMode int, modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	switch relayMode {
	case relayconstant.RelayModeResponses:
		return modelName == "gpt-5.4" || modelName == "gpt-5.4-mini"
	case relayconstant.RelayModeResponsesCompact:
		return modelName == "gpt-5.4-openai-compact" || modelName == "gpt-5.4-mini-openai-compact"
	default:
		return false
	}
}

func DetectResponsesRoutingIntent(relayMode int, modelName string, request *dto.OpenAIResponsesRequest) ResponsesRoutingIntent {
	intent := ResponsesRoutingIntent{}
	if !IsStatefulResponsesRelayMode(relayMode) {
		return intent
	}
	if request == nil {
		intent.StatefulRequested = IsStatefulResponsesModel(relayMode, modelName)
		return intent
	}
	intent.PreviousResponseID = strings.TrimSpace(request.PreviousResponseID)
	intent.StatefulRequested = intent.PreviousResponseID != "" ||
		hasRawMessageValue(request.Conversation) ||
		hasRawMessageValue(request.ContextManagement) ||
		hasRawMessageValue(request.PromptCacheKey) ||
		hasRawMessageValue(request.PromptCacheRetention) ||
		IsStatefulResponsesModel(relayMode, modelName)
	return intent
}

func GetResponsesRoutingIntentFromContext(c *gin.Context, relayMode int, modelName string) ResponsesRoutingIntent {
	if c == nil || !IsStatefulResponsesRelayMode(relayMode) {
		return ResponsesRoutingIntent{}
	}
	type requestLite struct {
		Model                string          `json:"model"`
		Conversation         json.RawMessage `json:"conversation,omitempty"`
		ContextManagement    json.RawMessage `json:"context_management,omitempty"`
		PreviousResponseID   string          `json:"previous_response_id,omitempty"`
		PromptCacheKey       json.RawMessage `json:"prompt_cache_key,omitempty"`
		PromptCacheRetention json.RawMessage `json:"prompt_cache_retention,omitempty"`
	}
	var req requestLite
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return ResponsesRoutingIntent{StatefulRequested: IsStatefulResponsesModel(relayMode, modelName)}
	}
	return DetectResponsesRoutingIntent(relayMode, modelName, &dto.OpenAIResponsesRequest{
		Model:                req.Model,
		Conversation:         req.Conversation,
		ContextManagement:    req.ContextManagement,
		PreviousResponseID:   req.PreviousResponseID,
		PromptCacheKey:       req.PromptCacheKey,
		PromptCacheRetention: req.PromptCacheRetention,
	})
}

func ResolveResponsesStatefulCapability(settings dto.ChannelOtherSettings, channelType int, relayMode int, modelName string, statefulRequested bool) string {
	if !IsStatefulResponsesRelayMode(relayMode) || !statefulRequested {
		return ResponsesStatefulCapabilityNone
	}
	compactAllowed := relayMode != relayconstant.RelayModeResponsesCompact || settings.SupportsCompactStatefulResponses || defaultSupportsStatefulReplay(channelType, relayMode)
	if IsStatefulResponsesModel(relayMode, modelName) {
		if !compactAllowed {
			return ResponsesStatefulCapabilityNone
		}
		if settings.SupportsNativeStatefulResponses {
			return ResponsesStatefulCapabilityNative
		}
		if settings.SupportsStatefulResponsesReplay || settings.SupportsCompactStatefulResponses || defaultSupportsStatefulReplay(channelType, relayMode) {
			return ResponsesStatefulCapabilityReplay
		}
	}
	return ResponsesStatefulCapabilityNone
}

func defaultSupportsStatefulReplay(channelType int, relayMode int) bool {
	if relayMode == relayconstant.RelayModeResponsesCompact {
		return channelType == constant.ChannelTypeAntigravity || channelType == constant.ChannelTypeCodex
	}
	return channelType == constant.ChannelTypeAntigravity || channelType == constant.ChannelTypeCodex
}

func ChannelSupportsRequestForRelay(c *gin.Context, channel *model.Channel, relayMode int, modelName string) bool {
	if channel == nil {
		return false
	}
	if !ChannelSupportsRequestedModelForRelay(channel, relayMode, modelName) {
		return false
	}
	intent := GetResponsesRoutingIntentFromContext(c, relayMode, modelName)
	if !intent.StatefulRequested {
		return true
	}
	capability := ResolveResponsesStatefulCapability(channel.GetOtherSettings(), channel.Type, relayMode, modelName, true)
	return capability == ResponsesStatefulCapabilityNative || capability == ResponsesStatefulCapabilityReplay
}

func ChannelSupportsRequestForRelayCapability(c *gin.Context, channel *model.Channel, relayMode int, modelName string, capability string) bool {
	if channel == nil {
		return false
	}
	if !ChannelSupportsRequestedModelForRelay(channel, relayMode, modelName) {
		return false
	}
	intent := GetResponsesRoutingIntentFromContext(c, relayMode, modelName)
	if !intent.StatefulRequested {
		return capability == ""
	}
	return ResolveResponsesStatefulCapability(channel.GetOtherSettings(), channel.Type, relayMode, modelName, true) == capability
}

func GetPreferredChannelByResponsesEntity(c *gin.Context, relayMode int, modelName string) (int, bool) {
	intent := GetResponsesRoutingIntentFromContext(c, relayMode, modelName)
	if !intent.StatefulRequested || intent.PreviousResponseID == "" {
		return 0, false
	}
	entity, found, err := GetResponsesReplayEntity(intent.PreviousResponseID)
	if err != nil || !found || entity.ChannelID <= 0 {
		return 0, false
	}
	return entity.ChannelID, true
}

func GetResponsesReplayEntity(responseID string) (*ResponsesReplayEntity, bool, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil, false, nil
	}
	entity, found, err := getResponsesReplayEntityCache().Get(responseID)
	if err != nil || !found {
		return nil, found, err
	}
	return &entity, true, nil
}

func PrepareResponsesReplayRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) error {
	if request == nil {
		return nil
	}
	intent := DetectResponsesRoutingIntent(info.RelayMode, info.OriginModelName, request)
	if !intent.StatefulRequested {
		return nil
	}
	common.SetContextKey(c, constant.ContextKeyResponsesStatefulRequested, true)
	common.SetContextKey(c, constant.ContextKeyResponsesStatefulCapability, ResponsesStatefulCapabilityReplay)
	common.SetContextKey(c, constant.ContextKeyResponsesStatefulReplayUsed, true)

	if intent.PreviousResponseID != "" {
		entity, found, err := GetResponsesReplayEntity(intent.PreviousResponseID)
		if err != nil {
			return err
		}
		if !found {
			common.SetContextKey(c, constant.ContextKeyResponsesEntityLookupMiss, true)
			return fmt.Errorf("responses state not found for previous_response_id %s", intent.PreviousResponseID)
		}
		common.SetContextKey(c, constant.ContextKeyResponsesEntityLookupHit, true)
		common.SetContextKey(c, constant.ContextKeyResponsesEntityID, entity.ResponseID)
		if entity.ChannelID > 0 && entity.ChannelID != info.ChannelId {
			common.SetContextKey(c, constant.ContextKeyResponsesFallbackFromNativeToReplay, true)
		}
		if merged, err := mergeReplayInputs(entity.Input, request.Input); err == nil {
			request.Input = merged
		} else {
			return err
		}
		if !hasRawMessageValue(request.Instructions) && hasRawMessageValue(entity.Instructions) {
			request.Instructions = entity.Instructions
		}
	}
	return nil
}

func StoreResponsesReplayEntity(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) error {
	if c == nil || info == nil || request == nil {
		return nil
	}
	if !common.GetContextKeyBool(c, constant.ContextKeyResponsesStatefulReplayUsed) {
		return nil
	}
	normalizedInput, err := normalizeResponsesInputForReplay(request.Input)
	if err != nil {
		return err
	}
	normalizedInput = truncateReplayInput(normalizedInput)
	now := time.Now()
	entity := ResponsesReplayEntity{
		ResponseID:        helper.GetResponseID(c),
		RelayMode:         info.RelayMode,
		OriginModelName:   info.OriginModelName,
		UpstreamModelName: info.UpstreamModelName,
		ChannelID:         info.ChannelId,
		ChannelType:       info.ChannelType,
		UsingGroup:        common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		Instructions:      cloneRawMessage(request.Instructions),
		Input:             normalizedInput,
		ToolChoice:        cloneRawMessage(request.ToolChoice),
		Tools:             cloneRawMessage(request.Tools),
		CreatedAt:         now.Unix(),
		UpdatedAt:         now.Unix(),
		ExpiresAt:         now.Add(responsesReplayEntityTTL).Unix(),
	}
	common.SetContextKey(c, constant.ContextKeyResponsesEntityID, entity.ResponseID)
	return getResponsesReplayEntityCache().SetWithTTL(entity.ResponseID, entity, responsesReplayEntityTTL)
}

func normalizeResponsesInputForReplay(raw json.RawMessage) (json.RawMessage, error) {
	if !hasRawMessageValue(raw) {
		return nil, nil
	}
	switch common.GetJsonType(raw) {
	case "string":
		var text string
		if err := common.Unmarshal(raw, &text); err != nil {
			return raw, err
		}
		return common.Marshal([]map[string]any{{
			"type": "message",
			"role": "user",
			"content": []map[string]any{{
				"type": "input_text",
				"text": text,
			}},
		}})
	case "array":
		normalized, changed := normalizeResponsesInputToStateless(raw)
		if changed {
			return normalized, nil
		}
		return raw, nil
	default:
		return raw, nil
	}
}

func mergeReplayInputs(previousRaw, currentRaw json.RawMessage) (json.RawMessage, error) {
	previousNormalized, err := normalizeResponsesInputForReplay(previousRaw)
	if err != nil {
		return nil, err
	}
	currentNormalized, err := normalizeResponsesInputForReplay(currentRaw)
	if err != nil {
		return nil, err
	}
	if !hasRawMessageValue(previousNormalized) {
		return currentNormalized, nil
	}
	if !hasRawMessageValue(currentNormalized) {
		return previousNormalized, nil
	}
	var previousItems []map[string]any
	if err := common.Unmarshal(previousNormalized, &previousItems); err != nil {
		return nil, err
	}
	var currentItems []map[string]any
	if err := common.Unmarshal(currentNormalized, &currentItems); err != nil {
		return nil, err
	}
	merged := append(previousItems, currentItems...)
	data, err := common.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return truncateReplayInput(data), nil
}

func truncateReplayInput(raw json.RawMessage) json.RawMessage {
	if !hasRawMessageValue(raw) || common.GetJsonType(raw) != "array" {
		return raw
	}
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return raw
	}
	if len(items) > responsesReplayEntityMaxTurns {
		items = items[len(items)-responsesReplayEntityMaxTurns:]
	}
	data, err := common.Marshal(items)
	if err != nil {
		return raw
	}
	for len(data) > responsesReplayEntityMaxBytes && len(items) > 1 {
		items = items[1:]
		data, err = common.Marshal(items)
		if err != nil {
			return raw
		}
	}
	return data
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}
