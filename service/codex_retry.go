package service

import (
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type CodexRetryLimits struct {
	MaxAttempts    int
	Max429Retries  int
	Max5xxRetries  int
	MaxSoftRetries int
}

type CodexAttemptResult struct {
	Response *http.Response
	Usage    any
	Error    *types.NewAPIError
}

type CodexRequestExecutor func(c *gin.Context, info *relaycommon.RelayInfo) *CodexAttemptResult

func ExecuteCodexWithRetries(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	executor CodexRequestExecutor,
	limits CodexRetryLimits,
) *CodexAttemptResult {
	if executor == nil {
		return &CodexAttemptResult{
			Error: types.NewErrorWithStatusCode(errors.New("missing codex executor"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError, types.ErrOptionWithSkipRetry()),
		}
	}
	if info == nil || !info.ChannelIsMultiKey || info.ChannelType != constant.ChannelTypeCodex {
		return executor(c, info)
	}
	if limits.MaxAttempts <= 0 {
		limits.MaxAttempts = 3
	}
	triedKeys := map[int]bool{}
	rateLimitRetries := 0
	serverRetries := 0
	softRetries := 0
	hotPathTriggered := false

	currentKeyIndex := info.ChannelMultiKeyIndex
	var lastResult *CodexAttemptResult
	for attempt := 0; attempt < limits.MaxAttempts; attempt++ {
		lastResult = executor(c, info)
		if lastResult == nil {
			lastResult = &CodexAttemptResult{}
		}
		if lastResult.Error == nil {
			_ = MarkCodexKeySuccess(info.ChannelId, currentKeyIndex, time.Now())
			return lastResult
		}

		kind := ClassifyCodexError(lastResult.Response, nil, lastResult.Error)
		switch kind {
		case CodexErrorKindRateLimit:
			_ = MarkCodexKeyRateLimited(info.ChannelId, currentKeyIndex, time.Now())
			rateLimitRetries++
		case CodexErrorKindInvalid:
			_ = MarkCodexKeyInvalid(info.ChannelId, currentKeyIndex, time.Now(), "invalid_key")
		case CodexErrorKindServer:
			_ = MarkCodexKeyServerError(info.ChannelId, currentKeyIndex, time.Now())
			serverRetries++
		case CodexErrorKindAuth:
			_ = MarkCodexKeyAuthFail(info.ChannelId, currentKeyIndex, time.Now())
		case CodexErrorKindSoftFail:
			_ = MarkCodexKeySoftFail(info.ChannelId, currentKeyIndex, time.Now())
			softRetries++
		default:
			return lastResult
		}

		triedKeys[currentKeyIndex] = true

		channel, err := model.CacheGetChannel(info.ChannelId)
		if err != nil || channel == nil {
			return lastResult
		}

		triedKeyCount := len(triedKeys)
		maybeTriggerHotPathReplacement := func(selectErr *types.NewAPIError) {
			if hotPathTriggered {
				return
			}
			if shouldTrigger, reason := ShouldTriggerCursorProReplacementOnHotPath(channel, kind, rateLimitRetries, triedKeyCount, selectErr, time.Now()); shouldTrigger {
				MaybeTriggerCursorProReplacementOnHotPath(info.ChannelId, reason, time.Now())
				hotPathTriggered = true
			}
		}

		if !shouldRetryCodexKey(kind, rateLimitRetries, serverRetries, softRetries, limits) {
			maybeTriggerHotPathReplacement(nil)
			return lastResult
		}
		nextSelection, nextErr := SelectCodexKey(channel, triedKeys, time.Now())
		if nextErr != nil {
			maybeTriggerHotPathReplacement(nextErr)
			return lastResult
		}
		if nextSelection == nil {
			maybeTriggerHotPathReplacement(nil)
			return lastResult
		}
		maybeTriggerHotPathReplacement(nil)
		ApplyCodexSelectionToContext(c, channel, nextSelection)
		info.InitChannelMeta(c)
		currentKeyIndex = nextSelection.KeyIndex

		switch kind {
		case CodexErrorKindRateLimit:
			if rateLimitRetries > 1 {
				time.Sleep(500 * time.Millisecond)
			} else {
				time.Sleep(200 * time.Millisecond)
			}
		case CodexErrorKindInvalid:
			time.Sleep(100 * time.Millisecond)
		case CodexErrorKindServer:
			if serverRetries > 1 {
				time.Sleep(1 * time.Second)
			} else {
				time.Sleep(300 * time.Millisecond)
			}
		case CodexErrorKindSoftFail:
			time.Sleep(300 * time.Millisecond)
		}
	}
	return lastResult
}

func shouldRetryCodexKey(kind CodexErrorKind, rateLimitRetries int, serverRetries int, softRetries int, limits CodexRetryLimits) bool {
	switch kind {
	case CodexErrorKindRateLimit:
		return rateLimitRetries <= limits.Max429Retries
	case CodexErrorKindServer:
		return serverRetries <= limits.Max5xxRetries
	case CodexErrorKindInvalid:
		return true
	case CodexErrorKindAuth:
		return false
	case CodexErrorKindSoftFail:
		return softRetries <= limits.MaxSoftRetries
	default:
		return false
	}
}
