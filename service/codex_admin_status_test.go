package service

import "testing"

func TestDeriveTriggerResultReportsNoYieldCooldown(t *testing.T) {
	result := deriveTriggerResult(&cursorProTriggerState{}, nil, cursorProBlockReasonNoYield)
	if result != "trigger_no_yield_cooldown" {
		t.Fatalf("unexpected trigger result: %s", result)
	}
}

func TestDeriveRecoveryResultReportsNoYield(t *testing.T) {
	result := deriveRecoveryResult(&cursorProTriggerState{
		LastResultStatus: "failed",
		LastErrorCode:    cursorProResultCodeNoYield,
	}, nil, nil, nil)
	if result != "register_no_yield" {
		t.Fatalf("unexpected recovery result: %s", result)
	}
}
