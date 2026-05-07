package service

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

func TestIsExternalPoolChannelCoolingDown(t *testing.T) {
	now := time.Now()
	ch := &model.Channel{
		Id:        1,
		Name:      "windsurf-pool-proxy",
		OtherInfo: `{"windsurf_pool_proxy":true,"external_pool_cooldown_until":9999999999}`,
	}
	if !IsExternalPoolChannelCoolingDown(ch, now) {
		t.Fatal("expected windsurf external pool channel to be cooling down")
	}

	ch.OtherInfo = `{"windsurf_pool_proxy":true,"external_pool_cooldown_until":1}`
	if IsExternalPoolChannelCoolingDown(ch, now) {
		t.Fatal("expected expired cooldown to be ignored")
	}
}

func TestClassifyExternalPoolCooldown(t *testing.T) {
	tests := []struct {
		name      string
		otherInfo string
		err       *types.NewAPIError
		wantKind  string
		wantWhy   string
	}{
		{
			name:      "windsurf model unavailable",
			otherInfo: `{"windsurf_pool_proxy":true}`,
			err:       types.NewErrorWithStatusCode(errors.New("model_not_entitled"), types.ErrorCodeBadResponseStatusCode, 403),
			wantKind:  ExternalPoolKindWindsurf,
			wantWhy:   "model_unavailable",
		},
		{
			name:      "kiro rate limited",
			otherInfo: `{"kiro_pool_proxy":true}`,
			err:       types.NewErrorWithStatusCode(errors.New("Too many requests, please wait before trying again."), types.ErrorCodeBadResponseStatusCode, 503),
			wantKind:  ExternalPoolKindKiro,
			wantWhy:   "rate_limited",
		},
		{
			name:      "non external pool",
			otherInfo: `{}`,
			err:       types.NewErrorWithStatusCode(errors.New("Too many requests"), types.ErrorCodeBadResponseStatusCode, 429),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := &model.Channel{Id: 1, OtherInfo: tt.otherInfo}
			gotKind, gotSecs, gotWhy, _ := classifyExternalPoolCooldown(ch, tt.err)
			if gotKind != tt.wantKind {
				t.Fatalf("expected kind %q, got %q", tt.wantKind, gotKind)
			}
			if tt.wantKind == "" {
				if gotSecs != 0 || gotWhy != "" {
					t.Fatalf("expected no cooldown classification, got secs=%d reason=%q", gotSecs, gotWhy)
				}
				return
			}
			if gotSecs <= 0 {
				t.Fatalf("expected cooldown seconds > 0, got %d", gotSecs)
			}
			if gotWhy != tt.wantWhy {
				t.Fatalf("expected reason %q, got %q", tt.wantWhy, gotWhy)
			}
		})
	}
}
