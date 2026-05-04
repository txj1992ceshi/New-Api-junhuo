package service

import (
	"testing"
	"time"
)

func TestEvaluateCursorProTriggerCooldownFailedStateBlocks(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:      now.Add(-60 * time.Second),
		LastResultStatus:   "failed",
		RecentTriggerTimes: []time.Time{now.Add(-60 * time.Second)},
	}
	health := &CodexPoolHealth{
		Healthy:        1,
		AvailableCount: 1,
	}

	decision := evaluateCursorProTriggerCooldown(state, nil, health, 0, 0, now)
	if decision.Allowed {
		t.Fatal("expected cooldown to block failed trigger")
	}
	if decision.BlockReason != "cooldown" {
		t.Fatalf("unexpected block reason: %s", decision.BlockReason)
	}
	if decision.CooldownBaseSeconds != 180 {
		t.Fatalf("expected failed trigger cooldown 180, got %d", decision.CooldownBaseSeconds)
	}
	if decision.CooldownBreakAllowed {
		t.Fatal("expected no cooldown break for non-empty pool")
	}
}

func TestEvaluateCursorProTriggerCooldownNearExhaustedUsesLongerWindows(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:      now.Add(-60 * time.Second),
		LastResultStatus:   "failed",
		RecentTriggerTimes: []time.Time{now.Add(-60 * time.Second)},
	}
	health := &CodexPoolHealth{
		Healthy:        1,
		AvailableCount: 1,
	}

	decision := evaluateCursorProTriggerCooldownWithMode(state, nil, health, 0, 5, now, cursorProReplacementModeNearExhausted)
	if decision.Allowed {
		t.Fatal("expected cooldown to block near-exhausted failed trigger")
	}
	if decision.CooldownBaseSeconds != 300 {
		t.Fatalf("expected failed trigger cooldown 300, got %d", decision.CooldownBaseSeconds)
	}
	if decision.CooldownBreakAllowed {
		t.Fatal("expected no cooldown break for non-empty pool in near-exhausted mode")
	}
}

func TestEvaluateCursorProTriggerCooldownBreaksWhenPoolIsEmpty(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:      now.Add(-60 * time.Second),
		LastResultStatus:   "failed",
		RecentTriggerTimes: []time.Time{now.Add(-60 * time.Second)},
	}
	health := &CodexPoolHealth{
		AvailableCount: 0,
	}

	decision := evaluateCursorProTriggerCooldown(state, nil, health, 4, 0, now)
	if !decision.Allowed {
		t.Fatal("expected pool-empty cooldown break to allow trigger")
	}
	if !decision.CooldownBreakAllowed {
		t.Fatal("expected cooldown break to be allowed")
	}
	if decision.CooldownBreakReason != "cooldown_break_available_count_zero" {
		t.Fatalf("unexpected cooldown break reason: %s", decision.CooldownBreakReason)
	}
	if decision.CooldownMode != "broken_by_pool_critical" {
		t.Fatalf("unexpected cooldown mode: %s", decision.CooldownMode)
	}
}

func TestEvaluateCursorProTriggerCooldownNearExhaustedOnlyBreaksOnEmptyPool(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:      now.Add(-60 * time.Second),
		LastResultStatus:   "succeeded",
		RecentTriggerTimes: []time.Time{now.Add(-60 * time.Second)},
	}
	health := &CodexPoolHealth{
		Healthy:        1,
		AvailableCount: 1,
	}

	decision := evaluateCursorProTriggerCooldownWithMode(state, nil, health, 4, 3, now, cursorProReplacementModeNearExhausted)
	if decision.Allowed {
		t.Fatal("expected near-exhausted cooldown to stay blocked when pool still has availability")
	}
	if decision.CooldownBreakAllowed {
		t.Fatal("expected near-exhausted mode to ignore hot-path break conditions")
	}
}

func TestEvaluateCursorProTriggerCooldownShortensAfterUsableRecovery(t *testing.T) {
	now := time.Now()
	triggerAt := now.Add(-30 * time.Second)
	state := &cursorProTriggerState{
		LastTriggerAt:                triggerAt,
		LastResultStatus:             "succeeded",
		LastSuccessfulRecoveryAt:     now.Add(-10 * time.Second),
		LastSuccessfulRecoveryReason: "source_sync_succeeded",
		RecentTriggerTimes:           []time.Time{triggerAt},
	}
	health := &CodexPoolHealth{
		Healthy:        1,
		AvailableCount: 1,
	}

	decision := evaluateCursorProTriggerCooldown(state, nil, health, 0, 0, now)
	if decision.Allowed {
		t.Fatal("expected short successful cooldown to still block at 30s")
	}
	if decision.CooldownBaseSeconds != 45 {
		t.Fatalf("expected usable recovery cooldown 45, got %d", decision.CooldownBaseSeconds)
	}
}

func TestEvaluateCursorProTriggerCooldownNearExhaustedShortensAfterRecovery(t *testing.T) {
	now := time.Now()
	triggerAt := now.Add(-30 * time.Second)
	state := &cursorProTriggerState{
		LastTriggerAt:                triggerAt,
		LastResultStatus:             "succeeded",
		LastSuccessfulRecoveryAt:     now.Add(-10 * time.Second),
		LastSuccessfulRecoveryReason: "source_sync_succeeded",
		RecentTriggerTimes:           []time.Time{triggerAt},
	}
	health := &CodexPoolHealth{
		Healthy:        1,
		AvailableCount: 1,
	}

	decision := evaluateCursorProTriggerCooldownWithMode(state, nil, health, 0, 0, now, cursorProReplacementModeNearExhausted)
	if decision.Allowed {
		t.Fatal("expected short successful cooldown to still block at 30s")
	}
	if decision.CooldownBaseSeconds != 120 {
		t.Fatalf("expected usable recovery cooldown 120, got %d", decision.CooldownBaseSeconds)
	}
}

func TestEvaluateCursorProTriggerCooldownHonorsRunningRegisterTask(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:      now.Add(-10 * time.Second),
		RecentTriggerTimes: []time.Time{now.Add(-10 * time.Second)},
	}
	registerStatus := &cursorProRegisterStatus{Status: "running"}

	decision := evaluateCursorProTriggerCooldown(state, registerStatus, nil, 10, 10, now)
	if decision.Allowed {
		t.Fatal("expected running register task to block trigger")
	}
	if decision.BlockReason != "already_running" {
		t.Fatalf("unexpected block reason: %s", decision.BlockReason)
	}
}
