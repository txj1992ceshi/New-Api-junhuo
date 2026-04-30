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
