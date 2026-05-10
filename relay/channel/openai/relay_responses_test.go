package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupResponsesStreamTest(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *http.Response) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	if oldStreamingTimeout <= 0 {
		constant.StreamingTimeout = 30
		t.Cleanup(func() {
			constant.StreamingTimeout = oldStreamingTimeout
		})
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:        true,
		OriginModelName: "gpt-5.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "default",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}
	resp.Body = ioNopCloser{strings.NewReader(body)}
	return c, rec, info, resp
}

type ioNopCloser struct {
	*strings.Reader
}

func (n ioNopCloser) Close() error { return nil }

func TestOaiResponsesStreamHandlerPreservesCompleted(t *testing.T) {
	c, rec, info, resp := setupResponsesStreamTest(t,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n"+
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"model\":\"gpt-5.5\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg_1\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n",
	)

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.completed"))
	assert.NotContains(t, rec.Body.String(), "data: [DONE]\n\nevent: response.completed\nevent: response.completed")
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyResponsesCompletedSynthesized))
}

func TestOaiResponsesStreamHandlerSynthesizesCompletedOnEOF(t *testing.T) {
	c, rec, info, resp := setupResponsesStreamTest(t,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n",
	)

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Contains(t, rec.Body.String(), "event: response.completed")
	assert.Contains(t, rec.Body.String(), "\"type\":\"response.completed\"")
	assert.Contains(t, rec.Body.String(), "\"text\":\"ok\"")
	assert.Contains(t, rec.Body.String(), "data: [DONE]")
	assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyResponsesCompletedSynthesized))
	assert.Equal(t, "eof_without_completed", common.GetContextKeyString(c, constant.ContextKeyResponsesCompletedSynthReason))
}

func TestOaiResponsesStreamHandlerDoesNotSynthesizeEmptyEOF(t *testing.T) {
	c, rec, info, resp := setupResponsesStreamTest(t,
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"model\":\"gpt-5.5\"}}\n",
	)

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.NotContains(t, rec.Body.String(), "event: response.completed")
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyResponsesCompletedSynthesized))
}

func TestOaiResponsesStreamHandlerDoesNotSynthesizeOnMalformedChunk(t *testing.T) {
	c, rec, info, resp := setupResponsesStreamTest(t,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"\n",
	)

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.NotNil(t, usage)
	require.Nil(t, apiErr)
	assert.NotContains(t, rec.Body.String(), "event: response.completed")
	assert.False(t, common.GetContextKeyBool(c, constant.ContextKeyResponsesCompletedSynthesized))
	assert.Equal(t, 0, usage.TotalTokens)
}
