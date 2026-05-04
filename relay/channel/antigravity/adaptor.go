package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
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
	req.Set("Authorization", "Bearer "+strings.TrimSpace(key.AccessToken))
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else {
		req.Set("Accept", "application/json")
	}
	// Current Antigravity content requests are stricter about client versioning.
	// Match the installed desktop client's lean content-request header shape:
	// send only User-Agent, omit extra Google client headers here.
	req.Set("User-Agent", antigravityContentUserAgent)
	req.Del("X-Goog-Api-Client")
	req.Del("Client-Metadata")
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("antigravity channel: request is nil")
	}
	resolvedModel, err := resolveRequestedModel(info.UpstreamModelName)
	if err != nil {
		return nil, err
	}
	info.UpstreamModelName = resolvedModel

	geminiAdaptor := gemini.Adaptor{}
	converted, err := geminiAdaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	geminiRequest, ok := converted.(*dto.GeminiChatRequest)
	if !ok {
		return nil, errors.New("antigravity channel: failed to convert request to gemini format")
	}
	return a.buildAntigravityEnvelope(c, info, resolvedModel, geminiRequest)
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
	chatReq, filteredTools, err := convertResponsesRequestToChatRequest(request)
	if err != nil {
		return nil, err
	}
	if len(filteredTools) > 0 {
		logger.LogInfo(c, fmt.Sprintf("antigravity filtered unsupported responses tools: %s", strings.Join(filteredTools, ", ")))
	}
	if info.ChannelSetting.SystemPrompt != "" {
		applyAntigravitySystemPrompt(info, chatReq)
	}
	return a.ConvertOpenAIRequest(c, info, chatReq)
}

func (a *Adaptor) buildAntigravityEnvelope(c *gin.Context, info *relaycommon.RelayInfo, resolvedModel string, geminiRequest *dto.GeminiChatRequest) (any, error) {
	if geminiRequest == nil {
		return nil, errors.New("antigravity channel: gemini request is nil")
	}
	normalizeAntigravityGeminiRequest(geminiRequest)

	key, err := a.resolveCredential(c, info)
	if err != nil {
		return nil, err
	}
	projectID := key.EffectiveProjectID()
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("antigravity channel: missing project_id or managed_project_id")
	}

	sessionID := deriveAntigravitySessionID(geminiRequest, resolvedModel, projectID)
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "agent-" + common.GetUUID()
	}
	requestID := "agent-" + common.GetUUID()
	geminiRequest.SessionID = sessionID

	envelope := &requestEnvelope{
		Project:     projectID,
		Model:       resolvedModel,
		Request:     geminiRequest,
		RequestType: "agent",
		RequestID:   requestID,
		UserAgent:   "antigravity",
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeResponsesCompact {
		envelope.RequestType = "compact"
	}
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
		apiErr := a.handleWrappedErrorResponse(c, info, resp)
		return nil, apiErr
	}
	a.markSuccess(info)
	unwrapped, unwrapErr := unwrapAntigravityHTTPResponse(resp)
	if unwrapErr != nil {
		a.markFailure(c, info, http.StatusBadGateway, unwrapErr.Error())
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
		if info.ChannelId > 0 && info.ChannelIsMultiKey {
			refreshed, _, refreshErr := service.RefreshAntigravityChannelKeyCredential(ctx, info.ChannelId, info.ChannelMultiKeyIndex, false)
			if refreshErr != nil {
				a.markFailure(c, info, http.StatusUnauthorized, refreshErr.Error())
				return nil, refreshErr
			}
			common.SetContextKey(c, constant.ContextKeyAntigravityCredentialRefreshApplied, true)
			info.ApiKey = mustJSON(refreshed)
			return refreshed, nil
		}
		if info.ChannelId > 0 {
			refreshed, _, refreshErr := service.RefreshAntigravityChannelCredential(ctx, info.ChannelId, service.AntigravityCredentialRefreshOptions{ResetCaches: false})
			if refreshErr != nil {
				a.markFailure(c, info, http.StatusUnauthorized, refreshErr.Error())
				return nil, refreshErr
			}
			common.SetContextKey(c, constant.ContextKeyAntigravityCredentialRefreshApplied, true)
			info.ApiKey = mustJSON(refreshed)
			return refreshed, nil
		}
		refreshed, refreshErr := service.RefreshAntigravityCredentialWithProxy(ctx, info.ApiKey, info.ChannelSetting.Proxy)
		if refreshErr != nil {
			a.markFailure(c, info, http.StatusUnauthorized, refreshErr.Error())
			return nil, refreshErr
		}
		common.SetContextKey(c, constant.ContextKeyAntigravityCredentialRefreshApplied, true)
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
		a.markFailure(c, info, resp.StatusCode, msg)
		return types.NewOpenAIError(errors.New(msg), types.ErrorCodeBadResponse, resp.StatusCode)
	}
	msg := fmt.Sprintf("antigravity upstream error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	a.markFailure(c, info, resp.StatusCode, msg)
	return types.NewOpenAIError(errors.New(msg), types.ErrorCodeBadResponse, resp.StatusCode)
}

func (a *Adaptor) markSuccess(info *relaycommon.RelayInfo) {
	if info == nil || info.ChannelType != constant.ChannelTypeAntigravity || !info.ChannelIsMultiKey {
		return
	}
	_ = service.MarkAntigravityKeySuccess(info.ChannelId, info.ChannelMultiKeyIndex, time.Now())
}

func (a *Adaptor) markFailure(c *gin.Context, info *relaycommon.RelayInfo, statusCode int, msg string) {
	if c != nil {
		class := string(service.ClassifyAntigravityError(statusCode, msg))
		common.SetContextKey(c, constant.ContextKeyAntigravityErrorClass, class)
	}
	if info == nil || info.ChannelType != constant.ChannelTypeAntigravity || !info.ChannelIsMultiKey {
		return
	}
	class := service.ClassifyAntigravityError(statusCode, msg)
	_ = service.MarkAntigravityKeyFailure(info.ChannelId, info.ChannelMultiKeyIndex, info.RelayMode, info.OriginModelName, class, time.Now(), msg)
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

func resolveRequestedModel(requested string) (string, error) {
	model := strings.TrimSpace(requested)
	if model == "" {
		return "", errors.New("antigravity channel: missing model name")
	}
	if upstream, ok := modelAliasToUpstream[model]; ok {
		return upstream, nil
	}
	for _, supported := range ModelList {
		if model == supported {
			if upstream, ok := modelAliasToUpstream[model]; ok {
				return upstream, nil
			}
		}
	}
	if strings.HasPrefix(model, "gemini-") || strings.HasPrefix(model, "claude-") {
		return model, nil
	}
	return "", fmt.Errorf("antigravity channel: model %q is not mapped", requested)
}

func mustJSON(key *service.AntigravityOAuthKey) string {
	data, _ := json.Marshal(key)
	return string(data)
}

func normalizeAntigravityGeminiRequest(request *dto.GeminiChatRequest) {
	ensureAntigravityThoughtSignatures(request)
	normalizeAntigravitySystemInstruction(request)
}

func normalizeAntigravitySystemInstruction(request *dto.GeminiChatRequest) {
	if request == nil || request.SystemInstructions == nil {
		return
	}
	request.SystemInstructions.Role = "user"
}

func deriveAntigravitySessionID(request *dto.GeminiChatRequest, model string, projectID string) string {
	if request == nil {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString(strings.ToLower(strings.TrimSpace(model)))
	builder.WriteString("\n")
	builder.WriteString(strings.TrimSpace(projectID))
	builder.WriteString("\n")
	if request.SystemInstructions != nil {
		builder.WriteString("system:")
		builder.WriteString(flattenGeminiContent(request.SystemInstructions))
		builder.WriteString("\n")
	}
	for _, content := range request.Contents {
		role := strings.ToLower(strings.TrimSpace(content.Role))
		if role != "user" && role != "model" && role != "assistant" {
			continue
		}
		builder.WriteString(role)
		builder.WriteString(":")
		builder.WriteString(flattenGeminiContent(&content))
		builder.WriteString("\n")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "newapi-antigravity:" + fmt.Sprintf("%x", sum[:12])
}

func flattenGeminiContent(content *dto.GeminiChatContent) string {
	if content == nil {
		return ""
	}
	parts := make([]string, 0, len(content.Parts))
	for _, part := range content.Parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			parts = append(parts, text)
			continue
		}
		if part.FunctionCall != nil {
			name := strings.TrimSpace(part.FunctionCall.FunctionName)
			if name != "" {
				parts = append(parts, "fn:"+name)
			}
			continue
		}
		if part.FunctionResponse != nil {
			name := strings.TrimSpace(part.FunctionResponse.Name)
			if name != "" {
				parts = append(parts, "fn_result:"+name)
			}
		}
	}
	return strings.Join(parts, "\n")
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
