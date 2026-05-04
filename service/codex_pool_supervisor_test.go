package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestShouldTriggerCursorProReplacement(t *testing.T) {
	channel := &model.Channel{Id: 1, Type: constant.ChannelTypeCodex}
	now := time.Now()

	should, reason := ShouldTriggerCursorProReplacement(channel, &CodexPoolHealth{
		Total:        10,
		Healthy:      2,
		New:          1,
		Dead:         0,
		HealthyRatio: 0.3,
	}, now)
	if !should || reason == "" {
		t.Fatalf("expected trigger for low healthy ratio, got should=%v reason=%q", should, reason)
	}

	should, reason = ShouldTriggerCursorProReplacement(channel, &CodexPoolHealth{
		Total:         10,
		Healthy:       6,
		New:           1,
		Cooldown:      1,
		RecentDead30m: 0,
		HealthyRatio:  0.7,
		CooldownRatio: 0.1,
	}, now)
	if should {
		t.Fatalf("expected no trigger for healthy pool, got reason=%q", reason)
	}
}

func TestShouldTriggerCursorProReplacementNearExhaustedMode(t *testing.T) {
	now := time.Now()
	channel := &model.Channel{Id: 2, Type: constant.ChannelTypeCodex}
	channel.SetOtherInfo(map[string]interface{}{
		"cursorpro_replacement_mode": "near_exhausted",
	})

	t.Run("skips low health soft signals when pool still has available keys", func(t *testing.T) {
		should, reason := ShouldTriggerCursorProReplacement(channel, &CodexPoolHealth{
			Total:          10,
			Healthy:        1,
			New:            0,
			Cooldown:       8,
			HealthyRatio:   0.1,
			CooldownRatio:  0.8,
			RecentDead30m:  9,
			AvailableCount: 1,
		}, now)
		if should {
			t.Fatalf("expected no trigger, got reason=%q", reason)
		}
	})

	t.Run("triggers when available count is zero", func(t *testing.T) {
		should, reason := ShouldTriggerCursorProReplacement(channel, &CodexPoolHealth{
			Total:          5,
			Healthy:        0,
			New:            0,
			AvailableCount: 0,
		}, now)
		if !should || reason != "near_exhausted_available_zero" {
			t.Fatalf("unexpected decision: should=%v reason=%q", should, reason)
		}
	})

	t.Run("triggers when recent no available spikes", func(t *testing.T) {
		channelID := channel.Id
		RecordCodexNoAvailable(channelID, now.Add(-2*time.Minute))
		RecordCodexNoAvailable(channelID, now.Add(-90*time.Second))
		RecordCodexNoAvailable(channelID, now.Add(-30*time.Second))
		should, reason := ShouldTriggerCursorProReplacement(channel, &CodexPoolHealth{
			Total:          5,
			Healthy:        1,
			New:            0,
			AvailableCount: 1,
		}, now)
		if !should || reason != "near_exhausted_recent_no_available" {
			t.Fatalf("unexpected decision: should=%v reason=%q", should, reason)
		}
	})
}
