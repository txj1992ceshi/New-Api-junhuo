package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const codexPoolSupervisorTickInterval = 30 * time.Second

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
