package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func LogResponsesCompatibilityApplied(c *gin.Context, relayInfo *relaycommon.RelayInfo, result ResponsesCompatibilityResult) {
	if c == nil || !result.Applied {
		return
	}
	requestPath := ""
	relayMode := 0
	if c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}
	if relayInfo != nil {
		relayMode = relayInfo.RelayMode
	}
	logger.LogInfo(c, fmt.Sprintf(
		"responses compatibility applied: request_path=%s origin_model_name=%s relay_mode=%d channel_type=%d profile=%s mode=%s normalized_input=%t removed=%s removed_tool_items=%d removed_history_items=%d include_dropped=%t store_dropped=%t parallel_tool_calls_dropped=%t tools_dropped=%t tool_choice_dropped=%t tool_strategy=%s had_previous_response_id=%t had_conversation=%t had_context_management=%t had_prompt_cache_key=%t",
		requestPath,
		result.OriginModel,
		relayMode,
		result.ChannelType,
		result.Profile,
		result.Mode,
		result.NormalizedInput,
		strings.Join(result.RemovedFields, ","),
		result.RemovedToolItems,
		result.RemovedHistoryItems,
		result.RemovedInclude,
		result.RemovedStore,
		result.RemovedParallelToolCall,
		result.RemovedTools,
		result.RemovedToolChoice,
		result.ToolStrategy,
		result.HadPreviousResponseID,
		result.HadConversation,
		result.HadContextManagement,
		result.HadPromptCacheKey,
	))
}
