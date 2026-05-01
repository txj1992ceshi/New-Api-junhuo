package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestComputeCodexPoolHealthTreatsExpiredCooldownAsSuspect(t *testing.T) {
	now := time.Now()
	channel := &model.Channel{
		Id:   1,
		Type: constant.ChannelTypeCodex,
		Key:  "k1\nk2\nk3",
		ChannelInfo: model.ChannelInfo{
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {State: model.CodexKeyStateHealthy},
				1: {State: model.CodexKeyStateNew},
				2: {State: model.CodexKeyStateCooldown, CooldownUntil: now.Add(-time.Minute).Unix()},
			},
		},
	}

	health := ComputeCodexPoolHealth(channel, now)
	if health.Healthy != 1 {
		t.Fatalf("expected healthy=1, got %d", health.Healthy)
	}
	if health.New != 1 {
		t.Fatalf("expected new=1, got %d", health.New)
	}
	if health.Suspect != 1 {
		t.Fatalf("expected suspect=1 for expired cooldown, got %d", health.Suspect)
	}
	if health.AvailableCount != 3 {
		t.Fatalf("expected available=3, got %d", health.AvailableCount)
	}
}
