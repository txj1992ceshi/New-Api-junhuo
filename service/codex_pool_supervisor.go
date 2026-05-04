package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const codexPoolSupervisorTickInterval = 30 * time.Second

const (
	cursorProReplacementModePoolHealth    = "pool_health"
	cursorProReplacementModeNearExhausted = "near_exhausted"
)

var (
	codexPoolSupervisorOnce    sync.Once
	codexPoolSupervisorRunning atomic.Bool
	codexNoAvailableWindows    sync.Map // channelID -> []time.Time
)

func RecordCodexNoAvailable(channelID int, at time.Time) {
	actual, _ := codexNoAvailableWindows.LoadOrStore(channelID, []time.Time{})
	times, _ := actual.([]time.Time)
	times = append(times, at)
	cutoff := at.Add(-5 * time.Minute)
	filtered := make([]time.Time, 0, len(times))
	for _, ts := range times {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	codexNoAvailableWindows.Store(channelID, filtered)
}

func recentCodexNoAvailableCount(channelID int, now time.Time) int {
	actual, ok := codexNoAvailableWindows.Load(channelID)
	if !ok {
		return 0
	}
	times, _ := actual.([]time.Time)
	cutoff := now.Add(-5 * time.Minute)
	filtered := make([]time.Time, 0, len(times))
	for _, ts := range times {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	codexNoAvailableWindows.Store(channelID, filtered)
	return len(filtered)
}

func RecentCodexNoAvailableCount(channelID int, now time.Time) int {
	return recentCodexNoAvailableCount(channelID, now)
}

func StartCodexPoolSupervisorTask() {
	codexPoolSupervisorOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("codex pool supervisor started: tick=%s", codexPoolSupervisorTickInterval))
			RunCodexPoolSupervisorOnce(context.Background())
			ticker := time.NewTicker(codexPoolSupervisorTickInterval)
			defer ticker.Stop()
			for range ticker.C {
				RunCodexPoolSupervisorOnce(context.Background())
			}
		}()
	})
}

func RunCodexPoolSupervisorOnce(ctx context.Context) {
	if !codexPoolSupervisorRunning.CompareAndSwap(false, true) {
		return
	}
	defer codexPoolSupervisorRunning.Store(false)

	ReconcileCursorProReplacementState(ctx)

	var channels []*model.Channel
	if err := model.DB.Where("type = ? AND status = ?", constant.ChannelTypeCodex, common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("codex pool supervisor: query channels failed: %v", err))
		return
	}

	now := time.Now()
	for _, channel := range channels {
		if channel == nil || !isCursorProAutoImportEnabled(channel) {
			continue
		}
		if scrubStats, err := ScrubCodexChannelKeys(channel.Id, now); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("codex pool supervisor: channel_id=%d scrub failed: %v", channel.Id, err))
		} else if scrubStats != nil && (scrubStats.InvalidDead > 0 || scrubStats.RateLimitDead > 0 || scrubStats.Normalized > 0) {
			logger.LogInfo(ctx, fmt.Sprintf(
				"codex pool supervisor: channel_id=%d scrubbed keys: inspected=%d normalized=%d invalid_dead=%d rate_limit_dead=%d",
				channel.Id,
				scrubStats.Inspected,
				scrubStats.Normalized,
				scrubStats.InvalidDead,
				scrubStats.RateLimitDead,
			))
			channel, _ = model.GetChannelById(channel.Id, true)
		}
		tokenStatus, _ := readCursorProTokenStatus(ctx)
		state := cursorProStateForChannel(channel.Id)
		updateCursorProSourceQuietSince(state, tokenStatus, now)
		if tokenStatus != nil && shouldPrioritizeCursorProSync(tokenStatus, state, now) {
			_, _ = SyncCursorProTokens(ctx, false, "supervisor_source_priority")
			_, _ = ImportCursorProExports(ctx, channel.Id)
			continue
		}
		health := ComputeCodexPoolHealth(channel, now)
		if health == nil {
			continue
		}
		if shouldTrigger, reason := ShouldTriggerCursorProReplacement(channel, health, now); shouldTrigger {
			result, err := TriggerCursorProReplacement(ctx, channel.Id, reason)
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("codex pool supervisor: channel_id=%d trigger failed: %v", channel.Id, err))
				continue
			}
			if result != nil && result.Triggered {
				logger.LogInfo(ctx, fmt.Sprintf("codex pool supervisor: channel_id=%d triggered cursorpro replacement, reason=%s", channel.Id, reason))
			}
		}
	}
}

func ShouldTriggerCursorProReplacement(channel *model.Channel, health *CodexPoolHealth, now time.Time) (bool, string) {
	if channel == nil || health == nil {
		return false, ""
	}
	mode := cursorProReplacementMode(channel)
	if mode == cursorProReplacementModeNearExhausted {
		if health.AvailableCount <= 0 {
			return true, "near_exhausted_available_zero"
		}
		if recentCodexNoAvailableCount(channel.Id, now) >= 3 {
			return true, "near_exhausted_recent_no_available"
		}
		return false, ""
	}

	minWatermark := 5
	info := channel.GetOtherInfo()
	if raw, ok := info["cursorpro_min_healthy_watermark"]; ok {
		if v, ok := raw.(float64); ok && int(v) > 0 {
			minWatermark = int(v)
		}
	}
	if health.Healthy+health.New < minWatermark {
		return true, "low_watermark"
	}
	if health.Total > 0 && health.HealthyRatio < 0.4 {
		return true, "healthy_ratio_low"
	}
	if health.Total > 0 && health.CooldownRatio >= 0.3 {
		return true, "cooldown_ratio_high"
	}
	if health.RecentDead30m >= 5 {
		return true, "dead_growth_fast"
	}
	if recentCodexNoAvailableCount(channel.Id, now) >= 3 {
		return true, "no_available_tokens"
	}
	return false, ""
}

func cursorProReplacementMode(channel *model.Channel) string {
	if channel == nil || channel.Type != constant.ChannelTypeCodex {
		return cursorProReplacementModePoolHealth
	}
	info := channel.GetOtherInfo()
	if raw, ok := info["cursorpro_replacement_mode"]; ok {
		if mode, ok := raw.(string); ok {
			switch strings.ToLower(strings.TrimSpace(mode)) {
			case cursorProReplacementModeNearExhausted:
				return cursorProReplacementModeNearExhausted
			case "", cursorProReplacementModePoolHealth:
				return cursorProReplacementModePoolHealth
			}
		}
	}
	return cursorProReplacementModePoolHealth
}

func shouldPrioritizeCursorProSync(tokenStatus *cursorProTokenStatus, state *cursorProTriggerState, now time.Time) bool {
	if tokenStatus == nil {
		return false
	}
	sourceLatest := parseRFC3339TimeOrZero(tokenStatus.SourceLatestMtime)
	exportLatest := parseRFC3339TimeOrZero(tokenStatus.ExportLatestMtime)
	if !sourceLatest.IsZero() && (exportLatest.IsZero() || sourceLatest.After(exportLatest)) {
		return true
	}
	if !sourceLatest.IsZero() && now.Sub(sourceLatest) <= sourceFreshWindow() {
		if state == nil || state.LastImportAt.IsZero() || sourceLatest.After(state.LastImportAt) {
			return true
		}
	}
	if state != nil && state.LastImportResult == "imported_to_channel" && state.LastProbeResult == "probe_pending" {
		return true
	}
	return false
}
