package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	codexProbeModel          = "gpt-5.4"
	codexProbeRequestTimeout = 20 * time.Second
	codexProbeAttemptLimit   = 3
	codexProbeWindow         = 5 * time.Minute
)

var (
	codexProbeSuccessWindows syncMapTimeWindow
	codexProbeFailWindows    syncMapTimeWindow
	codexInvalidKeyWindows   syncMapTimeWindow
	codexProbePendingCounts  syncMapCounter
	codexProbeInFlight       sync.Map
)

type syncMapCounter struct {
	mu    sync.Mutex
	store map[int]int
}

func (c *syncMapCounter) add(channelID int, delta int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[int]int)
	}
	next := c.store[channelID] + delta
	if next <= 0 {
		delete(c.store, channelID)
		return 0
	}
	c.store[channelID] = next
	return next
}

func (c *syncMapCounter) get(channelID int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		return 0
	}
	return c.store[channelID]
}

func recordWindowEvent(window *syncMapTimeWindow, channelID int, at time.Time, duration time.Duration) {
	if channelID <= 0 {
		return
	}
	times := window.load(channelID)
	times = append(times, at)
	cutoff := at.Add(-duration)
	filtered := make([]time.Time, 0, len(times))
	for _, ts := range times {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	window.storeTimes(channelID, filtered)
}

func recentWindowCount(window *syncMapTimeWindow, channelID int, now time.Time, duration time.Duration) int {
	if channelID <= 0 {
		return 0
	}
	times := window.load(channelID)
	cutoff := now.Add(-duration)
	filtered := make([]time.Time, 0, len(times))
	for _, ts := range times {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	window.storeTimes(channelID, filtered)
	return len(filtered)
}

func recordCodexProbeSuccess(channelID int, at time.Time) {
	recordWindowEvent(&codexProbeSuccessWindows, channelID, at, codexProbeWindow)
}

func recordCodexProbeFail(channelID int, at time.Time) {
	recordWindowEvent(&codexProbeFailWindows, channelID, at, codexProbeWindow)
}

func recordCodexInvalidKey(channelID int, at time.Time) {
	recordWindowEvent(&codexInvalidKeyWindows, channelID, at, codexProbeWindow)
}

func RecentCodexProbeSuccessCount(channelID int, now time.Time) int {
	return recentWindowCount(&codexProbeSuccessWindows, channelID, now, codexProbeWindow)
}

func RecentCodexProbeFailCount(channelID int, now time.Time) int {
	return recentWindowCount(&codexProbeFailWindows, channelID, now, codexProbeWindow)
}

func RecentCodexInvalidKeyCount(channelID int, now time.Time) int {
	return recentWindowCount(&codexInvalidKeyWindows, channelID, now, codexProbeWindow)
}

func PendingCodexProbeCount(channelID int) int {
	return codexProbePendingCounts.get(channelID)
}

func setCodexProbeStatus(channelID int, modelName string, result string, at time.Time) {
	state := cursorProStateForChannel(channelID)
	if state == nil {
		return
	}
	state.LastProbeAt = at
	state.LastProbeModel = modelName
	state.LastProbeResult = result
}

func EnqueueCodexNewKeyProbe(channelID int, keyIndex int, pendingResult string) bool {
	if channelID <= 0 || keyIndex < 0 {
		return false
	}
	probeID := fmt.Sprintf("%d:%d", channelID, keyIndex)
	if _, loaded := codexProbeInFlight.LoadOrStore(probeID, struct{}{}); loaded {
		return false
	}
	codexProbePendingCounts.add(channelID, 1)
	setCodexProbeStatus(channelID, codexProbeModel, pendingResult, time.Now())
	gopool.Go(func() {
		defer codexProbeInFlight.Delete(probeID)
		defer codexProbePendingCounts.add(channelID, -1)
		runCodexNewKeyProbe(channelID, keyIndex)
	})
	return true
}

func runCodexNewKeyProbe(channelID int, keyIndex int) {
	for attempt := 0; attempt < codexProbeAttemptLimit; attempt++ {
		now := time.Now()
		kind, apiErr := executeCodexKeyProbeAttempt(channelID, keyIndex)
		switch kind {
		case CodexErrorKindSuccess:
			_ = MarkCodexKeySuccess(channelID, keyIndex, now)
			recordCodexProbeSuccess(channelID, now)
			setCodexProbeStatus(channelID, codexProbeModel, "probe_succeeded", now)
			if attempt < codexProbeAttemptLimit-1 {
				time.Sleep(250 * time.Millisecond)
			}
		case CodexErrorKindInvalid:
			_ = MarkCodexKeyInvalid(channelID, keyIndex, now, "invalid_key")
			recordCodexProbeFail(channelID, now)
			setCodexProbeStatus(channelID, codexProbeModel, "probe_failed_invalid_key", now)
			return
		case CodexErrorKindRateLimit:
			_ = MarkCodexKeyRateLimited(channelID, keyIndex, now)
			recordCodexProbeFail(channelID, now)
			setCodexProbeStatus(channelID, codexProbeModel, "probe_failed_rate_limit", now)
			return
		case CodexErrorKindAuth:
			_ = MarkCodexKeyAuthFail(channelID, keyIndex, now)
			recordCodexProbeFail(channelID, now)
			setCodexProbeStatus(channelID, codexProbeModel, "probe_failed_auth", now)
			return
		case CodexErrorKindSoftFail:
			_ = MarkCodexKeySoftFail(channelID, keyIndex, now)
			recordCodexProbeFail(channelID, now)
			setCodexProbeStatus(channelID, codexProbeModel, "probe_failed_soft_fail", now)
			return
		default:
			_ = MarkCodexKeyServerError(channelID, keyIndex, now)
			recordCodexProbeFail(channelID, now)
			setCodexProbeStatus(channelID, codexProbeModel, "probe_failed_server", now)
			if apiErr != nil && strings.Contains(strings.ToLower(apiErr.Error()), "usage limit") {
				setCodexProbeStatus(channelID, codexProbeModel, "probe_failed_rate_limit", now)
			}
			return
		}
	}
}

func executeCodexKeyProbeAttempt(channelID int, keyIndex int) (CodexErrorKind, *types.NewAPIError) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil || channel == nil {
		return CodexErrorKindServer, types.NewErrorWithStatusCode(fmt.Errorf("probe load channel failed: %w", err), types.ErrorCodeGetChannelFailed, http.StatusInternalServerError)
	}
	if channel.Type != constant.ChannelTypeCodex {
		return CodexErrorKindFatal, types.NewErrorWithStatusCode(fmt.Errorf("channel type is not codex"), types.ErrorCodeInvalidRequest, http.StatusBadRequest)
	}

	keys := channel.GetKeys()
	if keyIndex < 0 || keyIndex >= len(keys) {
		return CodexErrorKindFatal, types.NewErrorWithStatusCode(fmt.Errorf("codex key index out of range"), types.ErrorCodeInvalidRequest, http.StatusBadRequest)
	}
	if invalidReason := validateCodexOAuthKeyPayload(keys[keyIndex]); invalidReason != "" {
		return CodexErrorKindInvalid, types.NewErrorWithStatusCode(errors.New(invalidReason), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(keys[keyIndex]))
	if err != nil {
		apiErr := types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		return ClassifyCodexError(nil, nil, apiErr), apiErr
	}

	payload := map[string]any{
		"model":        codexProbeModel,
		"stream":       true,
		"store":        false,
		"instructions": "",
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": "Reply with exactly OK.",
					},
				},
			},
		},
	}
	body, err := common.Marshal(payload)
	if err != nil {
		apiErr := types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest)
		return CodexErrorKindFatal, apiErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), codexProbeRequestTimeout)
	defer cancel()

	fullURL := relaycommon.GetFullRequestURL(strings.TrimRight(channel.GetBaseURL(), "/"), "/backend-api/codex/responses", channel.Type)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		apiErr := types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		return CodexErrorKindServer, apiErr
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(oauthKey.AccessToken))
	req.Header.Set("chatgpt-account-id", strings.TrimSpace(oauthKey.AccountID))
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", "codex_cli_rs")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	preferIPv4 := channel.GetSetting().PreferIPv4 || channel.Type == constant.ChannelTypeCodex
	client, err := GetHttpClientWithPreference(channel.GetSetting().Proxy, preferIPv4)
	if err != nil {
		apiErr := types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		return CodexErrorKindServer, apiErr
	}

	resp, err := client.Do(req)
	if err != nil {
		apiErr := types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		return ClassifyCodexError(nil, err, apiErr), apiErr
	}
	defer CloseResponseBodyGracefully(resp)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		apiErr := RelayErrorHandler(context.Background(), resp, true)
		return ClassifyCodexError(resp, nil, apiErr), apiErr
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr := types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		return ClassifyCodexError(nil, err, apiErr), apiErr
	}
	if len(respBody) == 0 {
		return CodexErrorKindSoftFail, types.NewErrorWithStatusCode(fmt.Errorf("empty probe response"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	var upstreamErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(respBody, &upstreamErr) == nil && strings.TrimSpace(upstreamErr.Error.Message) != "" {
		apiErr := types.NewErrorWithStatusCode(errors.New(strings.TrimSpace(upstreamErr.Error.Message)), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
		return ClassifyCodexError(resp, nil, apiErr), apiErr
	}
	return CodexErrorKindSuccess, nil
}
