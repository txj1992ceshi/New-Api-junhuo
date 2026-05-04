package service

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

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

func ShouldAvoidAntigravityForCodexResponses(c *gin.Context, ch *model.Channel, relayMode int) bool {
	if ch == nil || ch.Type != constant.ChannelTypeAntigravity {
		return false
	}
	if relayMode != relayconstant.RelayModeResponses && relayMode != relayconstant.RelayModeResponsesCompact {
		return false
	}
	return IsCodexCLIResponsesRequest(c)
}
