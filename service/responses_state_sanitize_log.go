package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func LogResponsesStateSanitized(c *gin.Context, relayInfo *relaycommon.RelayInfo, result ResponsesStateSanitizeResult) {
	if c == nil || !result.Applied {
		return
	}
	requestPath := ""
	if c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}
	originModelName := ""
	if relayInfo != nil {
		originModelName = relayInfo.OriginModelName
	}
	logger.LogInfo(c, fmt.Sprintf(
		"responses state sanitized: request_path=%s origin_model_name=%s relay_mode=%d removed=%s had_previous_response_id=%t had_conversation=%t had_context_management=%t had_prompt_cache_key=%t",
		requestPath,
		originModelName,
		func() int {
			if relayInfo == nil {
				return 0
			}
			return relayInfo.RelayMode
		}(),
		strings.Join(result.RemovedFields, ","),
		result.HadPreviousResponseID,
		result.HadConversation,
		result.HadContextManagement,
		result.HadPromptCacheKey,
	))
}
