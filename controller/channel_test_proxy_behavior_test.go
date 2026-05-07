package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestNormalizeChannelTestEndpointUsesResponsesForExternalPoolProxy(t *testing.T) {
	baseURL := "http://127.0.0.1:3401"
	channel := &model.Channel{
		Type:      constant.ChannelTypeOpenAI,
		BaseURL:   &baseURL,
		OtherInfo: `{"cursor_pool_proxy":true}`,
	}

	endpoint := normalizeChannelTestEndpoint(channel, "gpt-5.4", "")
	require.Equal(t, string(constant.EndpointTypeOpenAIResponse), endpoint)
}

func TestBuildTestRequestUsesResponsesPayloadForExternalPoolProxy(t *testing.T) {
	baseURL := "http://127.0.0.1:3501"
	channel := &model.Channel{
		Type:      constant.ChannelTypeOpenAI,
		BaseURL:   &baseURL,
		OtherInfo: `{"kiro_pool_proxy":true}`,
	}

	endpoint := normalizeChannelTestEndpoint(channel, "gpt-5.5", "")
	req := buildTestRequest("gpt-5.5", endpoint, channel, true)
	responseReq, ok := req.(*dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", responseReq.Model)
	require.NotNil(t, responseReq.Stream)
	require.True(t, *responseReq.Stream)
}
