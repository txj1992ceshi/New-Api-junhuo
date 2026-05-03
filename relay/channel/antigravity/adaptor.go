package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

type requestEnvelope struct {
	Project     string                 `json:"project"`
	Model       string                 `json:"model"`
	Request     *dto.GeminiChatRequest `json:"request"`
	RequestType string                 `json:"requestType,omitempty"`
	RequestID   string                 `json:"requestId,omitempty"`
	UserAgent   string                 `json:"userAgent,omitempty"`
}

type wrappedResponseEnvelope struct {
	Response json.RawMessage `json:"response"`
	Error    *struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		Status  string `json:"status,omitempty"`
	} `json:"error,omitempty"`
}

const (
	accessExpiryBuffer          = 5 * time.Minute
	antigravityContentVersion   = "1.23.2"
	antigravityContentUserAgent = "antigravity/" + antigravityContentVersion + " darwin/arm64"
	thoughtSignatureBypassValue = "context_engineering_is_the_way_to_go"
)

type resolvedAntigravityRequest struct {
	UpstreamModel string
	RequestStyle  antigravityRequestStyle
	RequestType   string
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("antigravity channel: missing relay info")
	}
	action := ":generateContent"
	if info.IsStream {
		action = ":streamGenerateContent?alt=sse"
	}
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	return fmt.Sprintf("%s/v1internal%s", baseURL, action), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	key, err := a.resolveCredential(c, info)
	if err != nil {
		return err
	}
	resolved, err := resolveRequestedModel(info.UpstreamModelName, info.RelayMode)
	if err != nil {
		return err
	}
	req.Set("Authorization", "Bearer "+strings.TrimSpace(key.AccessToken))
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else {
		req.Set("Accept", "application/json")
	}
	switch resolved.RequestStyle {
	case antigravityRequestStyleGeminiCLI:
		req.Set("User-Agent", antigravityGeminiCLIUserAgent)
		req.Set("X-Goog-Api-Client", antigravityGeminiCLIApiClient)
		req.Set("Client-Metadata", antigravityGeminiCLIClientMetadata)
	default:
		// Current Antigravity content requests are stricter about client versioning.
		// Match the installed desktop client's lean content-request header shape:
		// send only User-Agent, omit extra Google client headers here.
		req.Set("User-Agent", antigravityContentUserAgent)
		req.Del("X-Goog-Api-Client")
		req.Del("Client-Metadata")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("antigravity channel: request is nil")
	}
	resolved, err := resolveRequestedModel(info.UpstreamModelName, info.RelayMode)
	if err != nil {
		return nil, err
	}
	info.UpstreamModelName = resolved.UpstreamModel

	geminiAdaptor := gemini.Adaptor{}
	converted, err := geminiAdaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	geminiRequest, ok := converted.(*dto.GeminiChatRequest)
	if !ok {
		return nil, errors.New("antigravity channel: failed to convert request to gemini format")
	}
	ensureAntigravityThoughtSignatures(geminiRequest)

	key, err := a.resolveCredential(c, info)
	if err != nil {
		return nil, err
	}
	projectID := key.EffectiveProjectID()
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("antigravity channel: missing project_id or managed_project_id")
	}
	sessionID := "agent-" + common.GetUUID()
	geminiRequest.SessionID = sessionID
	envelope := &requestEnvelope{
		Project:     projectID,
		Model:       resolved.UpstreamModel,
		Request:     geminiRequest,
		RequestType: "agent",
		RequestID:   sessionID,
		UserAgent:   "antigravity",
	}
	applyRequestEnvelopeStyle(envelope, resolved, sessionID)
	return envelope, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("antigravity channel: audio endpoints not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("antigravity channel: image endpoints not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info == nil {
		return nil, errors.New("antigravity channel: missing relay info")
	}
	resolved, err := resolveRequestedModel(info.UpstreamModelName, info.RelayMode)
	if err != nil {
		return nil, err
	}
	info.UpstreamModelName = resolved.UpstreamModel

	geminiRequest, err := buildGeminiRequestFromResponses(c, info, request)
	if err != nil {
		return nil, err
	}
	ensureAntigravityThoughtSignatures(geminiRequest)

	key, err := a.resolveCredential(c, info)
	if err != nil {
		return nil, err
	}
	projectID := key.EffectiveProjectID()
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("antigravity channel: missing project_id or managed_project_id")
	}
	sessionID := "agent-" + common.GetUUID()
	geminiRequest.SessionID = sessionID

	envelope := &requestEnvelope{
		Project:     projectID,
		Model:       resolved.UpstreamModel,
		Request:     geminiRequest,
		RequestType: resolved.RequestType,
		RequestID:   sessionID,
		UserAgent:   "antigravity",
	}
	applyRequestEnvelopeStyle(envelope, resolved, sessionID)
	logger.LogInfo(c, fmt.Sprintf(
		"antigravity responses request: channel_id=%d relay_mode=%d origin_model=%s upstream_model=%s style=%s request_type=%s tools=%d messages=%d",
		info.ChannelId,
		info.RelayMode,
		strings.TrimSpace(info.OriginModelName),
		envelope.Model,
		resolved.RequestStyle,
		envelope.RequestType,
		len(geminiRequest.GetTools()),
		len(geminiRequest.Contents),
	))
	return envelope, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("antigravity channel: native gemini endpoint not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resp == nil {
		return nil, types.NewOpenAIError(errors.New("empty antigravity response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if resp.StatusCode >= 400 {
		return nil, a.handleWrappedErrorResponse(c, info, resp)
	}
	unwrapped, unwrapErr := unwrapAntigravityHTTPResponse(resp)
	if unwrapErr != nil {
		return nil, types.NewOpenAIError(unwrapErr, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if info != nil {
		switch info.RelayMode {
		case relayconstant.RelayModeResponses:
			if info.IsStream {
				return antigravityResponsesStreamHandler(c, info, unwrapped)
			}
			return antigravityResponsesHandler(c, info, unwrapped)
		case relayconstant.RelayModeResponsesCompact:
			return antigravityResponsesCompactHandler(c, info, unwrapped)
		}
	}
	geminiAdaptor := gemini.Adaptor{}
	return geminiAdaptor.DoResponse(c, unwrapped, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) resolveCredential(c *gin.Context, info *relaycommon.RelayInfo) (*service.AntigravityOAuthKey, error) {
	key, err := service.ParseAntigravityOAuthKey(strings.TrimSpace(info.ApiKey))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key.AccessToken) == "" || key.IsAccessExpired(accessExpiryBuffer) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
		defer cancel()
		if info.ChannelId > 0 {
			refreshed, _, refreshErr := service.RefreshAntigravityChannelCredential(ctx, info.ChannelId, service.AntigravityCredentialRefreshOptions{ResetCaches: false})
			if refreshErr != nil {
				return nil, refreshErr
			}
			info.ApiKey = mustJSON(refreshed)
			return refreshed, nil
		}
		refreshed, refreshErr := service.RefreshAntigravityCredentialWithProxy(ctx, info.ApiKey, info.ChannelSetting.Proxy)
		if refreshErr != nil {
			return nil, refreshErr
		}
		info.ApiKey = mustJSON(refreshed)
		return refreshed, nil
	}
	return key, nil
}

func (a *Adaptor) handleWrappedErrorResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) *types.NewAPIError {
	defer service.CloseResponseBodyGracefully(resp)
	body, _ := io.ReadAll(resp.Body)
	var envelope wrappedResponseEnvelope
	if err := common.Unmarshal(body, &envelope); err == nil && envelope.Error != nil {
		msg := strings.TrimSpace(envelope.Error.Message)
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		logAntigravityUpstreamError(c, info, resp.StatusCode, msg)
		return types.NewOpenAIError(errors.New(msg), types.ErrorCodeBadResponse, resp.StatusCode)
	}
	bodySummary := strings.TrimSpace(string(body))
	logAntigravityUpstreamError(c, info, resp.StatusCode, bodySummary)
	return types.NewOpenAIError(fmt.Errorf("antigravity upstream error: status=%d body=%s", resp.StatusCode, bodySummary), types.ErrorCodeBadResponse, resp.StatusCode)
}

func unwrapAntigravityHTTPResponse(resp *http.Response) (*http.Response, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("empty antigravity response")
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		reader, err := unwrapAntigravitySSE(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body = io.NopCloser(reader)
		resp.ContentLength = -1
		return resp, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()

	var envelope wrappedResponseEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		msg := strings.TrimSpace(envelope.Error.Message)
		if msg == "" {
			msg = "antigravity upstream error"
		}
		return nil, errors.New(msg)
	}
	if len(envelope.Response) == 0 {
		return nil, errors.New("antigravity response envelope missing response field")
	}

	resp.Body = io.NopCloser(bytes.NewReader(envelope.Response))
	resp.ContentLength = int64(len(envelope.Response))
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func unwrapAntigravitySSE(body io.ReadCloser) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		defer body.Close()
		defer pw.Close()
		scanner := bufio.NewScanner(body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 2*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
				if payload != "" && payload != "[DONE]" {
					var envelope wrappedResponseEnvelope
					if err := common.UnmarshalJsonStr(payload, &envelope); err == nil && len(envelope.Response) > 0 {
						line = "data: " + string(envelope.Response)
					}
				}
			}
			if _, err := io.WriteString(pw, line+"\n"); err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	return pr, nil
}

func resolveRequestedModel(requested string, relayMode int) (*resolvedAntigravityRequest, error) {
	model := strings.TrimSpace(requested)
	if model == "" {
		return nil, errors.New("antigravity channel: missing model name")
	}
	resolved := &resolvedAntigravityRequest{
		RequestStyle: antigravityRequestStyleNative,
		RequestType:  "agent",
	}

	if relayMode == relayconstant.RelayModeResponses || relayMode == relayconstant.RelayModeResponsesCompact {
		resolved.RequestStyle = antigravityRequestStyleGeminiCLI
		if relayMode == relayconstant.RelayModeResponsesCompact {
			resolved.RequestType = "compact"
		}
		if upstream, ok := responsesModelAliasToUpstream[model]; ok {
			resolved.UpstreamModel = upstream
			return resolved, nil
		}
	}

	if relayMode == relayconstant.RelayModeResponsesCompact {
		resolved.RequestType = "compact"
	}
	if upstream, ok := modelAliasToUpstream[model]; ok {
		resolved.UpstreamModel = upstream
		return resolved, nil
	}
	for _, supported := range ModelList {
		if model == supported {
			if upstream, ok := modelAliasToUpstream[model]; ok {
				resolved.UpstreamModel = upstream
				return resolved, nil
			}
		}
	}
	if strings.HasPrefix(model, "gemini-") || strings.HasPrefix(model, "claude-") {
		resolved.UpstreamModel = model
		return resolved, nil
	}
	return nil, fmt.Errorf("antigravity channel: model %q is not mapped", requested)
}

func mustJSON(key *service.AntigravityOAuthKey) string {
	data, _ := json.Marshal(key)
	return string(data)
}

func ensureAntigravityThoughtSignatures(request *dto.GeminiChatRequest) {
	if request == nil {
		return
	}
	for contentIdx := range request.Contents {
		role := strings.ToLower(strings.TrimSpace(request.Contents[contentIdx].Role))
		if role != "model" && role != "assistant" {
			continue
		}
		for partIdx := range request.Contents[contentIdx].Parts {
			part := &request.Contents[contentIdx].Parts[partIdx]
			if part.FunctionCall == nil || len(part.ThoughtSignature) > 0 {
				continue
			}
			part.ThoughtSignature = json.RawMessage(strconv.Quote(thoughtSignatureBypassValue))
		}
	}
}

func isAntigravityRelayResponses(info *relaycommon.RelayInfo) bool {
	return info != nil && (info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact)
}

func applyRequestEnvelopeStyle(envelope *requestEnvelope, resolved *resolvedAntigravityRequest, sessionID string) {
	if envelope == nil || resolved == nil {
		return
	}
	if resolved.RequestStyle == antigravityRequestStyleGeminiCLI {
		envelope.RequestType = ""
		envelope.RequestID = ""
		envelope.UserAgent = ""
		return
	}
	envelope.RequestID = sessionID
	envelope.UserAgent = "antigravity"
	envelope.RequestType = resolved.RequestType
}

func logAntigravityUpstreamError(c *gin.Context, info *relaycommon.RelayInfo, statusCode int, message string) {
	if c == nil {
		return
	}
	msg := strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if len(msg) > 240 {
		msg = msg[:240]
	}
	channelID := 0
	relayMode := 0
	originModel := ""
	upstreamModel := ""
	if info != nil {
		channelID = info.ChannelId
		relayMode = info.RelayMode
		originModel = strings.TrimSpace(info.OriginModelName)
		upstreamModel = strings.TrimSpace(info.UpstreamModelName)
	}
	logger.LogInfo(c, fmt.Sprintf(
		"antigravity upstream error: channel_id=%d relay_mode=%d origin_model=%s upstream_model=%s status=%d summary=%s",
		channelID,
		relayMode,
		originModel,
		upstreamModel,
		statusCode,
		msg,
	))
}

func antigravitySystemRoleName(modelName string) string {
	if strings.HasPrefix(modelName, "o") {
		if !strings.HasPrefix(modelName, "o1-mini") && !strings.HasPrefix(modelName, "o1-preview") {
			return "developer"
		}
	} else if strings.HasPrefix(modelName, "gpt-5") {
		return "developer"
	}
	return "system"
}

func mapGeminiFinishReason(finishReason *string) string {
	if finishReason == nil {
		return constant.FinishReasonStop
	}
	switch *finishReason {
	case "STOP":
		return constant.FinishReasonStop
	case "MAX_TOKENS":
		return constant.FinishReasonLength
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "OTHER":
		return constant.FinishReasonContentFilter
	default:
		return constant.FinishReasonContentFilter
	}
}
