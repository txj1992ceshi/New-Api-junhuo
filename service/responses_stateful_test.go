package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveResponsesStatefulCapability(t *testing.T) {
	native := ResolveResponsesStatefulCapability(dto.ChannelOtherSettings{
		SupportsNativeStatefulResponses: true,
	}, constant.ChannelTypeOpenAI, relayconstant.RelayModeResponses, "gpt-5.4", true)
	require.Equal(t, ResponsesStatefulCapabilityNative, native)

	replay := ResolveResponsesStatefulCapability(dto.ChannelOtherSettings{}, constant.ChannelTypeAntigravity, relayconstant.RelayModeResponses, "gpt-5.4", true)
	require.Equal(t, ResponsesStatefulCapabilityReplay, replay)

	none := ResolveResponsesStatefulCapability(dto.ChannelOtherSettings{}, constant.ChannelTypeOpenAI, relayconstant.RelayModeResponses, "gpt-5.4", true)
	require.Equal(t, ResponsesStatefulCapabilityNone, none)
}

func TestPrepareAndStoreResponsesReplayEntity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Set(common.RequestIdKey, "req-current")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4",
		RelayMode:       relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         3,
			ChannelType:       constant.ChannelTypeAntigravity,
			UpstreamModelName: "gemini-3-flash",
		},
	}
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-5.4",
		Input: mustMarshalCompatRaw("hello"),
	}
	require.NoError(t, PrepareResponsesReplayRequest(ctx, info, req))
	require.NoError(t, StoreResponsesReplayEntity(ctx, info, req))

	entity, found, err := GetResponsesReplayEntity("chatcmpl-req-current")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 3, entity.ChannelID)
	require.Equal(t, "gpt-5.4", entity.OriginModelName)
}

func TestPrepareResponsesReplayRequestMergesPreviousEntity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Set(common.RequestIdKey, "req-next")

	previous := ResponsesReplayEntity{
		ResponseID:      "chatcmpl-prev",
		RelayMode:       relayconstant.RelayModeResponses,
		OriginModelName: "gpt-5.4",
		ChannelID:       3,
		ChannelType:     constant.ChannelTypeAntigravity,
		Input: mustMarshalCompatRaw([]map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "first"},
				},
			},
		}),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, getResponsesReplayEntityCache().SetWithTTL(previous.ResponseID, previous, time.Hour))

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4",
		RelayMode:       relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         3,
			ChannelType:       constant.ChannelTypeAntigravity,
			UpstreamModelName: "gemini-3-flash",
		},
	}
	req := &dto.OpenAIResponsesRequest{
		Model:              "gpt-5.4",
		PreviousResponseID: previous.ResponseID,
		Input:              mustMarshalCompatRaw("second"),
	}
	require.NoError(t, PrepareResponsesReplayRequest(ctx, info, req))

	var items []map[string]any
	require.NoError(t, common.Unmarshal(req.Input, &items))
	require.Len(t, items, 2)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyResponsesEntityLookupHit))
}

func TestChannelSupportsRequestForRelayCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(common.KeyRequestBody, []byte(`{"model":"gpt-5.4","previous_response_id":"resp_1"}`))

	nativeChannel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	nativeChannel.SetOtherSettings(dto.ChannelOtherSettings{
		SupportsNativeStatefulResponses: true,
		ResponsesModelMapping: map[string]string{
			"gpt-5.4": "gpt-5.4-native",
		},
	})
	require.True(t, ChannelSupportsRequestForRelayCapability(ctx, nativeChannel, relayconstant.RelayModeResponses, "gpt-5.4", ResponsesStatefulCapabilityNative))

	replayChannel := &model.Channel{Type: constant.ChannelTypeAntigravity}
	require.True(t, ChannelSupportsRequestForRelayCapability(ctx, replayChannel, relayconstant.RelayModeResponses, "gpt-5.4", ResponsesStatefulCapabilityReplay))
}
