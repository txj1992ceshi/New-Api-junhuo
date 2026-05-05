package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestCodexUsageSelectionFailureMessage(t *testing.T) {
	allDeadChannel := &model.Channel{
		Id:   2,
		Type: constant.ChannelTypeCodex,
		Key:  "k1\nk2",
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {State: model.CodexKeyStateDead, LastErrorAt: time.Now().Unix()},
				1: {State: model.CodexKeyStateDead, LastErrorAt: time.Now().Unix()},
			},
		},
	}
	if got := codexUsageSelectionFailureMessage(allDeadChannel, fmt.Errorf("no available codex keys")); got != "all_keys_dead" {
		t.Fatalf("unexpected message for all-dead pool: %s", got)
	}

	mixedChannel := &model.Channel{
		Id:   2,
		Type: constant.ChannelTypeCodex,
		Key:  "k1\nk2",
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {State: model.CodexKeyStateHealthy},
				1: {State: model.CodexKeyStateDead, LastErrorAt: time.Now().Unix()},
			},
		},
	}
	if got := codexUsageSelectionFailureMessage(mixedChannel, fmt.Errorf("no available codex keys")); got != "no_available_codex_keys" {
		t.Fatalf("unexpected message for non-dead exhaustion: %s", got)
	}
	if got := codexUsageSelectionFailureMessage(mixedChannel, fmt.Errorf("bad oauth payload")); got != "invalid_codex_credential_payload" {
		t.Fatalf("unexpected credential payload message: %s", got)
	}
}

func TestCodexUsageFetchFailureMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "missing access token", err: fmt.Errorf("codex channel: access_token is required"), want: "missing_access_token"},
		{name: "missing account id", err: fmt.Errorf("codex channel: account_id is required"), want: "missing_account_id"},
		{name: "generic upstream failure", err: fmt.Errorf("upstream boom"), want: "upstream_fetch_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexUsageFetchFailureMessage(tc.err); got != tc.want {
				t.Fatalf("unexpected message: got=%s want=%s", got, tc.want)
			}
		})
	}
}
