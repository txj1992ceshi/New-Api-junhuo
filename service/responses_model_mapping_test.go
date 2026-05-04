package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestGetResponsesModelMappingUsesDefaultsAndOverrides(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeCodex}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		ResponsesModelMapping: map[string]string{
			"gpt-5.4": "gpt-5.5-custom",
		},
	})

	mapping := GetResponsesModelMapping(channel, relayconstant.RelayModeResponses)
	require.Equal(t, "gpt-5.5-custom", mapping["gpt-5.4"])
}

func TestChannelSupportsRequestedModelForRelay(t *testing.T) {
	openAIChannel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	require.False(t, ChannelSupportsRequestedModelForRelay(openAIChannel, relayconstant.RelayModeResponses, "gpt-5.4"))
	require.True(t, ChannelSupportsRequestedModelForRelay(openAIChannel, relayconstant.RelayModeResponses, "gpt-5.5"))

	antigravityChannel := &model.Channel{Type: constant.ChannelTypeAntigravity}
	require.True(t, ChannelSupportsRequestedModelForRelay(antigravityChannel, relayconstant.RelayModeResponses, "gpt-5.4"))
	require.True(t, ChannelSupportsRequestedModelForRelay(antigravityChannel, relayconstant.RelayModeResponses, "gpt-5.5"))
	require.True(t, ChannelSupportsRequestedModelForRelay(antigravityChannel, relayconstant.RelayModeResponses, "gpt-5.5-mini"))
	require.True(t, ChannelSupportsRequestedModelForRelay(antigravityChannel, relayconstant.RelayModeResponsesCompact, "gpt-5.4-openai-compact"))
	require.True(t, ChannelSupportsRequestedModelForRelay(antigravityChannel, relayconstant.RelayModeResponsesCompact, "gpt-5.5-openai-compact"))
}

func TestGetResponsesModelMappingAntigravityIncludes55Defaults(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeAntigravity}

	mapping := GetResponsesModelMapping(channel, relayconstant.RelayModeResponses)
	require.Equal(t, "gemini-3-flash", mapping["gpt-5.5"])
	require.Equal(t, "gemini-3-flash", mapping["gpt-5.5-mini"])
	require.Equal(t, "gemini-3-flash", mapping["gpt-5.4"])
}
