package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func TestShouldChatCompletionsUseResponsesPolicy_CodexAlwaysEnabled(t *testing.T) {
	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: false,
	}
	if !ShouldChatCompletionsUseResponsesPolicy(policy, 1, constant.ChannelTypeCodex, "gpt-5.4") {
		t.Fatal("expected codex channel to always use chat->responses compatibility")
	}
}
