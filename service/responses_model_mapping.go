package service

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

var defaultResponsesModelMappings = map[int]map[string]string{
	constant.ChannelTypeCodex: {
		"gpt-5.4": "gpt-5.5",
	},
	constant.ChannelTypeAntigravity: {
		"gpt-5.4": "gemini-3-flash",
	},
}

var defaultResponsesCompactModelMappings = map[int]map[string]string{
	constant.ChannelTypeCodex: {
		"gpt-5.4-openai-compact": "gpt-5.5-openai-compact",
	},
	constant.ChannelTypeAntigravity: {
		"gpt-5.4-openai-compact": "gemini-3-flash",
	},
}

func RequiresResponsesRouteModelMapping(relayMode int, modelName string) bool {
	switch relayMode {
	case relayconstant.RelayModeResponses:
		return strings.TrimSpace(modelName) == "gpt-5.4"
	case relayconstant.RelayModeResponsesCompact:
		return strings.TrimSpace(modelName) == "gpt-5.4-openai-compact"
	default:
		return false
	}
}

func GetResponsesModelMapping(channel *model.Channel, relayMode int) map[string]string {
	if channel == nil {
		return nil
	}

	merged := make(map[string]string)
	for k, v := range getDefaultResponsesModelMapping(channel.Type, relayMode) {
		merged[k] = v
	}
	for k, v := range getConfiguredResponsesModelMapping(channel.GetOtherSettings(), relayMode) {
		merged[k] = v
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func GetResponsesUpstreamModel(channel *model.Channel, relayMode int, modelName string) (string, bool) {
	mapping := GetResponsesModelMapping(channel, relayMode)
	if len(mapping) == 0 {
		return "", false
	}
	upstream, ok := mapping[strings.TrimSpace(modelName)]
	upstream = strings.TrimSpace(upstream)
	if !ok || upstream == "" {
		return "", false
	}
	return upstream, true
}

func ChannelSupportsRequestedModelForRelay(channel *model.Channel, relayMode int, modelName string) bool {
	if channel == nil {
		return false
	}
	if !RequiresResponsesRouteModelMapping(relayMode, modelName) {
		return true
	}
	_, ok := GetResponsesUpstreamModel(channel, relayMode, modelName)
	return ok
}

func MergeChannelModelMappings(baseMapping string, extra map[string]string) string {
	if len(extra) == 0 {
		return baseMapping
	}
	merged := make(map[string]string)
	if strings.TrimSpace(baseMapping) != "" && strings.TrimSpace(baseMapping) != "{}" {
		_ = common.UnmarshalJsonStr(baseMapping, &merged)
	}
	for k, v := range extra {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}
		merged[key] = value
	}
	if len(merged) == 0 {
		return ""
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return baseMapping
	}
	return string(data)
}

func getDefaultResponsesModelMapping(channelType int, relayMode int) map[string]string {
	switch relayMode {
	case relayconstant.RelayModeResponses:
		return cloneStringMap(defaultResponsesModelMappings[channelType])
	case relayconstant.RelayModeResponsesCompact:
		return cloneStringMap(defaultResponsesCompactModelMappings[channelType])
	default:
		return nil
	}
}

func getConfiguredResponsesModelMapping(settings dto.ChannelOtherSettings, relayMode int) map[string]string {
	switch relayMode {
	case relayconstant.RelayModeResponses:
		return normalizeStringMap(settings.ResponsesModelMapping)
	case relayconstant.RelayModeResponsesCompact:
		return normalizeStringMap(settings.ResponsesCompactModelMapping)
	default:
		return nil
	}
}

func normalizeStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(input))
	for k, v := range input {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
