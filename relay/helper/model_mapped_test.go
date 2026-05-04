package helper

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelperResponsesCompactPreservesOriginModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set("model_mapping", `{"gpt-5.4-openai-compact":"gpt-5.5-openai-compact"}`)

	req := &dto.OpenAIResponsesCompactionRequest{Model: "gpt-5.4-openai-compact"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4-openai-compact",
		RelayMode:       relayconstant.RelayModeResponsesCompact,
	}

	err := ModelMappedHelper(ctx, info, req)
	require.NoError(t, err)
	require.True(t, info.IsModelMapped)
	require.Equal(t, "gpt-5.4-openai-compact", info.OriginModelName)
	require.Equal(t, "gpt-5.5-openai-compact", info.UpstreamModelName)
	require.Equal(t, "gpt-5.5-openai-compact", req.Model)
}
