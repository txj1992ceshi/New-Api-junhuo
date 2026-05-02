package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type requestClientClass string

const (
	requestClientClassUnknown  requestClientClass = "unknown"
	requestClientClassCodex    requestClientClass = "codex"
	requestClientClassOpenClaw requestClientClass = "openclaw"
)

type channelRouteRole string

const (
	channelRouteRoleUnknown     channelRouteRole = "unknown"
	channelRouteRoleCodex       channelRouteRole = "codex"
	channelRouteRoleAntigravity channelRouteRole = "antigravity"
	channelRouteRoleCaowo       channelRouteRole = "caowo"
)

var clientRoutedModelPrefixes = []string{
	"gpt-5.4",
	"gpt-5.5",
}

func detectRequestClientClass(c *gin.Context) requestClientClass {
	if c == nil || c.Request == nil {
		return requestClientClassUnknown
	}

	userAgent := strings.ToLower(strings.TrimSpace(c.Request.UserAgent()))
	originator := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("Originator")))
	xApp := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("X-App")))

	if strings.Contains(userAgent, "openclaw") || strings.Contains(xApp, "openclaw") {
		return requestClientClassOpenClaw
	}
	if strings.Contains(userAgent, "codex") || strings.Contains(originator, "codex") || strings.Contains(xApp, "codex") {
		return requestClientClassCodex
	}
	return requestClientClassUnknown
}

func shouldApplyClientRoute(modelName string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if modelName == "" {
		return false
	}
	for _, prefix := range clientRoutedModelPrefixes {
		if strings.HasPrefix(modelName, prefix) {
			return true
		}
	}
	return false
}

func classifyChannelRouteRole(channel *model.Channel) channelRouteRole {
	if channel == nil {
		return channelRouteRoleUnknown
	}

	tag := strings.ToLower(strings.TrimSpace(channel.GetTag()))
	name := strings.ToLower(strings.TrimSpace(channel.Name))

	switch {
	case strings.Contains(tag, "codex_pool"), strings.Contains(name, "codex-e2e-temp"), channel.Type == constant.ChannelTypeCodex:
		return channelRouteRoleCodex
	case strings.Contains(tag, "antigravity_pool"), strings.Contains(name, "antigravity-openclaw"), channel.Type == constant.ChannelTypeAntigravity:
		return channelRouteRoleAntigravity
	case strings.Contains(tag, "caowo_pool"), strings.Contains(name, "caowo"):
		return channelRouteRoleCaowo
	default:
		return channelRouteRoleUnknown
	}
}

func preferredRouteRolesForClient(class requestClientClass) []channelRouteRole {
	switch class {
	case requestClientClassCodex:
		return []channelRouteRole{channelRouteRoleCodex, channelRouteRoleAntigravity}
	case requestClientClassOpenClaw:
		return []channelRouteRole{channelRouteRoleAntigravity, channelRouteRoleCaowo}
	default:
		return []channelRouteRole{channelRouteRoleAntigravity, channelRouteRoleCaowo}
	}
}

func selectPreferredChannelFromCandidates(candidates []*model.Channel, roles []channelRouteRole) *model.Channel {
	if len(candidates) == 0 || len(roles) == 0 {
		return nil
	}

	model.SortChannelsByPriorityAndWeight(candidates)
	for _, role := range roles {
		for _, channel := range candidates {
			if classifyChannelRouteRole(channel) == role {
				return channel
			}
		}
	}
	return nil
}

func SelectChannelByClientPreference(c *gin.Context, tokenGroup string, modelName string) (*model.Channel, string, bool) {
	if !shouldApplyClientRoute(modelName) {
		return nil, tokenGroup, false
	}

	clientClass := detectRequestClientClass(c)
	roles := preferredRouteRolesForClient(clientClass)
	if len(roles) == 0 {
		return nil, tokenGroup, false
	}

	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if tokenGroup == "auto" {
		autoGroups := GetUserAutoGroup(userGroup)
		for _, group := range autoGroups {
			candidates := model.GetEnabledChannelsForGroupModel(group, modelName)
			if channel := selectPreferredChannelFromCandidates(candidates, roles); channel != nil {
				common.SetContextKey(c, constant.ContextKeyAutoGroup, group)
				return channel, group, true
			}
		}
		return nil, tokenGroup, false
	}

	candidates := model.GetEnabledChannelsForGroupModel(tokenGroup, modelName)
	if channel := selectPreferredChannelFromCandidates(candidates, roles); channel != nil {
		return channel, tokenGroup, true
	}
	return nil, tokenGroup, false
}
