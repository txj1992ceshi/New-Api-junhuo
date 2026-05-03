package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other["request_path"] = path
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other["request_path"] = path
	}
}

func GenerateTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_ratio"] = modelRatio
	other["group_ratio"] = groupRatio
	other["completion_ratio"] = completionRatio
	other["cache_tokens"] = cacheTokens
	other["cache_ratio"] = cacheRatio
	other["model_price"] = modelPrice
	other["user_group_ratio"] = userGroupRatio
	other["frt"] = float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())
	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}
	if relayInfo.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = relayInfo.UpstreamModelName
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesModelMappingApplied) {
		other["responses_model_mapping_applied"] = true
		if relayInfo.UpstreamModelName != "" {
			other["upstream_model_name"] = relayInfo.UpstreamModelName
		}
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatApplied) {
		other["responses_compat_applied"] = true
		if profile := common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatProfile); profile != "" {
			other["responses_compat_profile"] = profile
		}
		if mode := common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatMode); mode != "" {
			other["responses_compat_mode"] = mode
		}
		if fields := common.GetContextKeyStringSlice(ctx, constant.ContextKeyResponsesCompatRemovedFields); len(fields) > 0 {
			other["responses_compat_removed_fields"] = fields
		}
		if originModel := common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatOriginModel); originModel != "" {
			other["responses_compat_origin_model"] = originModel
		}
		if channelType := common.GetContextKeyInt(ctx, constant.ContextKeyResponsesCompatChannelType); channelType != 0 {
			other["responses_compat_channel_type"] = channelType
		}
		if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatNormalizedInput) {
			other["responses_compat_normalized_input"] = true
		}
		if removedToolItems := common.GetContextKeyInt(ctx, constant.ContextKeyResponsesCompatRemovedToolItems); removedToolItems > 0 {
			other["responses_compat_removed_tool_items"] = removedToolItems
		}
		if removedHistoryItems := common.GetContextKeyInt(ctx, constant.ContextKeyResponsesCompatRemovedHistoryItems); removedHistoryItems > 0 {
			other["responses_compat_removed_history_items"] = removedHistoryItems
		}
		if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatIncludeDropped) {
			other["responses_compat_include_dropped"] = true
		}
		if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatStoreDropped) {
			other["responses_compat_store_dropped"] = true
		}
		if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatParallelToolsDropped) {
			other["responses_compat_parallel_tool_calls_dropped"] = true
		}
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesStateSanitized) {
		other["responses_state_sanitized"] = true
		if fields := common.GetContextKeyStringSlice(ctx, constant.ContextKeyResponsesStateSanitizedFields); len(fields) > 0 {
			other["responses_state_sanitized_fields"] = fields
		}
		other["responses_state_had_previous_response_id"] = common.GetContextKeyBool(ctx, constant.ContextKeyResponsesStateHadPreviousResponseID)
		other["responses_state_had_conversation"] = common.GetContextKeyBool(ctx, constant.ContextKeyResponsesStateHadConversation)
		other["responses_state_had_context_management"] = common.GetContextKeyBool(ctx, constant.ContextKeyResponsesStateHadContextManagement)
		other["responses_state_had_prompt_cache_key"] = common.GetContextKeyBool(ctx, constant.ContextKeyResponsesStateHadPromptCacheKey)
	}

	isSystemPromptOverwritten := common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride)
	if isSystemPromptOverwritten {
		other["is_system_prompt_overwritten"] = true
	}

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = ctx.GetStringSlice("use_channel")
	isMultiKey := common.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex)
	}
	if email := common.GetContextKeyString(ctx, constant.ContextKeyAntigravityEmail); email != "" {
		adminInfo["antigravity_email"] = email
	}
	if projectID := common.GetContextKeyString(ctx, constant.ContextKeyAntigravityEffectiveProjectID); projectID != "" {
		adminInfo["antigravity_effective_project_id"] = projectID
	}
	if state := common.GetContextKeyString(ctx, constant.ContextKeyAntigravityKeyState); state != "" {
		adminInfo["antigravity_key_state"] = state
	}
	if class := common.GetContextKeyString(ctx, constant.ContextKeyAntigravityErrorClass); class != "" {
		adminInfo["antigravity_error_class"] = class
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityAccountSwitched) {
		adminInfo["antigravity_account_switched"] = true
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyAntigravityCredentialRefreshApplied) {
		adminInfo["antigravity_credential_refresh_applied"] = true
	}
	if common.GetContextKeyInt(ctx, constant.ContextKeyChannelType) == constant.ChannelTypeAntigravity && common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatApplied) {
		adminInfo["antigravity_responses_mode"] = "stateless_transcript"
		if profile := common.GetContextKeyString(ctx, constant.ContextKeyResponsesCompatProfile); profile != "" {
			adminInfo["antigravity_responses_profile"] = profile
		}
		if fields := common.GetContextKeyStringSlice(ctx, constant.ContextKeyResponsesCompatRemovedFields); len(fields) > 0 {
			adminInfo["antigravity_responses_removed_fields"] = fields
		}
		if removedToolItems := common.GetContextKeyInt(ctx, constant.ContextKeyResponsesCompatRemovedToolItems); removedToolItems > 0 {
			adminInfo["antigravity_responses_tool_items_dropped"] = removedToolItems
		}
		if removedHistoryItems := common.GetContextKeyInt(ctx, constant.ContextKeyResponsesCompatRemovedHistoryItems); removedHistoryItems > 0 {
			adminInfo["antigravity_responses_history_items_dropped"] = removedHistoryItems
		}
		if common.GetContextKeyBool(ctx, constant.ContextKeyResponsesCompatIncludeDropped) {
			adminInfo["antigravity_responses_include_dropped"] = true
		}
		if relayInfo != nil {
			adminInfo["antigravity_responses_request_type"] = antigravityResponsesRequestType(relayInfo.RelayMode)
			if relayInfo.UpstreamModelName != "" {
				adminInfo["antigravity_responses_upstream_model"] = relayInfo.UpstreamModelName
			}
		}
	}
	if class := common.GetContextKeyString(ctx, constant.ContextKeyAntigravityErrorClass); class != "" && class == string(AntigravityErrorClassProtocolIncompatible) {
		adminInfo["antigravity_protocol_incompatible"] = true
	}

	isLocalCountTokens := common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens)
	if isLocalCountTokens {
		adminInfo["local_count_tokens"] = isLocalCountTokens
	}

	AppendChannelAffinityAdminInfo(ctx, adminInfo)

	other["admin_info"] = adminInfo
	appendRequestPath(ctx, relayInfo, other)
	appendRequestConversionChain(relayInfo, other)
	appendFinalRequestFormat(relayInfo, other)
	appendBillingInfo(relayInfo, other)
	appendParamOverrideInfo(relayInfo, other)
	appendStreamStatus(relayInfo, other)
	return other
}

func antigravityResponsesRequestType(relayMode int) string {
	switch relayMode {
	case relayconstant.RelayModeResponses:
		return "responses"
	case relayconstant.RelayModeResponsesCompact:
		return "responses_compact"
	default:
		return "unknown"
	}
}

func appendParamOverrideInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || len(relayInfo.ParamOverrideAudit) == 0 {
		return
	}
	other["po"] = relayInfo.ParamOverrideAudit
}

func appendStreamStatus(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || !relayInfo.IsStream || relayInfo.StreamStatus == nil {
		return
	}
	ss := relayInfo.StreamStatus
	status := "ok"
	if !ss.IsNormalEnd() || ss.HasErrors() {
		status = "error"
	}
	streamInfo := map[string]interface{}{
		"status":     status,
		"end_reason": string(ss.EndReason),
	}
	if ss.EndError != nil {
		streamInfo["end_error"] = ss.EndError.Error()
	}
	if ss.ErrorCount > 0 {
		streamInfo["error_count"] = ss.ErrorCount
		messages := make([]string, 0, len(ss.Errors))
		for _, e := range ss.Errors {
			messages = append(messages, e.Message)
		}
		streamInfo["errors"] = messages
	}
	other["stream_status"] = streamInfo
}

func appendBillingInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	// billing_source: "wallet" or "subscription"
	if relayInfo.BillingSource != "" {
		other["billing_source"] = relayInfo.BillingSource
	}
	if relayInfo.UserSetting.BillingPreference != "" {
		other["billing_preference"] = relayInfo.UserSetting.BillingPreference
	}
	if relayInfo.BillingSource == "subscription" {
		if relayInfo.SubscriptionId != 0 {
			other["subscription_id"] = relayInfo.SubscriptionId
		}
		if relayInfo.SubscriptionPreConsumed > 0 {
			other["subscription_pre_consumed"] = relayInfo.SubscriptionPreConsumed
		}
		// post_delta: settlement delta applied after actual usage is known (can be negative for refund)
		if relayInfo.SubscriptionPostDelta != 0 {
			other["subscription_post_delta"] = relayInfo.SubscriptionPostDelta
		}
		if relayInfo.SubscriptionPlanId != 0 {
			other["subscription_plan_id"] = relayInfo.SubscriptionPlanId
		}
		if relayInfo.SubscriptionPlanTitle != "" {
			other["subscription_plan_title"] = relayInfo.SubscriptionPlanTitle
		}
		// Compute "this request" subscription consumed + remaining
		consumed := relayInfo.SubscriptionPreConsumed + relayInfo.SubscriptionPostDelta
		usedFinal := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		if consumed < 0 {
			consumed = 0
		}
		if usedFinal < 0 {
			usedFinal = 0
		}
		if relayInfo.SubscriptionAmountTotal > 0 {
			remain := relayInfo.SubscriptionAmountTotal - usedFinal
			if remain < 0 {
				remain = 0
			}
			other["subscription_total"] = relayInfo.SubscriptionAmountTotal
			other["subscription_used"] = usedFinal
			other["subscription_remain"] = remain
		}
		if consumed > 0 {
			other["subscription_consumed"] = consumed
		}
		// Wallet quota is not deducted when billed from subscription.
		other["wallet_quota_deducted"] = 0
	}
}

func appendRequestConversionChain(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case types.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case types.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case types.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case types.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) == 0 {
		return
	}
	other["request_conversion"] = chain
}

func appendFinalRequestFormat(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		// claude indicates the final upstream request format is Claude Messages.
		// Frontend log rendering uses this to keep the original Claude input display.
		other["claude"] = true
	}
}

func GenerateWssOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["ws"] = true
	info["audio_input"] = usage.InputTokenDetails.AudioTokens
	info["audio_output"] = usage.OutputTokenDetails.AudioTokens
	info["text_input"] = usage.InputTokenDetails.TextTokens
	info["text_output"] = usage.OutputTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateAudioOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["audio"] = true
	info["audio_input"] = usage.PromptTokensDetails.AudioTokens
	info["audio_output"] = usage.CompletionTokenDetails.AudioTokens
	info["text_input"] = usage.PromptTokensDetails.TextTokens
	info["text_output"] = usage.CompletionTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateClaudeOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64,
	cacheCreationTokens int, cacheCreationRatio float64,
	cacheCreationTokens5m int, cacheCreationRatio5m float64,
	cacheCreationTokens1h int, cacheCreationRatio1h float64,
	modelPrice float64, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice, userGroupRatio)
	info["claude"] = true
	info["cache_creation_tokens"] = cacheCreationTokens
	info["cache_creation_ratio"] = cacheCreationRatio
	if cacheCreationTokens5m != 0 {
		info["cache_creation_tokens_5m"] = cacheCreationTokens5m
		info["cache_creation_ratio_5m"] = cacheCreationRatio5m
	}
	if cacheCreationTokens1h != 0 {
		info["cache_creation_tokens_1h"] = cacheCreationTokens1h
		info["cache_creation_ratio_1h"] = cacheCreationRatio1h
	}
	return info
}

func GenerateMjOtherInfo(relayInfo *relaycommon.RelayInfo, priceData types.PriceData) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_price"] = priceData.ModelPrice
	other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio
	if priceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = priceData.GroupRatioInfo.GroupSpecialRatio
	}
	appendRequestPath(nil, relayInfo, other)
	return other
}
