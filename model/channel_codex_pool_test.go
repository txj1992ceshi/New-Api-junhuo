package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
)

func TestGetNextAvailableCodexKeySkipsDeadAndCooldown(t *testing.T) {
	channel := &Channel{
		Id:   1,
		Type: constant.ChannelTypeCodex,
		Key:  "{\"account_id\":\"a1\"}\n{\"account_id\":\"a2\"}\n{\"account_id\":\"a3\"}",
		ChannelInfo: ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 3,
			MultiKeyMeta: map[int]ChannelKeyMeta{
				0: {State: CodexKeyStateDead},
				1: {State: CodexKeyStateCooldown, CooldownUntil: time.Now().Add(5 * time.Minute).Unix()},
				2: {State: CodexKeyStateHealthy},
			},
		},
	}

	key, index, meta, err := channel.GetNextAvailableCodexKey(nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if index != 2 {
		t.Fatalf("expected key index 2, got %d", index)
	}
	if key == "" || meta == nil || meta.State != CodexKeyStateHealthy {
		t.Fatalf("unexpected selection result: key=%q meta=%+v", key, meta)
	}
}
