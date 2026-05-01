package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

func TestShouldTriggerCursorProReplacementOnHotPath(t *testing.T) {
	now := time.Now()
	channel := &model.Channel{
		Id:   1,
		Type: constant.ChannelTypeCodex,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey: true,
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {State: model.CodexKeyStateCooldown, CooldownUntil: now.Add(2 * time.Minute).Unix()},
				1: {State: model.CodexKeyStateHealthy},
				2: {State: model.CodexKeyStateSuspect},
			},
		},
	}

	t.Run("first rate limit triggers fast replacement", func(t *testing.T) {
		should, reason := ShouldTriggerCursorProReplacementOnHotPath(channel, CodexErrorKindRateLimit, 1, 1, nil, now)
		if !should || reason != "request_rate_limit_hot_path" {
			t.Fatalf("unexpected decision: should=%v reason=%q", should, reason)
		}
	})

	t.Run("no available key triggers explicit reason", func(t *testing.T) {
		should, reason := ShouldTriggerCursorProReplacementOnHotPath(
			channel,
			CodexErrorKindRateLimit,
			1,
			1,
			types.NewError(nil, types.ErrorCodeChannelNoAvailableKey),
			now,
		)
		if !should || reason != "request_no_available_tokens" {
			t.Fatalf("unexpected decision: should=%v reason=%q", should, reason)
		}
	})

	t.Run("failover exhaustion triggers replacement", func(t *testing.T) {
		should, reason := ShouldTriggerCursorProReplacementOnHotPath(channel, CodexErrorKindServer, 0, 2, nil, now)
		if !should || reason != "request_exhausted_after_failover" {
			t.Fatalf("unexpected decision: should=%v reason=%q", should, reason)
		}
	})
}

func TestRecentCodexHotPathTriggerCount(t *testing.T) {
	channelID := 98765
	now := time.Now()
	recordCodexHotPathTrigger(channelID, now.Add(-6*time.Minute))
	recordCodexHotPathTrigger(channelID, now.Add(-2*time.Minute))
	recordCodexHotPathTrigger(channelID, now.Add(-30*time.Second))
	if got := RecentCodexHotPathTriggerCount(channelID, now); got != 2 {
		t.Fatalf("expected 2 recent hot-path triggers, got %d", got)
	}
}
