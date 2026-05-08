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

func TestDeriveTriggerResultReportsControlUnreachable(t *testing.T) {
	result := deriveTriggerResult(&cursorProTriggerState{
		LastResultStatus: "failed",
		LastErrorCode:    cursorProResultCodeControl,
	}, nil, "")
	if result != cursorProResultCodeControl {
		t.Fatalf("unexpected trigger result: %s", result)
	}
}

func TestDeriveRecoveryResultReportsControlUnreachable(t *testing.T) {
	result := deriveRecoveryResult(&cursorProTriggerState{
		LastResultStatus: "failed",
		LastErrorCode:    cursorProResultCodeControl,
	}, nil, nil, nil)
	if result != cursorProResultCodeControl {
		t.Fatalf("unexpected recovery result: %s", result)
	}
}

func TestDeriveTriggerResultReportsRegisterDisabled(t *testing.T) {
	result := deriveTriggerResult(&cursorProTriggerState{
		LastResultStatus: "disabled",
		LastErrorCode:    cursorProResultCodeDisabled,
	}, nil, "")
	if result != cursorProResultCodeDisabled {
		t.Fatalf("unexpected trigger result: %s", result)
	}
}

func TestDeriveRecoveryResultReportsRegisterDisabled(t *testing.T) {
	result := deriveRecoveryResult(&cursorProTriggerState{
		LastResultStatus: "disabled",
		LastErrorCode:    cursorProResultCodeDisabled,
	}, nil, nil, nil)
	if result != cursorProResultCodeDisabled {
		t.Fatalf("unexpected recovery result: %s", result)
	}
}
