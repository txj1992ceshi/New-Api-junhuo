package service

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

const codexHotPathTriggerWindow = 5 * time.Minute

var codexHotPathTriggerWindows syncMapTimeWindow

type syncMapTimeWindow struct {
	mu    sync.Mutex
	store map[int][]time.Time
}

func (w *syncMapTimeWindow) load(channelID int) []time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.store == nil {
		w.store = make(map[int][]time.Time)
	}
	times := w.store[channelID]
	return append([]time.Time(nil), times...)
}

func (w *syncMapTimeWindow) storeTimes(channelID int, times []time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.store == nil {
		w.store = make(map[int][]time.Time)
	}
	w.store[channelID] = append([]time.Time(nil), times...)
}

func recordCodexHotPathTrigger(channelID int, at time.Time) {
	if channelID <= 0 {
		return
	}
	times := codexHotPathTriggerWindows.load(channelID)
	times = append(times, at)
	cutoff := at.Add(-codexHotPathTriggerWindow)
	filtered := make([]time.Time, 0, len(times))
	for _, ts := range times {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	codexHotPathTriggerWindows.storeTimes(channelID, filtered)
}

func recentCodexHotPathTriggerCount(channelID int, now time.Time) int {
	if channelID <= 0 {
		return 0
	}
	times := codexHotPathTriggerWindows.load(channelID)
	cutoff := now.Add(-codexHotPathTriggerWindow)
	filtered := make([]time.Time, 0, len(times))
	for _, ts := range times {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	codexHotPathTriggerWindows.storeTimes(channelID, filtered)
	return len(filtered)
}

func RecentCodexHotPathTriggerCount(channelID int, now time.Time) int {
	return recentCodexHotPathTriggerCount(channelID, now)
}

func shouldTriggerCodexHotPathForLowHealth(channel *model.Channel, now time.Time) bool {
	if channel == nil || channel.Type != constant.ChannelTypeCodex {
		return false
	}
	health := ComputeCodexPoolHealth(channel, now)
	if health == nil {
		return false
	}
	minWatermark := codexMinHealthyWatermark(channel)
	if health.Healthy+health.New <= minWatermark {
		return true
	}
	return health.Total > 0 && health.HealthyRatio < 0.5
}

func ShouldTriggerCursorProReplacementOnHotPath(
	channel *model.Channel,
	kind CodexErrorKind,
	rateLimitRetries int,
	triedKeyCount int,
	selectErr *types.NewAPIError,
	now time.Time,
) (bool, string) {
	if channel == nil || channel.Type != constant.ChannelTypeCodex {
		return false, ""
	}
	if cursorProReplacementMode(channel) == cursorProReplacementModeNearExhausted {
		health := ComputeCodexPoolHealth(channel, now)
		recentNoAvailable := recentCodexNoAvailableCount(channel.Id, now)
		if selectErr != nil && selectErr.GetErrorCode() == types.ErrorCodeChannelNoAvailableKey {
			return true, "request_no_available_tokens"
		}
		if triedKeyCount >= 2 && (kind == CodexErrorKindRateLimit || kind == CodexErrorKindServer || kind == CodexErrorKindSoftFail) {
			if (health != nil && health.AvailableCount <= 0) || recentNoAvailable >= 3 {
				return true, "near_exhausted_hotpath_exhausted"
			}
		}
		return false, ""
	}
	if selectErr != nil && selectErr.GetErrorCode() == types.ErrorCodeChannelNoAvailableKey {
		return true, "request_no_available_tokens"
	}

	lowHealth := shouldTriggerCodexHotPathForLowHealth(channel, now)
	if kind == CodexErrorKindRateLimit && rateLimitRetries >= 1 {
		return true, "request_rate_limit_hot_path"
	}
	if triedKeyCount >= 2 && (kind == CodexErrorKindRateLimit || kind == CodexErrorKindServer || kind == CodexErrorKindSoftFail) {
		return true, "request_exhausted_after_failover"
	}
	if lowHealth && triedKeyCount >= 1 && (kind == CodexErrorKindServer || kind == CodexErrorKindSoftFail) {
		return true, "request_exhausted_after_failover"
	}
	return false, ""
}

func MaybeTriggerCursorProReplacementOnHotPath(channelID int, reason string, at time.Time) {
	if channelID <= 0 || reason == "" {
		return
	}
	recordCodexHotPathTrigger(channelID, at)
	go func() {
		_, _ = SyncCursorProTokens(context.Background(), false, "hot_path_pre_sync")
		tokenStatus, _ := readCursorProTokenStatus(context.Background())
		state := cursorProStateForChannel(channelID)
		updateCursorProSourceQuietSince(state, tokenStatus, time.Now())
		if cursorProSourceRecentlyUpdated(tokenStatus, time.Now()) {
			return
		}
		_, err := TriggerCursorProReplacement(context.Background(), channelID, reason)
		if err != nil {
			logger.LogWarn(context.Background(), "codex hot-path replacement trigger failed: "+err.Error())
		}
	}()
}
