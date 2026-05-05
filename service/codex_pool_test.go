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

func TestShouldPruneManagedCursorProDeadKeys(t *testing.T) {
	channel := &model.Channel{Id: cursorProManagedChannelID, Type: constant.ChannelTypeCodex}

	if shouldPruneManagedCursorProDeadKeys(channel, &CodexPoolHealth{AvailableCount: 0, Total: 40, Dead: 20}) != true {
		t.Fatal("expected managed channel with exhausted pool to trigger prune")
	}
	if shouldPruneManagedCursorProDeadKeys(channel, &CodexPoolHealth{AvailableCount: 1, Total: 40, Dead: 30}) {
		t.Fatal("did not expect prune when pool still has available keys")
	}
	if shouldPruneManagedCursorProDeadKeys(channel, &CodexPoolHealth{AvailableCount: 0, Total: 39, Dead: 19}) {
		t.Fatal("did not expect prune below dead threshold")
	}
	if shouldPruneManagedCursorProDeadKeys(&model.Channel{Id: 3, Type: constant.ChannelTypeCodex}, &CodexPoolHealth{AvailableCount: 0, Total: 40, Dead: 30}) {
		t.Fatal("did not expect prune for non-managed channel")
	}
}

func TestCursorProDeadPrunePriority(t *testing.T) {
	now := time.Now()

	if priority, ok := cursorProDeadPrunePriority(model.ChannelKeyMeta{
		State:         model.CodexKeyStateDead,
		LastErrorKind: "rate_limit_exhausted",
		LastErrorAt:   now.Add(-11 * time.Minute).Unix(),
	}, now); !ok || priority != 1 {
		t.Fatalf("expected rate_limit_exhausted priority 1, got priority=%d ok=%v", priority, ok)
	}
	if priority, ok := cursorProDeadPrunePriority(model.ChannelKeyMeta{
		State:         model.CodexKeyStateDead,
		LastErrorKind: "invalid_key_missing_account_id",
		LastErrorAt:   now.Add(-11 * time.Minute).Unix(),
	}, now); !ok || priority != 2 {
		t.Fatalf("expected invalid* priority 2, got priority=%d ok=%v", priority, ok)
	}
	if priority, ok := cursorProDeadPrunePriority(model.ChannelKeyMeta{
		State:         model.CodexKeyStateDead,
		LastErrorKind: "rate_limit",
		LastErrorAt:   now.Add(-11 * time.Minute).Unix(),
	}, now); !ok || priority != 3 {
		t.Fatalf("expected rate_limit priority 3, got priority=%d ok=%v", priority, ok)
	}
	if _, ok := cursorProDeadPrunePriority(model.ChannelKeyMeta{
		State:         model.CodexKeyStateDead,
		LastErrorKind: "rate_limit",
		LastErrorAt:   now.Add(-5 * time.Minute).Unix(),
	}, now); ok {
		t.Fatal("did not expect prune candidate before aging threshold")
	}
}
