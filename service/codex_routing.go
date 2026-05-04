package service

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

var codexResponsesSupportedChannelTypes = map[int]bool{
	constant.ChannelTypeCodex:      true,
	constant.ChannelTypeOpenAI:     true,
	constant.ChannelTypeAzure:      true,
	constant.ChannelTypeCustom:     true,
	constant.ChannelTypeOpenRouter: true,
	constant.ChannelTypeXinference: true,
	constant.ChannelCloudflare:     true,
	constant.ChannelTypeAli:        true,
	constant.ChannelTypeVolcEngine: true,
	constant.ChannelTypePerplexity: true,
	constant.ChannelTypeXai:        true,
}

func IsCodexCLIResponsesRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	if c.Request.Method != http.MethodPost {
		return false
	}
	switch relayconstant.Path2RelayMode(c.Request.URL.Path) {
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
	default:
		return false
	}
	originator := strings.TrimSpace(c.GetHeader("Originator"))
	return strings.Contains(strings.ToLower(originator), "codex cli")
}

func IsCodexResponsesRelayMode(relayMode int) bool {
	return relayMode == relayconstant.RelayModeResponses || relayMode == relayconstant.RelayModeResponsesCompact
}

func ChannelTypeSupportsCodexResponses(channelType int, relayMode int) bool {
	if !IsCodexResponsesRelayMode(relayMode) {
		return true
	}
	supported, ok := codexResponsesSupportedChannelTypes[channelType]
	if !ok {
		return false
	}
	return supported
}

func ChannelSupportsCodexResponsesRequest(c *gin.Context, ch *model.Channel, relayMode int) bool {
	if ch == nil {
		return false
	}
	if !IsCodexCLIResponsesRequest(c) {
		return true
	}
	return ChannelTypeSupportsCodexResponses(ch.Type, relayMode)
}
